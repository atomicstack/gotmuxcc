//go:build integration

package gotmuxcc

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/atomicstack/gotmuxcc/internal/testutil"
)

// emptySocket returns a socket path with no server behind it and arranges for
// any server a test starts there to be killed afterwards.
func emptySocket(t *testing.T) (tmuxBin, socketPath string) {
	t.Helper()
	if os.Getenv("GOTMUXCC_INTEGRATION") == "" {
		t.Skip("skipping tmux integration tests; set GOTMUXCC_INTEGRATION=1 to enable")
	}
	tmuxBin = requireTmux(t)
	t.Setenv("TMUX", "")
	dir := testutil.TempDir(t)
	socketPath = filepath.Join(dir, "tmux.sock")
	t.Cleanup(func() {
		_ = exec.Command(tmuxBin, "-S", socketPath, "kill-server").Run()
	})
	return tmuxBin, socketPath
}

func listNames(t *testing.T, tmuxBin, socketPath, command, format string) []string {
	t.Helper()
	out, err := exec.Command(tmuxBin, "-S", socketPath, command, "-F", format).Output()
	if err != nil {
		return nil
	}
	var names []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			names = append(names, line)
		}
	}
	return names
}

// TestEmptySocketLifecycle is the real-tmux counterpart of the lazy-dial unit
// tests: a client built on a socket with no server must not start one, must
// answer read-only queries, must start the server through NewSession, and
// must leave behind nothing but the session the caller asked for.
func TestEmptySocketLifecycle(t *testing.T) {
	tmuxBin, socketPath := emptySocket(t)

	tmux, err := NewTmuxWithOptions(socketPath)
	if err != nil {
		t.Fatalf("NewTmuxWithOptions on an empty socket returned error: %v", err)
	}
	defer tmux.Close()

	if _, err := os.Stat(socketPath); !os.IsNotExist(err) {
		t.Fatalf("constructing a client must not start a tmux server (socket exists, stat err=%v)", err)
	}

	sessions, err := tmux.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions with no server returned error: %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("expected no sessions, got %#v", sessions)
	}
	if tmux.HasSession("first") {
		t.Fatalf("HasSession must be false with no server")
	}
	if _, err := os.Stat(socketPath); !os.IsNotExist(err) {
		t.Fatalf("read-only queries must not start a tmux server (socket exists, stat err=%v)", err)
	}

	session, err := tmux.NewSession(&SessionOptions{Name: "first"})
	if err != nil {
		t.Fatalf("NewSession with no server returned error: %v", err)
	}
	if session == nil || session.Name != "first" {
		t.Fatalf("unexpected session: %#v", session)
	}

	sessions, err = tmux.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions after NewSession returned error: %v", err)
	}
	if len(sessions) != 1 || sessions[0].Name != "first" {
		t.Fatalf("expected exactly the requested session, got %#v", sessions)
	}
	windows, err := session.ListWindows()
	if err != nil {
		t.Fatalf("ListWindows over the lazily opened connection returned error: %v", err)
	}
	if len(windows) == 0 {
		t.Fatalf("expected the new session to have a window")
	}

	if err := tmux.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
	if names := listNames(t, tmuxBin, socketPath, "list-sessions", "#{session_name}"); len(names) != 1 || names[0] != "first" {
		t.Fatalf("after Close only the caller's session may remain, got %#v", names)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		clients := listNames(t, tmuxBin, socketPath, "list-clients", "#{client_pid}")
		if len(clients) == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("control client still attached after Close: %#v", clients)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// TestClientRecoversAfterServerExit covers the server going away underneath a
// connected client: tmux exits when its last session is killed, and the client
// must report an empty server rather than a dead transport, then reconnect
// once a session exists again.
func TestClientRecoversAfterServerExit(t *testing.T) {
	tmuxBin, socketPath := emptySocket(t)
	if out, err := exec.Command(tmuxBin, "-S", socketPath, "-f", "/dev/null", "new-session", "-d", "-s", "only").CombinedOutput(); err != nil {
		t.Skipf("failed to start tmux test server: %v (%s)", err, strings.TrimSpace(string(out)))
	}

	tmux, err := NewTmuxWithOptions(socketPath)
	if err != nil {
		t.Fatalf("NewTmuxWithOptions returned error: %v", err)
	}
	defer tmux.Close()
	if sessions, err := tmux.ListSessions(); err != nil || len(sessions) != 1 {
		t.Fatalf("expected one session before the server exits, got %#v err=%v", sessions, err)
	}

	if out, err := exec.Command(tmuxBin, "-S", socketPath, "kill-session", "-t", "only").CombinedOutput(); err != nil {
		t.Fatalf("kill-session failed: %v (%s)", err, strings.TrimSpace(string(out)))
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		sessions, err := tmux.ListSessions()
		if err == nil && len(sessions) == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("ListSessions never settled to an empty server: sessions=%#v err=%v", sessions, err)
		}
		time.Sleep(20 * time.Millisecond)
	}

	session, err := tmux.NewSession(&SessionOptions{Name: "again"})
	if err != nil {
		t.Fatalf("NewSession after the server exited returned error: %v", err)
	}
	if session == nil || session.Name != "again" {
		t.Fatalf("unexpected session: %#v", session)
	}
	sessions, err := tmux.ListSessions()
	if err != nil || len(sessions) != 1 || sessions[0].Name != "again" {
		t.Fatalf("expected the client to reconnect to the new server, got %#v err=%v", sessions, err)
	}
}
