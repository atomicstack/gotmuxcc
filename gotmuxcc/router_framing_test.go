package gotmuxcc

import (
	"errors"
	"testing"
	"time"
)

// closeClean simulates the transport reaching EOF without an error, i.e. the
// tmux client exiting normally. fakeTransport.Close reports a synthetic error
// instead, which is not what a clean shutdown looks like.
func (f *fakeTransport) closeClean() {
	f.closeOnce.Do(func() {
		close(f.lines)
		select {
		case f.done <- nil:
		default:
		}
		close(f.done)
	})
}

func assertLines(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("expected %d lines %#v, got %d: %#v", len(want), want, len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("line %d: want %q, got %q", i, want[i], got[i])
		}
	}
}

// TestRouterExitInsideBlockIsCommandOutput replays the raw stream captured from
// tmux next-3.8 when capture-pane returns a pane displaying a control-mode
// transcript. Everything between the real guards is pane content.
func TestRouterExitInsideBlockIsCommandOutput(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	go func() {
		<-ft.sendC
		ft.lines <- "%begin 1786931886 532 1"
		ft.lines <- "%exit fake-content"
		ft.lines <- "%begin 1 2 3"
		ft.lines <- "%end 1 2 3"
		ft.lines <- "%end 1786931886 532 1"
	}()

	result, err := r.runCommand("capture-pane -p -t %1")
	if err != nil {
		t.Fatalf("runCommand returned error: %v", err)
	}
	assertLines(t, result.Lines, []string{"%exit fake-content", "%begin 1 2 3", "%end 1 2 3"})
}

// TestRouterSurvivesExitInsidePaneOutput asserts the connection is not torn
// down by pane content that starts with %exit: a later command still resolves.
func TestRouterSurvivesExitInsidePaneOutput(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	go func() {
		<-ft.sendC
		ft.lines <- "%begin 100 1 1"
		ft.lines <- "%exit fake-content"
		ft.lines <- "%end 100 1 1"

		<-ft.sendC
		ft.lines <- "%begin 100 2 1"
		ft.lines <- "still here"
		ft.lines <- "%end 100 2 1"
	}()

	if _, err := r.runCommand("capture-pane -p -t %1"); err != nil {
		t.Fatalf("capture-pane returned error: %v", err)
	}

	result, err := r.runCommand("display-message -p ok")
	if err != nil {
		t.Fatalf("second command returned error: %v", err)
	}
	assertLines(t, result.Lines, []string{"still here"})
}

// TestRouterBeginInsideBlockDoesNotStealPendingRequest asserts that a %begin
// appearing in pane content cannot bind the next queued request to a bogus
// command number.
func TestRouterBeginInsideBlockDoesNotStealPendingRequest(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	type outcome struct {
		result commandResult
		err    error
	}
	first := make(chan outcome, 1)
	second := make(chan outcome, 1)

	go func() {
		res, err := r.runCommand("capture-pane -p -t %1")
		first <- outcome{res, err}
	}()
	<-ft.sendC

	go func() {
		res, err := r.runCommand("list-sessions")
		second <- outcome{res, err}
	}()
	<-ft.sendC

	// Both requests are queued; the first one's block opens and its content
	// contains a line that looks like a guard.
	ft.lines <- "%begin 100 1 1"
	ft.lines <- "%begin 1 2 3"
	ft.lines <- "%end 100 1 1"
	ft.lines <- "%begin 100 2 1"
	ft.lines <- "sessions"
	ft.lines <- "%end 100 2 1"

	select {
	case got := <-first:
		if got.err != nil {
			t.Fatalf("capture-pane returned error: %v", got.err)
		}
		assertLines(t, got.result.Lines, []string{"%begin 1 2 3"})
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for capture-pane")
	}

	select {
	case got := <-second:
		if got.err != nil {
			t.Fatalf("list-sessions returned error: %v", got.err)
		}
		assertLines(t, got.result.Lines, []string{"sessions"})
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for list-sessions")
	}
}

