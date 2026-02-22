# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

gotmuxcc is a Go library that provides a drop-in replacement for [gotmux](https://github.com/GianlucaP106/gotmux). It mirrors gotmux's public API but internally uses a persistent tmux control-mode (`tmux -C`) connection instead of spawning a new tmux process per command. Module path: `github.com/atomicstack/gotmuxcc`.

There is a `context/` directory containing project-specific context files (todo.md, done.md, scratchpad.md, context.md). Read these at the start of any session for current project state and pending work. Update todo.md and done.md when completing work items.

The tmux source code is available at `/Users/matt/git_tree/tmux` for reference when working on control-mode protocol details.

## Build & Test Commands

```bash
make unit              # Fast unit tests (no tmux required)
make integration       # Full suite including integration tests (requires tmux)
make cover             # Generate coverage report to coverage.out
make clean             # Remove local .gocache and .gomodcache

# Run a single test
go test ./gotmuxcc -run TestName
go test ./internal/control -run TestName

# Integration tests require the env var:
GOTMUXCC_INTEGRATION=1 go test -tags integration ./...
```

The Makefile pins `GOCACHE` and `GOMODCACHE` to local directories (`.gocache/`, `.gomodcache/`) for sandbox compatibility. The test scripts in `scripts/` do the same.

## Architecture

### Layer Stack (top to bottom)

1. **Public API** (`gotmuxcc/`) — `Tmux` struct is the entry point. Methods on `Tmux`, `Session`, `Window`, and `Pane` mirror the gotmux API. Created via `NewTmux(socketPath)` or `DefaultTmux()`.

2. **Query Builder** (`gotmuxcc/query.go`) — Fluent builder that assembles tmux commands with format-variable (`#{...}`) substitution, separated by `"-:-"`. Parses multi-field responses back into `queryResult` maps.

3. **Router** (`gotmuxcc/router.go`) — Concurrency-safe command dispatcher. Tracks pending/inflight commands, parses `%begin/%end/%error` control-mode frames, correlates responses, and surfaces async `%`-prefixed events. Runs a read loop goroutine and a transport-done observer.

4. **Control Bridge** (`gotmuxcc/control_bridge.go`) — Wires the `controlTransport` interface to the concrete `internal/control.Transport`. Discovers an existing session to attach to on startup.

5. **Control Transport** (`internal/control/transport.go`) — Launches `tmux -C` as a subprocess, manages stdin/stdout/stderr pipes, and exposes a line-oriented channel interface (`Send`, `Lines`, `Done`, `Close`).

### Key Interfaces

- `controlTransport` (defined in `gotmuxcc/tmux.go`) — abstraction over the tmux pipe; satisfied by `internal/control.Transport` in production and by fake/recording transports in tests.
- `Dialer` / `DialerFunc` — factory for `controlTransport`; injectable via `WithDialer()` constructor option for testing.

### Testing Strategy

- **Unit tests** use fake control transports (recording/simple fakes) that validate issued commands without invoking tmux. These live alongside source files as `*_test.go`.
- **Integration tests** (guarded by `GOTMUXCC_INTEGRATION=1` and `//go:build integration`) spin up isolated tmux servers with temporary sockets. They create temp directories under the repo root (`.testtmp/`) so filesystem writes stay in-sandbox.
- Test helpers live in `internal/testutil/`.

### Tracing

Set `GOTMUXCC_TRACE=1` (or component names like `router,transport`) to enable debug logging. Trace output goes to `gotmuxcc_trace.log` in the repo root (override with `GOTMUXCC_TRACE_FILE`).

### Other Notable Files

- `gotmuxcc/types.go` — All public struct definitions (Session, Window, Pane, Client, Server, option types)
- `gotmuxcc/variables.go` — tmux format variable name constants
- `gotmuxcc/helpers.go` — Shared parsing utilities (atoi, isOne, parseList, etc.)
- `gotmuxcc/options.go` — Option get/set/delete helpers across session/window/pane scopes
- `docs/api_inventory.md` — Tracks one-to-one API compatibility with gotmux
