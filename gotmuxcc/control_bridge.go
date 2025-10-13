package gotmuxcc

import (
	"context"
	"fmt"

	"github.com/atomicstack/gotmuxcc/internal/control"
)

func newControlTransport(ctx context.Context, socketPath string) (controlTransport, error) {
	cfg := control.Config{
		SocketPath: socketPath,
	}
	transport, err := control.New(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("gotmux: failed to establish control transport: %w", err)
	}
	return transport, nil
}
