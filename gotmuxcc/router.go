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

	// ErrServerExit indicates the tmux server sent a %exit notification.
	ErrServerExit = errors.New("gotmuxcc: server sent %exit")
)

// controlGuardFlags is the flags field tmux writes on the %begin/%end/%error
// guards of commands typed by this control client.
const controlGuardFlags = "1"

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

	// initialCmd tracks the implicit %begin/%end pair that tmux emits
	// when a control-mode session starts (from attach-session or
	// new-session passed on the command line). The ready channel is
	// closed once the initial pair has been consumed so that callers
	// can wait for the router to settle before sending commands.
	initialCmd  bool
	initialDone bool
	ready       chan struct{}
	readyOnce   sync.Once

	events       chan Event
	eventsOnce   sync.Once
	closed       chan struct{}
	eventsClosed bool
}

func newRouter(t controlTransport) *router {
	return newRouterWithInit(t, true)
}

func newRouterWithInit(t controlTransport, expectInitial bool) *router {
	trace.Printf("router", "new router created transport=%T expectInitial=%v", t, expectInitial)
	r := &router{
		transport:  t,
		inflight:   make(map[string]*commandState),
		events:     make(chan Event, 64),
		closed:     make(chan struct{}),
		ready:      make(chan struct{}),
		initialCmd: expectInitial,
	}

	if !expectInitial {
		r.readyOnce.Do(func() { close(r.ready) })
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

	if !strings.HasPrefix(line, "%") {
		r.appendOutput(line)
		return
	}

	// tmux does not escape command output (e.g. capture-pane -p), and since
	// tmux 6db5175e notifications are queued so they can never appear inside
	// an open block. So while a block is open every line is content until the
	// exact closing guard for that block arrives — matching the contract
	// upstream asserts in regress/control-notify-guard.sh. Without this a pane
	// displaying a control-mode transcript could complete, mis-bind or tear
	// down real requests.
	if open, endGuard, errorGuard := r.openBlockGuards(); open {
		switch line {
		case endGuard:
			r.handleEnd(line)
		case errorGuard:
			r.handleError(line)
		default:
			r.appendOutput(line)
		}
		return
	}

	switch {
	case isGuardLine(line, "%begin"):
		r.handleBegin(line)
	case isGuardLine(line, "%end"):
		r.handleEnd(line)
	case isGuardLine(line, "%error"):
		r.handleError(line)
	case isGuardLine(line, "%exit"):
		// %exit is written by the client process after proc_loop returns
		// (client.c), so it can only legitimately arrive at depth 0.
		r.handleExit(line)
	default:
		r.emitEvent(parseEvent(line))
	}
}

// openBlockGuards reports whether a %begin block is currently open and, if so,
// the exact %end and %error lines that close it. tmux writes the closing guard
// with the same (time, number, flags) triple as the opening one, so matching on
// the command number alone would let colliding pane content close the block.
func (r *router) openBlockGuards() (open bool, endGuard, errorGuard string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.stack) == 0 {
		return false, "", ""
	}

	state := r.inflight[r.stack[len(r.stack)-1]]
	if state == nil {
		// The block is open but its state is gone; nothing can close it, so
		// treat every line as output rather than guessing.
		return true, "", ""
	}

	fields := " " + state.time + " " + state.number + " " + state.flags
	return true, "%end" + fields, "%error" + fields
}

// isGuardLine reports whether line is the guard keyword on its own or followed
// by a space. tmux emits guards from a single
// xasprintf(&line, "%%%s %ld %u %d", ...) in control_write_guard, so a prefix
// match would also accept notifications such as %beginning or %exited.
func isGuardLine(line, keyword string) bool {
	return line == keyword || strings.HasPrefix(line, keyword+" ")
}

