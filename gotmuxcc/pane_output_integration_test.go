//go:build integration
// +build integration

package gotmuxcc

import (
	"strings"
	"testing"
)

// firstPaneID returns the id of a pane on the test server.
func firstPaneID(t *testing.T, tmux *Tmux) string {
	t.Helper()
	panes, err := tmux.ListAllPanes()
	skipIfUnsupported(t, err)
	if err != nil {
		t.Fatalf("list panes: %v", err)
	}
	if len(panes) == 0 {
		t.Fatal("expected at least one pane on the test server")
	}
	return panes[0].Id
}

// TestPaneOutputControlsAcceptPaneIDs exercises the refresh-client -A family
// against a real server. tmux's parser treats a word starting with "%" as a
// conditional keyword unless everything after it is digits, so an unquoted
// "%0:on" is a syntax error rather than a pane-output request.
func TestPaneOutputControlsAcceptPaneIDs(t *testing.T) {
	tmux := newTestTmux(t)
	pane := firstPaneID(t, tmux)

	cases := []struct {
		name string
		call func() error
	}{
		{"disable", func() error { return tmux.DisablePaneOutput(pane) }},
		{"enable", func() error { return tmux.EnablePaneOutput(pane) }},
		{"pause", func() error { return tmux.PausePaneOutput(pane) }},
		{"continue", func() error { return tmux.ContinuePaneOutput(pane) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call()
			skipIfUnsupported(t, err)
			if err != nil {
				t.Fatalf("%s pane output for %s: %v", tc.name, pane, err)
			}
		})
	}
}

// TestSetMultiplePaneOutputsAcceptsPaneIDs covers the batched form, which
// builds its own command line rather than going through SetPaneOutput.
func TestSetMultiplePaneOutputsAcceptsPaneIDs(t *testing.T) {
	tmux := newTestTmux(t)
	pane := firstPaneID(t, tmux)

	err := tmux.SetMultiplePaneOutputs([]string{pane + ":off", pane + ":on"})
	skipIfUnsupported(t, err)
	if err != nil {
		t.Fatalf("set multiple pane outputs: %v", err)
	}
}

// TestReportPaneColorsAcceptsPaneID covers refresh-client -r, which has the
// same leading-% argument shape as -A.
func TestReportPaneColorsAcceptsPaneID(t *testing.T) {
	tmux := newTestTmux(t)
	pane := firstPaneID(t, tmux)

	err := tmux.ReportPaneColors(pane, "rgb:1111/2222/3333")
	skipIfUnsupported(t, err)
	if err != nil {
		t.Fatalf("report pane colors for %s: %v", pane, err)
	}
}

// TestPaneOutputSurfacesTmuxParseErrors guards the specific failure this
// family had: tmux answering with a parse error instead of applying the
// request. A parse error must never be the result of a well-formed call.
func TestPaneOutputSurfacesTmuxParseErrors(t *testing.T) {
	tmux := newTestTmux(t)
	pane := firstPaneID(t, tmux)

	if err := tmux.EnablePaneOutput(pane); err != nil {
		skipIfUnsupported(t, err)
		if strings.Contains(err.Error(), "parse error") {
			t.Fatalf("tmux could not parse the command gotmuxcc sent: %v", err)
		}
		t.Fatalf("enable pane output: %v", err)
	}
}
