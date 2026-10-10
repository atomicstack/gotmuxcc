package gotmuxcc

import (
	"context"
	"errors"
	"regexp"
	"sync"
	"testing"
	"time"
)

// handshakeLines is the %begin/%end pair tmux emits for the attach-session
// given on the control client's command line; the router waits for it before
// it is ready.
var handshakeLines = []string{"%begin 1712000000 0 0", "%end 1712000000 0 0"}

// lazyDialer counts Dial calls and hands each one to next, so a test can make
// the first dial find no server and a later one succeed.
type lazyDialer struct {
	mu    sync.Mutex
	calls int
	next  func(call int) (controlTransport, error)
}

func (d *lazyDialer) Dial(_ context.Context, _ string) (controlTransport, error) {
	d.mu.Lock()
	d.calls++
	call := d.calls
	d.mu.Unlock()
	return d.next(call)
}

func (d *lazyDialer) count() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.calls
}

// connectedTransport returns a scripted transport whose handshake has already
// arrived, ready to answer the given responses in order.
func connectedTransport(responses ...scriptedResponse) *scriptedTransport {
	tr := newScriptedTransport(responses)
	for _, line := range handshakeLines {
		tr.lines <- line
	}
	return tr
}

var formatVariable = regexp.MustCompile(`#\{([^}]+)\}`)

// fillFormat renders a tmux -F format the way tmux would, substituting the
// given variable values and leaving unknown variables empty.
func fillFormat(format string, values map[string]string) string {
	return formatVariable.ReplaceAllStringFunc(format, func(m string) string {
		return values[m[2:len(m)-1]]
	})
}

func stubTmuxExec(t *testing.T, fn func(socketPath string, args []string) ([]byte, error)) {
	t.Helper()
	original := tmuxExecCommand
	tmuxExecCommand = fn
	t.Cleanup(func() { tmuxExecCommand = original })
}

// stubSocketValidation makes newSocket accept any path without running tmux.
func stubSocketValidation(t *testing.T) {
	t.Helper()
	original := tmuxListClients
	tmuxListClients = func(string) ([]byte, error) { return nil, nil }
	t.Cleanup(func() { tmuxListClients = original })
}

func TestNewTmuxWithoutServerYieldsDisconnectedClient(t *testing.T) {
	stubTmuxExec(t, func(string, []string) ([]byte, error) {
		t.Fatalf("no command should be executed while constructing a client")
		return nil, nil
	})
	dialer := &lazyDialer{next: func(int) (controlTransport, error) { return nil, ErrNoServer }}

	tmux, err := NewTmuxWithOptions("/tmp/sock", WithDialer(dialer))
	if err != nil {
		t.Fatalf("NewTmuxWithOptions must not fail when no server is running, got %v", err)
	}
	defer tmux.Close()
	if tmux.Socket == nil || tmux.Socket.Path != "/tmp/sock" {
		t.Fatalf("expected the socket path to be recorded, got %#v", tmux.Socket)
	}

	sessions, err := tmux.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions on an empty server must succeed, got %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("expected no sessions, got %#v", sessions)
	}
	if tmux.HasSession("anything") {
		t.Fatalf("HasSession must be false with no server")
	}

	if _, err := tmux.GetServerInformation(); !errors.Is(err, ErrNoServer) {
		t.Fatalf("expected a command needing a server to report ErrNoServer, got %v", err)
	}
}

