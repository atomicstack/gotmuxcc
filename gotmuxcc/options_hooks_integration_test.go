//go:build integration
// +build integration

package gotmuxcc

import (
	"testing"
)

// TestOptionsIncludesUserOptionsRegisteredAsHooks asserts a user option set
// with set-hook is still listed by Options.
//
// tmux 7277712c ("Switch show-options over to using a format and add -F")
// made show-options skip "@"-prefixed options that are user hooks unless -H
// is given (cmd-show-options.c, cmd_show_options_all). Before that every user
// option was printed unconditionally, so a caller that had been seeing such an
// option silently stops seeing it.
func TestOptionsIncludesUserOptionsRegisteredAsHooks(t *testing.T) {
	tmux := newTestTmux(t)

	sessions, err := tmux.ListSessions()
	skipIfUnsupported(t, err)
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(sessions) == 0 {
		t.Fatal("expected a session on the test server")
	}
	target := sessions[0].Id

	// A plain user option, and one registered as a hook. Both are "@"-prefixed
	// user options as far as show-options is concerned.
	if err := tmux.SetOption(target, "@plain-user-option", "kept", ""); err != nil {
		skipIfUnsupported(t, err)
		t.Fatalf("set plain user option: %v", err)
	}
	if _, err := tmux.Command("set-hook", "-t", target, "@hook-user-option", "display-message hi"); err != nil {
		skipIfUnsupported(t, err)
		t.Skipf("could not register a user hook: %v", err)
	}

	opts, err := tmux.Options(target, "")
	skipIfUnsupported(t, err)
	if err != nil {
		t.Fatalf("list options: %v", err)
	}

	seen := make(map[string]bool, len(opts))
	for _, o := range opts {
		if o != nil {
			seen[o.Key] = true
		}
	}

	if !seen["@plain-user-option"] {
		t.Fatalf("plain user option missing from Options: %#v", seen)
	}
	if !seen["@hook-user-option"] {
		t.Fatalf("user option registered as a hook is missing from Options; show-options needs -H since tmux 7277712c")
	}
}
