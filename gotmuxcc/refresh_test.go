package gotmuxcc

import "testing"

func TestSetClientSize(t *testing.T) {
	ft := newFakeTransport()
	r := newRouter(ft)
	defer r.close()

	tmux := &Tmux{transport: ft, router: r}

	go func() {
		cmd := <-ft.sendC
		expected := "refresh-client -C 120x40"
		if cmd != expected {
			t.Errorf("expected %q, got %q", expected, cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "%end 1 1 0"
	}()

	err := tmux.SetClientSize(120, 40)
	if err != nil {
		t.Fatalf("SetClientSize returned error: %v", err)
	}
}

func TestSetWindowSize(t *testing.T) {
	ft := newFakeTransport()
	r := newRouter(ft)
	defer r.close()

	tmux := &Tmux{transport: ft, router: r}

	go func() {
		cmd := <-ft.sendC
		expected := "refresh-client -C @0:80x24"
		if cmd != expected {
			t.Errorf("expected %q, got %q", expected, cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "%end 1 1 0"
	}()

	err := tmux.SetWindowSize("@0", 80, 24)
	if err != nil {
		t.Fatalf("SetWindowSize returned error: %v", err)
	}
}

func TestClearWindowSize(t *testing.T) {
	ft := newFakeTransport()
	r := newRouter(ft)
	defer r.close()

	tmux := &Tmux{transport: ft, router: r}

	go func() {
		cmd := <-ft.sendC
		expected := "refresh-client -C @1:"
		if cmd != expected {
			t.Errorf("expected %q, got %q", expected, cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "%end 1 1 0"
	}()

	err := tmux.ClearWindowSize("@1")
	if err != nil {
		t.Fatalf("ClearWindowSize returned error: %v", err)
	}
}
