package gotmuxcc

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type blockedSendTransport struct {
	*silentTransport
	release     chan struct{}
	releaseOnce sync.Once
}

func (b *blockedSendTransport) Send(cmd string) error {
	b.sentC <- cmd
	<-b.release
	return nil
}
func (b *blockedSendTransport) unblock()     { b.releaseOnce.Do(func() { close(b.release) }) }
func (b *blockedSendTransport) Close() error { b.unblock(); return b.silentTransport.Close() }

func blockedSendTmux(t *testing.T) (*Tmux, *blockedSendTransport) {
	t.Helper()
	tr := &blockedSendTransport{silentTransport: newSilentTransport(), release: make(chan struct{})}
	tmux := &Tmux{transport: tr, router: newRouterWithInit(tr, false)}
	t.Cleanup(func() { tr.unblock(); _ = tmux.Close() })
	return tmux, tr
}

func awaitCommandError(t *testing.T, result <-chan error, want error) {
	t.Helper()
	select {
	case err := <-result:
		if !errors.Is(err, want) {
			t.Fatalf("got %v, want %v", err, want)
		}
	case <-time.After(time.Second):
		t.Fatal("command did not unblock")
	}
}

func TestRouterBlockedSendCancellation(t *testing.T) {
	tmux, tr := blockedSendTmux(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() { _, err := tmux.CommandContext(ctx, "first"); result <- err }()
	awaitSentCommand(t, tr.sentC, "first")
	cancel()
	awaitCommandError(t, result, context.Canceled)

	// a request waiting behind the writer must also be cancellable and unsent.
	queuedCtx, queuedCancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer queuedCancel()
	go func() { _, err := tmux.CommandContext(queuedCtx, "never-sent"); result <- err }()
	awaitCommandError(t, result, context.DeadlineExceeded)
	select {
	case cmd := <-tr.sentC:
		t.Fatalf("concurrent write while first send blocked: %s", cmd)
	default:
	}

	// the writer keeps its place after caller abandonment, even when the reply
	// arrives before Send returns. that reply cannot be assigned to the next call.
	for _, line := range []string{"%begin 1 1 1", "abandoned", "%end 1 1 1"} {
		tr.lines <- line
	}
	tr.lines <- "%window-add @5"
	select {
	case event := <-tmux.router.events:
		if event.Name != "window-add" {
			t.Fatalf("unexpected event: %+v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("blocked send prevented response and event processing")
	}
	tr.unblock()
	live := make(chan string, 1)
	go func() {
		out, err := tmux.Command("second")
		if err != nil {
			live <- "error: " + err.Error()
			return
		}
		live <- out
	}()
	awaitSentCommand(t, tr.sentC, "second")
	for _, line := range []string{"%begin 1 2 1", "live", "%end 1 2 1"} {
		tr.lines <- line
	}
	select {
	case got := <-live:
		if got != "live" {
			t.Fatalf("incorrect reply: %q", got)
		}
	case <-time.After(time.Second):
		t.Fatal("subsequent command did not finish")
	}
}

func TestRouterCloseInterruptsBlockedSend(t *testing.T) {
	tmux, tr := blockedSendTmux(t)
	result := make(chan error, 1)
	go func() { _, err := tmux.Command("blocked"); result <- err }()
	awaitSentCommand(t, tr.sentC, "blocked")
	closed := make(chan error, 1)
	go func() { closed <- tmux.Close() }()
	awaitCommandError(t, closed, nil)
	awaitCommandError(t, result, errRouterClosed)
}

// an error from Send can follow a partial write, so no later command may be sent.
type failedSendTransport struct {
	*blockedSendTransport
	err error
}

func (f *failedSendTransport) Send(cmd string) error {
	_ = f.blockedSendTransport.Send(cmd)
	return f.err
}

func TestRouterSendFailureClosesConnection(t *testing.T) {
	sendErr := errors.New("partial write failed")
	tr := &failedSendTransport{
		blockedSendTransport: &blockedSendTransport{silentTransport: newSilentTransport(), release: make(chan struct{})},
		err:                  sendErr,
	}
	tmux := &Tmux{transport: tr, router: newRouterWithInit(tr, false)}
	t.Cleanup(func() { tr.unblock(); _ = tmux.Close() })
	result := make(chan error, 2)
	go func() { _, err := tmux.Command("partial"); result <- err }()
	awaitSentCommand(t, tr.sentC, "partial")
	go func() { _, err := tmux.Command("later"); result <- err }()
	tr.unblock()
	awaitCommandError(t, result, sendErr)
	awaitCommandError(t, result, sendErr)
	if _, err := tmux.Command("after-close"); !errors.Is(err, sendErr) {
		t.Fatalf("unexpected terminal error: %v", err)
	}
	select {
	case cmd := <-tr.sentC:
		t.Fatalf("command sent after partial write failure: %s", cmd)
	default:
	}
}

func TestRouterDrainsOutputAfterTransportDone(t *testing.T) {
	tr := newSilentTransport()
	r := newRouterWithInit(tr, false)
	release := make(chan struct{})
	drained := make(chan struct{})
	t.Cleanup(func() { close(release); <-drained; _ = r.close() })
	// stderr may report termination before the transport's stdout producer
	// finishes. that producer still needs a reader so it can close and reap.
	tr.done <- errors.New("stderr failure")
	go func() {
		defer close(drained)
		<-r.closed
		for range 400 {
			select {
			case tr.lines <- "remaining stdout":
			case <-release:
				return
			}
		}
	}()
	select {
	case <-drained:
	case <-time.After(time.Second):
		t.Fatal("terminal router stopped draining stdout")
	}
}
