package gotmuxcc

import (
	"strings"
	"testing"
)

// TestOptionsRequestsHookOptions asserts the -H flag is sent, so user options
// registered as hooks are not silently dropped on tmux 7277712c and later.
func TestOptionsRequestsHookOptions(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	tmux := &Tmux{transport: ft, router: r}

	go func() {
		cmd := <-ft.sendC
		if !strings.Contains(cmd, " -H ") {
			t.Errorf("expected -H in show-options command, got %q", cmd)
		}
		ft.lines <- "%begin 1 1 1"
		ft.lines <- "@hook-user-option \"display-message hi\""
		ft.lines <- "%end 1 1 1"
	}()

	opts, err := tmux.Options("$1", "")
	if err != nil {
		t.Fatalf("Options returned error: %v", err)
	}
	if len(opts) != 1 || opts[0].Key != "@hook-user-option" {
		t.Fatalf("unexpected options: %#v", opts)
	}
}

// TestOptionsFallsBackWhenHookFlagIsRejected covers a server old enough not to
// know -H: the flag was only added in tmux 7277712c, and such a server answers
// with a usage error. The listing must still come back rather than failing.
func TestOptionsFallsBackWhenHookFlagIsRejected(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	tmux := &Tmux{transport: ft, router: r}

	go func() {
		first := <-ft.sendC
		if !strings.Contains(first, " -H ") {
			t.Errorf("expected the first attempt to carry -H, got %q", first)
		}
		ft.lines <- "%begin 1 1 1"
		ft.lines <- "usage: show-options [-AgHpqsvw] [-t target]"
		ft.lines <- "%error 1 1 1"

		second := <-ft.sendC
		if strings.Contains(second, " -H ") {
			t.Errorf("expected the retry to drop -H, got %q", second)
		}
		ft.lines <- "%begin 1 2 1"
		ft.lines <- "@plain-user-option kept"
		ft.lines <- "%end 1 2 1"
	}()

	opts, err := tmux.Options("$1", "")
	if err != nil {
		t.Fatalf("Options returned error after falling back: %v", err)
	}
	if len(opts) != 1 || opts[0].Key != "@plain-user-option" || opts[0].Value != "kept" {
		t.Fatalf("unexpected options after fallback: %#v", opts)
	}
}

// TestOptionsReportsTheFallbackError asserts a genuine failure is still
// reported, rather than the retry hiding it.
func TestOptionsReportsTheFallbackError(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	tmux := &Tmux{transport: ft, router: r}

	go func() {
		<-ft.sendC
		ft.lines <- "%begin 1 1 1"
		ft.lines <- "can't find session: $9"
		ft.lines <- "%error 1 1 1"

		<-ft.sendC
		ft.lines <- "%begin 1 2 1"
		ft.lines <- "can't find session: $9"
		ft.lines <- "%error 1 2 1"
	}()

	if _, err := tmux.Options("$9", ""); err == nil {
		t.Fatal("expected an error when both attempts fail")
	} else if !strings.Contains(err.Error(), "can't find session") {
		t.Fatalf("expected the tmux error to survive, got %v", err)
	}
}
