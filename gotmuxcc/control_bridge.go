package gotmuxcc

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/atomicstack/gotmuxcc/internal/control"
)

type initialSessionPlan struct {
	extraArgs        []string
	bootstrapSession string
	tmuxBinary       string
}

func newControlTransport(ctx context.Context, socketPath string) (controlTransport, error) {
	plan, err := planInitialSession("", socketPath)
	if err != nil {
		return nil, fmt.Errorf("gotmux: failed to determine initial control-mode command: %w", err)
	}
	cfg := control.Config{
		SocketPath: socketPath,
		ExtraArgs:  plan.extraArgs,
	}
	transport, err := control.New(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("gotmux: failed to establish control transport: %w", err)
	}
	if plan.bootstrapSession != "" {
		return &bootstrapCleanupTransport{
			controlTransport: transport,
			tmuxBinary:       plan.tmuxBinary,
			socketPath:       socketPath,
			bootstrapSession: plan.bootstrapSession,
		}, nil
	}
	return transport, nil
}

func initialAttachArgs(tmuxBinary, socketPath string) []string {
	plan, err := planInitialSession(tmuxBinary, socketPath)
	if err != nil {
		return nil
	}
	return append([]string(nil), plan.extraArgs...)
}

func planInitialSession(tmuxBinary, socketPath string) (initialSessionPlan, error) {
	bin := strings.TrimSpace(tmuxBinary)
	if bin == "" {
		bin = "tmux"
	}
	target, err := discoverAttachTarget(bin, socketPath)
	if err != nil {
		return initialSessionPlan{}, err
	}
	if target == "" {
		sessionName := bootstrapSessionName()
		return initialSessionPlan{
			extraArgs:        []string{"new-session", "-d", "-s", sessionName},
			bootstrapSession: sessionName,
			tmuxBinary:       bin,
		}, nil
	}
	return initialSessionPlan{
		extraArgs:  []string{"attach-session", "-t", target},
		tmuxBinary: bin,
	}, nil
}

func discoverAttachTarget(tmuxBinary, socketPath string) (string, error) {
	args := make([]string, 0, 6)
	if strings.TrimSpace(socketPath) != "" {
		args = append(args, "-S", socketPath)
	}
	args = append(args, "list-sessions", "-F", "#{session_name}")
	cmd := exec.Command(tmuxBinary, args...)
	output, err := cmd.Output()
	if err != nil {
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

func bootstrapSessionName() string {
	return fmt.Sprintf("__gotmuxcc_bootstrap_%d_%d", os.Getpid(), time.Now().UnixNano())
}

type bootstrapCleanupTransport struct {
	controlTransport
	tmuxBinary       string
	socketPath       string
	bootstrapSession string
	cleanupOnce      sync.Once
}

func (t *bootstrapCleanupTransport) Close() error {
	var err error
	if t.controlTransport != nil {
		err = t.controlTransport.Close()
	}
	t.cleanupOnce.Do(func() {
		cleanupBootstrapSession(t.tmuxBinary, t.socketPath, t.bootstrapSession)
	})
	return err
}

func cleanupBootstrapSession(tmuxBinary, socketPath, sessionName string) {
	if strings.TrimSpace(sessionName) == "" {
		return
	}

	bin := strings.TrimSpace(tmuxBinary)
	if bin == "" {
		bin = "tmux"
	}

	args := make([]string, 0, 5)
	if strings.TrimSpace(socketPath) != "" {
		args = append(args, "-S", socketPath)
	}
	args = append(args, "kill-session", "-t", sessionName)

	cmd := exec.Command(bin, args...)
	_ = cmd.Run()
}
