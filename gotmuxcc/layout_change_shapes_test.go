package gotmuxcc

import "testing"

// tmux builds %layout-change from the fixed template
// "%layout-change #{window_id} #{window_layout} #{window_visible_layout} #{window_raw_flags}"
// (control-notify.c), expanding each variable in place. Two of those four can
// come back empty, so the line has fewer than four whitespace-separated
// fields while still carrying all four positions.
//
// Both shapes below were captured from a live next-3.9 server.
func TestLayoutChangeNotificationShapes(t *testing.T) {
	tests := []struct {
		name              string
		line              string
		wantWindowID      string
		wantLayout        string
		wantVisibleLayout string
		wantFlags         string
	}{
		{
			name:              "all fields present",
			line:              "%layout-change @0 b25d,80x24,0,0,0 b25d,80x24,0,0,0 *",
			wantWindowID:      "@0",
			wantLayout:        "b25d,80x24,0,0,0",
			wantVisibleLayout: "b25d,80x24,0,0,0",
			wantFlags:         "*",
		},
		{
			// #{window_raw_flags} is empty for any window that is not
			// current, last, marked or zoomed and has no activity, bell,
			// silence or modal pane. The line then ends with a space.
			name:              "empty trailing flags",
			line:              "%layout-change @1 419a,80x24,0,0[80x12,0,0,1,80x11,0,13,2] 419a,80x24,0,0[80x12,0,0,1,80x11,0,13,2] ",
			wantWindowID:      "@1",
			wantLayout:        "419a,80x24,0,0[80x12,0,0,1,80x11,0,13,2]",
			wantVisibleLayout: "419a,80x24,0,0[80x12,0,0,1,80x11,0,13,2]",
			wantFlags:         "",
		},
		{
			// A window whose only panes are floating has no tiled content, so
			// the v1 compatibility dump is empty and both layout fields come
			// back blank. Reachable with `break-pane -W`.
			name:              "all floating window has empty layouts",
			line:              "%layout-change @0   *",
			wantWindowID:      "@0",
			wantLayout:        "",
			wantVisibleLayout: "",
			wantFlags:         "*",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := parseEvent(tt.line).Notification()
			if n == nil {
				t.Fatalf("notification was dropped for %q", tt.line)
			}
			lc, ok := n.(*LayoutChangeNotification)
			if !ok {
				t.Fatalf("expected *LayoutChangeNotification, got %T", n)
			}
			if lc.WindowID != tt.wantWindowID {
				t.Errorf("WindowID = %q, want %q", lc.WindowID, tt.wantWindowID)
			}
			if lc.Layout != tt.wantLayout {
				t.Errorf("Layout = %q, want %q", lc.Layout, tt.wantLayout)
			}
			if lc.VisibleLayout != tt.wantVisibleLayout {
				t.Errorf("VisibleLayout = %q, want %q", lc.VisibleLayout, tt.wantVisibleLayout)
			}
			if lc.Flags != tt.wantFlags {
				t.Errorf("Flags = %q, want %q", lc.Flags, tt.wantFlags)
			}
		})
	}
}

// TestLayoutChangeNotificationRejectsTruncatedLines asserts a line that is
// genuinely malformed is still refused, rather than the fix accepting
// anything with a window id.
func TestLayoutChangeNotificationRejectsTruncatedLines(t *testing.T) {
	for _, line := range []string{
		"%layout-change",
		"%layout-change @0",
		"%layout-change @0 b25d,80x24,0,0,0",
	} {
		if n := parseEvent(line).Notification(); n != nil {
			t.Errorf("expected %q to be refused, got %#v", line, n)
		}
	}
}

// TestLayoutChangeNotificationHandlesJSONLayouts covers the v2 layout format a
// client receives once it sets the new-layouts flag. The strings contain no
// whitespace, so field positions are unaffected.
func TestLayoutChangeNotificationHandlesJSONLayouts(t *testing.T) {
	const layout = `{"V":2,"L":{"t":"p","w":80,"h":24,"x":0,"y":0,"a":true,"i":0,"I":"%0"}}`
	n := parseEvent("%layout-change @0 " + layout + " " + layout + " *").Notification()
	lc, ok := n.(*LayoutChangeNotification)
	if !ok {
		t.Fatalf("expected *LayoutChangeNotification, got %T", n)
	}
	if lc.Layout != layout || lc.VisibleLayout != layout {
		t.Fatalf("json layout not carried verbatim: %#v", lc)
	}
	if lc.Flags != "*" {
		t.Fatalf("Flags = %q, want %q", lc.Flags, "*")
	}
}