// TestRouterEndInsideBlockRequiresExactTriple asserts a block is only closed by
// a %end that matches the open %begin on time, number and flags — a colliding
// command number alone must not complete the command early.
func TestRouterEndInsideBlockRequiresExactTriple(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	go func() {
		<-ft.sendC
		ft.lines <- "%begin 100 7 1"
		ft.lines <- "%end 200 7 0"
		ft.lines <- "tail"
		ft.lines <- "%end 100 7 1"
	}()

	result, err := r.runCommand("capture-pane -p -t %1")
	if err != nil {
		t.Fatalf("runCommand returned error: %v", err)
	}
	assertLines(t, result.Lines, []string{"%end 200 7 0", "tail"})
}

// TestRouterErrorInsideBlockRequiresExactTriple asserts the same for %error:
// pane content must not turn a successful command into a failure.
func TestRouterErrorInsideBlockRequiresExactTriple(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	go func() {
		<-ft.sendC
		ft.lines <- "%begin 100 8 1"
		ft.lines <- "%error 200 8 0"
		ft.lines <- "tail"
		ft.lines <- "%end 100 8 1"
	}()

	result, err := r.runCommand("capture-pane -p -t %1")
	if err != nil {
		t.Fatalf("runCommand returned error: %v", err)
	}
	assertLines(t, result.Lines, []string{"%error 200 8 0", "tail"})
}

// TestRouterGuardPrefixesAreNotSubstringMatched asserts notifications whose
// names merely start with a guard keyword are not parsed as guards.
func TestRouterGuardPrefixesAreNotSubstringMatched(t *testing.T) {
	cases := []struct {
		line string
		name string
	}{
		{"%beginning 1 2 3", "beginning"},
		{"%errors 1 2 3", "errors"},
		{"%exited 1 2 3", "exited"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ft := newFakeTransport()
			r := newRouterWithInit(ft, false)
			defer r.close()

			ft.lines <- tc.line

			select {
			case evt := <-r.eventsChannel():
				if evt.Name != tc.name {
					t.Fatalf("expected %q event, got %#v", tc.name, evt)
				}
			case <-time.After(2 * time.Second):
				t.Fatalf("timed out waiting for %q event", tc.name)
			}

			// The connection must still serve commands.
			go func() {
				<-ft.sendC
				ft.lines <- "%begin 100 1 1"
				ft.lines <- "ok"
				ft.lines <- "%end 100 1 1"
			}()
			result, err := r.runCommand("display-message -p ok")
			if err != nil {
				t.Fatalf("runCommand returned error: %v", err)
			}
			assertLines(t, result.Lines, []string{"ok"})
		})
	}
}

// TestRouterMalformedBeginDoesNotConsumePendingRequest asserts a %begin whose
// fields are not the three integers tmux always writes is rejected instead of
// being bound to a queued request.
func TestRouterMalformedBeginDoesNotConsumePendingRequest(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	done := make(chan error, 1)
	go func() {
		result, err := r.runCommand("list-sessions")
		if err == nil {
			assertLines(t, result.Lines, []string{"sessions"})
		}
		done <- err
	}()
	<-ft.sendC

	ft.lines <- "%begin abc 1 0"

	select {
	case evt := <-r.eventsChannel():
		if evt.Name != "malformed-begin" {
			t.Fatalf("expected malformed-begin event, got %#v", evt)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for malformed-begin event")
	}

	// The queued request must still be waiting for its real guard.
	ft.lines <- "%begin 100 1 1"
	ft.lines <- "sessions"
	ft.lines <- "%end 100 1 1"

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runCommand returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for command to complete")
	}
}

