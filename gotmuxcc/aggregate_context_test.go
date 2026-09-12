package gotmuxcc

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func aggregateResponse(command string, rows ...string) scriptedResponse {
	lines := append([]string{"%begin 1 1 1"}, rows...)
	return scriptedResponse{match: command, lines: append(lines, "%end 1 1 1")}
}

func aggregateError(command string) scriptedResponse {
	return scriptedResponse{match: command, lines: []string{"%begin 1 1 1", "resource vanished", "%error 1 1 1"}}
}

func aggregateTmux(t *testing.T, responses []scriptedResponse) (*Tmux, *scriptedTransport) {
	t.Helper()
	tr := newScriptedTransport(responses)
	tmux := &Tmux{transport: tr, router: newRouterWithInit(tr, false)}
	t.Cleanup(func() { _ = tmux.Close() })
	return tmux, tr
}

func assertCommandCount(t *testing.T, tr *scriptedTransport, want int) {
	t.Helper()
	tr.mu.Lock()
	defer tr.mu.Unlock()
	if len(tr.sent) != want {
		t.Fatalf("got %d commands, want %d: %v", len(tr.sent), want, tr.sent)
	}
}

func linkedWindowRows() []string {
	vars := newQuery(nil).windowVars().variables
	return []string{
		formatRecord(vars, map[string]string{varWindowId: "@1", varSessionName: "alpha", varWindowIndex: "1", varWindowActive: "1"}),
		formatRecord(vars, map[string]string{varWindowId: "@1", varSessionName: "beta", varWindowIndex: "7", varWindowActive: "0"}),
		formatRecord(vars, map[string]string{varWindowId: "@2", varSessionName: "beta", varWindowIndex: "8", varWindowActive: "1"}),
	}
}

func assertLinkedWindows(t *testing.T, windows []*Window) {
	t.Helper()
	if len(windows) != 3 {
		t.Fatalf("got %d window links, want 3", len(windows))
	}
	want := []string{"alpha:1:@1:true", "beta:7:@1:false", "beta:8:@2:true"}
	for i, w := range windows {
		got := fmt.Sprintf("%s:%d:%s:%t", w.Session, w.Index, w.Id, w.Active)
		if got != want[i] {
			t.Errorf("row %d: got %s, want %s", i, got, want[i])
		}
	}
}

