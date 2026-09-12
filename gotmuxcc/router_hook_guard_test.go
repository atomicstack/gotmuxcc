package gotmuxcc

import (
	"testing"
	"time"
)

// tmux writes the guard flags field from !!(state->flags & CMDQ_STATE_CONTROL)
// (cmd-queue.c), and CMDQ_STATE_CONTROL is set in exactly one place — for
// commands this control client typed on its own stdin (control.c,
// control_read_callback). A hook that runs a command is inserted into this
// client's queue and emits a complete, unrequested guard pair carrying 0.
//
// Captured from a live next-3.9 server, one typed `new-window -d`, with
// `set-hook -g after-new-window 'display-message -p HOOKRAN'` installed:
//
//	%begin 1789230572 537 1
//	%end 1789230572 537 1
//	%window-add @1
//	%begin 1789230572 538 0
//	HOOKRAN
//	%end 1789230572 538 0

// TestRouterDoesNotPairHookGuardWithQueuedRequest asserts an unrequested
// flags=0 block arriving before a queued command's own block does not steal
// that command's reply.
func TestRouterDoesNotPairHookGuardWithQueuedRequest(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	go func() {
		<-ft.sendC
		// A hook fires first: unrequested, flags 0.
		ft.lines <- "%begin 100 538 0"
		ft.lines <- "HOOKRAN"
		ft.lines <- "%end 100 538 0"
		// Then the reply to the command we actually sent.
		ft.lines <- "%begin 100 539 1"
		ft.lines <- "real-reply"
		ft.lines <- "%end 100 539 1"
	}()

	result, err := r.runCommand("list-sessions -F '#{session_name}'")
	if err != nil {
		t.Fatalf("runCommand returned error: %v", err)
	}
	assertLines(t, result.Lines, []string{"real-reply"})
}

// TestRouterHookGuardDoesNotDesyncLaterCommands asserts the queue stays
// aligned after a hook block: a second command must receive its own reply
// rather than the one belonging to a later frame.
func TestRouterHookGuardDoesNotDesyncLaterCommands(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	go func() {
		<-ft.sendC
		ft.lines <- "%begin 100 540 1"
		ft.lines <- "first-reply"
		ft.lines <- "%end 100 540 1"
		// Hook fires between the two commands.
		ft.lines <- "%begin 100 541 0"
		ft.lines <- "HOOKRAN"
		ft.lines <- "%end 100 541 0"
		<-ft.sendC
		ft.lines <- "%begin 100 542 1"
		ft.lines <- "second-reply"
		ft.lines <- "%end 100 542 1"
	}()

	first, err := r.runCommand("list-sessions")
	if err != nil {
		t.Fatalf("first runCommand returned error: %v", err)
	}
	assertLines(t, first.Lines, []string{"first-reply"})

	second, err := r.runCommand("list-windows")
	if err != nil {
		t.Fatalf("second runCommand returned error: %v", err)
	}
	assertLines(t, second.Lines, []string{"second-reply"})
}

// TestRouterHookGuardOutputIsNotOrphaned asserts a hook block's body is
// consumed as that block's output rather than surfacing as orphan-output
// noise, and that the block does not tear anything down.
func TestRouterHookGuardOutputIsNotOrphaned(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	go func() {
		<-ft.sendC
		ft.lines <- "%begin 100 550 0"
		ft.lines <- "HOOKRAN"
		ft.lines <- "%end 100 550 0"
		ft.lines <- "%begin 100 551 1"
		ft.lines <- "reply"
		ft.lines <- "%end 100 551 1"
	}()

	result, err := r.runCommand("list-sessions")
	if err != nil {
		t.Fatalf("runCommand returned error: %v", err)
	}
	assertLines(t, result.Lines, []string{"reply"})

	deadline := time.After(200 * time.Millisecond)
	for {
		select {
		case e := <-r.eventsChannel():
			switch e.Name {
			case "orphan-output", "unexpected-begin", "unexpected-end", "unknown-command-output":
				t.Fatalf("hook guard block produced %s: %q", e.Name, e.Raw)
			}
		case <-deadline:
			return
		}
	}
}
