//go:build integration
// +build integration

package gotmuxcc

import (
	"errors"
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

func startTestServer(t *testing.T) string {
	t.Helper()
	tmuxBin := requireTmux(t)
	dir := testutil.TempDir(t)
	socketPath := filepath.Join(dir, "tmux.sock")

	cmd := exec.Command(tmuxBin, "-S", socketPath, "-f", "/dev/null", "new-session", "-d", "-s", "gotmuxcctest")
	env := append(os.Environ(), fmt.Sprintf("TMUX_TMPDIR=%s", dir))
	env = append(env, "TMUX=")
	cmd.Env = env
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("skipping tmux integration tests: failed to start tmux test server: %v (%s)", err, strings.TrimSpace(string(out)))
	}

	t.Cleanup(func() {
		_ = exec.Command(tmuxBin, "-S", socketPath, "kill-server").Run()
	})

	return socketPath
}

func newTestTmux(t *testing.T) *Tmux {
	t.Helper()
	if os.Getenv("GOTMUXCC_INTEGRATION") == "" {
		t.Skip("skipping tmux integration tests; set GOTMUXCC_INTEGRATION=1 to enable")
	}
	t.Setenv("TMUX", "")
	socket := startTestServer(t)
	tmux, err := NewTmuxWithOptions(socket)
	if err != nil {
		t.Skipf("skipping tmux integration tests: failed to create tmux client: %v", err)
	}
	t.Cleanup(func() {
		_ = tmux.Close()
	})
	return tmux
}

func skipIfUnsupported(t *testing.T, err error) {
	if err == nil {
		return
	}
	if errors.Is(err, ErrTransportClosed) {
		t.Skipf("tmux not available in test environment: %v", err)
	}
	for e := err; e != nil; e = errors.Unwrap(e) {
		msg := e.Error()
		if strings.Contains(msg, "tmux exit") ||
			strings.Contains(msg, "Operation not permitted") ||
			strings.Contains(msg, "error connecting") ||
			strings.Contains(msg, "no clients") ||
			strings.Contains(msg, "no such client") ||
			strings.Contains(msg, "can't find session") ||
			strings.Contains(msg, "not running") {
			t.Skipf("tmux not available in test environment: %v", err)
		}
	}
}

func waitForCondition(t *testing.T, desc string, fn func() (bool, error)) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		done, err := fn()
		if err != nil {
			skipIfUnsupported(t, err)
			t.Fatalf("failed while waiting for %s: %v", desc, err)
		}
		if done {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %s", desc)
}

func TestGetServerInformation(t *testing.T) {
	tmux := newTestTmux(t)

	server, err := tmux.GetServerInformation()
	skipIfUnsupported(t, err)
	if err != nil {
		t.Fatalf("GetServerInformation returned error: %v", err)
	}
	if server == nil {
		t.Fatalf("expected server information, got nil")
	}
	if server.Socket == nil || server.Socket.Path == "" {
		t.Skipf("tmux socket path not available in test environment")
	}
}

func TestListClients(t *testing.T) {
	tmux := newTestTmux(t)

	clients, err := tmux.ListClients()
	skipIfUnsupported(t, err)
	if err != nil {
		t.Fatalf("ListClients returned error: %v", err)
	}
	if clients == nil {
		t.Fatalf("expected client slice, got nil")
	}
}

func TestSessionLifecycle(t *testing.T) {
	tmux := newTestTmux(t)

	sessions, err := tmux.ListSessions()
	skipIfUnsupported(t, err)
	if err != nil {
		t.Fatalf("ListSessions returned error: %v", err)
	}
	if len(sessions) == 0 {
		t.Fatalf("expected at least one session after bootstrap")
	}

	name := fmt.Sprintf("sess%d", time.Now().UnixNano())
	session, err := tmux.NewSession(&SessionOptions{Name: name})
	skipIfUnsupported(t, err)
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}
	if session == nil {
		t.Fatalf("expected session instance, got nil")
	}

	if !tmux.HasSession(name) {
		t.Fatalf("expected HasSession to return true for %q", name)
	}

	fetched, err := tmux.GetSessionByName(name)
	skipIfUnsupported(t, err)
	if err != nil {
		t.Fatalf("GetSessionByName returned error: %v", err)
	}
	if fetched == nil || fetched.Name != name {
		t.Fatalf("expected to fetch session %q, got %#v", name, fetched)
	}

	newName := name + "_renamed"
	if err := session.Rename(newName); err != nil {
		skipIfUnsupported(t, err)
		t.Fatalf("Rename returned error: %v", err)
	}
	if !tmux.HasSession(newName) {
		t.Fatalf("expected renamed session %q to exist", newName)
	}

	if err := session.SetOption("@gotmuxcc", "value"); err != nil {
		skipIfUnsupported(t, err)
		t.Fatalf("SetOption returned error: %v", err)
	}
	opt, err := session.Option("@gotmuxcc")
	skipIfUnsupported(t, err)
	if err != nil {
		t.Fatalf("Option returned error: %v", err)
	}
	if opt == nil || opt.Value != "value" {
		t.Fatalf("expected custom option value, got %#v", opt)
	}
	if err := session.DeleteOption("@gotmuxcc"); err != nil {
		skipIfUnsupported(t, err)
		t.Fatalf("DeleteOption returned error: %v", err)
	}

	if err := session.Kill(); err != nil {
		skipIfUnsupported(t, err)
		t.Fatalf("Kill returned error: %v", err)
	}
	if tmux.HasSession(newName) {
		t.Fatalf("expected session %q to be removed after Kill", newName)
	}
}

