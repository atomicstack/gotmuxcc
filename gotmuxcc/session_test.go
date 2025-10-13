package gotmuxcc

import (
	"strings"
	"sync"
	"testing"
)

type simpleTransport struct {
	sendMu sync.Mutex
	sent   []string

	lines chan string
	done  chan error
	sendC chan string
}

func newSimpleTransport() *simpleTransport {
	return &simpleTransport{
		lines: make(chan string, 32),
		done:  make(chan error, 1),
		sendC: make(chan string, 1),
	}
}

func (s *simpleTransport) Send(cmd string) error {
	s.sendMu.Lock()
	s.sent = append(s.sent, cmd)
	s.sendMu.Unlock()
	select {
	case s.sendC <- cmd:
	default:
	}
	return nil
}

func (s *simpleTransport) Lines() <-chan string { return s.lines }
func (s *simpleTransport) Done() <-chan error   { return s.done }
func (s *simpleTransport) Close() error {
	close(s.lines)
	close(s.done)
	return nil
}

func TestSessionListAndGet(t *testing.T) {
	tr := newSimpleTransport()
	tmux := &Tmux{transport: tr}
	tmux.router = newRouter(tr)
	defer tmux.Close()

	go func() {
		for range tr.sendC {
			tr.lines <- "%begin 1 1 0"
			tr.lines <- "'sess-1-:-alert-:-1-:-sess-1-:-created-:-1-:-group-:-2-:-sess-1-:-/tmp-:-stack-:-3'"
			tr.lines <- "%end 1 1 0"
		}
	}()

	sessions, err := tmux.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions returned error: %v", err)
	}
	if sessions == nil {
		t.Fatalf("expected sessions slice, got nil")
	}

	if _, err := tmux.GetSessionByName("sess-1"); err != nil {
		t.Fatalf("GetSessionByName returned error: %v", err)
	}
}

func TestSessionLifecycleCommands(t *testing.T) {
	tr := newSimpleTransport()
	tmux := &Tmux{transport: tr}
	tmux.router = newRouter(tr)
	defer tmux.Close()

	go func() {
		for range tr.sendC {
			tr.lines <- "%begin 1 1 0"
			tr.lines <- "%end 1 1 0"
		}
	}()

	sess, err := tmux.NewSession(&SessionOptions{Name: "newsess"})
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}
	if sess == nil {
		t.Fatalf("expected session instance")
	}

	if err := tmux.DetachClient(&DetachClientOptions{TargetSession: "newsess"}); err != nil {
		t.Fatalf("DetachClient returned error: %v", err)
	}
	if err := tmux.SwitchClient(&SwitchClientOptions{TargetSession: "newsess"}); err != nil {
		t.Fatalf("SwitchClient returned error: %v", err)
	}
	if err := tmux.KillServer(); err != nil {
		t.Fatalf("KillServer returned error: %v", err)
	}

	tr.sendMu.Lock()
	defer tr.sendMu.Unlock()
	joined := strings.Join(tr.sent, "\n")
	if !strings.Contains(joined, "new-session") || !strings.Contains(joined, "detach-client") || !strings.Contains(joined, "switch-client") {
		t.Fatalf("expected session commands in output: %s", joined)
	}
}

func TestSessionAttachDetachHelpers(t *testing.T) {
	tr := newSimpleTransport()
	tmux := &Tmux{transport: tr}
	tmux.router = newRouter(tr)
	defer tmux.Close()

	session := &Session{Name: "sess", tmux: tmux}

	go func() {
		for range tr.sendC {
			tr.lines <- "%begin 1 1 0"
			tr.lines <- "%end 1 1 0"
		}
	}()

	if err := session.AttachSession(nil); err != nil {
		t.Fatalf("AttachSession returned error: %v", err)
	}
	if err := session.Attach(); err != nil {
		t.Fatalf("Attach returned error: %v", err)
	}
	if err := session.Detach(); err != nil {
		t.Fatalf("Detach returned error: %v", err)
	}
	if err := session.Kill(); err != nil {
		t.Fatalf("Kill returned error: %v", err)
	}
	if err := session.Rename("new"); err != nil {
		t.Fatalf("Rename returned error: %v", err)
	}
}