func TestNewSessionWithoutServerStartsItThenConnects(t *testing.T) {
	stubSocketValidation(t)
	var execMu sync.Mutex
	var execSocket string
	var execArgs []string
	serverStarted := false
	stubTmuxExec(t, func(socketPath string, args []string) ([]byte, error) {
		execMu.Lock()
		defer execMu.Unlock()
		serverStarted = true
		execSocket = socketPath
		execArgs = append([]string(nil), args...)
		for i, arg := range args {
			if arg == "-F" && i+1 < len(args) {
				return []byte(fillFormat(args[i+1], map[string]string{"session_id": "$5", "session_name": "first"}) + "\n"), nil
			}
		}
		return nil, errors.New("no -F format in new-session args")
	})

	listReply := scriptedResponse{match: "list-sessions", lines: []string{
		"%begin 1712000001 1 1",
		"", // filled in below once the format is known
		"%end 1712000001 1 1",
	}}
	// Discovery keeps finding no server until new-session has run, exactly as
	// the real dialer would.
	dialer := &lazyDialer{next: func(int) (controlTransport, error) {
		execMu.Lock()
		started := serverStarted
		execMu.Unlock()
		if !started {
			return nil, ErrNoServer
		}
		return connectedTransport(listReply), nil
	}}

	tmux, err := NewTmuxWithOptions("/tmp/sock", WithDialer(dialer))
	if err != nil {
		t.Fatalf("NewTmuxWithOptions returned error: %v", err)
	}
	defer tmux.Close()

	session, err := tmux.NewSession(&SessionOptions{Name: "first"})
	if err != nil {
		t.Fatalf("NewSession with no server must start one, got %v", err)
	}
	if session == nil || session.Name != "first" || session.Id != "$5" {
		t.Fatalf("unexpected session from exec-backed new-session: %#v", session)
	}

	execMu.Lock()
	gotSocket, gotArgs := execSocket, execArgs
	execMu.Unlock()
	if gotSocket != "/tmp/sock" {
		t.Fatalf("new-session must be executed against the client's socket, got %q", gotSocket)
	}
	if len(gotArgs) < 3 || gotArgs[0] != "new-session" || gotArgs[1] != "-d" || gotArgs[2] != "-P" {
		t.Fatalf("expected a detached, printing new-session, got %#v", gotArgs)
	}
	if !containsPair(gotArgs, "-s", "first") {
		t.Fatalf("expected -s first in new-session args, got %#v", gotArgs)
	}
	if dialer.count() != 2 {
		t.Fatalf("expected the constructor and NewSession to each probe for a server once, got %d dials", dialer.count())
	}

	// The next command finds the server that new-session started and
	// connects to it lazily.
	format := ""
	for i, arg := range gotArgs {
		if arg == "-F" {
			format = gotArgs[i+1]
		}
	}
	listReply.lines[1] = fillFormat(format, map[string]string{"session_id": "$5", "session_name": "first"})

	sessions, err := tmux.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions after NewSession returned error: %v", err)
	}
	if len(sessions) != 1 || sessions[0].Name != "first" {
		t.Fatalf("expected the new session to be listed over control mode, got %#v", sessions)
	}
	if dialer.count() != 3 {
		t.Fatalf("expected one more dial once a server exists, got %d", dialer.count())
	}
}

func containsPair(args []string, flag, value string) bool {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == flag && args[i+1] == value {
			return true
		}
	}
	return false
}

func TestCommandsReconnectAfterTransportExit(t *testing.T) {
	stubSocketValidation(t)
	reply := func() scriptedResponse {
		return scriptedResponse{match: "has-session", lines: []string{
			"%begin 1712000002 1 1",
			"%end 1712000002 1 1",
		}}
	}
	var first *scriptedTransport
	dialer := &lazyDialer{next: func(call int) (controlTransport, error) {
		tr := connectedTransport(reply())
		if call == 1 {
			first = tr
		}
		return tr, nil
	}}

	tmux, err := NewTmuxWithOptions("/tmp/sock", WithDialer(dialer))
	if err != nil {
		t.Fatalf("NewTmuxWithOptions returned error: %v", err)
	}
	defer tmux.Close()
	if !tmux.HasSession("main") {
		t.Fatalf("expected the first transport to answer has-session")
	}

	// tmux exits (e.g. its last session was killed): the transport ends on
	// its own, without the client being closed.
	first.Close()
	deadline := time.Now().Add(2 * time.Second)
	for !tmux.HasSession("main") {
		if time.Now().After(deadline) {
			t.Fatalf("has-session never recovered after the transport exited; dials=%d", dialer.count())
		}
		time.Sleep(10 * time.Millisecond)
	}
	if dialer.count() != 2 {
		t.Fatalf("expected exactly one reconnect, got %d dials", dialer.count())
	}
}