func TestSessionDetachAndSwitch(t *testing.T) {
	tmux := newTestTmux(t)

	base := fmt.Sprintf("sess-attach-%d", time.Now().UnixNano())
	s1, err := tmux.NewSession(&SessionOptions{Name: base + "-a"})
	skipIfUnsupported(t, err)
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}
	defer func() {
		_ = s1.Kill()
	}()

	s2, err := tmux.NewSession(&SessionOptions{Name: base + "-b"})
	skipIfUnsupported(t, err)
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}
	defer func() {
		_ = s2.Kill()
	}()

	if err := tmux.DetachClient(&DetachClientOptions{TargetSession: s1.Name}); err != nil {
		skipIfUnsupported(t, err)
		if err != nil {
			t.Fatalf("DetachClient returned error: %v", err)
		}
	}

	if err := tmux.SwitchClient(&SwitchClientOptions{TargetSession: s2.Name}); err != nil {
		skipIfUnsupported(t, err)
		if err != nil {
			t.Fatalf("SwitchClient returned error: %v", err)
		}
	}
}

func TestSessionEnumeration(t *testing.T) {
	tmux := newTestTmux(t)

	name := fmt.Sprintf("sess-enum-%d", time.Now().UnixNano())
	session, err := tmux.NewSession(&SessionOptions{Name: name})
	skipIfUnsupported(t, err)
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}
	defer func() {
		_ = session.Kill()
	}()

	if !tmux.HasSession(name) {
		t.Fatalf("HasSession returned false for %q", name)
	}

	fetched, err := tmux.Session(name)
	skipIfUnsupported(t, err)
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if fetched == nil || fetched.Name != name {
		t.Fatalf("expected session %q, got %#v", name, fetched)
	}

	clients, err := session.ListClients()
	skipIfUnsupported(t, err)
	if err != nil {
		t.Fatalf("ListClients returned error: %v", err)
	}
	if clients == nil {
		t.Fatalf("expected client slice, got nil")
	}
}

func TestWindowOperations(t *testing.T) {
	tmux := newTestTmux(t)

	base := fmt.Sprintf("win-ops-%d", time.Now().UnixNano())
	s1, err := tmux.NewSession(&SessionOptions{Name: base + "-a"})
	skipIfUnsupported(t, err)
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}
	defer func() { _ = s1.Kill() }()

	s2, err := tmux.NewSession(&SessionOptions{Name: base + "-b"})
	skipIfUnsupported(t, err)
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}
	defer func() { _ = s2.Kill() }()

	window, err := s1.NewWindow(&NewWindowOptions{WindowName: "ops"})
	skipIfUnsupported(t, err)
	if err != nil {
		t.Fatalf("NewWindow returned error: %v", err)
	}

	if err := window.SelectLayout(WindowLayoutEvenVertical); err != nil {
		skipIfUnsupported(t, err)
		if err != nil {
			t.Fatalf("SelectLayout returned error: %v", err)
		}
	}

	if err := window.Move(s2.Name, 1); err != nil {
		skipIfUnsupported(t, err)
		if err != nil {
			t.Fatalf("Move returned error: %v", err)
		}
	}

	winList, err := s2.ListWindows()
	skipIfUnsupported(t, err)
	if err != nil {
		t.Fatalf("ListWindows returned error: %v", err)
	}
	found := false
	for _, w := range winList {
		if w.Name == "ops" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("moved window not found in destination session")
	}

	if err := window.Select(); err != nil {
		skipIfUnsupported(t, err)
		if err != nil {
			t.Fatalf("Select returned error: %v", err)
		}
	}

	_, err = window.ListLinkedSessions()
	skipIfUnsupported(t, err)
	if err != nil {
		t.Fatalf("ListLinkedSessions returned error: %v", err)
	}

	_, err = window.ListActiveSessions()
	skipIfUnsupported(t, err)
	if err != nil {
		t.Fatalf("ListActiveSessions returned error: %v", err)
	}

	_, err = window.ListActiveClients()
	skipIfUnsupported(t, err)
	if err != nil {
		t.Fatalf("ListActiveClients returned error: %v", err)
	}
}

