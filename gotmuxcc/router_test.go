package gotmuxcc

import (
	"errors"
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
	r := newRouter(ft)
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
	r := newRouter(ft)
	defer r.close()

	go func() {
		<-ft.sendC
		ft.lines <- "%begin 1712000000 2 0"
		ft.lines <- "partial output"
		ft.lines <- "%error 1712000000 2 0 failed"
	}()

	_, err := r.runCommand("list-panes")
	if err == nil {
		t.Fatalf("expected error from runCommand")
	}
	cmdErr, ok := err.(*commandError)
	if !ok {
		t.Fatalf("expected *commandError, got %T", err)
	}
	if cmdErr.Message != "failed" {
		t.Fatalf("unexpected error message: %q", cmdErr.Message)
	}
}

func TestRouterEmitsEvents(t *testing.T) {
	ft := newFakeTransport()
	r := newRouter(ft)
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
	}

	r.finishCommand("2", "1", "0", nil)
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