func TestCloseStopsReconnecting(t *testing.T) {
	dialer := &lazyDialer{next: func(int) (controlTransport, error) { return nil, ErrNoServer }}
	tmux, err := NewTmuxWithOptions("/tmp/sock", WithDialer(dialer))
	if err != nil {
		t.Fatalf("NewTmuxWithOptions returned error: %v", err)
	}
	if err := tmux.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	if _, err := tmux.ListSessions(); err == nil {
		t.Fatalf("commands after Close must fail")
	}
	if dialer.count() != 1 {
		t.Fatalf("a closed client must not dial again, got %d dials", dialer.count())
	}
}

// deadTransport returns a transport whose tmux has already exited, as happens
// when attach-session finds the server gone: no handshake, just closure.
func deadTransport() *scriptedTransport {
	tr := newScriptedTransport(nil)
	tr.Close()
	return tr
}

// TestHandshakeFailureRechecksForServer covers the server vanishing between
// discovery and attach: the attach dies before the handshake, and the client
// must ask again rather than install a dead connection or fail construction.
func TestHandshakeFailureRechecksForServer(t *testing.T) {
	dialer := &lazyDialer{next: func(call int) (controlTransport, error) {
		if call == 1 {
			return deadTransport(), nil
		}
		return nil, ErrNoServer
	}}

	tmux, err := NewTmuxWithOptions("/tmp/sock", WithDialer(dialer), WithHandshakeTimeout(time.Second))
	if err != nil {
		t.Fatalf("a failed attach on a vanished server must not fail construction: %v", err)
	}
	defer tmux.Close()

	sessions, err := tmux.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions must treat the vanished server as empty, got %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("expected no sessions, got %#v", sessions)
	}
	if dialer.count() < 2 {
		t.Fatalf("expected the client to re-check for a server after the failed attach, got %d dials", dialer.count())
	}
}

// TestCloseInterruptsSlowReconnect pins that Close is prompt even while a
// reconnect is waiting on a tmux that never completes the handshake: it must
// not queue behind the handshake timeout, and the wedged transport must be
// torn down.
func TestCloseInterruptsSlowReconnect(t *testing.T) {
	wedged := newWedgedTransport()
	dialer := &lazyDialer{next: func(call int) (controlTransport, error) {
		if call == 1 {
			return nil, ErrNoServer
		}
		return wedged, nil
	}}

	tmux, err := NewTmuxWithOptions("/tmp/sock", WithDialer(dialer), WithHandshakeTimeout(10*time.Second))
	if err != nil {
		t.Fatalf("NewTmuxWithOptions returned error: %v", err)
	}

	listDone := make(chan error, 1)
	go func() {
		_, err := tmux.GetServerInformation()
		listDone <- err
	}()
	// Wait until the reconnect is parked in the handshake.
	deadline := time.Now().Add(2 * time.Second)
	for dialer.count() < 2 {
		if time.Now().After(deadline) {
			t.Fatalf("reconnect never dialed")
		}
		time.Sleep(5 * time.Millisecond)
	}

	closeDone := make(chan error, 1)
	go func() { closeDone <- tmux.Close() }()
	select {
	case err := <-closeDone:
		if err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("Close blocked behind the in-flight reconnect")
	}

	select {
	case err := <-listDone:
		if err == nil {
			t.Fatalf("the command interrupted by Close must fail")
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("the interrupted command never returned")
	}
	select {
	case <-wedged.waitClosed():
	case <-time.After(2 * time.Second):
		t.Fatalf("Close did not tear down the transport dialed by the reconnect")
	}
}
