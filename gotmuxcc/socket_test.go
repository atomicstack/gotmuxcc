package gotmuxcc

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestNewSocketEmptyPath(t *testing.T) {
	sock, err := newSocket("")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if sock != nil {
		t.Fatalf("expected nil socket, got %#v", sock)
	}
}

func TestNewSocketValidPath(t *testing.T) {
	called := false
	original := tmuxListClients
	tmuxListClients = func(path string) ([]byte, error) {
		called = true
		if path != "/tmp/tmux-valid" {
			return nil, fmt.Errorf("unexpected path %s", path)
		}
		return []byte(""), nil
	}
	defer func() { tmuxListClients = original }()

	socket, err := newSocket("/tmp/tmux-valid")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if socket == nil || socket.Path != "/tmp/tmux-valid" {
		t.Fatalf("unexpected socket: %#v", socket)
	}
	if !called {
		t.Fatalf("expected tmuxListClients to be called")
	}
}

func TestNewSocketInvalidPath(t *testing.T) {
	original := tmuxListClients
	tmuxListClients = func(path string) ([]byte, error) {
		return nil, errors.New("no such file or directory")
	}
	defer func() { tmuxListClients = original }()

	if _, err := newSocket("/tmp/missing"); err == nil {
		t.Fatalf("expected error for invalid socket")
	}
}

func TestValidateSocketErrorPropagation(t *testing.T) {
	original := tmuxListClients
	tmuxListClients = func(path string) ([]byte, error) {
		return []byte("permission denied"), errors.New("exit status 1")
	}
	defer func() { tmuxListClients = original }()

	err := validateSocket("/tmp/protected")
	if err == nil || !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("expected permission error, got %v", err)
	}
}
