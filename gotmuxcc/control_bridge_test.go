package gotmuxcc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/atomicstack/gotmuxcc/internal/testutil"
)

func TestPlanInitialSessionUsesExistingSession(t *testing.T) {
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
	plan, err := planInitialSession(path, "/tmp/sock")
	if err != nil {
		t.Fatalf("planInitialSession returned error: %v", err)
	}
	if len(plan.extraArgs) != 3 || plan.extraArgs[0] != "attach-session" || plan.extraArgs[1] != "-t" || plan.extraArgs[2] != "dev" {
		t.Fatalf("unexpected attach args: %#v", plan.extraArgs)
	}
	if plan.bootstrapSession != "" {
		t.Fatalf("expected no bootstrap session, got %q", plan.bootstrapSession)
	}
}

func TestPlanInitialSessionNoSessionsUsesBootstrapSession(t *testing.T) {
	script := `
exit 1
`
	path := writeFakeTmux(t, script)
	plan, err := planInitialSession(path, "/tmp/sock")
	if err != nil {
		t.Fatalf("planInitialSession returned error: %v", err)
	}
	if len(plan.extraArgs) != 4 || plan.extraArgs[0] != "new-session" || plan.extraArgs[1] != "-d" || plan.extraArgs[2] != "-s" {
		t.Fatalf("unexpected bootstrap args: %#v", plan.extraArgs)
	}
	if plan.bootstrapSession == "" {
		t.Fatalf("expected bootstrap session name")
	}
	if plan.extraArgs[3] != plan.bootstrapSession {
		t.Fatalf("expected bootstrap session name in args, got %#v", plan.extraArgs)
	}
	if !strings.HasPrefix(plan.bootstrapSession, "__gotmuxcc_bootstrap_") {
		t.Fatalf("unexpected bootstrap session name %q", plan.bootstrapSession)
	}
}

func TestPlanInitialSessionDiscoveryError(t *testing.T) {
	script := `
echo "boom" >&2
exit 3
`
	path := writeFakeTmux(t, script)
	if _, err := planInitialSession(path, "/tmp/sock"); err == nil {
		t.Fatalf("expected planInitialSession to return error")
	}
}

func TestBootstrapCleanupTransportCloseKillsBootstrapSession(t *testing.T) {
	logPath := filepath.Join(testutil.TempDir(t), "tmux.log")
	script := `
if [ -n "$FAKE_TMUX_LOG" ]; then
	printf '%s\n' "$*" >> "$FAKE_TMUX_LOG"
fi

if [ "$1" = "-S" ]; then
	shift
	shift
fi

if [ "$1" != "kill-session" ]; then
	echo "unexpected command: $1" >&2
	exit 3
fi
`
	path := writeFakeTmux(t, script)
	t.Setenv("FAKE_TMUX_LOG", logPath)

	transport := &bootstrapCleanupTransport{
		controlTransport: newRecordTransport(),
		tmuxBinary:       path,
		socketPath:       "/tmp/sock",
		bootstrapSession: "__gotmuxcc_bootstrap_test",
	}

	if err := transport.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
	if err := transport.Close(); err != nil {
		t.Fatalf("second Close returned error: %v", err)
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected one cleanup command, got %#v", lines)
	}
	if want := "kill-session -t __gotmuxcc_bootstrap_test"; !strings.Contains(lines[0], want) {
		t.Fatalf("expected cleanup command to contain %q, got %q", want, lines[0])
	}
	if want := "-S /tmp/sock"; !strings.Contains(lines[0], want) {
		t.Fatalf("expected cleanup command to contain %q, got %q", want, lines[0])
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
