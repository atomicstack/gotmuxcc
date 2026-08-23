package gotmuxcc

import (
	"errors"
	"sync"
	"testing"
)

// TestCloseConcurrentWithRunCommand exercises the shutdown path that
// tmux-popup-control's watcher now relies on: a fetch is abandoned mid-flight,
// Close() unblocks it via failAll, and the abandoned goroutine then makes its
// next call concurrently with Close() tearing the client down.
//
// Run under -race this catches the unsynchronised writes to t.router /
// t.transport / t.Socket; without -race it can still catch the check-then-use
// nil dereference in runCommand.
func TestCloseConcurrentWithRunCommand(t *testing.T) {
	for i := 0; i < 50; i++ {
		ft := newFakeTransport()
		tmux := &Tmux{transport: ft, Socket: &Socket{Path: "/tmp/whatever.sock"}}
		tmux.router = newRouterWithInit(ft, false)

		var wg sync.WaitGroup
		wg.Add(2)

		start := make(chan struct{})

		go func() {
			defer wg.Done()
			<-start
			// The abandoned poller resuming: it does not know Close() is running.
			for j := 0; j < 20; j++ {
				if _, err := tmux.runCommand("list-sessions"); err != nil &&
					!errors.Is(err, errRouterClosed) && !errors.Is(err, ErrTransportClosed) {
					// Any other error is fine; we only care that this never
					// races or panics.
					continue
				}
			}
		}()

		go func() {
			defer wg.Done()
			<-start
			_ = tmux.Close()
		}()

		close(start)
		wg.Wait()
	}
}

// TestCloseIsIdempotentUnderConcurrency asserts repeated concurrent Close calls
// neither race nor panic. Consumers wire Close into shutdown paths that can fire
// from more than one place.
func TestCloseIsIdempotentUnderConcurrency(t *testing.T) {
	ft := newFakeTransport()
	tmux := &Tmux{transport: ft, Socket: &Socket{Path: "/tmp/whatever.sock"}}
	tmux.router = newRouterWithInit(ft, false)

	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_ = tmux.Close()
		}()
	}
	close(start)
	wg.Wait()
}
