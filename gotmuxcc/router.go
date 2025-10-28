package gotmuxcc

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/atomicstack/gotmuxcc/internal/trace"
)

var (
	errRouterClosed    = errors.New("gotmuxcc: router closed")
	errEmptyCommand    = errors.New("gotmuxcc: empty command")
	errUnexpectedBegin = errors.New("gotmuxcc: unexpected %begin without pending request")
	errUnexpectedEnd   = errors.New("gotmuxcc: unexpected %end without matching request")
	errUnexpectedError = errors.New("gotmuxcc: unexpected %error without matching request")

	// ErrTransportClosed indicates the underlying control transport terminated.
	ErrTransportClosed = errors.New("gotmuxcc: control transport closed")
)

// Event represents an asynchronous notification emitted by tmux control mode.
type Event struct {
	Name   string   // event name without leading '%'
	Fields []string // whitespace-separated fields following the event name
	Data   string   // raw tail of the line (fields joined with spaces)
	Raw    string   // full raw line including leading '%'
}

type commandResponse struct {
	result commandResult
	err    error
}

type commandRequest struct {
	command string
	reply   chan commandResponse
}

func newCommandRequest(command string) *commandRequest {
	return &commandRequest{
		command: command,
		reply:   make(chan commandResponse, 1),
	}
}

func (cr *commandRequest) complete(res commandResult) {
	cr.reply <- commandResponse{result: res}
}

func (cr *commandRequest) fail(err error) {
	cr.reply <- commandResponse{err: err}
}

func (cr *commandRequest) wait() (commandResult, error) {
	resp := <-cr.reply
	return resp.result, resp.err
}

type commandState struct {
	request *commandRequest
	time    string
	number  string
	flags   string
	output  []string
}

type commandResult struct {
	Command string
	Time    string
	Number  string
	Flags   string
	Lines   []string
}

type commandError struct {
	Command string
	Message string
	Result  commandResult
}

func (e *commandError) Error() string {
	return fmt.Sprintf("tmux error for %q: %s", e.Command, e.Message)
}

type router struct {
	transport controlTransport

	mu       sync.Mutex
	pending  []*commandRequest
	inflight map[string]*commandState
	stack    []string
	err      error

	events     chan Event
	eventsOnce sync.Once
	closed     chan struct{}
}

func newRouter(t controlTransport) *router {
	trace.Printf("router", "new router created transport=%T", t)
	r := &router{
		transport: t,
		inflight:  make(map[string]*commandState),
		events:    make(chan Event, 64),
		closed:    make(chan struct{}),
	}

	go r.readLoop()
	go r.observeDone()

	return r
}

func (r *router) observeDone() {
	if r.transport == nil {
		trace.Printf("router", "observeDone transport nil")
		r.failAll(ErrTransportClosed)
		return
	}
	done := r.transport.Done()
	if done == nil {
		trace.Printf("router", "observeDone done channel nil")
		r.failAll(ErrTransportClosed)
		return
	}
	trace.Printf("router", "observeDone waiting for transport done")
	err := <-done
	trace.Printf("router", "observeDone received err=%v", err)
	if err == nil {
		err = ErrTransportClosed
	}
	r.failAll(err)
}

func (r *router) readLoop() {
	trace.Printf("router", "readLoop starting")
	if r.transport == nil {
		r.failAll(ErrTransportClosed)
		return
	}
	lines := r.transport.Lines()
	if lines == nil {
		trace.Printf("router", "readLoop lines channel nil")
		r.failAll(ErrTransportClosed)
		return
	}
	for line := range lines {
		line = strings.TrimRight(line, "\r\n")
		r.handleLine(line)
	}
	trace.Printf("router", "readLoop lines channel closed")
	// Lines channel closed; ensure we surface closure if observeDone hasn't yet.
	r.failAll(ErrTransportClosed)
}

func (r *router) handleLine(line string) {
	if line == "" {
		r.appendOutput("")
		return
	}

	switch {
	case strings.HasPrefix(line, "%begin"):
		r.handleBegin(line)
	case strings.HasPrefix(line, "%end"):
		r.handleEnd(line)
	case strings.HasPrefix(line, "%error"):
		r.handleError(line)
	case strings.HasPrefix(line, "%"):
		r.emitEvent(parseEvent(line))
	default:
		r.appendOutput(line)
	}
}