func TestPaneOperations(t *testing.T) {
	tmux := newTestTmux(t)

	name := fmt.Sprintf("pane-ops-%d", time.Now().UnixNano())
	session, err := tmux.NewSession(&SessionOptions{Name: name})
	skipIfUnsupported(t, err)
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}
	defer func() { _ = session.Kill() }()

	window, err := session.NewWindow(nil)
	skipIfUnsupported(t, err)
	if err != nil {
		t.Fatalf("NewWindow returned error: %v", err)
	}

	panes, err := window.ListPanes()
	skipIfUnsupported(t, err)
	if err != nil {
		t.Fatalf("ListPanes returned error: %v", err)
	}
	if len(panes) == 0 {
		t.Fatalf("expected at least one pane")
	}
	first := panes[0]

	if err := first.SplitWindow(&SplitWindowOptions{SplitDirection: PaneSplitDirectionVertical}); err != nil {
		skipIfUnsupported(t, err)
		if err != nil {
			t.Fatalf("SplitWindow returned error: %v", err)
		}
	}

	panes, err = window.ListPanes()
	skipIfUnsupported(t, err)
	if err != nil {
		t.Fatalf("ListPanes returned error after split: %v", err)
	}
	if len(panes) < 2 {
		t.Fatalf("expected multiple panes after split")
	}

	if err := first.SelectPane(&SelectPaneOptions{TargetPosition: PanePositionRight}); err != nil {
		skipIfUnsupported(t, err)
		if err != nil {
			t.Fatalf("SelectPane returned error: %v", err)
		}
	}

	if err := first.SendKeys("printf 'pane-test'\n"); err != nil {
		skipIfUnsupported(t, err)
		if err != nil {
			t.Fatalf("SendKeys returned error: %v", err)
		}
	}

	output, err := first.Capture()
	skipIfUnsupported(t, err)
	if err != nil {
		t.Fatalf("Capture returned error: %v", err)
	}
	if output == "" {
		t.Fatalf("expected capture output")
	}

	if err := first.ChooseTree(nil); err != nil {
		skipIfUnsupported(t, err)
		if err != nil {
			t.Fatalf("ChooseTree returned error: %v", err)
		}
	}
}
func TestWindowPaneLifecycle(t *testing.T) {
	tmux := newTestTmux(t)

	name := fmt.Sprintf("testwin-%d", time.Now().UnixNano())
	session, err := tmux.NewSession(&SessionOptions{Name: name})
	skipIfUnsupported(t, err)
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}
	defer func() {
		_ = session.Kill()
	}()

	window, err := session.NewWindow(&NewWindowOptions{WindowName: "dev"})
	skipIfUnsupported(t, err)
	if err != nil {
		t.Fatalf("NewWindow returned error: %v", err)
	}

	panes, err := window.ListPanes()
	skipIfUnsupported(t, err)
	if err != nil {
		t.Fatalf("ListPanes returned error: %v", err)
	}
	if len(panes) == 0 {
		t.Fatalf("expected at least one pane")
	}

	pane := panes[0]
	if err := pane.SendKeys("printf 'hello'\n"); err != nil {
		skipIfUnsupported(t, err)
		t.Fatalf("SendKeys returned error: %v", err)
	}

	captured, err := pane.Capture()
	skipIfUnsupported(t, err)
	if err != nil {
		t.Fatalf("Capture returned error: %v", err)
	}
	if captured == "" {
		t.Fatalf("expected capture output")
	}

	if err := pane.SetOption("@gotmuxcc-pane", "true"); err != nil {
		skipIfUnsupported(t, err)
		t.Fatalf("SetOption returned error: %v", err)
	}

	opt, err := pane.Option("@gotmuxcc-pane")
	skipIfUnsupported(t, err)
	if err != nil {
		t.Fatalf("Option returned error: %v", err)
	}
	if opt == nil || opt.Value != "true" {
		t.Fatalf("expected pane option value")
	}

	if err := pane.DeleteOption("@gotmuxcc-pane"); err != nil {
		skipIfUnsupported(t, err)
		t.Fatalf("DeleteOption returned error: %v", err)
	}
}

