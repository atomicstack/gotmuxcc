package gotmuxcc

import (
	"errors"
	"fmt"
	"sync"
	"testing"
)

type fakeTransport struct {
	sendMu sync.Mutex
	sent   []string

	lines     chan string
	done      chan error
	sendC     chan string
	closeOnce sync.Once
}

func newFakeTransport() *fakeTransport {
	return &fakeTransport{
		lines: make(chan string, 32),
		done:  make(chan error, 1),
		sendC: make(chan string, 1),
	}
}

func (f *fakeTransport) Send(cmd string) error {
	f.sendMu.Lock()
	f.sent = append(f.sent, cmd)
	f.sendMu.Unlock()

	select {
	case f.sendC <- cmd:
	default:
	}
	return nil
}

func (f *fakeTransport) Lines() <-chan string {
	return f.lines
}

func (f *fakeTransport) Done() <-chan error {
	return f.done
}

func (f *fakeTransport) Close() error {
	f.closeOnce.Do(func() {
		close(f.lines)
		select {
		case f.done <- errors.New("closed"):
		default:
		}
		close(f.done)
	})
	return nil
}

func TestRouterRunCommandSuccess(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	go func() {
		<-ft.sendC
		ft.lines <- "%begin 1712000000 1 0"
		ft.lines <- "value"
		ft.lines <- "%end 1712000000 1 0"
	}()

	result, err := r.runCommand("display-message")
	if err != nil {
		t.Fatalf("runCommand returned error: %v", err)
	}
	if len(result.Lines) != 1 || result.Lines[0] != "value" {
		t.Fatalf("unexpected result lines: %#v", result.Lines)
	}
}

func TestRouterRunCommandError(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	go func() {
		<-ft.sendC
		ft.lines <- "%begin 1712000000 2 0"
		ft.lines <- "no such session: foo"
		ft.lines <- "%error 1712000000 2 0"
	}()

	_, err := r.runCommand("list-panes")
	if err == nil {
		t.Fatalf("expected error from runCommand")
	}
	cmdErr, ok := err.(*commandError)
	if !ok {
		t.Fatalf("expected *commandError, got %T", err)
	}
	if cmdErr.Message != "no such session: foo" {
		t.Fatalf("unexpected error message: %q", cmdErr.Message)
	}
}

func TestRouterEmitsEvents(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	go func() {
		<-ft.sendC
		ft.lines <- "%window-layout-changed @1"
		ft.lines <- "%begin 1 3 0"
		ft.lines <- "ok"
		ft.lines <- "%end 1 3 0"
	}()

	result, err := r.runCommand("list-windows")
	if err != nil {
		t.Fatalf("runCommand returned error: %v", err)
	}
	if len(result.Lines) != 1 {
		t.Fatalf("expected single result line, got %d", len(result.Lines))
	}

	select {
	case evt := <-r.eventsChannel():
		if evt.Name != "window-layout-changed" {
			t.Fatalf("unexpected event: %#v", evt)
		}
	default:
		t.Fatalf("expected event emission")
	}
}

type errorTransport struct {
	err error
}

func (e *errorTransport) Send(string) error { return e.err }
func (e *errorTransport) Lines() <-chan string {
	return nil
}
func (e *errorTransport) Done() <-chan error { return nil }
func (e *errorTransport) Close() error       { return nil }

