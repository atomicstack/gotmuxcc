package gotmuxcc

import "testing"

func queryVars(build func(*query) *query) map[string]bool {
	q := newQuery(nil)
	build(q)
	set := make(map[string]bool, len(q.variables))
	for _, v := range q.variables {
		set[v] = true
	}
	return set
}

// TestPaneVarsRequestFloatingFormats asserts list-panes asks for the formats
// that distinguish floating and modal panes. Since tmux next-3.8 floating panes
// are ordinary members of w->panes, so without these they are indistinguishable
// from tiled panes in list-panes output.
func TestPaneVarsRequestFloatingFormats(t *testing.T) {
	vars := queryVars(func(q *query) *query { return q.paneVars() })

	for _, want := range []string{
		"pane_floating_flag",
		"pane_modal_flag",
		"pane_z",
		"pane_flags",
		"pane_x",
		"pane_y",
		"pane_unzoomed_width",
		"pane_unzoomed_height",
	} {
		if !vars[want] {
			t.Errorf("paneVars() does not request %q", want)
		}
	}
}

func TestPaneConversionReadsFloatingFormats(t *testing.T) {
	qr := queryResult{
		varPaneId:             "%7",
		varPaneFloatingFlag:   "1",
		varPaneModalFlag:      "1",
		varPaneZ:              "3",
		varPaneFlags:          "F",
		varPaneX:              "12",
		varPaneY:              "4",
		varPaneUnzoomedWidth:  "80",
		varPaneUnzoomedHeight: "24",
	}

	p := qr.toPane(&Tmux{})
	if !p.FloatingFlag {
		t.Errorf("expected FloatingFlag true")
	}
	if !p.ModalFlag {
		t.Errorf("expected ModalFlag true")
	}
	if p.Z != 3 {
		t.Errorf("expected Z 3, got %d", p.Z)
	}
	if p.Flags != "F" {
		t.Errorf("expected Flags %q, got %q", "F", p.Flags)
	}
	if p.X != 12 || p.Y != 4 {
		t.Errorf("expected X/Y 12/4, got %d/%d", p.X, p.Y)
	}
	if p.UnzoomedWidth != 80 || p.UnzoomedHeight != 24 {
		t.Errorf("expected unzoomed 80x24, got %dx%d", p.UnzoomedWidth, p.UnzoomedHeight)
	}
}

// TestPaneConversionDefaultsWithoutFloatingFormats asserts older tmux servers,
// which resolve the new formats to the empty string, still convert cleanly.
func TestPaneConversionDefaultsWithoutFloatingFormats(t *testing.T) {
	qr := queryResult{varPaneId: "%1"}

	p := qr.toPane(&Tmux{})
	if p.FloatingFlag || p.ModalFlag {
		t.Errorf("expected floating/modal flags false, got %v/%v", p.FloatingFlag, p.ModalFlag)
	}
	if p.Z != 0 || p.X != 0 || p.Y != 0 {
		t.Errorf("expected zero z/x/y, got %d/%d/%d", p.Z, p.X, p.Y)
	}
	if p.Flags != "" {
		t.Errorf("expected empty flags, got %q", p.Flags)
	}
}

func TestWindowVarsRequestFloatingFormats(t *testing.T) {
	vars := queryVars(func(q *query) *query { return q.windowVars() })

	for _, want := range []string{
		"window_modal_pane",
		"window_manual_width",
		"window_manual_height",
	} {
		if !vars[want] {
			t.Errorf("windowVars() does not request %q", want)
		}
	}
}

func TestWindowConversionReadsFloatingFormats(t *testing.T) {
	qr := queryResult{
		varWindowId:           "@2",
		varWindowModalPane:    "%9",
		varWindowManualWidth:  "100",
		varWindowManualHeight: "40",
	}

	w := qr.toWindow(&Tmux{})
	if w.ModalPane != "%9" {
		t.Errorf("expected ModalPane %q, got %q", "%9", w.ModalPane)
	}
	if w.ManualWidth != 100 || w.ManualHeight != 40 {
		t.Errorf("expected manual size 100x40, got %dx%d", w.ManualWidth, w.ManualHeight)
	}
}
