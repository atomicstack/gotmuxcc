package gotmuxcc

import (
	"bytes"
	"testing"
)

func TestOutputNotification(t *testing.T) {
	evt := parseEvent("%output %0 hello\\012world")
	n := evt.Notification()
	out, ok := n.(*OutputNotification)
	if !ok {
		t.Fatalf("expected *OutputNotification, got %T", n)
	}
	if out.PaneID != "%0" {
		t.Fatalf("unexpected pane ID: %q", out.PaneID)
	}
	if !bytes.Equal(out.Data, []byte("hello\nworld")) {
		t.Fatalf("unexpected data: %q", out.Data)
	}
}

func TestOutputNotificationPreservesSpaces(t *testing.T) {
	// Spaces in the data payload must not be lost.
	evt := parseEvent("%output %3 foo  bar  baz")
	n := evt.Notification()
	out, ok := n.(*OutputNotification)
	if !ok {
		t.Fatalf("expected *OutputNotification, got %T", n)
	}
	if !bytes.Equal(out.Data, []byte("foo  bar  baz")) {
		t.Fatalf("unexpected data: %q", out.Data)
	}
}

func TestOutputNotificationEmptyData(t *testing.T) {
	evt := parseEvent("%output %0")
	n := evt.Notification()
	out, ok := n.(*OutputNotification)
	if !ok {
		t.Fatalf("expected *OutputNotification, got %T", n)
	}
	if len(out.Data) != 0 {
		t.Fatalf("expected empty data, got %q", out.Data)
	}
}

func TestOutputNotificationBinaryData(t *testing.T) {
	// Simulate escape sequence: ESC [ 3 1 m
	evt := parseEvent("%output %1 \\033[31m")
	n := evt.Notification()
	out, ok := n.(*OutputNotification)
	if !ok {
		t.Fatalf("expected *OutputNotification, got %T", n)
	}
	if !bytes.Equal(out.Data, []byte("\033[31m")) {
		t.Fatalf("unexpected data: %q", out.Data)
	}
}

func TestExtendedOutputNotification(t *testing.T) {
	evt := parseEvent("%extended-output %2 150 : hello\\012world")
	n := evt.Notification()
	eout, ok := n.(*ExtendedOutputNotification)
	if !ok {
		t.Fatalf("expected *ExtendedOutputNotification, got %T", n)
	}
	if eout.PaneID != "%2" {
		t.Fatalf("unexpected pane ID: %q", eout.PaneID)
	}
	if eout.Age != 150 {
		t.Fatalf("unexpected age: %d", eout.Age)
	}
	if !bytes.Equal(eout.Data, []byte("hello\nworld")) {
		t.Fatalf("unexpected data: %q", eout.Data)
	}
}

func TestExtendedOutputNotificationNoColon(t *testing.T) {
	evt := parseEvent("%extended-output %0 100 nodata")
	n := evt.Notification()
	if n != nil {
		t.Fatalf("expected nil for malformed extended-output, got %T", n)
	}
}

func TestLayoutChangeNotification(t *testing.T) {
	evt := parseEvent("%layout-change @1 ab34,80x24,0,0 ab34,80x24,0,0 *")
	n := evt.Notification()
	lc, ok := n.(*LayoutChangeNotification)
	if !ok {
		t.Fatalf("expected *LayoutChangeNotification, got %T", n)
	}
	if lc.WindowID != "@1" {
		t.Fatalf("unexpected window ID: %q", lc.WindowID)
	}
	if lc.Layout != "ab34,80x24,0,0" {
		t.Fatalf("unexpected layout: %q", lc.Layout)
	}
	if lc.VisibleLayout != "ab34,80x24,0,0" {
		t.Fatalf("unexpected visible layout: %q", lc.VisibleLayout)
	}
	if lc.Flags != "*" {
		t.Fatalf("unexpected flags: %q", lc.Flags)
	}
}

func TestLayoutChangeNotificationTooFewFields(t *testing.T) {
	evt := parseEvent("%layout-change @1 ab34")
	n := evt.Notification()
	if n != nil {
		t.Fatalf("expected nil for short layout-change, got %T", n)
	}
}

