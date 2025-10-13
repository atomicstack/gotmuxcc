//go:build integration
// +build integration

package control

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/atomicstack/gotmuxcc/internal/testutil"
)

func requireTmux(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux binary not found; skipping integration tests")
	}
	return path
}

func startIntegrationServer(t *testing.T) string {
	t.Helper()
	tmuxBin := requireTmux(t)
	dir := testutil.TempDir(t)
	socketPath := filepath.Join(dir, "tmux.sock")

	cmd := exec.Command(tmuxBin, "-S", socketPath, "-f", "/dev/null", "new-session", "-d", "-s", "tmuxcctest")
	cmd.Env = append(os.Environ(), fmt.Sprintf("TMUX_TMPDIR=%s", dir), "TMUX=")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("failed to start tmux server: %v (%s)", err, strings.TrimSpace(string(out)))
	}

	t.Cleanup(func() {
		_ = exec.Command(tmuxBin, "-S", socketPath, "kill-server").Run()
	})

	return socketPath
}

func TestTransportLifecycle(t *testing.T) {
	socket := startIntegrationServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tr, err := New(ctx, Config{SocketPath: socket})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	defer tr.Close()

	if err := tr.Send("list-sessions"); err != nil {
		t.Fatalf("Send returned error: %v", err)
	}

	timeout := time.After(2 * time.Second)
	var lines []string
	for {
		select {
		case line, ok := <-tr.Lines():
			if !ok {
				t.Fatalf("unexpected close of lines channel")
			}
			lines = append(lines, line)
			if strings.HasPrefix(line, "%end") || strings.HasPrefix(line, "%error") {
				goto done
			}
		case <-timeout:
			t.Fatalf("timed out waiting for control output")
		}
	}

done:
	if len(lines) == 0 {
		t.Fatalf("expected control output, got none")
	}

	cancel()

	select {
	case err := <-tr.Done():
		if err != nil && !strings.Contains(err.Error(), "context canceled") && !strings.Contains(err.Error(), "killed") && !strings.Contains(err.Error(), "closed") {
			t.Fatalf("unexpected done error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for transport shutdown")
	}
}