func TestRouterEnqueueSendError(t *testing.T) {
	sendErr := errors.New("boom")
	r := &router{
		transport: &errorTransport{err: sendErr},
		inflight:  make(map[string]*commandState),
		events:    make(chan Event, 1),
		closed:    make(chan struct{}),
		ready:     make(chan struct{}),
	}

	if _, err := r.runCommand("list-sessions"); !errors.Is(err, sendErr) {
		t.Fatalf("expected sendErr, got %v", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.pending) != 0 {
		t.Fatalf("expected pending commands to be cleared, got %d", len(r.pending))
	}
}

func TestRouterRemoveFromStackNonTail(t *testing.T) {
	req := newCommandRequest("display-message")
	state := &commandState{
		request: req,
		time:    "1",
		number:  "2",
		flags:   "0",
	}
	r := &router{
		inflight: map[string]*commandState{
			"2": state,
		},
		stack:  []string{"1", "2", "3"},
		events: make(chan Event, 1),
		ready:  make(chan struct{}),
	}

	r.finishCommand("2", "1", "0", nil, "")
	result, err := req.wait()
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if result.Number != "2" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if len(r.stack) != 2 || r.stack[0] != "1" || r.stack[1] != "3" {
		t.Fatalf("expected stack [1 3], got %#v", r.stack)
	}
}

func TestRouterAppendOutputEdgeCases(t *testing.T) {
	r := &router{
		inflight: make(map[string]*commandState),
		events:   make(chan Event, 2),
		ready:    make(chan struct{}),
	}

	r.appendOutput("orphan")
	select {
	case evt := <-r.events:
		if evt.Name != "orphan-output" {
			t.Fatalf("expected orphan-output event, got %#v", evt)
		}
	default:
		t.Fatalf("expected orphan-output event")
	}

	r.stack = []string{"7"}
	r.appendOutput("dangling")
	select {
	case evt := <-r.events:
		if evt.Name != "unknown-command-output" {
			t.Fatalf("expected unknown-command-output event, got %#v", evt)
		}
	default:
		t.Fatalf("expected unknown-command-output event")
	}
}

func TestRouterUnexpectedEndEmitsErrorEvent(t *testing.T) {
	r := &router{
		inflight: make(map[string]*commandState),
		events:   make(chan Event, 1),
		ready:    make(chan struct{}),
	}

	r.handleEnd("%end 100 9 0")
	select {
	case evt := <-r.events:
		if evt.Name != "unexpected-end" {
			t.Fatalf("expected unexpected-end event, got %#v", evt)
		}
	default:
		t.Fatalf("expected unexpected-end event")
	}
}

func TestEventForError(t *testing.T) {
	evt := eventForError("test-error", "%line", errors.New("fail"))
	if evt.Name != "test-error" {
		t.Fatalf("unexpected name: %s", evt.Name)
	}
	if len(evt.Fields) != 1 || evt.Fields[0] != "%line" {
		t.Fatalf("unexpected fields: %#v", evt.Fields)
	}
	if evt.Data != "fail" || evt.Raw != "%line" {
		t.Fatalf("unexpected data/raw: %q / %q", evt.Data, evt.Raw)
	}
}

func TestRouterInitialCommandAbsorbed(t *testing.T) {
	ft := newFakeTransport()
	// Simulate the initial attach-session %begin/%end arriving before
	// any user command is enqueued.
	ft.lines <- "%begin 1712000000 0 0"
	ft.lines <- "%end 1712000000 0 0"

	r := newRouter(ft)
	defer r.close()

	// ready should be closed once the initial pair is consumed.
	<-r.ready

	// Now send a real command — it should not be affected by the initial pair.
	go func() {
		<-ft.sendC
		ft.lines <- "%begin 1712000001 1 0"
		ft.lines <- "line1"
		ft.lines <- "line2"
		ft.lines <- "%end 1712000001 1 0"
	}()

	result, err := r.runCommand("list-commands")
	if err != nil {
		t.Fatalf("runCommand returned error: %v", err)
	}
	if len(result.Lines) != 2 || result.Lines[0] != "line1" || result.Lines[1] != "line2" {
		t.Fatalf("unexpected result lines: %#v", result.Lines)
	}
}

func TestRouterInitialCommandWithOutput(t *testing.T) {
	ft := newFakeTransport()
	// Initial attach may produce output lines between %begin/%end.
	ft.lines <- "%begin 1712000000 0 0"
	ft.lines <- "session created"
	ft.lines <- "%end 1712000000 0 0"

	r := newRouter(ft)
	defer r.close()

	<-r.ready

	go func() {
		<-ft.sendC
		ft.lines <- "%begin 1712000001 1 0"
		ft.lines <- "result"
		ft.lines <- "%end 1712000001 1 0"
	}()

	result, err := r.runCommand("display-message")
	if err != nil {
		t.Fatalf("runCommand returned error: %v", err)
	}
	if len(result.Lines) != 1 || result.Lines[0] != "result" {
		t.Fatalf("unexpected result lines: %#v", result.Lines)
	}
}

func TestRouterInitialCommandWithEvents(t *testing.T) {
	ft := newFakeTransport()
	// tmux often sends notification events before the initial %begin.
	ft.lines <- "%sessions-changed"
	ft.lines <- "%session-changed $0 main"
	ft.lines <- "%begin 1712000000 0 0"
	ft.lines <- "%end 1712000000 0 0"

	r := newRouter(ft)
	defer r.close()

	<-r.ready

	go func() {
		<-ft.sendC
		ft.lines <- "%begin 1712000001 1 0"
		ft.lines <- "ok"
		ft.lines <- "%end 1712000001 1 0"
	}()

	result, err := r.runCommand("list-sessions")
	if err != nil {
		t.Fatalf("runCommand returned error: %v", err)
	}
	if len(result.Lines) != 1 || result.Lines[0] != "ok" {
		t.Fatalf("unexpected result lines: %#v", result.Lines)
	}
}

func TestRouterEnqueueOrderMatchesSendOrder(t *testing.T) {
	// orderedFakeTransport records send order via a channel so we can
	// verify that enqueue serialises append+send atomically.
	ft := &orderedFakeTransport{
		sendOrder: make(chan string, 64),
		lines:     make(chan string, 128),
		done:      make(chan error, 1),
	}

	r := newRouterWithInit(ft, false)
	defer r.close()

	const N = 20
	var wg sync.WaitGroup
	wg.Add(N)

	// Launch N goroutines that each send a unique command concurrently.
	for i := 0; i < N; i++ {
		go func(idx int) {
			defer wg.Done()
			_, _ = r.runCommand(fmt.Sprintf("cmd-%d", idx))
		}(i)
	}

	// Collect the order commands arrived at the transport.
	sent := make([]string, 0, N)
	for i := 0; i < N; i++ {
		sent = append(sent, <-ft.sendOrder)
	}

	// Verify that the pending queue (which determines response matching)
	// has the same order as the commands were sent to the transport.
	r.mu.Lock()
	pendingCmds := make([]string, len(r.pending))
	for i, req := range r.pending {
		pendingCmds[i] = req.command
	}
	r.mu.Unlock()

	if len(pendingCmds) != len(sent) {
		t.Fatalf("pending=%d sent=%d", len(pendingCmds), len(sent))
	}
	for i := range sent {
		if sent[i] != pendingCmds[i] {
			t.Fatalf("order mismatch at %d: sent=%q pending=%q", i, sent[i], pendingCmds[i])
		}
	}

	// Complete all commands so goroutines can exit.
	for i, cmd := range sent {
		num := fmt.Sprintf("%d", i+1)
		ft.lines <- fmt.Sprintf("%%begin 1 %s 0", num)
		ft.lines <- cmd
		ft.lines <- fmt.Sprintf("%%end 1 %s 0", num)
	}
	wg.Wait()
}

type orderedFakeTransport struct {
	sendMu    sync.Mutex
	sendOrder chan string

	lines     chan string
	done      chan error
	closeOnce sync.Once
}

func (f *orderedFakeTransport) Send(cmd string) error {
	f.sendMu.Lock()
	f.sendOrder <- cmd
	f.sendMu.Unlock()
	return nil
}

func (f *orderedFakeTransport) Lines() <-chan string { return f.lines }
func (f *orderedFakeTransport) Done() <-chan error   { return f.done }
func (f *orderedFakeTransport) Close() error {
	f.closeOnce.Do(func() {
		close(f.lines)
		select {
		case f.done <- errors.New("closed"):
		default:
		}
		close(f.done)
	})
	return nil
}

func TestRouterPercentLinesAsOutputInsideCommand(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	// Simulate capture-pane -p output that contains lines starting with %.
	// These should be treated as command output, not events.
	go func() {
		<-ft.sendC
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "$ echo hello"
		ft.lines <- "hello"
		ft.lines <- "%prompt > "
		ft.lines <- "%sessions-changed"
		ft.lines <- ""
		ft.lines <- "trailing"
		ft.lines <- "%end 1 1 0"
	}()

	result, err := r.runCommand("capture-pane -p -t %1")
	if err != nil {
		t.Fatalf("runCommand returned error: %v", err)
	}

	expected := []string{
		"$ echo hello",
		"hello",
		"%prompt > ",
		"%sessions-changed",
		"",
		"trailing",
	}
	if len(result.Lines) != len(expected) {
		t.Fatalf("expected %d lines, got %d: %#v", len(expected), len(result.Lines), result.Lines)
	}
	for i, want := range expected {
		if result.Lines[i] != want {
			t.Fatalf("line %d: want %q, got %q", i, want, result.Lines[i])
		}
	}

	// Verify no spurious events were emitted.
	select {
	case evt := <-r.eventsChannel():
		t.Fatalf("unexpected event: %#v", evt)
	default:
	}
}

func TestRouterPercentLinesAsEventsOutsideCommand(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	// When no command is inflight, %-prefixed lines should still be events.
	ft.lines <- "%window-add @1"

	evt := <-r.eventsChannel()
	if evt.Name != "window-add" {
		t.Fatalf("expected window-add event, got %#v", evt)
	}
}