func TestSubscriptionChangedNotification(t *testing.T) {
	evt := parseEvent("%subscription-changed mytitle $0 @1 0 %0 : my window title")
	n := evt.Notification()
	sc, ok := n.(*SubscriptionChangedNotification)
	if !ok {
		t.Fatalf("expected *SubscriptionChangedNotification, got %T", n)
	}
	if sc.Name != "mytitle" {
		t.Fatalf("unexpected name: %q", sc.Name)
	}
	if sc.SessionID != "$0" {
		t.Fatalf("unexpected session ID: %q", sc.SessionID)
	}
	if sc.WindowID != "@1" {
		t.Fatalf("unexpected window ID: %q", sc.WindowID)
	}
	if sc.Index != "0" {
		t.Fatalf("unexpected index: %q", sc.Index)
	}
	if sc.PaneID != "%0" {
		t.Fatalf("unexpected pane ID: %q", sc.PaneID)
	}
	if sc.Value != "my window title" {
		t.Fatalf("unexpected value: %q", sc.Value)
	}
}

func TestSubscriptionChangedSessionLevel(t *testing.T) {
	evt := parseEvent("%subscription-changed mysub $0 - - - : value here")
	n := evt.Notification()
	sc, ok := n.(*SubscriptionChangedNotification)
	if !ok {
		t.Fatalf("expected *SubscriptionChangedNotification, got %T", n)
	}
	if sc.WindowID != "-" {
		t.Fatalf("unexpected window ID: %q", sc.WindowID)
	}
	if sc.PaneID != "-" {
		t.Fatalf("unexpected pane ID: %q", sc.PaneID)
	}
	if sc.Value != "value here" {
		t.Fatalf("unexpected value: %q", sc.Value)
	}
}

func TestSubscriptionChangedNoColon(t *testing.T) {
	evt := parseEvent("%subscription-changed name $0 - - -")
	n := evt.Notification()
	if n != nil {
		t.Fatalf("expected nil for malformed subscription-changed, got %T", n)
	}
}

func TestSessionChangedNotification(t *testing.T) {
	evt := parseEvent("%session-changed $3 my session")
	n := evt.Notification()
	sc, ok := n.(*SessionChangedNotification)
	if !ok {
		t.Fatalf("expected *SessionChangedNotification, got %T", n)
	}
	if sc.SessionID != "$3" {
		t.Fatalf("unexpected session ID: %q", sc.SessionID)
	}
	if sc.Name != "my session" {
		t.Fatalf("unexpected name: %q", sc.Name)
	}
}

func TestSessionRenamedNotification(t *testing.T) {
	evt := parseEvent("%session-renamed $0 newname")
	n := evt.Notification()
	sr, ok := n.(*SessionRenamedNotification)
	if !ok {
		t.Fatalf("expected *SessionRenamedNotification, got %T", n)
	}
	if sr.SessionID != "$0" || sr.Name != "newname" {
		t.Fatalf("unexpected: %+v", sr)
	}
}

func TestSessionWindowChangedNotification(t *testing.T) {
	evt := parseEvent("%session-window-changed $0 @2")
	n := evt.Notification()
	sw, ok := n.(*SessionWindowChangedNotification)
	if !ok {
		t.Fatalf("expected *SessionWindowChangedNotification, got %T", n)
	}
	if sw.SessionID != "$0" || sw.WindowID != "@2" {
		t.Fatalf("unexpected: %+v", sw)
	}
}

func TestWindowAddNotification(t *testing.T) {
	evt := parseEvent("%window-add @5")
	n := evt.Notification()
	wa, ok := n.(*WindowAddNotification)
	if !ok {
		t.Fatalf("expected *WindowAddNotification, got %T", n)
	}
	if wa.WindowID != "@5" {
		t.Fatalf("unexpected window ID: %q", wa.WindowID)
	}
}

func TestWindowCloseNotification(t *testing.T) {
	evt := parseEvent("%window-close @3")
	n := evt.Notification()
	wc, ok := n.(*WindowCloseNotification)
	if !ok {
		t.Fatalf("expected *WindowCloseNotification, got %T", n)
	}
	if wc.WindowID != "@3" {
		t.Fatalf("unexpected window ID: %q", wc.WindowID)
	}
}

