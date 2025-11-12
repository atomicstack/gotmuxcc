package gotmuxcc

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/atomicstack/gotmuxcc/internal/testutil"
)

func TestInitialAttachArgsUsesExistingSession(t *testing.T) {
	script := `
if [ "$1" = "-S" ]; then
	shift
	shift
fi

if [ "$1" != "list-sessions" ]; then
	echo "unexpected command: $1" >&2
	exit 3
fi

printf 'dev\n'
`
	path := writeFakeTmux(t, script)
	args := initialAttachArgs(path, "/tmp/sock")
	if len(args) != 3 || args[0] != "attach-session" || args[1] != "-t" || args[2] != "dev" {
		t.Fatalf("unexpected attach args: %#v", args)
	}
}

func TestInitialAttachArgsNoSessions(t *testing.T) {
	script := `
exit 1
`
	path := writeFakeTmux(t, script)
	if args := initialAttachArgs(path, "/tmp/sock"); args != nil {
		t.Fatalf("expected nil args, got %#v", args)
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
