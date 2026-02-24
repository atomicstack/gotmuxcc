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