func TestWindowRenamedNotification(t *testing.T) {
	evt := parseEvent("%window-renamed @0 my window")
	n := evt.Notification()
	wr, ok := n.(*WindowRenamedNotification)
	if !ok {
		t.Fatalf("expected *WindowRenamedNotification, got %T", n)
	}
	if wr.WindowID != "@0" || wr.Name != "my window" {
		t.Fatalf("unexpected: %+v", wr)
	}
}

func TestWindowPaneChangedNotification(t *testing.T) {
	evt := parseEvent("%window-pane-changed @0 %1")
	n := evt.Notification()
	wp, ok := n.(*WindowPaneChangedNotification)
	if !ok {
		t.Fatalf("expected *WindowPaneChangedNotification, got %T", n)
	}
	if wp.WindowID != "@0" || wp.PaneID != "%1" {
		t.Fatalf("unexpected: %+v", wp)
	}
}

func TestUnlinkedWindowAddNotification(t *testing.T) {
	evt := parseEvent("%unlinked-window-add @7")
	n := evt.Notification()
	u, ok := n.(*UnlinkedWindowAddNotification)
	if !ok {
		t.Fatalf("expected *UnlinkedWindowAddNotification, got %T", n)
	}
	if u.WindowID != "@7" {
		t.Fatalf("unexpected window ID: %q", u.WindowID)
	}
}

func TestUnlinkedWindowCloseNotification(t *testing.T) {
	evt := parseEvent("%unlinked-window-close @2")
	n := evt.Notification()
	u, ok := n.(*UnlinkedWindowCloseNotification)
	if !ok {
		t.Fatalf("expected *UnlinkedWindowCloseNotification, got %T", n)
	}
	if u.WindowID != "@2" {
		t.Fatalf("unexpected window ID: %q", u.WindowID)
	}
}

func TestUnlinkedWindowRenamedNotification(t *testing.T) {
	evt := parseEvent("%unlinked-window-renamed @4 renamed")
	n := evt.Notification()
	u, ok := n.(*UnlinkedWindowRenamedNotification)
	if !ok {
		t.Fatalf("expected *UnlinkedWindowRenamedNotification, got %T", n)
	}
	if u.WindowID != "@4" || u.Name != "renamed" {
		t.Fatalf("unexpected: %+v", u)
	}
}

func TestPaneModeChangedNotification(t *testing.T) {
	evt := parseEvent("%pane-mode-changed %0")
	n := evt.Notification()
	pm, ok := n.(*PaneModeChangedNotification)
	if !ok {
		t.Fatalf("expected *PaneModeChangedNotification, got %T", n)
	}
	if pm.PaneID != "%0" {
		t.Fatalf("unexpected pane ID: %q", pm.PaneID)
	}
}

func TestClientSessionChangedNotification(t *testing.T) {
	evt := parseEvent("%client-session-changed /dev/pts/1 $0 default")
	n := evt.Notification()
	cs, ok := n.(*ClientSessionChangedNotification)
	if !ok {
		t.Fatalf("expected *ClientSessionChangedNotification, got %T", n)
	}
	if cs.ClientName != "/dev/pts/1" || cs.SessionID != "$0" || cs.SessionName != "default" {
		t.Fatalf("unexpected: %+v", cs)
	}
}

func TestClientDetachedNotification(t *testing.T) {
	evt := parseEvent("%client-detached /dev/pts/2")
	n := evt.Notification()
	cd, ok := n.(*ClientDetachedNotification)
	if !ok {
		t.Fatalf("expected *ClientDetachedNotification, got %T", n)
	}
	if cd.ClientName != "/dev/pts/2" {
		t.Fatalf("unexpected client name: %q", cd.ClientName)
	}
}

func TestSessionsChangedNotification(t *testing.T) {
	evt := parseEvent("%sessions-changed")
	n := evt.Notification()
	_, ok := n.(*SessionsChangedNotification)
	if !ok {
		t.Fatalf("expected *SessionsChangedNotification, got %T", n)
	}
}

func TestPasteBufferChangedNotification(t *testing.T) {
	evt := parseEvent("%paste-buffer-changed buffer0")
	n := evt.Notification()
	pb, ok := n.(*PasteBufferChangedNotification)
	if !ok {
		t.Fatalf("expected *PasteBufferChangedNotification, got %T", n)
	}
	if pb.Name != "buffer0" {
		t.Fatalf("unexpected name: %q", pb.Name)
	}
}

