package gotmuxcc

import (
	"strings"
	"testing"
)

func newAutoTransport() *simpleTransport {
	tr := newSimpleTransport()
	go func() {
		for range tr.sendC {
			tr.lines <- "%begin 1 1 0"
			tr.lines <- "%end 1 1 0"
		}
	}()
	return tr
}

func TestWindowCommands(t *testing.T) {
	tr := newAutoTransport()
	tmux := &Tmux{transport: tr}
	tmux.router = newRouter(tr)
	defer tmux.Close()

	if _, err := tmux.ListAllWindows(); err != nil {
		t.Fatalf("ListAllWindows error: %v", err)
	}
	if _, err := tmux.GetWindowById("@1"); err != nil {
		t.Fatalf("GetWindowById error: %v", err)
	}

	session := &Session{Name: "sess", tmux: tmux}
	if _, err := session.ListWindows(); err != nil {
		t.Fatalf("ListWindows error: %v", err)
	}

	window := &Window{Id: "@1", tmux: tmux}
	if err := window.Kill(); err != nil {
		t.Fatalf("Kill error: %v", err)
	}
	if err := window.Rename("new"); err != nil {
		t.Fatalf("Rename error: %v", err)
	}
	if err := window.Select(); err != nil {
		t.Fatalf("Select error: %v", err)
	}
	if err := window.SelectLayout(WindowLayoutEvenHorizontal); err != nil {
		t.Fatalf("SelectLayout error: %v", err)
	}
	if err := window.Move("sess", 1); err != nil {
		t.Fatalf("Move error: %v", err)
	}

	tr.sendMu.Lock()
	joined := strings.Join(tr.sent, "\n")
	tr.sendMu.Unlock()
	if !strings.Contains(joined, "list-windows") || !strings.Contains(joined, "kill-window") || !strings.Contains(joined, "rename-window") {
		t.Fatalf("expected window commands in log: %s", joined)
	}
}

func TestPaneCommands(t *testing.T) {
	tr := newAutoTransport()
	tmux := &Tmux{transport: tr}
	tmux.router = newRouter(tr)
	defer tmux.Close()

	if _, err := tmux.ListAllPanes(); err != nil {
		t.Fatalf("ListAllPanes error: %v", err)
	}
	if _, err := tmux.GetPaneById("%1"); err != nil {
		t.Fatalf("GetPaneById error: %v", err)
	}

	window := &Window{Id: "@1", tmux: tmux}
	if _, err := window.ListPanes(); err != nil {
		t.Fatalf("ListPanes error: %v", err)
	}

	pane := &Pane{Id: "%1", tmux: tmux}
	if err := pane.SendKeys("ls"); err != nil {
		t.Fatalf("SendKeys error: %v", err)
	}
	if err := pane.Kill(); err != nil {
		t.Fatalf("Kill error: %v", err)
	}
	if err := pane.SelectPane(nil); err != nil {
		t.Fatalf("SelectPane error: %v", err)
	}
	if err := pane.SplitWindow(nil); err != nil {
		t.Fatalf("SplitWindow error: %v", err)
	}
	if err := pane.ChooseTree(nil); err != nil {
		t.Fatalf("ChooseTree error: %v", err)
	}
	if _, err := pane.Capture(); err != nil {
		t.Fatalf("Capture error: %v", err)
	}

	tr.sendMu.Lock()
	joined := strings.Join(tr.sent, "\n")
	tr.sendMu.Unlock()
	if !strings.Contains(joined, "send-keys") || !strings.Contains(joined, "split-window") || !strings.Contains(joined, "choose-tree") {
		t.Fatalf("expected pane commands in log: %s", joined)
	}
}
