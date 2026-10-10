package gotmuxcc

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// controlTransport is the low-level interface used by Tmux to communicate with
// a tmux process. It is designed to be satisfied by the control-mode transport.
type controlTransport interface {
	// Send writes a command line to the tmux control-mode connection.
	Send(cmd string) error
	// Lines streams raw lines received from tmux stdout in control mode.
	Lines() <-chan string
	// Done signals the transport has terminated and returns the exit error, if any.
	Done() <-chan error
	// Close terminates the transport and underlying tmux process.
	// It must unblock any concurrent Send call without waiting for that call.
	Close() error
}

// Dialer constructs a control transport for a given socket path.
type Dialer interface {
	Dial(ctx context.Context, socketPath string) (controlTransport, error)
}

// ConstructorOption customises how a Tmux instance is created.
type ConstructorOption func(*constructorConfig)

type constructorConfig struct {
	ctx              context.Context
	dialer           Dialer
	handshakeTimeout time.Duration
}

// DefaultHandshakeTimeout bounds how long a constructor waits for tmux to
// complete the initial control-mode handshake. It exists so that a tmux that
// neither finishes the handshake nor closes the transport cannot wedge a caller
// forever; callers arriving via NewTmux or DefaultTmux have no context to
// cancel, so without a bound their only escape is the transport closing.
//
// It is deliberately generous: the handshake is local IPC that normally settles
// in milliseconds, so this only fires when something is genuinely wrong.
const DefaultHandshakeTimeout = 10 * time.Second

// WithHandshakeTimeout overrides how long the constructor waits for the initial
// control-mode handshake. A non-positive duration disables the bound entirely,
// restoring the previous behaviour of waiting indefinitely.
func WithHandshakeTimeout(d time.Duration) ConstructorOption {
	return func(cfg *constructorConfig) {
		cfg.handshakeTimeout = d
	}
}

type DialerFunc func(ctx context.Context, socketPath string) (controlTransport, error)

func (f DialerFunc) Dial(ctx context.Context, socketPath string) (controlTransport, error) {
	return f(ctx, socketPath)
}

// WithContext allows overriding the context used to establish the transport.
func WithContext(ctx context.Context) ConstructorOption {
	return func(cfg *constructorConfig) {
		if ctx != nil {
			cfg.ctx = ctx
		}
	}
}

// WithDialer overrides the transport dialer, enabling dependency injection in tests.
func WithDialer(d Dialer) ConstructorOption {
	return func(cfg *constructorConfig) {
		if d != nil {
			cfg.dialer = d
		}
	}
}

// NewTmux initializes a Tmux client bound to the provided socket path.
// It mirrors the original gotmux constructor signature for compatibility.
func NewTmux(socketPath string) (*Tmux, error) {
	return NewTmuxWithOptions(socketPath)
}

// DefaultTmux initializes a Tmux client using tmux defaults (current socket).
func DefaultTmux() (*Tmux, error) {
	return NewTmux("")
}

// NewTmuxContext initializes a Tmux client whose underlying tmux -C subprocess
// is bound to the supplied context. Cancelling ctx tears down the transport and
// kills the child, letting consumers tie a client's lifetime to, e.g.,
// signal.NotifyContext so the child is reaped on shutdown without relying solely
// on an explicit Close(). Additional options may still be supplied.
func NewTmuxContext(ctx context.Context, socketPath string, opts ...ConstructorOption) (*Tmux, error) {
	return NewTmuxWithOptions(socketPath, append([]ConstructorOption{WithContext(ctx)}, opts...)...)
}

