//go:build integration
// +build integration

package gotmuxcc

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestListPanesReportsFloatingPanes asserts floating panes are identifiable in
// list-panes output. tmux 3.8 keeps them in the window's ordinary pane list, so
// only the floating/modal formats tell them apart.
func TestListPanesReportsFloatingPanes(t *testing.T) {
	if os.Getenv("GOTMUXCC_INTEGRATION") == "" {
		t.Skip("skipping tmux integration tests; set GOTMUXCC_INTEGRATION=1 to enable")
	}
	t.Setenv("TMUX", "")

	tmuxBin := requireTmux(t)
	socket := startTestServer(t)

	if out, err := exec.Command(tmuxBin, "-S", socket, "new-pane", "-d", "-O", "sleep 300").CombinedOutput(); err != nil {
		t.Skipf("skipping: this tmux has no floating panes: %v (%s)", err, strings.TrimSpace(string(out)))
	}

	tmux, err := NewTmuxWithOptions(socket)
	if err != nil {
		t.Skipf("skipping: failed to create tmux client: %v", err)
	}
	defer func() { _ = tmux.Close() }()

	panes, err := tmux.ListAllPanes()
	if err != nil {
		t.Fatalf("ListAllPanes returned error: %v", err)
	}

	var floating, tiled *Pane
	for _, pane := range panes {
		if pane.FloatingFlag {
			floating = pane
		} else {
			tiled = pane
		}
	}
	if floating == nil {
		t.Fatalf("no floating pane reported in %d panes", len(panes))
	}
	if tiled == nil {
		t.Fatalf("no tiled pane reported in %d panes", len(panes))
	}
	if !floating.ModalFlag {
		t.Errorf("expected the floating pane to be modal, got flags %q", floating.Flags)
	}
	if !strings.Contains(floating.Flags, "F") {
		t.Errorf("expected pane flags to contain F, got %q", floating.Flags)
	}
	if tiled.ModalFlag {
		t.Errorf("tiled pane should not be modal, got flags %q", tiled.Flags)
	}

	windows, err := tmux.ListAllWindows()
	if err != nil {
		t.Fatalf("ListAllWindows returned error: %v", err)
	}
	if len(windows) == 0 {
		t.Fatalf("expected at least one window")
	}
	if windows[0].ModalPane != floating.Id {
		t.Errorf("expected window modal pane %q, got %q", floating.Id, windows[0].ModalPane)
	}
}
