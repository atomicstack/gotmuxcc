package gotmuxcc

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/atomicstack/gotmuxcc/internal/testutil"
)

func TestAttachArgsUsesExistingSession(t *testing.T) {
	script := `
if [ "$1" = "-S" ]; then
	shift
	shift
fi

if [ "$1" != "list-sessions" ]; then
	echo "unexpected command: $1" >&2
	exit 3
fi

printf '$0\n'
`
	path := writeFakeTmux(t, script)
	args, err := attachArgs(path, "/tmp/sock")
	if err != nil {
		t.Fatalf("attachArgs returned error: %v", err)
	}
	if len(args) != 3 || args[0] != "attach-session" || args[1] != "-t" || args[2] != "$0" {
		t.Fatalf("unexpected attach args: %#v", args)
	}
}

// TestAttachArgsNoSessionsReportsNoServer pins the replacement for the old
// bootstrap session: with nothing to attach to, the planner must report
// ErrNoServer and hand back no command at all, so that the caller never starts
// a tmux server (and never leaves a stray session behind) just to open a
// control-mode connection.
func TestAttachArgsNoSessionsReportsNoServer(t *testing.T) {
	script := `
echo "no server running on /tmp/sock" >&2
exit 1
`
	path := writeFakeTmux(t, script)
	args, err := attachArgs(path, "/tmp/sock")
	if !errors.Is(err, ErrNoServer) {
		t.Fatalf("expected ErrNoServer, got args=%#v err=%v", args, err)
	}
	if len(args) != 0 {
		t.Fatalf("expected no command when there is no server, got %#v", args)
	}
}

func TestAttachArgsDiscoveryError(t *testing.T) {
	script := `
echo "boom" >&2
exit 3
`
	path := writeFakeTmux(t, script)
	_, err := attachArgs(path, "/tmp/sock")
	if err == nil {
		t.Fatalf("expected attachArgs to return error")
	}
	if errors.Is(err, ErrNoServer) {
		t.Fatalf("a discovery failure must not be reported as ErrNoServer: %v", err)
	}
}

func writeFakeTmux(t testing.TB, script string) string {
	t.Helper()
	dir := testutil.TempDir(t)
	path := filepath.Join(dir, "fake_tmux.sh")
	content := []byte("#!/bin/sh\n" + script + "\n")
	if err := os.WriteFile(path, content, 0o755); err != nil {
		t.Fatalf("failed to write fake tmux: %v", err)
	}
	return path
}
