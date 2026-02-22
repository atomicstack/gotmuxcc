package gotmuxcc

import (
	"strings"
	"testing"
)

func TestSubscribeBuildCommand(t *testing.T) {
	ft := newFakeTransport()
	r := newRouter(ft)
	defer r.close()

	tmux := &Tmux{transport: ft, router: r}

	go func() {
		cmd := <-ft.sendC
		if !strings.HasPrefix(cmd, "refresh-client -B ") {
			t.Errorf("unexpected command: %s", cmd)
		}
		// Verify the argument contains name:target:format
		arg := strings.TrimPrefix(cmd, "refresh-client -B ")
		if !strings.Contains(arg, "mytest") || !strings.Contains(arg, "window_name") {
			t.Errorf("unexpected argument: %s", arg)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "%end 1 1 0"
	}()

	err := tmux.Subscribe("mytest", SubWindow("@0"), "#{window_name}")
	if err != nil {
		t.Fatalf("Subscribe returned error: %v", err)
	}
}

func TestSubscribeSessionTarget(t *testing.T) {
	ft := newFakeTransport()
	r := newRouter(ft)
	defer r.close()

	tmux := &Tmux{transport: ft, router: r}

	go func() {
		cmd := <-ft.sendC
		// Session-level subscription: name::format (empty target)
		if !strings.Contains(cmd, "mytest::") {
			t.Errorf("expected session target (empty), got: %s", cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "%end 1 1 0"
	}()

	err := tmux.Subscribe("mytest", SubSession, "#{session_name}")
	if err != nil {
		t.Fatalf("Subscribe returned error: %v", err)
	}
}

func TestSubscribeAllPanes(t *testing.T) {
	ft := newFakeTransport()
	r := newRouter(ft)
	defer r.close()

	tmux := &Tmux{transport: ft, router: r}

	go func() {
		cmd := <-ft.sendC
		if !strings.Contains(cmd, "mytest:%*:") {
			t.Errorf("expected all-panes target, got: %s", cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "%end 1 1 0"
	}()

	err := tmux.Subscribe("mytest", SubAllPanes, "#{pane_title}")
	if err != nil {
		t.Fatalf("Subscribe returned error: %v", err)
	}
}

func TestUnsubscribe(t *testing.T) {
	ft := newFakeTransport()
	r := newRouter(ft)
	defer r.close()

	tmux := &Tmux{transport: ft, router: r}

	go func() {
		cmd := <-ft.sendC
		// Unsubscribe sends just the name, no colons
		expected := "refresh-client -B mytest"
		if cmd != expected {
			t.Errorf("expected %q, got %q", expected, cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "%end 1 1 0"
	}()

	err := tmux.Unsubscribe("mytest")
	if err != nil {
		t.Fatalf("Unsubscribe returned error: %v", err)
	}
}

func TestSubscribePaneTarget(t *testing.T) {
	ft := newFakeTransport()
	r := newRouter(ft)
	defer r.close()

	tmux := &Tmux{transport: ft, router: r}

	go func() {
		cmd := <-ft.sendC
		if !strings.Contains(cmd, "title:%5:") {
			t.Errorf("expected pane target %%5, got: %s", cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "%end 1 1 0"
	}()

	err := tmux.Subscribe("title", SubPane("%5"), "#{pane_title}")
	if err != nil {
		t.Fatalf("Subscribe returned error: %v", err)
	}
}