func TestSessionWindowPaneState(t *testing.T) {
	tmux := newTestTmux(t)

	name := fmt.Sprintf("state-%d", time.Now().UnixNano())
	session, err := tmux.NewSession(&SessionOptions{Name: name})
	skipIfUnsupported(t, err)
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}
	defer func() { _ = session.Kill() }()

	windows, err := session.ListWindows()
	skipIfUnsupported(t, err)
	if err != nil {
		t.Fatalf("ListWindows returned error: %v", err)
	}
	if len(windows) == 0 {
		t.Fatalf("expected initial window for session")
	}

	initial := windows[0]
	if err := initial.Rename("code"); err != nil {
		skipIfUnsupported(t, err)
		t.Fatalf("Rename returned error: %v", err)
	}

	logsWindow, err := session.NewWindow(&NewWindowOptions{WindowName: "logs"})
	skipIfUnsupported(t, err)
	if err != nil {
		t.Fatalf("NewWindow returned error: %v", err)
	}

	waitForCondition(t, "renamed and new windows visible", func() (bool, error) {
		wins, err := session.ListWindows()
		if err != nil {
			return false, err
		}
		seen := map[string]bool{}
		for _, w := range wins {
			seen[w.Name] = true
		}
		return seen["code"] && seen["logs"], nil
	})

	windows, err = session.ListWindows()
	skipIfUnsupported(t, err)
	if err != nil {
		t.Fatalf("ListWindows returned error: %v", err)
	}

	var codeWindow *Window
	for _, w := range windows {
		if w.Name == "code" {
			codeWindow = w
		}
	}
	if codeWindow == nil {
		t.Fatalf("failed to locate renamed code window")
	}

	panes, err := codeWindow.ListPanes()
	skipIfUnsupported(t, err)
	if err != nil {
		t.Fatalf("ListPanes returned error: %v", err)
	}
	if len(panes) == 0 {
		t.Fatalf("expected code window to contain at least one pane")
	}

	if err := panes[0].SplitWindow(&SplitWindowOptions{SplitDirection: PaneSplitDirectionVertical}); err != nil {
		skipIfUnsupported(t, err)
		t.Fatalf("SplitWindow returned error: %v", err)
	}

	waitForCondition(t, "second pane to appear", func() (bool, error) {
		panes, err := codeWindow.ListPanes()
		if err != nil {
			return false, err
		}
		return len(panes) >= 2, nil
	})

	leftPane, err := codeWindow.GetPaneByIndex(0)
	skipIfUnsupported(t, err)
	if err != nil {
		t.Fatalf("GetPaneByIndex returned error: %v", err)
	}
	if leftPane == nil {
		t.Fatalf("expected pane index 0 to exist")
	}

	rightPane, err := codeWindow.GetPaneByIndex(1)
	skipIfUnsupported(t, err)
	if err != nil {
		t.Fatalf("GetPaneByIndex returned error: %v", err)
	}
	if rightPane == nil {
		t.Fatalf("expected pane index 1 to exist after split")
	}

	if err := leftPane.SendKeys("printf 'code-left'\n"); err != nil {
		skipIfUnsupported(t, err)
		t.Fatalf("SendKeys returned error for left pane: %v", err)
	}
	if err := rightPane.SendKeys("printf 'code-right'\n"); err != nil {
		skipIfUnsupported(t, err)
		t.Fatalf("SendKeys returned error for right pane: %v", err)
	}

	waitForCondition(t, "left pane capture to include output", func() (bool, error) {
		out, err := leftPane.Capture()
		if err != nil {
			return false, err
		}
		return strings.Contains(out, "code-left"), nil
	})

	waitForCondition(t, "right pane capture to include output", func() (bool, error) {
		out, err := rightPane.Capture()
		if err != nil {
			return false, err
		}
		return strings.Contains(out, "code-right"), nil
	})

	waitForCondition(t, "session to report multiple panes", func() (bool, error) {
		panes, err := session.ListPanes()
		if err != nil {
			return false, err
		}
		return len(panes) >= 2, nil
	})

	waitForCondition(t, "session metadata to reflect new window count", func() (bool, error) {
		refetched, err := tmux.GetSessionByName(name)
		if err != nil {
			return false, err
		}
		return refetched != nil && refetched.Windows >= 2, nil
	})

	if err := logsWindow.Select(); err != nil {
		skipIfUnsupported(t, err)
		t.Fatalf("Select returned error: %v", err)
	}

	waitForCondition(t, "logs window to become active", func() (bool, error) {
		wins, err := session.ListWindows()
		if err != nil {
			return false, err
		}
		for _, w := range wins {
			if w.Name == "logs" && w.Active {
				return true, nil
			}
		}
		return false, nil
	})
}