// TestRouterGuardWithTrailingFieldIsRejected asserts guards carry exactly three
// fields; tmux writes them from a single format string so a fourth field means
// the line is not a guard.
func TestRouterGuardWithTrailingFieldIsRejected(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	ft.lines <- "%end 100 1 0 extra"

	select {
	case evt := <-r.eventsChannel():
		if evt.Name != "malformed-end" {
			t.Fatalf("expected malformed-end event, got %#v", evt)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for malformed-end event")
	}
}

func TestRouterMalformedGuardShapes(t *testing.T) {
	cases := []struct {
		name string
		line string
		want string
	}{
		{"no fields", "%end", "malformed-end"},
		{"empty field", "%end  1 0", "malformed-end"},
		{"non-numeric field", "%error 100 one 0", "malformed-error"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ft := newFakeTransport()
			r := newRouterWithInit(ft, false)
			defer r.close()

			ft.lines <- tc.line

			select {
			case evt := <-r.eventsChannel():
				if evt.Name != tc.want {
					t.Fatalf("expected %q event, got %#v", tc.want, evt)
				}
			case <-time.After(2 * time.Second):
				t.Fatalf("timed out waiting for %q event", tc.want)
			}
		})
	}
}

// TestRouterErrorGuardWithoutRequestEmitsEvent covers a well-formed %error
// arriving at depth 0 with nothing to attribute it to.
func TestRouterErrorGuardWithoutRequestEmitsEvent(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	ft.lines <- "%error 100 9 1"

	select {
	case evt := <-r.eventsChannel():
		if evt.Name != "unexpected-error" {
			t.Fatalf("expected unexpected-error event, got %#v", evt)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for unexpected-error event")
	}
}

// TestRouterOpenBlockWithoutStateTreatsGuardsAsOutput asserts a block whose
// state has gone missing cannot be closed by a guard-shaped line.
func TestRouterOpenBlockWithoutStateTreatsGuardsAsOutput(t *testing.T) {
	r := &router{
		inflight: make(map[string]*commandState),
		events:   make(chan Event, 2),
		ready:    make(chan struct{}),
		stack:    []string{"7"},
	}

	r.handleLine("%end 100 7 1")

	select {
	case evt := <-r.events:
		if evt.Name != "unknown-command-output" {
			t.Fatalf("expected unknown-command-output event, got %#v", evt)
		}
	default:
		t.Fatal("expected unknown-command-output event")
	}
}

func TestParseFrameRejectsMismatchedKeyword(t *testing.T) {
	if _, _, _, err := parseFrame("%end 1 2 3", "%begin"); err == nil {
		t.Fatal("expected error when the line does not carry the requested guard")
	}
}

// TestRouterInitialBlockDoesNotBindPendingRequest asserts the implicit block
// tmux emits for the attach-session/new-session given on its command line is
// never paired with a queued request. tmux only sets the guard flags field to 1
// for commands typed by this control client.
func TestRouterInitialBlockDoesNotBindPendingRequest(t *testing.T) {
	ft := newFakeTransport()
	r := newRouter(ft)
	defer r.close()

	go func() {
		<-ft.sendC
		ft.lines <- "%begin 100 533 0"
		ft.lines <- "%end 100 533 0"
		ft.lines <- "%begin 100 536 1"
		ft.lines <- "real output"
		ft.lines <- "%end 100 536 1"
	}()

	result, err := r.runCommand("list-sessions")
	if err != nil {
		t.Fatalf("runCommand returned error: %v", err)
	}
	assertLines(t, result.Lines, []string{"real output"})
}

// TestRouterTransportEOFWithoutExitFailsCleanly asserts a transport that ends
// without a trailing %exit is a clean close: tmux's exit timer can discard
// buffered output and force the client out before the notification is written.
func TestRouterTransportEOFWithoutExitFailsCleanly(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	go func() {
		<-ft.sendC
		ft.lines <- "%begin 100 1 1"
		ft.lines <- "partial"
		ft.closeClean()
	}()

	_, err := r.runCommand("list-sessions")
	if !errors.Is(err, ErrTransportClosed) {
		t.Fatalf("expected ErrTransportClosed, got %v", err)
	}
	var exitErr *ExitError
	if errors.As(err, &exitErr) {
		t.Fatalf("EOF without %%exit must not surface as a server-exit error: %v", err)
	}
}
