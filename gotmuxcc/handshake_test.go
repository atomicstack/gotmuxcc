package gotmuxcc

import (
	"context"
	"strings"
	"testing"
	"time"
)

// wedgedTransport never completes the handshake and never closes, modelling a
// tmux subprocess that starts but neither answers nor exits. Before the
// handshake bound this wedged the constructor forever for any caller arriving
// via NewTmux or DefaultTmux, which have no context to cancel.
type wedgedTransport struct {
	lines  chan string
	done   chan error
	closed chan struct{}
}

func newWedgedTransport() *wedgedTransport {
	return &wedgedTransport{
		lines:  make(chan string),
		done:   make(chan error),
		closed: make(chan struct{}),
	}
}

func (w *wedgedTransport) Send(string) error           { return nil }
func (w *wedgedTransport) Lines() <-chan string        { return w.lines }
func (w *wedgedTransport) Done() <-chan error          { return w.done }
func (w *wedgedTransport) Close() error                { close(w.closed); return nil }
func (w *wedgedTransport) waitClosed() <-chan struct{} { return w.closed }

func wedgedDialer(tr controlTransport) ConstructorOption {
	return WithDialer(DialerFunc(func(context.Context, string) (controlTransport, error) {
		return tr, nil
	}))
}

func TestHandshakeTimeoutBoundsTheWait(t *testing.T) {
	tr := newWedgedTransport()

	start := time.Now()
	tmux, err := NewTmuxWithOptions("", wedgedDialer(tr), WithHandshakeTimeout(80*time.Millisecond))
	elapsed := time.Since(start)

	if err == nil {
		_ = tmux.Close()
		t.Fatal("expected the constructor to give up on the handshake")
	}
	if !strings.Contains(err.Error(), "handshake") {
		t.Fatalf("error should name the handshake, got: %v", err)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("constructor took %s, expected it to bail out near the timeout", elapsed)
	}
}

// TestHandshakeTimeoutClosesTheTransport asserts giving up does not orphan the
// tmux -C child: the transport is closed on the way out.
func TestHandshakeTimeoutClosesTheTransport(t *testing.T) {
	tr := newWedgedTransport()

	if _, err := NewTmuxWithOptions("", wedgedDialer(tr), WithHandshakeTimeout(50*time.Millisecond)); err == nil {
		t.Fatal("expected a handshake timeout")
	}

	select {
	case <-tr.waitClosed():
	case <-time.After(2 * time.Second):
		t.Fatal("transport was not closed after the handshake timed out")
	}
}

// TestHandshakeTimeoutDoesNotFireOnASuccessfulHandshake guards against the bound
// turning a slow-but-working server into a spurious failure.
func TestHandshakeTimeoutDoesNotFireOnASuccessfulHandshake(t *testing.T) {
	ft := newFakeTransport()
	ft.lines <- "%begin 1712000000 0 0"
	ft.lines <- "%end 1712000000 0 0"

	tmux, err := NewTmuxWithOptions("", wedgedDialer(ft), WithHandshakeTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("constructor returned error on a good handshake: %v", err)
	}
	defer tmux.Close()
}

// TestHandshakeTimeoutDisabled asserts a non-positive duration opts out of the
// bound, for callers who prefer the previous unbounded behaviour.
func TestHandshakeTimeoutDisabled(t *testing.T) {
	ft := newFakeTransport()
	ft.lines <- "%begin 1712000000 0 0"
	ft.lines <- "%end 1712000000 0 0"

	tmux, err := NewTmuxWithOptions("", wedgedDialer(ft), WithHandshakeTimeout(0))
	if err != nil {
		t.Fatalf("constructor returned error with the bound disabled: %v", err)
	}
	defer tmux.Close()
}

// TestDefaultHandshakeTimeoutIsGenerous pins the default so it cannot be
// tightened into flakiness by accident.
func TestDefaultHandshakeTimeoutIsGenerous(t *testing.T) {
	if DefaultHandshakeTimeout < 5*time.Second {
		t.Fatalf("default handshake timeout %s is too tight for a loaded machine", DefaultHandshakeTimeout)
	}
}
