package gotmuxcc

import (
	"strings"
	"testing"
)

func newTestWindow(ft *fakeTransport, r *router) *Window {
	tmux := &Tmux{transport: ft, router: r}
	return &Window{Id: "@0", tmux: tmux}
}

func TestWindowUnlink(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	win := newTestWindow(ft, r)

	go func() {
		cmd := <-ft.sendC
		if cmd != "unlink-window -k -t @0" {
			t.Errorf("unexpected command: %q", cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "%end 1 1 0"
	}()

	err := win.Unlink()
	if err != nil {
		t.Fatalf("Unlink returned error: %v", err)
	}
}

func TestWindowLink(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	win := newTestWindow(ft, r)

	go func() {
		cmd := <-ft.sendC
		if cmd != "link-window -a -s @0 -t othersess" {
			t.Errorf("unexpected command: %q", cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "%end 1 1 0"
	}()

	err := win.Link("othersess")
	if err != nil {
		t.Fatalf("Link returned error: %v", err)
	}
}

func TestWindowMoveToSession(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	win := newTestWindow(ft, r)

	go func() {
		cmd := <-ft.sendC
		if cmd != "move-window -a -s @0 -t newsess" {
			t.Errorf("unexpected command: %q", cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "%end 1 1 0"
	}()

	err := win.MoveToSession("newsess")
	if err != nil {
		t.Fatalf("MoveToSession returned error: %v", err)
	}
}

func TestWindowSwap(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	win := newTestWindow(ft, r)

	go func() {
		cmd := <-ft.sendC
		if cmd != "swap-window -s @0 -t @1" {
			t.Errorf("unexpected command: %q", cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "%end 1 1 0"
	}()

	err := win.Swap("@1")
	if err != nil {
		t.Fatalf("Swap returned error: %v", err)
	}
}

func TestTmuxUnlinkWindow(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	tmux := &Tmux{transport: ft, router: r}

	go func() {
		cmd := <-ft.sendC
		if cmd != "unlink-window -k -t default:0" {
			t.Errorf("unexpected command: %q", cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "%end 1 1 0"
	}()

	err := tmux.UnlinkWindow("default:0")
	if err != nil {
		t.Fatalf("UnlinkWindow returned error: %v", err)
	}
}

func TestTmuxLinkWindow(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	tmux := &Tmux{transport: ft, router: r}

	go func() {
		cmd := <-ft.sendC
		if cmd != "link-window -a -s @0 -t target" {
			t.Errorf("unexpected command: %q", cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "%end 1 1 0"
	}()

	err := tmux.LinkWindow("@0", "target")
	if err != nil {
		t.Fatalf("LinkWindow returned error: %v", err)
	}
}

func TestTmuxMoveWindowToSession(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	tmux := &Tmux{transport: ft, router: r}

	go func() {
		cmd := <-ft.sendC
		if cmd != "move-window -a -s @0 -t newsess" {
			t.Errorf("unexpected command: %q", cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "%end 1 1 0"
	}()

	err := tmux.MoveWindowToSession("@0", "newsess")
	if err != nil {
		t.Fatalf("MoveWindowToSession returned error: %v", err)
	}
}

func TestTmuxSwapWindows(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	tmux := &Tmux{transport: ft, router: r}

	go func() {
		cmd := <-ft.sendC
		if cmd != "swap-window -s @0 -t @1" {
			t.Errorf("unexpected command: %q", cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "%end 1 1 0"
	}()

	err := tmux.SwapWindows("@0", "@1")
	if err != nil {
		t.Fatalf("SwapWindows returned error: %v", err)
	}
}

func TestTmuxSelectWindow(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	tmux := &Tmux{transport: ft, router: r}

	go func() {
		cmd := <-ft.sendC
		if cmd != "select-window -t default:0" {
			t.Errorf("unexpected command: %q", cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "%end 1 1 0"
	}()

	err := tmux.SelectWindow("default:0")
	if err != nil {
		t.Fatalf("SelectWindow returned error: %v", err)
	}
}

func TestTmuxSelectPane(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	tmux := &Tmux{transport: ft, router: r}

	go func() {
		cmd := <-ft.sendC
		if cmd != "select-pane -t %5" {
			t.Errorf("unexpected command: %q", cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "%end 1 1 0"
	}()

	err := tmux.SelectPane("%5")
	if err != nil {
		t.Fatalf("SelectPane returned error: %v", err)
	}
}

func newTestSession(ft *fakeTransport, r *router) *Session {
	tmux := &Tmux{transport: ft, router: r}
	return &Session{Name: "mysess", tmux: tmux}
}

func TestNewWindowWithIndex(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	sess := &Session{Id: "$1", Name: "mysess", tmux: &Tmux{transport: ft, router: r}}
	idx := 2

	go func() {
		cmd := <-ft.sendC
		if !strings.Contains(cmd, "-t $1:2") {
			t.Errorf("expected target $1:2, got command: %q", cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "%end 1 1 0"
	}()

	_, err := sess.NewWindow(&NewWindowOptions{Index: &idx})
	if err != nil {
		t.Fatalf("NewWindow returned error: %v", err)
	}
}

func TestNewWindowWithIndexZero(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	sess := &Session{Id: "$1", Name: "mysess", tmux: &Tmux{transport: ft, router: r}}
	idx := 0

	go func() {
		cmd := <-ft.sendC
		if !strings.Contains(cmd, "-t $1:0") {
			t.Errorf("expected target $1:0, got command: %q", cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "%end 1 1 0"
	}()

	_, err := sess.NewWindow(&NewWindowOptions{Index: &idx})
	if err != nil {
		t.Fatalf("NewWindow returned error: %v", err)
	}
}

func TestNewWindowWithShellCommand(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	sess := newTestSession(ft, r)

	go func() {
		cmd := <-ft.sendC
		if !strings.Contains(cmd, "'cat /tmp/pane.txt; exec bash'") {
			t.Errorf("expected shell command in output, got: %q", cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "%end 1 1 0"
	}()

	_, err := sess.NewWindow(&NewWindowOptions{
		ShellCommand: "cat /tmp/pane.txt; exec bash",
	})
	if err != nil {
		t.Fatalf("NewWindow returned error: %v", err)
	}
}

func TestNewWindowQuotesShellCommand(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	tmux := &Tmux{transport: ft, router: r}
	session := &Session{Name: "mysess", tmux: tmux}

	go func() {
		cmd := <-ft.sendC
		// shell command with embedded single quotes must be properly escaped
		if !strings.HasSuffix(cmd, "'printf '\\''hi'\\''; exec bash'") {
			t.Errorf("expected properly escaped shell command, got %q", cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "%end 1 1 0"
	}()

	_, err := session.NewWindow(&NewWindowOptions{
		ShellCommand: "printf 'hi'; exec bash",
	})
	if err != nil {
		t.Fatalf("NewWindow returned error: %v", err)
	}
}

func TestSessionWindowNavigationUsesSessionId(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	tmux := &Tmux{transport: ft, router: r}
	session := &Session{Id: "$1", Name: "mysess", tmux: tmux}

	go func() {
		for range ft.sendC {
			ft.lines <- "%begin 1 1 0"
			ft.lines <- "%end 1 1 0"
		}
	}()

	if err := session.NextWindow(); err != nil {
		t.Fatalf("NextWindow returned error: %v", err)
	}
	if err := session.PreviousWindow(); err != nil {
		t.Fatalf("PreviousWindow returned error: %v", err)
	}

	ft.sendMu.Lock()
	sent := append([]string(nil), ft.sent...)
	ft.sendMu.Unlock()
	if len(sent) != 2 {
		t.Fatalf("expected 2 commands, got %d", len(sent))
	}
	if sent[0] != "next-window -t $1" {
		t.Fatalf("unexpected next-window command: %q", sent[0])
	}
	if sent[1] != "previous-window -t $1" {
		t.Fatalf("unexpected previous-window command: %q", sent[1])
	}
}

func TestTmuxSelectLayout(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	tmux := &Tmux{transport: ft, router: r}

	go func() {
		cmd := <-ft.sendC
		if !strings.Contains(cmd, "select-layout") {
			t.Errorf("unexpected command: %s", cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "%end 1 1 0"
	}()

	win := &Window{Id: "@0", tmux: tmux}
	err := win.SelectLayout(WindowLayoutTiled)
	if err != nil {
		t.Fatalf("SelectLayout returned error: %v", err)
	}
}

func TestTmuxSelectLayoutByTarget(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	tmux := &Tmux{transport: ft, router: r}

	go func() {
		cmd := <-ft.sendC
		if !strings.Contains(cmd, "select-layout") {
			t.Errorf("expected select-layout, got: %q", cmd)
		}
		if !strings.Contains(cmd, "-t mysess:2") {
			t.Errorf("expected -t mysess:2, got: %q", cmd)
		}
		if !strings.Contains(cmd, "bb62,80x24,0,0{40x24,0,0,1,39x24,41,0,2}") {
			t.Errorf("expected layout string, got: %q", cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "%end 1 1 0"
	}()

	err := tmux.SelectLayout("mysess:2", "bb62,80x24,0,0{40x24,0,0,1,39x24,41,0,2}")
	if err != nil {
		t.Fatalf("SelectLayout returned error: %v", err)
	}
}
