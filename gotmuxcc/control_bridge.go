package gotmuxcc

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/atomicstack/gotmuxcc/internal/control"
)

// ErrNoServer reports that no tmux server is running on the client's socket.
//
// gotmuxcc never starts a tmux server on its own: a control-mode client has to
// attach to an existing session to stay alive, and the only way to conjure one
// on an empty server is to create a session the user did not ask for. So when
// there is nothing to attach to, the constructor returns a client without a
// connection, and commands wrap this error until a session exists. NewSession
// is the exception: it starts the server exactly as plain tmux would, after
// which the client connects lazily. Use errors.Is to test for it.
var ErrNoServer = errors.New("gotmuxcc: no tmux server running")

func newControlTransport(ctx context.Context, socketPath string) (controlTransport, error) {
	args, err := attachArgs("", socketPath)
	if err != nil {
		if errors.Is(err, ErrNoServer) {
			return nil, err
		}
		return nil, fmt.Errorf("gotmux: failed to determine initial control-mode command: %w", err)
	}
	cfg := control.Config{
		SocketPath: socketPath,
		ExtraArgs:  args,
	}
	transport, err := control.New(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("gotmux: failed to establish control transport: %w", err)
	}
	return transport, nil
}

// attachArgs returns the command that opens the control-mode connection: an
// attach-session targeting the first existing session by id. With no session
// to attach to it returns ErrNoServer and no command, so the caller spawns
// nothing.
//
// Discovery and attach are two tmux invocations, and attach-session carries
// CMD_STARTSERVER, so if the server exits in between, the attach itself
// starts one. That server sources the user's configuration, fails the attach
// with "no sessions", and exits on its own; the dialer then sees the
// handshake die and re-runs discovery. The window is a few milliseconds and
// tmux offers no client flag to close it.
func attachArgs(tmuxBinary, socketPath string) ([]string, error) {
	bin := strings.TrimSpace(tmuxBinary)
	if bin == "" {
		bin = "tmux"
	}
	target, err := discoverAttachTarget(bin, socketPath)
	if err != nil {
		return nil, err
	}
	if target == "" {
		return nil, fmt.Errorf("%w on socket %q", ErrNoServer, socketPath)
	}
	return []string{"attach-session", "-t", target}, nil
}

func discoverAttachTarget(tmuxBinary, socketPath string) (string, error) {
	args := make([]string, 0, 6)
	if strings.TrimSpace(socketPath) != "" {
		args = append(args, "-S", socketPath)
	}
	// Ask for the session id, not the name. tmux relaxed name validation in
	// 166267c8 ("Allow :. in names again"), so a name may contain ":" or "."
	// — and attach-session parses those as the session:window.pane
	// separators, making such a name unusable as a target ("can't find
	// window: ..."). "$N" is unambiguous and cannot collide.
	args = append(args, "list-sessions", "-F", "#{session_id}")
	cmd := exec.Command(tmuxBinary, args...)
	output, err := cmd.Output()
	if err != nil {
		// tmux exits 1 both when the server has no sessions and when there
		// is no server at all ("no server running on ..." / "error
		// connecting to ..."); either way there is nothing to attach to.
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return "", nil
		}
		return "", err
	}
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		name := strings.TrimSpace(line)
		if name != "" {
			return name, nil
		}
	}
	return "", nil
}
