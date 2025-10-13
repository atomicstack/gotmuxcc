package gotmuxcc

import (
	"context"
	"errors"
	"testing"
)

func TestNewTmuxWithOptionsUsesDialer(t *testing.T) {
	called := false
	fakeTransport := newRecordTransport()
	dialer := DialerFunc(func(ctx context.Context, socketPath string) (controlTransport, error) {
		called = true
		return fakeTransport, nil
	})

	tmux, err := NewTmuxWithOptions("", WithDialer(dialer))
	if err != nil {
		t.Fatalf("NewTmuxWithOptions returned error: %v", err)
	}
	if !called {
		t.Fatalf("expected dialer to be called")
	}
	if tmux.transport != fakeTransport {
		t.Fatalf("expected transport to be set")
	}
	_ = tmux.Close()
}

func TestNewTmuxWithOptionsDialerError(t *testing.T) {
	dialer := DialerFunc(func(ctx context.Context, socketPath string) (controlTransport, error) {
		return nil, errors.New("dialer failed")
	})

	if _, err := NewTmuxWithOptions("", WithDialer(dialer)); err == nil {
		t.Fatalf("expected error from dialer")
	}
}

func TestWithContextOption(t *testing.T) {
	var gotCtx context.Context
	dialer := DialerFunc(func(ctx context.Context, socketPath string) (controlTransport, error) {
		gotCtx = ctx
		return newRecordTransport(), nil
	})

	ctx := context.WithValue(context.Background(), struct{}{}, "value")
	tmux, err := NewTmuxWithOptions("", WithDialer(dialer), WithContext(ctx))
	if err != nil {
		t.Fatalf("NewTmuxWithOptions returned error: %v", err)
	}
	if gotCtx != ctx {
		t.Fatalf("expected context to be passed to dialer")
	}
	_ = tmux.Close()
}