func TestListAllWindowsAggregateSuccess(t *testing.T) {
	for _, empty := range []bool{false, true} {
		t.Run(fmt.Sprintf("empty=%t", empty), func(t *testing.T) {
			rows := linkedWindowRows()
			if empty {
				rows = nil
			}
			tmux, tr := aggregateTmux(t, []scriptedResponse{aggregateResponse("list-windows -a", rows...)})
			windows, err := tmux.ListAllWindowsContext(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			assertCommandCount(t, tr, 1)
			if empty {
				if len(windows) != 0 {
					t.Fatalf("expected empty result, got %v", windows)
				}
			} else {
				assertLinkedWindows(t, windows)
			}
		})
	}
}

func TestListAllPanesAggregateSuccess(t *testing.T) {
	vars := newQuery(nil).paneVars().variables
	for _, empty := range []bool{false, true} {
		t.Run(fmt.Sprintf("empty=%t", empty), func(t *testing.T) {
			rows := []string{
				formatRecord(vars, map[string]string{varPaneId: "%1", varPaneWindowIndex: "1"}),
				formatRecord(vars, map[string]string{varPaneId: "%1", varPaneWindowIndex: "7"}),
				formatRecord(vars, map[string]string{varPaneId: "%2", varPaneWindowIndex: "8"}),
			}
			if empty {
				rows = nil
			}
			tmux, tr := aggregateTmux(t, []scriptedResponse{aggregateResponse("list-panes -a", rows...)})
			panes, err := tmux.ListAllPanesContext(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			assertCommandCount(t, tr, 1)
			if len(panes) != len(rows) {
				t.Fatalf("got %d rows, want %d", len(panes), len(rows))
			}
			if !empty && (panes[0].WindowIndex != 1 || panes[1].WindowIndex != 7 || panes[2].Id != "%2") {
				t.Fatalf("aggregate pane rows changed: %+v", panes)
			}
		})
	}
}

func TestListAllWindowsFallbackLinkedWindows(t *testing.T) {
	vars := newQuery(nil).sessionVars().variables
	rows := linkedWindowRows()
	responses := []scriptedResponse{
		aggregateError("list-windows -a"),
		aggregateResponse("list-sessions",
			formatRecord(vars, map[string]string{varSessionName: "alpha", varSessionId: "$1"}),
			formatRecord(vars, map[string]string{varSessionName: "gone", varSessionId: "$2"}),
			formatRecord(vars, map[string]string{varSessionName: "beta", varSessionId: "$3"})),
		aggregateResponse("list-windows -t $1", rows[0]),
		aggregateError("list-windows -t $2"),
		aggregateError("list-windows -t gone"),
		aggregateResponse("list-windows -t $3", rows[1], rows[2], rows[2]),
	}
	tmux, tr := aggregateTmux(t, responses)
	windows, err := tmux.ListAllWindowsContext(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assertLinkedWindows(t, windows)
	assertCommandCount(t, tr, len(responses))
}

func TestListAllPanesFallbackVanishedWindow(t *testing.T) {
	wvars := newQuery(nil).windowVars().variables
	pvars := newQuery(nil).paneVars().variables
	responses := []scriptedResponse{
		aggregateError("list-panes -a"),
		aggregateResponse("list-windows -a",
			formatRecord(wvars, map[string]string{varWindowId: "@1"}),
			formatRecord(wvars, map[string]string{varWindowId: "@2"}),
			formatRecord(wvars, map[string]string{varWindowId: "@3"})),
		aggregateResponse("list-panes -t @1", formatRecord(pvars, map[string]string{varPaneId: "%1"})),
		aggregateError("list-panes -t @2"),
		aggregateResponse("list-panes -t @3", formatRecord(pvars, map[string]string{varPaneId: "%3"})),
	}
	tmux, tr := aggregateTmux(t, responses)
	panes, err := tmux.ListAllPanesContext(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(panes) != 2 || panes[0].Id != "%1" || panes[1].Id != "%3" {
		t.Fatalf("unexpected panes: %+v", panes)
	}
	assertCommandCount(t, tr, len(responses))
}

func awaitSentCommand(t *testing.T, sent <-chan string, prefix string) {
	t.Helper()
	select {
	case cmd := <-sent:
		if !strings.HasPrefix(cmd, prefix) {
			t.Fatalf("got command %q, want prefix %q", cmd, prefix)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for command")
	}
}

func TestListAllFallbackCancellation(t *testing.T) {
	for _, panes := range []bool{false, true} {
		for _, partial := range []bool{false, true} {
			t.Run(fmt.Sprintf("panes=%t/partial=%t", panes, partial), func(t *testing.T) {
				tmux, tr := silentTmux(t)
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				type outcome struct {
					err     error
					hasRows bool
				}
				result := make(chan outcome, 1)
				go func() {
					if panes {
						rows, err := tmux.ListAllPanesContext(ctx)
						result <- outcome{err: err, hasRows: rows != nil}
					} else {
						rows, err := tmux.ListAllWindowsContext(ctx)
						result <- outcome{err: err, hasRows: rows != nil}
					}
				}()
				direct, enumerate, perResource := "list-windows -a", "list-sessions", "list-windows -t $1"
				vars := newQuery(nil).sessionVars().variables
				rows := []string{formatRecord(vars, map[string]string{varSessionId: "$1", varSessionName: "alpha"}), formatRecord(vars, map[string]string{varSessionId: "$2", varSessionName: "beta"})}
				if panes {
					direct, enumerate, perResource = "list-panes -a", "list-windows -a", "list-panes -t @1"
					vars = newQuery(nil).windowVars().variables
					rows = []string{formatRecord(vars, map[string]string{varWindowId: "@1"}), formatRecord(vars, map[string]string{varWindowId: "@2"})}
				}
				awaitSentCommand(t, tr.sentC, direct)
				for _, line := range aggregateError(direct).lines {
					tr.lines <- line
				}
				awaitSentCommand(t, tr.sentC, enumerate)
				if partial {
					for _, line := range aggregateResponse(enumerate, rows...).lines {
						tr.lines <- line
					}
					awaitSentCommand(t, tr.sentC, perResource)
					row := linkedWindowRows()[0]
					next := "list-windows -t $2"
					if panes {
						row = formatRecord(newQuery(nil).paneVars().variables, map[string]string{varPaneId: "%1"})
						next = "list-panes -t @2"
					}
					for _, line := range aggregateResponse(perResource, row).lines {
						tr.lines <- line
					}
					awaitSentCommand(t, tr.sentC, next)
				}
				cancel()
				select {
				case got := <-result:
					if !errors.Is(got.err, context.Canceled) || got.hasRows {
						t.Fatalf("got error=%v, partial rows=%t; want cancellation and nil rows", got.err, got.hasRows)
					}
				case <-time.After(2 * time.Second):
					t.Fatal("fallback did not cancel")
				}
				select {
				case cmd := <-tr.sentC:
					t.Fatalf("extra fallback command: %s", cmd)
				default:
				}
			})
		}
	}
}

func TestListAllFallbackPreservesDirectError(t *testing.T) {
	for _, panes := range []bool{false, true} {
		for _, failedRecovery := range []bool{false, true} {
			t.Run(fmt.Sprintf("panes=%t/failed-recovery=%t", panes, failedRecovery), func(t *testing.T) {
				direct, recovery := "list-windows -a", "list-sessions"
				if panes {
					direct, recovery = "list-panes -a", "list-windows -a"
				}
				directFailure := aggregateError(direct)
				directFailure.lines[1] = "original aggregate failure"
				responses := []scriptedResponse{directFailure, aggregateResponse(recovery)}
				if failedRecovery {
					responses[1] = aggregateError(recovery)
					if panes {
						responses = append(responses, aggregateError("list-sessions"))
					}
				}
				tmux, tr := aggregateTmux(t, responses)
				var err error
				if panes {
					var rows []*Pane
					rows, err = tmux.ListAllPanesContext(context.Background())
					if rows != nil {
						t.Errorf("expected nil panes on error")
					}
				} else {
					var rows []*Window
					rows, err = tmux.ListAllWindowsContext(context.Background())
					if rows != nil {
						t.Errorf("expected nil windows on error")
					}
				}
				var commandErr *commandError
				if !errors.As(err, &commandErr) || commandErr.Message != "original aggregate failure" || !strings.HasPrefix(commandErr.Command, direct) {
					t.Fatalf("original aggregate error lost: %v", err)
				}
				assertCommandCount(t, tr, len(responses))
			})
		}
	}
}
