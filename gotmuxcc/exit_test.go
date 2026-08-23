package gotmuxcc

import (
	"errors"
	"testing"
	"time"
)

func TestHandleExitEmitsEvent(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	ft.lines <- "%exit server exiting"

	select {
	case evt := <-r.eventsChannel():
		if evt.Name != "exit" {
			t.Fatalf("expected exit event, got %q", evt.Name)
		}
		if evt.Data != "server exiting" {
			t.Fatalf("unexpected exit data: %q", evt.Data)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for exit event")
	}
}

func TestHandleExitNoReason(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	ft.lines <- "%exit"

	select {
	case evt := <-r.eventsChannel():
		if evt.Name != "exit" {
			t.Fatalf("expected exit event, got %q", evt.Name)
		}
		if evt.Data != "" {
			t.Fatalf("expected empty data, got %q", evt.Data)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for exit event")
	}
}

func TestHandleExitFailsPendingCommands(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	go func() {
		<-ft.sendC
		ft.lines <- "%exit detached"
	}()

	_, err := r.runCommand("list-sessions")
	if err == nil {
		t.Fatal("expected error from runCommand after exit notification")
	}
	if !errors.Is(err, ErrServerExit) {
		t.Fatalf("expected ErrServerExit, got: %v", err)
	}

	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected *ExitError, got: %T", err)
	}
	if exitErr.Reason != "detached" {
		t.Fatalf("unexpected reason: %q", exitErr.Reason)
	}
}

// TestHandleExitInsideBlockIsCommandOutput asserts %exit is only honoured at
// depth 0. tmux prints it from the client process after proc_loop returns, so
// inside an open block it is always command output (typically capture-pane
// content) and must not tear the connection down.
func TestHandleExitInsideBlockIsCommandOutput(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	go func() {
		<-ft.sendC
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "%exit detached"
		ft.lines <- "%end 1 1 0"
	}()

	result, err := r.runCommand("capture-pane -p -t %1")
	if err != nil {
		t.Fatalf("runCommand returned error: %v", err)
	}
	if len(result.Lines) != 1 || result.Lines[0] != "%exit detached" {
		t.Fatalf("unexpected result lines: %#v", result.Lines)
	}
}

func TestHandleExitFailsInflightCommands(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	// The command's block never opens: tmux exits between accepting the
	// command and running it, so %exit arrives at depth 0.
	go func() {
		<-ft.sendC
		ft.lines <- "%exit detached"
	}()

	_, err := r.runCommand("list-sessions")
	if err == nil {
		t.Fatal("expected error from runCommand after exit")
	}
	if !errors.Is(err, ErrServerExit) {
		t.Fatalf("expected ErrServerExit, got: %v", err)
	}
}

func TestExitErrorIs(t *testing.T) {
	err := &ExitError{Reason: "test"}
	if !errors.Is(err, ErrServerExit) {
		t.Fatal("ExitError should match ErrServerExit via Is()")
	}
	if errors.Is(err, ErrTransportClosed) {
		t.Fatal("ExitError should not match ErrTransportClosed")
	}
}

func TestExitErrorMessage(t *testing.T) {
	withReason := &ExitError{Reason: "server shutting down"}
	if withReason.Error() != "gotmuxcc: server sent %exit: server shutting down" {
		t.Fatalf("unexpected error string: %q", withReason.Error())
	}

	noReason := &ExitError{}
	if noReason.Error() != ErrServerExit.Error() {
		t.Fatalf("unexpected error string: %q", noReason.Error())
	}
}
