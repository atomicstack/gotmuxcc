package gotmuxcc

import (
	"strings"
	"testing"
)

func TestDisplayMessage(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	tmux := &Tmux{transport: ft, router: r}

	go func() {
		cmd := <-ft.sendC
		if !strings.Contains(cmd, "display-message") {
			t.Errorf("unexpected command: %s", cmd)
		}
		if !strings.Contains(cmd, "-p") {
			t.Errorf("expected -p flag: %s", cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "myvalue"
		ft.lines <- "%end 1 1 0"
	}()

	result, err := tmux.DisplayMessage("", "#{client_name}")
	if err != nil {
		t.Fatalf("DisplayMessage returned error: %v", err)
	}
	if result != "myvalue" {
		t.Fatalf("unexpected result: %q", result)
	}
}

func TestDisplayMessageWithTarget(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	tmux := &Tmux{transport: ft, router: r}

	go func() {
		cmd := <-ft.sendC
		if !strings.Contains(cmd, "-t %0") {
			t.Errorf("expected target flag: %s", cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "session1"
		ft.lines <- "%end 1 1 0"
	}()

	result, err := tmux.DisplayMessage("%0", "#{session_name}")
	if err != nil {
		t.Fatalf("DisplayMessage returned error: %v", err)
	}
	if result != "session1" {
		t.Fatalf("unexpected result: %q", result)
	}
}

func TestListSessionsFormat(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	tmux := &Tmux{transport: ft, router: r}

	go func() {
		cmd := <-ft.sendC
		if !strings.Contains(cmd, "list-sessions") || !strings.Contains(cmd, "-F") {
			t.Errorf("unexpected command: %s", cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "session1\t2\t1"
		ft.lines <- "session2\t3\t0"
		ft.lines <- "%end 1 1 0"
	}()

	lines, err := tmux.ListSessionsFormat("#{session_name}\t#{session_windows}\t#{session_attached}")
	if err != nil {
		t.Fatalf("ListSessionsFormat returned error: %v", err)
	}
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
}

func TestListSessionsFormatQuotesWhitespace(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	tmux := &Tmux{transport: ft, router: r}

	go func() {
		cmd := <-ft.sendC
		// Format contains a tab, so it must be single-quoted
		if !strings.Contains(cmd, "'#{session_name}\t#{session_windows}'") {
			t.Errorf("format with whitespace should be quoted: %s", cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "%end 1 1 0"
	}()

	_, err := tmux.ListSessionsFormat("#{session_name}\t#{session_windows}")
	if err != nil {
		t.Fatalf("ListSessionsFormat returned error: %v", err)
	}
}

func TestDisplayMessageQuotesFormat(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	tmux := &Tmux{transport: ft, router: r}

	go func() {
		cmd := <-ft.sendC
		// Format with space must be quoted
		if !strings.Contains(cmd, "'#S: #{session_windows} windows'") {
			t.Errorf("format with space should be quoted: %s", cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "main: 3 windows"
		ft.lines <- "%end 1 1 0"
	}()

	_, err := tmux.DisplayMessage("", "#S: #{session_windows} windows")
	if err != nil {
		t.Fatalf("DisplayMessage returned error: %v", err)
	}
}

func TestListWindowsFormatQuotesFilter(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	tmux := &Tmux{transport: ft, router: r}

	go func() {
		cmd := <-ft.sendC
		// Filter with spaces must be quoted
		if !strings.Contains(cmd, "-f '#{window_name} == bash'") {
			t.Errorf("filter with spaces should be quoted: %s", cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "%end 1 1 0"
	}()

	_, err := tmux.ListWindowsFormat("", "#{window_name} == bash", "#{window_id}")
	if err != nil {
		t.Fatalf("ListWindowsFormat returned error: %v", err)
	}
}

func TestListWindowsFormatAll(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	tmux := &Tmux{transport: ft, router: r}

	go func() {
		cmd := <-ft.sendC
		if !strings.Contains(cmd, "list-windows") || !strings.Contains(cmd, "-a") {
			t.Errorf("expected -a flag: %s", cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "@0\tdefault:0\tbash"
		ft.lines <- "%end 1 1 0"
	}()

	lines, err := tmux.ListWindowsFormat("", "", "#{window_id}\t#{session_name}:#{window_index}\t#{window_name}")
	if err != nil {
		t.Fatalf("ListWindowsFormat returned error: %v", err)
	}
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
}

func TestListWindowsFormatWithTarget(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	tmux := &Tmux{transport: ft, router: r}

	go func() {
		cmd := <-ft.sendC
		if !strings.Contains(cmd, "-t mysession") {
			t.Errorf("expected -t flag: %s", cmd)
		}
		if strings.Contains(cmd, "-a") {
			t.Errorf("should not have -a with target: %s", cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "window1"
		ft.lines <- "%end 1 1 0"
	}()

	lines, err := tmux.ListWindowsFormat("mysession", "", "#{window_name}")
	if err != nil {
		t.Fatalf("ListWindowsFormat returned error: %v", err)
	}
	if len(lines) != 1 || lines[0] != "window1" {
		t.Fatalf("unexpected lines: %v", lines)
	}
}

func TestListWindowsFormatWithFilter(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	tmux := &Tmux{transport: ft, router: r}

	go func() {
		cmd := <-ft.sendC
		if !strings.Contains(cmd, "-f") {
			t.Errorf("expected -f flag: %s", cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "%end 1 1 0"
	}()

	_, err := tmux.ListWindowsFormat("", "#{window_name}==bash", "#{window_id}")
	if err != nil {
		t.Fatalf("ListWindowsFormat returned error: %v", err)
	}
}

func TestListPanesFormatAll(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	tmux := &Tmux{transport: ft, router: r}

	go func() {
		cmd := <-ft.sendC
		if !strings.Contains(cmd, "list-panes") || !strings.Contains(cmd, "-a") {
			t.Errorf("expected list-panes -a: %s", cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "pane0\tdefault:0.0"
		ft.lines <- "%end 1 1 0"
	}()

	lines, err := tmux.ListPanesFormat("", "", "#{pane_index}\t#S:#{window_index}.#{pane_index}")
	if err != nil {
		t.Fatalf("ListPanesFormat returned error: %v", err)
	}
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
}
