//go:build integration
// +build integration

package gotmuxcc

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/atomicstack/gotmuxcc/internal/testutil"
)

// TestCapturePaneWithGuardLikeContent captures a pane that is displaying a
// control-mode transcript. tmux writes command output verbatim, so those lines
// arrive on the control connection looking exactly like protocol guards; they
// must come back as content and must not disturb the connection.
func TestCapturePaneWithGuardLikeContent(t *testing.T) {
	if os.Getenv("GOTMUXCC_INTEGRATION") == "" {
		t.Skip("skipping tmux integration tests; set GOTMUXCC_INTEGRATION=1 to enable")
	}
	t.Setenv("TMUX", "")

	tmuxBin := requireTmux(t)
	socket := startTestServer(t)

	payload := []string{"%exit fake-content", "%begin 1 2 3", "%end 1 2 3"}
	payloadPath := filepath.Join(testutil.TempDir(t), "transcript.txt")
	if err := os.WriteFile(payloadPath, []byte(strings.Join(payload, "\n")+"\n"), 0o600); err != nil {
		t.Fatalf("failed to write payload: %v", err)
	}

	// Build the pane with the tmux binary so the test does not depend on the
	// library's own quoting when setting up its fixture.
	out, err := exec.Command(tmuxBin, "-S", socket, "new-window", "-P", "-F", "#{pane_id}",
		"cat "+payloadPath+"; sleep 300").CombinedOutput()
	if err != nil {
		t.Skipf("skipping: failed to create fixture window: %v (%s)", err, strings.TrimSpace(string(out)))
	}
	paneID := strings.TrimSpace(string(out))

	tmux, err := NewTmuxWithOptions(socket)
	if err != nil {
		t.Skipf("skipping: failed to create tmux client: %v", err)
	}
	defer func() { _ = tmux.Close() }()

	var captured string
	waitForCondition(t, "pane to display the transcript", func() (bool, error) {
		captured, err = tmux.CapturePane(paneID, nil)
		if err != nil {
			return false, err
		}
		return strings.Contains(captured, payload[len(payload)-1]), nil
	})

	for _, want := range payload {
		if !strings.Contains(captured, want) {
			t.Fatalf("captured output is missing %q: %q", want, captured)
		}
	}

	// The control connection must be unharmed.
	if _, err := tmux.ListSessions(); err != nil {
		t.Fatalf("control connection broken after capturing guard-like content: %v", err)
	}
}