// NewTmuxWithOptions creates a Tmux client with custom constructor options.
//
// If a tmux server with at least one session is running on the socket, it
// opens the control-mode connection and blocks until the initial handshake
// completes so that subsequent commands are not affected by the startup
// %begin/%end pair. If there is no server (or it has no sessions) the client
// is returned without a connection: nothing is spawned, read-only queries
// report nothing, other commands wrap ErrNoServer, and NewSession starts the
// server. Once a session exists the connection is opened on the next command.
func NewTmuxWithOptions(socketPath string, opts ...ConstructorOption) (*Tmux, error) {
	cfg := constructorConfig{
		ctx:              context.Background(),
		dialer:           defaultDialer{},
		handshakeTimeout: DefaultHandshakeTimeout,
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.dialer == nil {
		return nil, errors.New("gotmuxcc: no transport dialer configured")
	}
	t := &Tmux{
		socketPath: socketPath,
		dial:       cfg,
	}
	if _, err := t.liveRouter(cfg.ctx); err != nil {
		if !errors.Is(err, ErrNoServer) {
			return nil, err
		}
		if socketPath != "" {
			t.Socket = &Socket{Path: socketPath}
		}
		return t, nil
	}
	if socketPath != "" {
		socket, sockErr := newSocket(socketPath)
		if sockErr != nil {
			_ = t.Close()
			return nil, sockErr
		}
		t.Socket = socket
	}
	return t, nil
}

// Tmux is the entry point to the library.
//
// A Tmux value is safe for concurrent use, including calling Close while
// commands are still in flight. The connection is opened lazily and reopened
// after tmux exits, so transport and router are guarded by mu; commands take
// a snapshot of the router and never hold mu across tmux I/O.
type Tmux struct {
	Socket *Socket

	socketPath string
	dial       constructorConfig

	mu        sync.RWMutex
	transport controlTransport
	router    *router
	closed    bool
	// connecting is non-nil while one goroutine is dialing on behalf of all
	// callers; it is closed when that attempt finishes. connectCancel aborts
	// the attempt's handshake wait so Close never queues behind it.
	connecting    chan struct{}
	connectCancel context.CancelFunc

	closeOnce sync.Once
	closeErr  error
}

// errHandshakeClosed reports that tmux ended the connection before the
// startup handshake completed, typically because the server went away between
// discovery and attach.
var errHandshakeClosed = errors.New("gotmuxcc: control transport closed before the handshake completed")

// liveRouter returns a router that can take commands, connecting (or
// reconnecting after tmux exited) when needed. Only one connect runs at a
// time; concurrent callers wait for it and then re-check. A Tmux built
// without a dialer keeps whatever router it was given.
func (t *Tmux) liveRouter(ctx context.Context) (*router, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	for {
		t.mu.RLock()
		r, closed, lazy, connecting := t.router, t.closed, t.dial.dialer != nil, t.connecting
		t.mu.RUnlock()

		if closed {
			return nil, errRouterClosed
		}
		if r != nil && !r.isClosed() {
			return r, nil
		}
		if !lazy {
			if r == nil {
				return nil, errRouterClosed
			}
			return r, nil
		}
		if connecting != nil {
			select {
			case <-connecting:
				continue
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		t.mu.Lock()
		if t.closed {
			t.mu.Unlock()
			return nil, errRouterClosed
		}
		if t.router != nil && !t.router.isClosed() {
			r := t.router
			t.mu.Unlock()
			return r, nil
		}
		if t.connecting != nil {
			t.mu.Unlock()
			continue
		}
		done := make(chan struct{})
		connCtx, cancel := context.WithCancel(ctx)
		t.connecting, t.connectCancel = done, cancel
		stale := t.router
		t.router, t.transport = nil, nil
		t.mu.Unlock()

		if stale != nil {
			// The old connection ended on its own; release its transport
			// before replacing it.
			_ = stale.close()
		}
		transport, r, err := t.dialAndHandshake(connCtx)
		cancel()

		t.mu.Lock()
		t.connecting, t.connectCancel = nil, nil
		if t.closed {
			if err == nil {
				err = errRouterClosed
			}
		} else if err == nil {
			t.transport, t.router = transport, r
		}
		t.mu.Unlock()
		close(done)

		if err != nil {
			if r != nil {
				_ = r.close()
			}
			if t.isClosed() {
				return nil, errRouterClosed
			}
			return nil, err
		}
		return r, nil
	}
}

func (t *Tmux) isClosed() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.closed
}

// dialAndHandshake dials the socket and waits for the control-mode handshake,
// without holding t.mu. It returns ErrNoServer (via errors.Is) when there is
// nothing to attach to. If tmux ends the connection before the handshake —
// the server vanished between discovery and attach-session — it dials once
// more so the outcome reflects the server's current state rather than a dead
// transport; on success the returned router is non-nil and ready, on failure
// it is nil and already closed.
func (t *Tmux) dialAndHandshake(ctx context.Context) (controlTransport, *router, error) {
	for attempt := 0; ; attempt++ {
		transport, err := t.dial.dialer.Dial(t.dial.ctx, t.socketPath)
		if err != nil {
			return nil, nil, err
		}
		r := newRouter(transport)
		err = t.awaitHandshake(ctx, r)
		if err == nil {
			return transport, r, nil
		}
		_ = r.close()
		if attempt == 0 && errors.Is(err, errHandshakeClosed) {
			continue
		}
		return nil, nil, err
	}
}

// awaitHandshake waits for the initial %begin/%end from the attach-session to
// be consumed so that the first user command isn't mismatched.
//
// A tmux that neither completes the handshake nor closes the transport would
// otherwise block here forever, which matters most for NewTmux and
// DefaultTmux callers: they get context.Background(), so the closed channel
// is their only other escape.
func (t *Tmux) awaitHandshake(ctx context.Context, r *router) error {
	var timeout <-chan time.Time
	if t.dial.handshakeTimeout > 0 {
		timer := time.NewTimer(t.dial.handshakeTimeout)
		defer timer.Stop()
		timeout = timer.C
	}

	select {
	case <-r.ready:
		if r.isClosed() {
			return fmt.Errorf("%w: %v", errHandshakeClosed, r.failure())
		}
		return nil
	case <-r.closed:
		return fmt.Errorf("%w: %v", errHandshakeClosed, r.failure())
	case <-ctx.Done():
		return ctx.Err()
	case <-t.dial.ctx.Done():
		return t.dial.ctx.Err()
	case <-timeout:
		return fmt.Errorf("gotmuxcc: timed out after %s waiting for the tmux control-mode handshake", t.dial.handshakeTimeout)
	}
}

// Close shuts down the underlying control-mode transport and kills the tmux -C
// subprocess. Always call Close (e.g. via defer) when finished with a client.
// A closed client never reconnects; a reconnect in flight is aborted.
//
// On Linux the child is also reaped automatically if the process dies without
// Close (parent-death signal). macOS and the BSDs have no such mechanism, so on
// those platforms calling Close — or binding the client to a cancelable context
// via NewTmuxContext — is the only way to avoid orphaning the subprocess.
//
// Close is safe to call concurrently with in-flight commands and safe to call
// more than once; repeat calls return the same error as the first.
//
// Closing does not clear transport or router. Clearing them would race with
// any command still running, and it is unnecessary: router.close fails every
// pending and in-flight request via failAll, so subsequent commands return
// errRouterClosed on their own.
func (t *Tmux) Close() error {
	if t == nil {
		return nil
	}
	t.closeOnce.Do(func() {
		t.mu.Lock()
		t.closed = true
		cancel, connecting := t.connectCancel, t.connecting
		t.mu.Unlock()
		if cancel != nil {
			cancel()
		}
		if connecting != nil {
			// The aborted connect closes its own router before signalling.
			<-connecting
		}
		t.mu.RLock()
		r, transport := t.router, t.transport
		t.mu.RUnlock()
		switch {
		case r != nil:
			t.closeErr = r.close()
		case transport != nil:
			t.closeErr = transport.Close()
		}
	})
	return t.closeErr
}

// events exposes the router's asynchronous event stream.
func (t *Tmux) events() <-chan Event {
	if t == nil {
		return nil
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.router == nil {
		return nil
	}
	return t.router.eventsChannel()
}

func (t *Tmux) runCommand(command string) (commandResult, error) {
	return t.runCommandContext(context.Background(), command)
}

func (t *Tmux) runCommandContext(ctx context.Context, command string) (commandResult, error) {
	if t == nil {
		return commandResult{}, errRouterClosed
	}
	r, err := t.liveRouter(ctx)
	if err != nil {
		return commandResult{}, err
	}
	return r.runCommandContext(ctx, command)
}

func (t *Tmux) query() *query {
	return newQuery(t)
}

// defaultDialer implements Dialer using the control-mode transport.
type defaultDialer struct{}

func (d defaultDialer) Dial(ctx context.Context, socketPath string) (controlTransport, error) {
	return newControlTransport(ctx, socketPath)
}