func TestPasteBufferDeletedNotification(t *testing.T) {
	evt := parseEvent("%paste-buffer-deleted buffer1")
	n := evt.Notification()
	pb, ok := n.(*PasteBufferDeletedNotification)
	if !ok {
		t.Fatalf("expected *PasteBufferDeletedNotification, got %T", n)
	}
	if pb.Name != "buffer1" {
		t.Fatalf("unexpected name: %q", pb.Name)
	}
}

func TestPauseNotification(t *testing.T) {
	evt := parseEvent("%pause %3")
	n := evt.Notification()
	p, ok := n.(*PauseNotification)
	if !ok {
		t.Fatalf("expected *PauseNotification, got %T", n)
	}
	if p.PaneID != "%3" {
		t.Fatalf("unexpected pane ID: %q", p.PaneID)
	}
}

func TestContinueNotification(t *testing.T) {
	evt := parseEvent("%continue %3")
	n := evt.Notification()
	c, ok := n.(*ContinueNotification)
	if !ok {
		t.Fatalf("expected *ContinueNotification, got %T", n)
	}
	if c.PaneID != "%3" {
		t.Fatalf("unexpected pane ID: %q", c.PaneID)
	}
}

func TestConfigErrorNotification(t *testing.T) {
	evt := parseEvent("%config-error /home/user/.tmux.conf:5: unknown option")
	n := evt.Notification()
	ce, ok := n.(*ConfigErrorNotification)
	if !ok {
		t.Fatalf("expected *ConfigErrorNotification, got %T", n)
	}
	if ce.Message != "/home/user/.tmux.conf:5: unknown option" {
		t.Fatalf("unexpected message: %q", ce.Message)
	}
}

func TestMessageNotification(t *testing.T) {
	evt := parseEvent("%message hello from display-message")
	n := evt.Notification()
	m, ok := n.(*MessageNotification)
	if !ok {
		t.Fatalf("expected *MessageNotification, got %T", n)
	}
	if m.Message != "hello from display-message" {
		t.Fatalf("unexpected message: %q", m.Message)
	}
}

func TestExitNotification(t *testing.T) {
	evt := parseEvent("%exit server exiting")
	n := evt.Notification()
	ex, ok := n.(*ExitNotification)
	if !ok {
		t.Fatalf("expected *ExitNotification, got %T", n)
	}
	if ex.Reason != "server exiting" {
		t.Fatalf("unexpected reason: %q", ex.Reason)
	}
}

func TestExitNotificationNoReason(t *testing.T) {
	evt := parseEvent("%exit")
	n := evt.Notification()
	ex, ok := n.(*ExitNotification)
	if !ok {
		t.Fatalf("expected *ExitNotification, got %T", n)
	}
	if ex.Reason != "" {
		t.Fatalf("expected empty reason, got %q", ex.Reason)
	}
}

func TestUnknownNotificationReturnsNil(t *testing.T) {
	evt := parseEvent("%some-future-event data")
	n := evt.Notification()
	if n != nil {
		t.Fatalf("expected nil for unknown event, got %T", n)
	}
}

func TestSplitOutputLine(t *testing.T) {
	pane, data, ok := splitOutputLine("%output %5 hello world", "output")
	if !ok {
		t.Fatal("expected ok")
	}
	if pane != "%5" {
		t.Fatalf("unexpected pane: %q", pane)
	}
	if data != "hello world" {
		t.Fatalf("unexpected data: %q", data)
	}
}

func TestSplitOutputLineNoData(t *testing.T) {
	pane, data, ok := splitOutputLine("%output %5", "output")
	if !ok {
		t.Fatal("expected ok")
	}
	if pane != "%5" {
		t.Fatalf("unexpected pane: %q", pane)
	}
	if data != "" {
		t.Fatalf("expected empty data, got %q", data)
	}
}

func TestSplitOutputLineWrongPrefix(t *testing.T) {
	_, _, ok := splitOutputLine("%other %5 data", "output")
	if ok {
		t.Fatal("expected not ok for wrong prefix")
	}
}
