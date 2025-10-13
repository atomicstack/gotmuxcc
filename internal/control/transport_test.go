package control

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/atomicstack/gotmuxcc/internal/testutil"
)

func writeFakeTmux(t testing.TB, script string) string {
	t.Helper()
	dir := testutil.TempDir(t)
	path := filepath.Join(dir, "fake_tmux.sh")
	content := []byte("#!/bin/sh\n" + script + "\n")
	if err := os.WriteFile(path, content, 0o755); err != nil {
		t.Fatalf("failed to write fake tmux: %v", err)
	}
	return path
}

func readLine(t testing.TB, ch <-chan string) string {
	t.Helper()
	select {
	case line, ok := <-ch:
		if !ok {
			t.Fatalf("expected line, channel closed")
		}
		return line
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for transport line")
		return ""
	}
}

func waitDone(t testing.TB, done <-chan error) error {
	t.Helper()
	select {
	case err, ok := <-done:
		if !ok {
			return nil
		}
		return err
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for transport shutdown")
		return nil
	}
}

func TestTransportSendAndReceive(t *testing.T) {
	script := `
while [ $# -gt 0 ]; do
	shift
done

while IFS= read -r line; do
	case "$line" in
		list-sessions)
			printf '%%begin 1 1 0\n'
			printf '%s\n' "$FOO"
			printf '%%end 1 1 0\n'
			;;
		*)
			printf '%%error 1 1 0 unknown\n'
			;;
	esac
done
`
	path := writeFakeTmux(t, script)
	socketDir := testutil.TempDir(t)
	socketPath := filepath.Join(socketDir, "sock")

	tr, err := New(context.Background(), Config{
		TmuxBinary: path,
		SocketPath: socketPath,
		ExtraArgs:  []string{"-f", "/dev/null"},
		Env:        []string{"FOO=session"},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	defer tr.Close()

	if err := tr.Send("list-sessions"); err != nil {
		t.Fatalf("Send returned error: %v", err)
	}

	begin := readLine(t, tr.Lines())
	if begin != "%begin 1 1 0" {
		t.Fatalf("unexpected begin line: %q", begin)
	}
	session := readLine(t, tr.Lines())
	if session != "session" {
		t.Fatalf("unexpected session line: %q", session)
	}
	end := readLine(t, tr.Lines())
	if end != "%end 1 1 0" {
		t.Fatalf("unexpected end line: %q", end)
	}

	if err := tr.Close(); err != nil {
		t.Fatalf("unexpected close error: %v", err)
	}
	doneErr := waitDone(t, tr.Done())
	if doneErr != nil {
		t.Fatalf("unexpected done error: %v", doneErr)
	}

	if err := tr.Close(); err != nil {
		t.Fatalf("second close should remain nil, got %v", err)
	}
}

func TestTransportPropagatesStderr(t *testing.T) {
	script := `
echo "boom" >&2
sleep 0.1
exit 3
`
	path := writeFakeTmux(t, script)

	tr, err := New(context.Background(), Config{TmuxBinary: path})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	defer tr.Close()

	doneErr := waitDone(t, tr.Done())
	if doneErr == nil || !strings.Contains(doneErr.Error(), "boom") {
		t.Fatalf("expected stderr error, got %v", doneErr)
	}

	if err := tr.Send("list-sessions"); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected send to fail with stderr error, got %v", err)
	}
}

func TestTransportNewNilContext(t *testing.T) {
	if _, err := New(nil, Config{}); err == nil || !strings.Contains(err.Error(), "context must not be nil") {
		t.Fatalf("expected nil context to error, got %v", err)
	}
}

func TestTransportNewMissingBinary(t *testing.T) {
	_, err := New(context.Background(), Config{TmuxBinary: "/nonexistent/gotmuxcc"})
	if err == nil || !strings.Contains(err.Error(), "failed to start tmux") {
		t.Fatalf("expected missing binary error, got %v", err)
	}
}

type bufferWriteCloser struct {
	builder  strings.Builder
	writeErr error
}

func (b *bufferWriteCloser) Write(p []byte) (int, error) {
	if b.writeErr != nil {
		return 0, b.writeErr
	}
	return b.builder.Write(p)
}

func (b *bufferWriteCloser) Close() error { return nil }

func TestTransportSendAppendsNewline(t *testing.T) {
	buf := &bufferWriteCloser{}
	tr := &Transport{
		stdin: buf,
		lines: make(chan string, 1),
		done:  make(chan error, 1),
	}
	if err := tr.Send("display-message"); err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	if buf.builder.String() != "display-message\n" {
		t.Fatalf("expected newline appended, got %q", buf.builder.String())
	}

	buf.builder.Reset()
	if err := tr.Send("list-sessions\n"); err != nil {
		t.Fatalf("Send with newline returned error: %v", err)
	}
	if buf.builder.String() != "list-sessions\n" {
		t.Fatalf("expected command unchanged, got %q", buf.builder.String())
	}
}

func TestTransportSendClosed(t *testing.T) {
	tr := &Transport{
		stdin:    &bufferWriteCloser{},
		lines:    make(chan string, 1),
		done:     make(chan error, 1),
		finished: true,
	}
	if err := tr.Send("list-sessions"); !errors.Is(err, ErrClosed) {
		t.Fatalf("expected ErrClosed, got %v", err)
	}
}

func TestTransportSendCloseError(t *testing.T) {
	closeErr := errors.New("boom")
	tr := &Transport{
		stdin:    &bufferWriteCloser{},
		lines:    make(chan string, 1),
		done:     make(chan error, 1),
		closeErr: closeErr,
	}
	if err := tr.Send("list-sessions"); !errors.Is(err, closeErr) {
		t.Fatalf("expected close error, got %v", err)
	}
}

func TestTransportSendWriteFailure(t *testing.T) {
	writeErr := errors.New("write failed")
	tr := &Transport{
		stdin: &bufferWriteCloser{writeErr: writeErr},
		lines: make(chan string, 1),
		done:  make(chan error, 1),
	}
	err := tr.Send("list-sessions")
	if err == nil || !strings.Contains(err.Error(), "write failed") {
		t.Fatalf("expected wrapped write error, got %v", err)
	}
}

func TestTransportNilReceivers(t *testing.T) {
	var tr *Transport
	if err := tr.Send("list-sessions"); err == nil || !strings.Contains(err.Error(), "transport is nil") {
		t.Fatalf("expected nil transport error, got %v", err)
	}
	if tr.Lines() != nil {
		t.Fatalf("expected nil Lines channel for nil transport")
	}
	if tr.Done() != nil {
		t.Fatalf("expected nil Done channel for nil transport")
	}
	if err := tr.Close(); err != nil {
		t.Fatalf("expected nil Close error for nil transport, got %v", err)
	}
}
