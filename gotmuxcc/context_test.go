package gotmuxcc

import (
	"context"
	"errors"
	"testing"
	"time"
)

// silentTransport accepts commands and never replies, modelling a tmux that has
// wedged. It is what makes the cancellation tests deterministic: without a
// context the calls below would block forever.
type silentTransport struct {
	lines chan string
	done  chan error
	sentC chan string
}

func newSilentTransport() *silentTransport {
	return &silentTransport{
		lines: make(chan string, 8),
		done:  make(chan error, 1),
		sentC: make(chan string, 8),
	}
}

func (s *silentTransport) Send(cmd string) error {
	select {
	case s.sentC <- cmd:
	default:
	}
	return nil
}
func (s *silentTransport) Lines() <-chan string { return s.lines }
func (s *silentTransport) Done() <-chan error   { return s.done }
func (s *silentTransport) Close() error         { return nil }

func silentTmux(t *testing.T) (*Tmux, *silentTransport) {
	t.Helper()
	tr := newSilentTransport()
	tmux := &Tmux{transport: tr}
	tmux.router = newRouterWithInit(tr, false)
	t.Cleanup(func() { _ = tmux.Close() })
	return tmux, tr
}

// TestContextCancellationAbandonsWedgedCommand is the case tmux-popup-control's
// watcher needs: a poll that never gets a reply must be abandonable.
func TestContextCancellationAbandonsWedgedCommand(t *testing.T) {
	tmux, _ := silentTmux(t)

	ctx, cancel := context.WithCancel(context.Background())
	errC := make(chan error, 1)
	go func() {
		_, err := tmux.ListSessionsContext(ctx)
		errC <- err
	}()

	// Let the command reach the transport, then abandon it.
	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-errC:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cancelling the context did not unblock the command")
	}
}

func TestContextDeadlineAbandonsWedgedCommand(t *testing.T) {
	tmux, _ := silentTmux(t)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := tmux.ListAllPanesContext(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded, got %v", err)
	}
}

// TestAlreadyCancelledContextIsNotSent asserts a command whose context is
// already done never reaches tmux, rather than being written and abandoned.
func TestAlreadyCancelledContextIsNotSent(t *testing.T) {
	tmux, tr := silentTmux(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := tmux.CommandContext(ctx, "list-sessions"); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}

	select {
	case cmd := <-tr.sentC:
		t.Fatalf("command was written to tmux despite a cancelled context: %q", cmd)
	default:
	}
}

// TestCancellationDoesNotDesyncTheRouter is the correctness risk of abandoning:
// the router pairs each %begin with pending[0], so an abandoned request must
// stay in the queue and its reply must be discarded, not misapplied to the next
// caller.
func TestCancellationDoesNotDesyncTheRouter(t *testing.T) {
	ft := newFakeTransport()
	tmux := &Tmux{transport: ft}
	tmux.router = newRouterWithInit(ft, false)
	defer tmux.Close()

	// First command: issued, then abandoned before any reply arrives.
	ctx, cancel := context.WithCancel(context.Background())
	first := make(chan error, 1)
	go func() {
		_, err := tmux.CommandContext(ctx, "display-message -p abandoned")
		first <- err
	}()
	<-ft.sendC
	cancel()
	if err := <-first; !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}

	// Second command, with its own reply. The abandoned request is still
	// pending, so the router must consume the first reply for it and only then
	// hand the second reply to this caller.
	second := make(chan string, 1)
	go func() {
		out, err := tmux.Command("display-message", "-p", "live")
		if err != nil {
			second <- "error: " + err.Error()
			return
		}
		second <- out
	}()
	<-ft.sendC

	ft.lines <- "%begin 100 1 1"
	ft.lines <- "reply-for-abandoned"
	ft.lines <- "%end 100 1 1"
	ft.lines <- "%begin 100 2 1"
	ft.lines <- "reply-for-live"
	ft.lines <- "%end 100 2 1"

	select {
	case got := <-second:
		if got != "reply-for-live" {
			t.Fatalf("second caller got the wrong reply: %q", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the second command")
	}
}

// TestNonContextMethodsStillWork asserts the existing signatures keep behaving
// as plain blocking calls now that they delegate through context.Background().
func TestNonContextMethodsStillWork(t *testing.T) {
	ft := newFakeTransport()
	tmux := &Tmux{transport: ft}
	tmux.router = newRouterWithInit(ft, false)
	defer tmux.Close()

	go func() {
		<-ft.sendC
		ft.lines <- "%begin 100 1 1"
		ft.lines <- "ok"
		ft.lines <- "%end 100 1 1"
	}()

	out, err := tmux.Command("display-message", "-p", "ok")
	if err != nil {
		t.Fatalf("Command returned error: %v", err)
	}
	if out != "ok" {
		t.Fatalf("unexpected output: %q", out)
	}
}
