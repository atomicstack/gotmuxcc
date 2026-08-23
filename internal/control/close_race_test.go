package control

import (
	"context"
	"testing"
	"time"
)

// TestStderrOutputDoesNotCloseLinesUnderSender reproduces a send-on-closed-channel
// panic in a transport-owned goroutine.
//
// finish() closes t.lines, and the stderr collector calls finish() as soon as
// tmux writes anything to stderr and exits. Meanwhile the stdout forwarder may
// still be blocked sending a scanned line into that same channel. Closing a
// channel out from under a blocked sender panics, and because the panic happens
// on a goroutine the library owns, a consumer cannot recover from it — the whole
// process dies.
//
// The fake tmux here writes far more stdout than the 128-slot lines channel can
// hold, so the forwarder is parked on a send, then writes to stderr and exits to
// trigger finish(). A consumer that is slow to drain (or has stopped draining
// during shutdown) hits exactly this.
func TestStderrOutputDoesNotCloseLinesUnderSender(t *testing.T) {
	script := `
i=0
while [ $i -lt 400 ]; do
	echo "line $i"
	i=$((i+1))
done
echo "boom" >&2
`
	path := writeFakeTmux(t, script)

	tr, err := New(context.Background(), Config{TmuxBinary: path})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	defer tr.Close()

	// Deliberately do not drain Lines() until after the stderr path has run, so
	// the stdout forwarder is blocked on a full channel when finish() fires.
	if err := waitDone(t, tr.Done()); err == nil {
		t.Fatalf("expected the stderr payload to surface as the done error")
	}

	// Give the forwarder a moment to hit the closed channel if it is going to.
	// Draining is what would let it proceed, so drain only now.
	deadline := time.After(2 * time.Second)
	for {
		select {
		case _, ok := <-tr.Lines():
			if !ok {
				return // lines closed cleanly, no panic
			}
		case <-deadline:
			return
		}
	}
}
