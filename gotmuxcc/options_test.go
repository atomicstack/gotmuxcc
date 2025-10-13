package gotmuxcc

import (
	"strings"
	"sync"
	"testing"
)

type recordTransport struct {
	sendMu sync.Mutex
	sent   []string

	lines     chan string
	done      chan error
	sendC     chan string
	closeOnce sync.Once
}

func newRecordTransport() *recordTransport {
	return &recordTransport{
		lines: make(chan string, 32),
		done:  make(chan error, 1),
		sendC: make(chan string, 1),
	}
}

func (r *recordTransport) Send(cmd string) error {
	r.sendMu.Lock()
	r.sent = append(r.sent, cmd)
	r.sendMu.Unlock()
	select {
	case r.sendC <- cmd:
	default:
	}
	return nil
}

func (r *recordTransport) Lines() <-chan string {
	return r.lines
}

func (r *recordTransport) Done() <-chan error {
	return r.done
}

func (r *recordTransport) Close() error {
	r.closeOnce.Do(func() {
		close(r.lines)
		select {
		case r.done <- nil:
		default:
		}
		close(r.done)
	})
	return nil
}

func (r *recordTransport) respond(lines ...string) {
	for _, line := range lines {
		r.lines <- line
	}
}

func TestSetOptionCommand(t *testing.T) {
	rt := newRecordTransport()
	tmux := &Tmux{transport: rt}
	tmux.router = newRouter(rt)
	defer tmux.Close()

	go func() {
		for range rt.sendC {
			rt.respond("%begin 1 1 0", "%end 1 1 0")
		}
	}()

	if err := tmux.SetOption("foo", "bar", "baz", ""); err != nil {
		t.Fatalf("SetOption returned error: %v", err)
	}

	if len(rt.sent) == 0 {
		t.Fatalf("expected command to be sent")
	}
	cmd := strings.Join(rt.sent, "\n")
	if !strings.Contains(cmd, "set-option") || !strings.Contains(cmd, "-t foo") || !strings.Contains(cmd, "bar baz") {
		t.Fatalf("unexpected command: %q", cmd)
	}
}

func TestDeleteOptionCommand(t *testing.T) {
	rt := newRecordTransport()
	tmux := &Tmux{transport: rt}
	tmux.router = newRouter(rt)
	defer tmux.Close()

	go func() {
		for range rt.sendC {
			rt.respond("%begin 1 1 0", "%end 1 1 0")
		}
	}()

	if err := tmux.DeleteOption("target", "myoption", "-g"); err != nil {
		t.Fatalf("DeleteOption returned error: %v", err)
	}

	rt.sendMu.Lock()
	defer rt.sendMu.Unlock()
	cmd := strings.Join(rt.sent, "\n")
	if !strings.Contains(cmd, "set-option") ||
		!strings.Contains(cmd, "-g") ||
		!strings.Contains(cmd, "-t target") ||
		!strings.Contains(cmd, "-u myoption") {
		t.Fatalf("unexpected delete command: %q", cmd)
	}
}

func TestDeleteOptionErrorPropagation(t *testing.T) {
	rt := newRecordTransport()
	tmux := &Tmux{transport: rt}
	tmux.router = newRouter(rt)
	defer tmux.Close()

	go func() {
		for cmd := range rt.sendC {
			_ = cmd
			rt.respond("%begin 1 1 0", "%error 1 1 0 failure")
		}
	}()

	if err := tmux.DeleteOption("target", "bad", ""); err == nil || !strings.Contains(err.Error(), "failed to delete option") {
		t.Fatalf("expected wrapped delete error, got %v", err)
	}
}

func TestOptionRetrieval(t *testing.T) {
	rt := newRecordTransport()
	tmux := &Tmux{transport: rt}
	tmux.router = newRouter(rt)
	defer tmux.Close()

	go func() {
		for cmd := range rt.sendC {
			if strings.Contains(cmd, "show-option") {
				rt.respond("%begin 1 1 0", "value", "%end 1 1 0")
			}
		}
	}()

	opt, err := tmux.Option("target", "@foo", "-g")
	if err != nil {
		t.Fatalf("Option returned error: %v", err)
	}
	if opt.Key != "@foo" || opt.Value != "value" {
		t.Fatalf("unexpected option result: %#v", opt)
	}
}

func TestOptionErrorPropagation(t *testing.T) {
	rt := newRecordTransport()
	tmux := &Tmux{transport: rt}
	tmux.router = newRouter(rt)
	defer tmux.Close()

	go func() {
		for range rt.sendC {
			rt.respond("%begin 1 1 0", "%error 1 1 0 missing")
		}
	}()

	if _, err := tmux.Option("target", "foo", ""); err == nil || !strings.Contains(err.Error(), "failed to retrieve option") {
		t.Fatalf("expected wrapped option error, got %v", err)
	}
}

func TestOptionsRetrieval(t *testing.T) {
	rt := newRecordTransport()
	tmux := &Tmux{transport: rt}
	tmux.router = newRouter(rt)
	defer tmux.Close()

	go func() {
		for range rt.sendC {
			rt.respond("%begin 1 1 0",
				"@foo value",
				"@bar other",
				"%end 1 1 0")
		}
	}()

	opts, err := tmux.Options("target", "")
	if err != nil {
		t.Fatalf("Options returned error: %v", err)
	}
	if len(opts) != 2 {
		t.Fatalf("expected two options, got %d", len(opts))
	}
	if opts[0].Key != "@foo" || opts[0].Value != "value" {
		t.Fatalf("unexpected first option: %#v", opts[0])
	}
	if opts[1].Key != "@bar" || opts[1].Value != "other" {
		t.Fatalf("unexpected second option: %#v", opts[1])
	}
}

func TestCommandMultiLineOutput(t *testing.T) {
	rt := newRecordTransport()
	tmux := &Tmux{transport: rt}
	tmux.router = newRouter(rt)
	defer tmux.Close()

	go func() {
		for range rt.sendC {
			rt.respond("%begin 1 1 0", "line1", "line2", "%end 1 1 0")
		}
	}()

	out, err := tmux.Command("display-message", "hello world")
	if err != nil {
		t.Fatalf("Command returned error: %v", err)
	}
	if out != "line1\nline2" {
		t.Fatalf("unexpected command output: %q", out)
	}
}

func TestCommandErrorPropagation(t *testing.T) {
	rt := newRecordTransport()
	tmux := &Tmux{transport: rt}
	tmux.router = newRouter(rt)
	defer tmux.Close()

	go func() {
		for range rt.sendC {
			rt.respond("%begin 1 1 0", "%error 1 1 0 bad")
		}
	}()

	if _, err := tmux.Command("list-panes"); err == nil || !strings.Contains(err.Error(), "failed to run command") {
		t.Fatalf("expected wrapped command error, got %v", err)
	}
}
