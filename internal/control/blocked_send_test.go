package control

import (
	"context"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

type notifyingWriter struct {
	io.WriteCloser
	entered chan struct{}
	once    sync.Once
}

func (w *notifyingWriter) Write(p []byte) (int, error) {
	w.once.Do(func() { close(w.entered) })
	return w.WriteCloser.Write(p)
}

func TestTransportCloseInterruptsBlockedWrite(t *testing.T) {
	path := writeFakeTmux(t, "exec sleep 30")
	tr, err := New(context.Background(), Config{TmuxBinary: path})
	if err != nil {
		t.Fatal(err)
	}
	writer := &notifyingWriter{WriteCloser: tr.stdin, entered: make(chan struct{})}
	tr.stdin = writer
	sent := make(chan error, 1)
	closed := make(chan error, 1)
	t.Cleanup(func() {
		// release the real subprocess even when the old close path deadlocks.
		tr.cancel()
		_ = tr.Close()
		waitDone(t, tr.Done())
	})
	go func() { sent <- tr.Send(strings.Repeat("x", 8*1024*1024)) }()
	select {
	case <-writer.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("write did not start")
	}
	go func() { closed <- tr.Close() }()
	select {
	case err := <-closed:
		if err != nil {
			t.Fatalf("close: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("close blocked behind transport write")
	}
	select {
	case err := <-sent:
		if err == nil {
			t.Fatal("blocked write unexpectedly succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("write remained blocked after close")
	}
}
