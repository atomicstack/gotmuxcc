//go:build integration
// +build integration

package gotmuxcc

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/atomicstack/gotmuxcc/internal/testutil"
)

// TestNewTmuxAttachesToSessionNamedWithASeparator starts a server whose only
// session is named with a ":" in it. tmux has allowed that since 166267c8, and
// attach-session parses the ":" as the session:window separator, so attaching
// by name fails outright with "can't find window: ...". Discovery must use the
// session id.
func TestNewTmuxAttachesToSessionNamedWithASeparator(t *testing.T) {
	if os.Getenv("GOTMUXCC_INTEGRATION") == "" {
		t.Skip("skipping tmux integration tests; set GOTMUXCC_INTEGRATION=1 to enable")
	}
	t.Setenv("TMUX", "")

	tmuxBin := requireTmux(t)
	dir := testutil.TempDir(t)
	socketPath := filepath.Join(dir, "tmux.sock")

	// The name sorts ahead of anything else so it is the session discovery
	// picks, and carries both separators tmux targets understand.
	const weird = "aa:weird.name"

	cmd := exec.Command(tmuxBin, "-S", socketPath, "-f", "/dev/null", "new-session", "-d", "-s", weird)
	env := append(os.Environ(), fmt.Sprintf("TMUX_TMPDIR=%s", dir))
	env = append(env, "TMUX=")
	cmd.Env = env
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("could not start a server with a separator-bearing session name: %v (%s)", err, strings.TrimSpace(string(out)))
	}
	t.Cleanup(func() {
		_ = exec.Command(tmuxBin, "-S", socketPath, "kill-server").Run()
	})

	tmux, err := NewTmuxWithOptions(socketPath)
	if err != nil {
		t.Fatalf("NewTmux could not attach to a server whose session is named %q: %v", weird, err)
	}
	t.Cleanup(func() { _ = tmux.Close() })

	// The connection must be usable, not merely constructed. Deliberately not
	// routed through skipIfUnsupported: a failed attach surfaces here as
	// "tmux exit", which that helper treats as a missing environment and
	// would turn this into a skip instead of a failure.
	sessions, err := tmux.ListSessions()
	if err != nil {
		t.Fatalf("attached client is unusable, attach-session did not resolve %q: %v", weird, err)
	}
	if len(sessions) != 1 || sessions[0].Name != weird {
		t.Fatalf("expected the one session %q, got %#v", weird, sessions)
	}
}
