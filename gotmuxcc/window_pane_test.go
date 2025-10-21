package gotmuxcc

import (
	"fmt"
	"strings"
	"sync"
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

type scriptedResponse struct {
	match string
	lines []string
}

type scriptedTransport struct {
	responses []scriptedResponse
	index     int

	lines chan string
	done  chan error

	mu   sync.Mutex
	sent []string

	closeOnce sync.Once
}

func newScriptedTransport(responses []scriptedResponse) *scriptedTransport {
	return &scriptedTransport{
		responses: responses,
		lines:     make(chan string, 64),
		done:      make(chan error, 1),
	}
}

func (s *scriptedTransport) Send(cmd string) error {
	s.mu.Lock()
	s.sent = append(s.sent, cmd)
	if s.index >= len(s.responses) {
		s.mu.Unlock()
		return fmt.Errorf("unexpected command: %s", cmd)
	}
	resp := s.responses[s.index]
	s.index++
	s.mu.Unlock()

	if resp.match != "" && !strings.Contains(cmd, resp.match) {
		return fmt.Errorf("unexpected command %q (expected %q)", cmd, resp.match)
	}

	go func(lines []string) {
		for _, line := range lines {
			s.lines <- line
		}
	}(append([]string(nil), resp.lines...))

	return nil
}

func (s *scriptedTransport) Lines() <-chan string { return s.lines }

func (s *scriptedTransport) Done() <-chan error { return s.done }

func (s *scriptedTransport) Close() error {
	s.closeOnce.Do(func() {
		close(s.lines)
		close(s.done)
	})
	return nil
}

func formatRecord(vars []string, overrides map[string]string) string {
	values := make([]string, len(vars))
	for i, key := range vars {
		if val, ok := overrides[key]; ok {
			values[i] = val
		}
	}
	return strings.Join(values, querySeparator)
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

func TestListAllWindowsFallback(t *testing.T) {
	sessionVars := func() []string {
		q := newQuery(nil)
		q.sessionVars()
		return append([]string(nil), q.variables...)
	}()
	windowVars := func() []string {
		q := newQuery(nil)
		q.windowVars()
		return append([]string(nil), q.variables...)
	}()

	responses := []scriptedResponse{
		{match: "list-windows -a", lines: []string{"%begin 1 1 0", "%end 1 1 0"}},
		{match: "list-sessions", lines: []string{
			"%begin 1 1 0",
			formatRecord(sessionVars, map[string]string{
				varSessionName:    "popup",
				varSessionId:      "$1",
				varSessionWindows: "1",
			}),
			"%end 1 1 0",
		}},
		{match: "list-windows -t popup", lines: []string{
			"%begin 1 1 0",
			formatRecord(windowVars, map[string]string{
				varWindowId:     "@1",
				varWindowName:   "popup",
				varWindowIndex:  "0",
				varWindowPanes:  "1",
				varWindowActive: "1",
			}),
			"%end 1 1 0",
		}},
	}

	tr := newScriptedTransport(responses)
	tmux := &Tmux{transport: tr}
	tmux.router = newRouter(tr)
	defer tmux.Close()

	windows, err := tmux.ListAllWindows()
	if err != nil {
		t.Fatalf("ListAllWindows returned error: %v", err)
	}
	if len(windows) != 1 || windows[0].Name != "popup" || windows[0].Id != "@1" {
		t.Fatalf("unexpected windows result: %#v", windows)
	}

	tr.mu.Lock()
	sent := append([]string(nil), tr.sent...)
	tr.mu.Unlock()
	if len(sent) != len(responses) {
		t.Fatalf("expected %d commands, saw %d (%v)", len(responses), len(sent), sent)
	}
	if !strings.Contains(sent[1], "list-sessions") || !strings.Contains(sent[2], "list-windows -t popup") {
		t.Fatalf("fallback commands were not issued as expected: %v", sent)
	}
}

func TestListAllPanesFallback(t *testing.T) {
	sessionVars := func() []string {
		q := newQuery(nil)
		q.sessionVars()
		return append([]string(nil), q.variables...)
	}()
	windowVars := func() []string {
		q := newQuery(nil)
		q.windowVars()
		return append([]string(nil), q.variables...)
	}()
	paneVars := func() []string {
		q := newQuery(nil)
		q.paneVars()
		return append([]string(nil), q.variables...)
	}()

	responses := []scriptedResponse{
		{match: "list-panes -a", lines: []string{"%begin 1 1 0", "%end 1 1 0"}},
		{match: "list-windows -a", lines: []string{"%begin 1 1 0", "%end 1 1 0"}},
		{match: "list-sessions", lines: []string{
			"%begin 1 1 0",
			formatRecord(sessionVars, map[string]string{
				varSessionName:    "popup",
				varSessionId:      "$1",
				varSessionWindows: "1",
			}),
			"%end 1 1 0",
		}},
		{match: "list-windows -t popup", lines: []string{
			"%begin 1 1 0",
			formatRecord(windowVars, map[string]string{
				varWindowId:    "@1",
				varWindowName:  "popup",
				varWindowIndex: "0",
				varWindowPanes: "1",
			}),
			"%end 1 1 0",
		}},
		{match: "list-panes -t @1", lines: []string{
			"%begin 1 1 0",
			formatRecord(paneVars, map[string]string{
				varPaneId:             "%1",
				varPaneIndex:          "0",
				varPaneWindowIndex:    "0",
				varPaneCurrentCommand: "vim",
				varPaneWidth:          "120",
				varPaneHeight:         "30",
			}),
			"%end 1 1 0",
		}},
	}

	tr := newScriptedTransport(responses)
	tmux := &Tmux{transport: tr}
	tmux.router = newRouter(tr)
	defer tmux.Close()

	panes, err := tmux.ListAllPanes()
	if err != nil {
		t.Fatalf("ListAllPanes returned error: %v", err)
	}
	if len(panes) != 1 || panes[0].Id != "%1" {
		t.Fatalf("unexpected panes result: %#v", panes)
	}

	tr.mu.Lock()
	sent := append([]string(nil), tr.sent...)
	tr.mu.Unlock()
	if len(sent) != len(responses) {
		t.Fatalf("expected %d commands, saw %d (%v)", len(responses), len(sent), sent)
	}
	if !strings.Contains(sent[len(sent)-1], "list-panes -t @1") {
		t.Fatalf("fallback panes command missing: %v", sent)
	}
}