func (r *router) handleBegin(line string) {
	timeStr, number, flags, err := parseFrame(line, "%begin")
	if err != nil {
		r.emitEvent(eventForError("malformed-begin", line, err))
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.err != nil {
		return
	}

	// tmux sets the guard flags field from
	// !!(state->flags & CMDQ_STATE_CONTROL) (cmd-queue.c), so it is 1 only for
	// commands this control client typed. The implicit block for the
	// attach-session/new-session on tmux's own command line carries 0 and must
	// never be paired with a queued request.
	awaitingInitial := r.initialCmd && !r.initialDone
	if len(r.pending) == 0 || (flags != controlGuardFlags && awaitingInitial) {
		// If we're still waiting for the initial command response,
		// track it so its output and %end are consumed cleanly.
		if awaitingInitial {
			trace.Printf("router", "begin <- #%s (initial command)", number)
			r.inflight[number] = &commandState{
				request: newCommandRequest("(initial)"),
				time:    timeStr,
				number:  number,
				flags:   flags,
			}
			r.stack = append(r.stack, number)
			return
		}
		r.emitEventLocked(eventForError("unexpected-begin", line, errUnexpectedBegin))
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
	timeStr, number, flags, err := parseFrame(line, "%end")
	if err != nil {
		r.emitEvent(eventForError("malformed-end", line, err))
		return
	}
	r.finishCommand(number, timeStr, flags, nil, "")
}

func (r *router) handleError(line string) {
	timeStr, number, flags, err := parseFrame(line, "%error")
	if err != nil {
		r.emitEvent(eventForError("malformed-error", line, err))
		return
	}

	// tmux writes error text as output lines between %begin and %error
	// (via cmdq_error → cmdq_print), not in the %error frame itself.
	// Extract the accumulated output as the error message.
	r.mu.Lock()
	msg := ""
	if state := r.inflight[number]; state != nil && len(state.output) > 0 {
		msg = strings.Join(state.output, "\n")
	}
	r.mu.Unlock()

	if msg == "" {
		msg = "tmux reported an error"
	}
	r.finishCommand(number, timeStr, flags, errors.New(msg), msg)
}

func (r *router) handleExit(line string) {
	reason := ""
	if trimmed := strings.TrimPrefix(line, "%exit"); len(trimmed) > 0 {
		reason = strings.TrimSpace(trimmed)
	}
	trace.Printf("router", "exit <- reason=%q", reason)

	// Emit the exit event before shutting down so callers can observe it.
	r.emitEvent(parseEvent(line))

	// Fail all pending/inflight commands with an ExitError.
	r.failAll(&ExitError{Reason: reason})
}

// ExitError is returned when tmux sends a %exit notification. The Reason
// field contains the optional reason string from the notification.
type ExitError struct {
	Reason string
}

func (e *ExitError) Error() string {
	if e.Reason != "" {
		return fmt.Sprintf("gotmuxcc: server sent %%exit: %s", e.Reason)
	}
	return ErrServerExit.Error()
}

// Is reports whether target matches ErrServerExit.
func (e *ExitError) Is(target error) bool {
	return target == ErrServerExit
}

func (r *router) appendOutput(line string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.err != nil {
		return
	}

	if len(r.stack) == 0 {
		r.emitEventLocked(Event{
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
		r.emitEventLocked(Event{
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
	var isInitial bool

	r.mu.Lock()
	if r.err != nil {
		r.mu.Unlock()
		return
	}

	state = r.inflight[number]
	if state != nil {
		delete(r.inflight, number)
		r.removeFromStack(number)
		if state.request.command == "(initial)" {
			isInitial = true
			r.initialDone = true
		}
	}
	pendingCount := len(r.pending)
	inflightCount := len(r.inflight)
	r.mu.Unlock()

	if isInitial {
		trace.Printf("router", "initial command settled #%s", number)
		r.readyOnce.Do(func() { close(r.ready) })
		return
	}

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
	r.mu.Lock()
	sent, ok := r.enqueueEvent(evt)
	r.mu.Unlock()
	if !ok {
		return
	}
	r.logEvent(evt, sent)
}

func (r *router) emitEventLocked(evt Event) {
	sent, ok := r.enqueueEvent(evt)
	if !ok {
		return
	}
	r.logEvent(evt, sent)
}

func (r *router) enqueueEvent(evt Event) (sent bool, ok bool) {
	if r.err != nil || r.eventsClosed {
		return false, false
	}

	select {
	case r.events <- evt:
		return true, true
	default:
		return false, true
	}
}

func (r *router) logEvent(evt Event, sent bool) {
	if sent {
		trace.Printf("router", "event <- %s data=%s", evt.Name, trace.FormatControlLine(evt.Data))
		return
	}
	trace.Printf("router", "emitEvent dropped name=%s", evt.Name)
	// Drop event to avoid blocking; router consumers should drain events when needed.
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
	r.eventsClosed = true

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

	r.readyOnce.Do(func() { close(r.ready) })
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

	// Send while still holding the mutex so that the pending queue order
	// matches the order commands are written to the transport. Releasing
	// the lock before Send would allow a concurrent enqueue to append and
	// send its command first, causing response mismatch in handleBegin.
	if err := r.transport.Send(req.command); err != nil {
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
	r.mu.Unlock()

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

// parseFrame parses a control-mode guard line. tmux writes guards as the
// keyword plus exactly three integers (time, command number, flags) from one
// format string, so anything else is pane content rather than protocol.
func parseFrame(line, prefix string) (timeStr, number, flags string, err error) {
	if !isGuardLine(line, prefix) {
		return "", "", "", fmt.Errorf("unexpected prefix for %s: %q", prefix, line)
	}

	payload := strings.TrimPrefix(line, prefix)
	if !strings.HasPrefix(payload, " ") {
		return "", "", "", fmt.Errorf("malformed %s line: %q", prefix, line)
	}

	parts := strings.Split(payload[1:], " ")
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("malformed %s line: %q", prefix, line)
	}
	for _, part := range parts {
		if !isUnsignedDecimal(part) {
			return "", "", "", fmt.Errorf("malformed %s line: %q", prefix, line)
		}
	}

	return parts[0], parts[1], parts[2], nil
}

func isUnsignedDecimal(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
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