func (r *router) handleBegin(line string) {
	timeStr, number, flags, _, err := parseFrame(line, "%begin")
	if err != nil {
		r.emitEvent(eventForError("malformed-begin", line, err))
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.err != nil {
		return
	}

	if len(r.pending) == 0 {
		r.emitEvent(eventForError("unexpected-begin", line, errUnexpectedBegin))
		return
	}

	req := r.pending[0]
	r.pending = r.pending[1:]

	state := &commandState{
		request: req,
		time:    timeStr,
		number:  number,
		flags:   flags,
	}
	r.inflight[number] = state
	r.stack = append(r.stack, number)
	trace.Printf("router", "begin <- #%s time=%s flags=%s command=%s", number, timeStr, flags, trace.FormatControlCommand(req.command))
}

func (r *router) handleEnd(line string) {
	timeStr, number, flags, _, err := parseFrame(line, "%end")
	if err != nil {
		r.emitEvent(eventForError("malformed-end", line, err))
		return
	}
	r.finishCommand(number, timeStr, flags, nil, "")
}

func (r *router) handleError(line string) {
	timeStr, number, flags, rest, err := parseFrame(line, "%error")
	if err != nil {
		r.emitEvent(eventForError("malformed-error", line, err))
		return
	}
	if rest == "" {
		rest = "tmux reported an error"
	}
	r.finishCommand(number, timeStr, flags, errors.New(rest), rest)
}

func (r *router) appendOutput(line string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.err != nil {
		return
	}

	if len(r.stack) == 0 {
		r.emitEvent(Event{
			Name:   "orphan-output",
			Fields: []string{line},
			Data:   line,
			Raw:    line,
		})
		trace.Printf("router", "orphan output <- %s", trace.FormatControlLine(line))
		return
	}

	current := r.stack[len(r.stack)-1]
	state := r.inflight[current]
	if state == nil {
		r.emitEvent(Event{
			Name:   "unknown-command-output",
			Fields: []string{line},
			Data:   line,
			Raw:    line,
		})
		trace.Printf("router", "unknown output <- #%s %s", current, trace.FormatControlLine(line))
		return
	}

	state.output = append(state.output, line)
}

func (r *router) finishCommand(number, timeStr, flags string, cmdErr error, detail string) {
	var state *commandState

	r.mu.Lock()
	if r.err != nil {
		r.mu.Unlock()
		return
	}

	state = r.inflight[number]
	if state != nil {
		delete(r.inflight, number)
		r.removeFromStack(number)
	}
	pendingCount := len(r.pending)
	inflightCount := len(r.inflight)
	r.mu.Unlock()

	if state == nil {
		if cmdErr != nil {
			r.emitEvent(eventForError("unexpected-error", number, errUnexpectedError))
		} else {
			r.emitEvent(eventForError("unexpected-end", number, errUnexpectedEnd))
		}
		trace.Printf("router", "missing state for #%s err=%v (pending=%d inflight=%d)", number, cmdErr, pendingCount, inflightCount)
		return
	}

	result := commandResult{
		Command: state.request.command,
		Time:    state.time,
		Number:  number,
		Flags:   flags,
		Lines:   append([]string(nil), state.output...),
	}

	commandDisplay := trace.FormatControlCommand(state.request.command)
	summary := trace.SummariseControlLines(result.Lines)

	if cmdErr != nil {
		msg := detail
		if msg == "" {
			msg = cmdErr.Error()
		}
		trace.Printf("router", "error <- #%s time=%s flags=%s command=%s msg=%s %s", number, timeStr, flags, commandDisplay, trace.FormatControlLine(msg), summary)
		state.request.fail(&commandError{
			Command: state.request.command,
			Message: cmdErr.Error(),
			Result:  result,
		})
		return
	}

	trace.Printf("router", "complete <- #%s time=%s flags=%s command=%s %s", number, timeStr, flags, commandDisplay, summary)
	state.request.complete(result)
}

func (r *router) removeFromStack(number string) {
	if len(r.stack) == 0 {
		return
	}
	// Fast path: most of the time the finished command is the most recent.
	if r.stack[len(r.stack)-1] == number {
		r.stack = r.stack[:len(r.stack)-1]
		return
	}

	for idx, n := range r.stack {
		if n == number {
			r.stack = append(r.stack[:idx], r.stack[idx+1:]...)
			trace.Printf("router", "removeFromStack removed number=%s remaining=%d", number, len(r.stack))
			return
		}
	}
}

func (r *router) emitEvent(evt Event) {
	select {
	case r.events <- evt:
		trace.Printf("router", "event <- %s data=%s", evt.Name, trace.FormatControlLine(evt.Data))
	default:
		trace.Printf("router", "emitEvent dropped name=%s", evt.Name)
		// Drop event to avoid blocking; router consumers should drain events when needed.
	}
}

func (r *router) failAll(err error) {
	r.mu.Lock()
	if r.err != nil {
		trace.Printf("router", "failAll already failed err=%v", r.err)
		r.mu.Unlock()
		return
	}
	if err == nil {
		err = ErrTransportClosed
	}
	r.err = err

	pending := r.pending
	r.pending = nil

	inflight := r.inflight
	r.inflight = make(map[string]*commandState)
	r.stack = nil

	trace.Printf("router", "failAll err=%v pending=%d inflight=%d", err, len(pending), len(inflight))
	r.mu.Unlock()

	for _, req := range pending {
		req.fail(err)
	}
	for _, state := range inflight {
		state.request.fail(err)
	}

	r.eventsOnce.Do(func() {
		close(r.events)
		close(r.closed)
	})
}

func (r *router) enqueue(req *commandRequest) error {
	r.mu.Lock()
	if r.err != nil {
		err := r.err
		r.mu.Unlock()
		trace.Printf("router", "reject -> %s err=%v", trace.FormatControlCommand(req.command), err)
		return err
	}
	r.pending = append(r.pending, req)
	trace.Printf("router", "queued -> %s (pending=%d)", trace.FormatControlCommand(req.command), len(r.pending))
	r.mu.Unlock()

	if err := r.transport.Send(req.command); err != nil {
		r.mu.Lock()
		for idx, pending := range r.pending {
			if pending == req {
				r.pending = append(r.pending[:idx], r.pending[idx+1:]...)
				break
			}
		}
		trace.Printf("router", "send failed -> %s err=%v", trace.FormatControlCommand(req.command), err)
		r.mu.Unlock()
		return err
	}

	return nil
}

func (r *router) runCommand(cmd string) (commandResult, error) {
	if cmd = strings.TrimSpace(cmd); cmd == "" {
		return commandResult{}, errEmptyCommand
	}

	trace.Printf("router", "dispatch -> %s", trace.FormatControlCommand(cmd))

	req := newCommandRequest(cmd)
	if err := r.enqueue(req); err != nil {
		return commandResult{}, err
	}

	return req.wait()
}

func (r *router) eventsChannel() <-chan Event {
	return r.events
}

func (r *router) close() error {
	r.failAll(errRouterClosed)
	if r.transport != nil {
		return r.transport.Close()
	}
	return nil
}

func parseFrame(line, prefix string) (timeStr, number, flags, rest string, err error) {
	if !strings.HasPrefix(line, prefix) {
		return "", "", "", "", fmt.Errorf("unexpected prefix for %s: %q", prefix, line)
	}

	payload := strings.TrimSpace(strings.TrimPrefix(line, prefix))
	parts := strings.SplitN(payload, " ", 4)
	if len(parts) < 3 {
		return "", "", "", "", fmt.Errorf("malformed %s line: %q", prefix, line)
	}

	timeStr = parts[0]
	number = parts[1]
	flags = parts[2]
	if len(parts) == 4 {
		rest = strings.TrimSpace(parts[3])
	}

	return timeStr, number, flags, rest, nil
}

func parseEvent(line string) Event {
	raw := strings.TrimSpace(line)
	if strings.HasPrefix(raw, "%") {
		raw = raw[1:]
	}

	name := raw
	data := ""
	if idx := strings.IndexRune(raw, ' '); idx >= 0 {
		name = raw[:idx]
		data = strings.TrimSpace(raw[idx+1:])
	}

	fields := []string{}
	if data != "" {
		fields = strings.Fields(data)
	}

	return Event{
		Name:   name,
		Fields: fields,
		Data:   data,
		Raw:    line,
	}
}

func eventForError(name string, raw interface{}, err error) Event {
	return Event{
		Name:   name,
		Fields: []string{fmt.Sprint(raw)},
		Data:   fmt.Sprint(err),
		Raw:    fmt.Sprint(raw),
	}
}
