package gotmuxcc

import (
	"strings"
	"testing"
)

func newTestPane(ft *fakeTransport, r *router) *Pane {
	tmux := &Tmux{transport: ft, router: r}
	return &Pane{Id: "%0", tmux: tmux}
}

func TestPaneRename(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	pane := newTestPane(ft, r)

	go func() {
		cmd := <-ft.sendC
		if !strings.Contains(cmd, "select-pane") || !strings.Contains(cmd, "-T") || !strings.Contains(cmd, "newtitle") {
			t.Errorf("unexpected command: %s", cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "%end 1 1 0"
	}()

	err := pane.Rename("newtitle")
	if err != nil {
		t.Fatalf("Rename returned error: %v", err)
	}
}

func TestPaneSwap(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	pane := newTestPane(ft, r)

	go func() {
		cmd := <-ft.sendC
		if cmd != "swap-pane -s %0 -t %1" {
			t.Errorf("unexpected command: %q", cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "%end 1 1 0"
	}()

	err := pane.Swap("%1")
	if err != nil {
		t.Fatalf("Swap returned error: %v", err)
	}
}

func TestPaneMove(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	pane := newTestPane(ft, r)

	go func() {
		cmd := <-ft.sendC
		if cmd != "move-pane -s %0 -t @1" {
			t.Errorf("unexpected command: %q", cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "%end 1 1 0"
	}()

	err := pane.Move("@1")
	if err != nil {
		t.Fatalf("Move returned error: %v", err)
	}
}

func TestPaneMoveNoTarget(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	pane := newTestPane(ft, r)

	go func() {
		cmd := <-ft.sendC
		if cmd != "move-pane -s %0" {
			t.Errorf("unexpected command: %q", cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "%end 1 1 0"
	}()

	err := pane.Move("")
	if err != nil {
		t.Fatalf("Move returned error: %v", err)
	}
}

func TestPaneBreak(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	pane := newTestPane(ft, r)

	go func() {
		cmd := <-ft.sendC
		if cmd != "break-pane -s %0 -t mysess:1" {
			t.Errorf("unexpected command: %q", cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "%end 1 1 0"
	}()

	err := pane.Break("mysess:1")
	if err != nil {
		t.Fatalf("Break returned error: %v", err)
	}
}

func TestPaneJoin(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	pane := newTestPane(ft, r)

	go func() {
		cmd := <-ft.sendC
		if cmd != "join-pane -s %0 -t @2" {
			t.Errorf("unexpected command: %q", cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "%end 1 1 0"
	}()

	err := pane.Join("@2")
	if err != nil {
		t.Fatalf("Join returned error: %v", err)
	}
}

func TestPaneResize(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	pane := newTestPane(ft, r)

	go func() {
		cmd := <-ft.sendC
		if cmd != "resize-pane -t %0 -L 5" {
			t.Errorf("unexpected command: %q", cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "%end 1 1 0"
	}()

	err := pane.Resize(ResizeLeft, 5)
	if err != nil {
		t.Fatalf("Resize returned error: %v", err)
	}
}

func TestCaptureOptionsStartEndLine(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	tmux := &Tmux{transport: ft, router: r}

	go func() {
		cmd := <-ft.sendC
		if !strings.Contains(cmd, "-S -40") || !strings.Contains(cmd, "-E -1") {
			t.Errorf("expected -S and -E flags: %s", cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "line1"
		ft.lines <- "%end 1 1 0"
	}()

	_, err := tmux.CapturePane("%0", &CaptureOptions{
		EscTxtNBgAttr: true,
		StartLine:     "-40",
		EndLine:       "-1",
	})
	if err != nil {
		t.Fatalf("CapturePane returned error: %v", err)
	}
}

func TestTmuxRenamePane(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	tmux := &Tmux{transport: ft, router: r}

	go func() {
		cmd := <-ft.sendC
		if !strings.Contains(cmd, "select-pane") || !strings.Contains(cmd, "-t %5") || !strings.Contains(cmd, "-T mytitle") {
			t.Errorf("unexpected command: %s", cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "%end 1 1 0"
	}()

	err := tmux.RenamePane("%5", "mytitle")
	if err != nil {
		t.Fatalf("RenamePane returned error: %v", err)
	}
}

func TestTmuxSwapPanes(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	tmux := &Tmux{transport: ft, router: r}

	go func() {
		cmd := <-ft.sendC
		if cmd != "swap-pane -s %0 -t %1" {
			t.Errorf("unexpected command: %q", cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "%end 1 1 0"
	}()

	err := tmux.SwapPanes("%0", "%1")
	if err != nil {
		t.Fatalf("SwapPanes returned error: %v", err)
	}
}

func TestTmuxResizePane(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	tmux := &Tmux{transport: ft, router: r}

	go func() {
		cmd := <-ft.sendC
		if cmd != "resize-pane -t %0 -D 10" {
			t.Errorf("unexpected command: %q", cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "%end 1 1 0"
	}()

	err := tmux.ResizePane("%0", ResizeDown, 10)
	if err != nil {
		t.Fatalf("ResizePane returned error: %v", err)
	}
}

func TestPaneSplitWindowDetached(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	pane := newTestPane(ft, r)

	go func() {
		cmd := <-ft.sendC
		if !strings.Contains(cmd, "-d") {
			t.Errorf("expected -d flag, got: %q", cmd)
		}
		if !strings.Contains(cmd, "-t %0") {
			t.Errorf("expected -t %%0, got: %q", cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "%end 1 1 0"
	}()

	err := pane.SplitWindow(&SplitWindowOptions{Detached: true})
	if err != nil {
		t.Fatalf("SplitWindow returned error: %v", err)
	}
}

func TestTmuxSplitWindow(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	tmux := &Tmux{transport: ft, router: r}

	go func() {
		cmd := <-ft.sendC
		if !strings.Contains(cmd, "split-window") {
			t.Errorf("expected split-window command, got: %q", cmd)
		}
		if !strings.Contains(cmd, "-t mysess:2") {
			t.Errorf("expected -t mysess:2, got: %q", cmd)
		}
		if !strings.Contains(cmd, "-d") {
			t.Errorf("expected -d flag, got: %q", cmd)
		}
		if !strings.Contains(cmd, "-h") {
			t.Errorf("expected -h flag, got: %q", cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "%end 1 1 0"
	}()

	err := tmux.SplitWindow("mysess:2", &SplitWindowOptions{
		SplitDirection: PaneSplitDirectionHorizontal,
		Detached:       true,
	})
	if err != nil {
		t.Fatalf("SplitWindow returned error: %v", err)
	}
}

func TestTmuxSplitWindowWithCommand(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	tmux := &Tmux{transport: ft, router: r}

	go func() {
		cmd := <-ft.sendC
		if !strings.Contains(cmd, "-t mysess:2") {
			t.Errorf("expected -t mysess:2, got: %q", cmd)
		}
		if !strings.Contains(cmd, "'cat /tmp/pane.txt; exec bash'") {
			t.Errorf("expected shell command, got: %q", cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "%end 1 1 0"
	}()

	err := tmux.SplitWindow("mysess:2", &SplitWindowOptions{
		ShellCommand: "cat /tmp/pane.txt; exec bash",
	})
	if err != nil {
		t.Fatalf("SplitWindow returned error: %v", err)
	}
}

func TestTmuxSplitWindowQuotesShellCommand(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	tmux := &Tmux{transport: ft, router: r}

	go func() {
		cmd := <-ft.sendC
		expected := "split-window -t mysess:2 'printf '\\''hi'\\''; exec bash'"
		if cmd != expected {
			t.Errorf("expected %q, got %q", expected, cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "%end 1 1 0"
	}()

	err := tmux.SplitWindow("mysess:2", &SplitWindowOptions{
		ShellCommand: "printf 'hi'; exec bash",
	})
	if err != nil {
		t.Fatalf("SplitWindow returned error: %v", err)
	}
}

func TestPaneSendKeysQuotesLine(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	pane := newTestPane(ft, r)

	go func() {
		cmd := <-ft.sendC
		expected := "send-keys -t %0 'printf '\\''hi'\\''; kill-session'"
		if cmd != expected {
			t.Errorf("expected %q, got %q", expected, cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "%end 1 1 0"
	}()

	err := pane.SendKeys("printf 'hi'; kill-session")
	if err != nil {
		t.Fatalf("SendKeys returned error: %v", err)
	}
}

func TestTmuxSplitWindowNilOpts(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	tmux := &Tmux{transport: ft, router: r}

	go func() {
		cmd := <-ft.sendC
		if cmd != "split-window -t mysess:2" {
			t.Errorf("unexpected command: %q", cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "%end 1 1 0"
	}()

	err := tmux.SplitWindow("mysess:2", nil)
	if err != nil {
		t.Fatalf("SplitWindow returned error: %v", err)
	}
}
