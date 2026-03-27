package gotmuxcc

import (
	"strings"
	"testing"
)

// --- quoteArgument must strip control characters ---

func TestQuoteArgumentStripsNewlines(t *testing.T) {
	result := quoteArgument("hello\nworld")
	if strings.Contains(result, "\n") {
		t.Errorf("quoteArgument should strip newlines, got %q", result)
	}
}

func TestQuoteArgumentStripsCarriageReturn(t *testing.T) {
	result := quoteArgument("hello\rworld")
	if strings.Contains(result, "\r") {
		t.Errorf("quoteArgument should strip carriage returns, got %q", result)
	}
}

func TestQuoteArgumentStripsNullByte(t *testing.T) {
	result := quoteArgument("hello\x00world")
	if strings.Contains(result, "\x00") {
		t.Errorf("quoteArgument should strip null bytes, got %q", result)
	}
}

// --- pargs must be quoted in build() ---

func TestBuildQuotesPosArgs(t *testing.T) {
	q := newQuery(&Tmux{})
	q.cmd("rename-session").
		fargs("-t", "mysess").
		pargs("name with spaces")

	built, err := q.build()
	if err != nil {
		t.Fatalf("build returned error: %v", err)
	}
	// positional args should be quoted when they contain spaces
	if !strings.Contains(built, "'name with spaces'") {
		t.Errorf("expected pargs to be quoted, got %q", built)
	}
}

func TestBuildRejectsNewlineInPosArgs(t *testing.T) {
	q := newQuery(&Tmux{})
	q.cmd("send-keys").
		fargs("-t", "%0").
		pargs("hello\nkill-server")

	built, err := q.build()
	if err != nil {
		t.Fatalf("build returned error: %v", err)
	}
	if strings.Contains(built, "\n") {
		t.Errorf("built command must not contain newlines, got %q", built)
	}
}

// --- ShellCommand must use quoteArgument, not manual wrapping ---

func TestShellCommandWithSingleQuote(t *testing.T) {
	q := newQuery(&Tmux{})
	q.cmd("new-session").
		fargs("-d", "-s", "test")

	// simulate what session.go:137 does with ShellCommand
	shellCmd := "echo it's done"
	q.pargs(shellCmd)

	built, err := q.build()
	if err != nil {
		t.Fatalf("build returned error: %v", err)
	}
	// the single quote must be properly escaped, not raw
	if strings.Contains(built, "'echo it's done'") {
		t.Errorf("shell command with single quote is incorrectly wrapped: %q", built)
	}
	// should contain the properly escaped form
	if !strings.Contains(built, `'echo it'\''s done'`) {
		t.Errorf("expected properly escaped single quote, got %q", built)
	}
}

// --- checkSessionName must reject control characters ---

func TestCheckSessionNameRejectsNewline(t *testing.T) {
	if checkSessionName("foo\nbar") {
		t.Error("checkSessionName should reject names containing newlines")
	}
}

func TestCheckSessionNameRejectsCarriageReturn(t *testing.T) {
	if checkSessionName("foo\rbar") {
		t.Error("checkSessionName should reject names containing carriage returns")
	}
}

func TestCheckSessionNameRejectsNullByte(t *testing.T) {
	if checkSessionName("foo\x00bar") {
		t.Error("checkSessionName should reject names containing null bytes")
	}
}

// --- subscription name must not contain colons ---

func TestSubscribeRejectsColonInName(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	tmux := &Tmux{transport: ft, router: r}

	// respond in case the command gets through (it shouldn't)
	go func() {
		cmd, ok := <-ft.sendC
		if ok && cmd != "" {
			ft.lines <- "%begin 1 1 0"
			ft.lines <- "%end 1 1 0"
		}
	}()

	err := tmux.Subscribe("sub:evil", SubSession, "#{session_name}")
	if err == nil {
		t.Error("Subscribe should reject names containing colons")
	}
}

// --- refresh.go windowID must be quoted ---

func TestSetWindowSizeQuotesWindowID(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	tmux := &Tmux{transport: ft, router: r}

	go func() {
		cmd := <-ft.sendC
		// a windowID with spaces should be safely handled
		if strings.Contains(cmd, "\n") {
			t.Errorf("command must not contain newlines: %q", cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "%end 1 1 0"
	}()

	// normal usage should still work
	err := tmux.SetWindowSize("@0", 80, 24)
	if err != nil {
		t.Fatalf("SetWindowSize returned error: %v", err)
	}
}
