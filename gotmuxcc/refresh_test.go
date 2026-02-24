package gotmuxcc

import (
	"strings"
	"testing"
)

func TestSetClientSize(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
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
	r := newRouterWithInit(ft, false)
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
	r := newRouterWithInit(ft, false)
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

// --- Flow control tests ---

func TestSetControlFlags(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	tmux := &Tmux{transport: ft, router: r}

	go func() {
		cmd := <-ft.sendC
		if !strings.Contains(cmd, "refresh-client -f") {
			t.Errorf("expected refresh-client -f: %s", cmd)
		}
		if !strings.Contains(cmd, "pause-after=1000,no-output") {
			t.Errorf("expected flags in command: %s", cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "%end 1 1 0"
	}()

	err := tmux.SetControlFlags("pause-after=1000,no-output")
	if err != nil {
		t.Fatalf("SetControlFlags returned error: %v", err)
	}
}

func TestEnablePauseAfter(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	tmux := &Tmux{transport: ft, router: r}

	go func() {
		cmd := <-ft.sendC
		if !strings.Contains(cmd, "pause-after=300") {
			t.Errorf("expected pause-after=300: %s", cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "%end 1 1 0"
	}()

	err := tmux.EnablePauseAfter(300)
	if err != nil {
		t.Fatalf("EnablePauseAfter returned error: %v", err)
	}
}

func TestDisablePauseAfter(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	tmux := &Tmux{transport: ft, router: r}

	go func() {
		cmd := <-ft.sendC
		if !strings.Contains(cmd, "!pause-after") {
			t.Errorf("expected !pause-after: %s", cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "%end 1 1 0"
	}()

	err := tmux.DisablePauseAfter()
	if err != nil {
		t.Fatalf("DisablePauseAfter returned error: %v", err)
	}
}

func TestSetPaneOutput(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	tmux := &Tmux{transport: ft, router: r}

	go func() {
		cmd := <-ft.sendC
		expected := "refresh-client -A %0:on"
		if cmd != expected {
			t.Errorf("expected %q, got %q", expected, cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "%end 1 1 0"
	}()

	err := tmux.SetPaneOutput("%0", "on")
	if err != nil {
		t.Fatalf("SetPaneOutput returned error: %v", err)
	}
}

func TestEnablePaneOutput(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	tmux := &Tmux{transport: ft, router: r}

	go func() {
		cmd := <-ft.sendC
		if cmd != "refresh-client -A %1:on" {
			t.Errorf("unexpected command: %q", cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "%end 1 1 0"
	}()

	err := tmux.EnablePaneOutput("%1")
	if err != nil {
		t.Fatalf("EnablePaneOutput returned error: %v", err)
	}
}

func TestDisablePaneOutput(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	tmux := &Tmux{transport: ft, router: r}

	go func() {
		cmd := <-ft.sendC
		if cmd != "refresh-client -A %2:off" {
			t.Errorf("unexpected command: %q", cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "%end 1 1 0"
	}()

	err := tmux.DisablePaneOutput("%2")
	if err != nil {
		t.Fatalf("DisablePaneOutput returned error: %v", err)
	}
}

func TestPausePaneOutput(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	tmux := &Tmux{transport: ft, router: r}

	go func() {
		cmd := <-ft.sendC
		if cmd != "refresh-client -A %0:pause" {
			t.Errorf("unexpected command: %q", cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "%end 1 1 0"
	}()

	err := tmux.PausePaneOutput("%0")
	if err != nil {
		t.Fatalf("PausePaneOutput returned error: %v", err)
	}
}

func TestContinuePaneOutput(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	tmux := &Tmux{transport: ft, router: r}

	go func() {
		cmd := <-ft.sendC
		if cmd != "refresh-client -A %0:continue" {
			t.Errorf("unexpected command: %q", cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "%end 1 1 0"
	}()

	err := tmux.ContinuePaneOutput("%0")
	if err != nil {
		t.Fatalf("ContinuePaneOutput returned error: %v", err)
	}
}

func TestSetMultiplePaneOutputs(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	tmux := &Tmux{transport: ft, router: r}

	go func() {
		cmd := <-ft.sendC
		if !strings.Contains(cmd, "-A %0:on") || !strings.Contains(cmd, "-A %1:off") {
			t.Errorf("expected multiple -A flags: %s", cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "%end 1 1 0"
	}()

	err := tmux.SetMultiplePaneOutputs([]string{"%0:on", "%1:off"})
	if err != nil {
		t.Fatalf("SetMultiplePaneOutputs returned error: %v", err)
	}
}

func TestSetMultiplePaneOutputsEmpty(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	tmux := &Tmux{transport: ft, router: r}

	// Should be a no-op
	err := tmux.SetMultiplePaneOutputs(nil)
	if err != nil {
		t.Fatalf("SetMultiplePaneOutputs returned error: %v", err)
	}
}

// --- Pane color report tests ---

func TestReportPaneColors(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	tmux := &Tmux{transport: ft, router: r}

	go func() {
		cmd := <-ft.sendC
		// The report contains special chars, so it may be quoted
		if !strings.Contains(cmd, "refresh-client -r") {
			t.Errorf("expected refresh-client -r: %s", cmd)
		}
		if !strings.Contains(cmd, "%0:") {
			t.Errorf("expected pane ID in command: %s", cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "%end 1 1 0"
	}()

	err := tmux.ReportPaneColors("%0", "rgb:ff/00/00")
	if err != nil {
		t.Fatalf("ReportPaneColors returned error: %v", err)
	}
}

func TestReportPaneColorsSimple(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	tmux := &Tmux{transport: ft, router: r}

	go func() {
		cmd := <-ft.sendC
		expected := "refresh-client -r %5:STD1234567"
		if cmd != expected {
			t.Errorf("expected %q, got %q", expected, cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "%end 1 1 0"
	}()

	err := tmux.ReportPaneColors("%5", "STD1234567")
	if err != nil {
		t.Fatalf("ReportPaneColors returned error: %v", err)
	}
}
