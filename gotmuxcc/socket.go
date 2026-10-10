package gotmuxcc

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

var tmuxListClients = func(path string) ([]byte, error) {
	cmd := exec.Command("tmux", "-S", path, "list-clients")
	cmd.Env = append(os.Environ(), "TMUX=")
	return cmd.CombinedOutput()
}

// tmuxExecCommand runs one tmux command as a plain client, the way gotmux does
// for every call. gotmuxcc only needs it for new-session while no server is
// running, since that is the one command that has to work without a
// control-mode connection to attach to.
var tmuxExecCommand = func(socketPath string, args []string) ([]byte, error) {
	full := make([]string, 0, len(args)+2)
	if strings.TrimSpace(socketPath) != "" {
		full = append(full, "-S", socketPath)
	}
	full = append(full, args...)
	cmd := exec.Command("tmux", full...)
	cmd.Env = append(os.Environ(), "TMUX=")
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if msg := strings.TrimSpace(string(exitErr.Stderr)); msg != "" {
				return nil, fmt.Errorf("%s: %w", msg, err)
			}
		}
		return nil, err
	}
	return out, nil
}

func newSocket(path string) (*Socket, error) {
	if path == "" {
		return nil, nil
	}
	if err := validateSocket(path); err != nil {
		return nil, err
	}
	return &Socket{Path: path}, nil
}

func validateSocket(path string) error {
	if out, err := tmuxListClients(path); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return fmt.Errorf("tmux binary not found while validating socket %q", path)
		}
		msg := strings.TrimSpace(string(out))
		if strings.Contains(strings.ToLower(msg), "no such file or directory") {
			return fmt.Errorf("tmux socket %q not available: %s", path, msg)
		}
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("invalid tmux socket %q: %s", path, msg)
	}
	return nil
}
