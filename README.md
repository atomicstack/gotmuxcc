gotmuxcc
========

`gotmuxcc` is a drop-in replacement for
[`github.com/GianlucaP106/gotmux`](https://github.com/GianlucaP106/gotmux).
It mirrors the public API of the original library while internally using a
persistent tmux control-mode (`tmux -C`) connection instead of spawning a new
`tmux` process for every command. This dramatically reduces overhead when
issuing many tmux operations from Go.

## Features

- **API-compatible with gotmux** — public structs, method signatures, and
  return types match the original library so switching requires only a module
  path change.
- **Persistent control-mode connection** — a single `tmux -C` subprocess is
  kept alive for the lifetime of the `Tmux` handle; all commands flow over
  stdin/stdout pipes.
- **Concurrency-safe router** — commands are dispatched, correlated with
  `%begin/%end/%error` frames, and returned to callers without external
  locking. The router automatically absorbs the initial control-mode
  handshake so the first user command is never mismatched.
- **24 typed notification structs** — every tmux control-mode notification
  (`%output`, `%layout-change`, `%session-changed`, etc.) is parsed into a
  concrete Go type via `Event.Notification()`.
- **Subscriptions** — `Subscribe`/`Unsubscribe` wrappers for
  `refresh-client -B` with helpers for session, window, and pane targets.
- **Flow control** — pause-after mode, per-pane output control, and arbitrary
  control flag management via `SetControlFlags`.
- **Client & window sizing** — `SetClientSize`, `SetWindowSize`, and
  `ClearWindowSize` for programmatic layout control.
- **Custom format queries** — `ListSessionsFormat`, `ListWindowsFormat`, and
  `ListPanesFormat` accept arbitrary `-F` format strings and `-f` filters.
- **Target-string APIs** — `Tmux.SplitWindow`, `Tmux.SelectLayout`,
  `Tmux.SelectPane`, `Tmux.SelectWindow`, and `Tmux.GlobalOption` accept raw
  tmux target strings, enabling bulk session/window/pane creation during
  restore without needing materialized objects.
- **Debug tracing** — set `GOTMUXCC_TRACE=1` (or component names like
  `router,transport`) to write structured debug logs to a trace file.

## Getting Started

Requires Go 1.22+ and a working tmux installation.

Install the module:

```bash
go get github.com/atomicstack/gotmuxcc/gotmuxcc
```

Create a client bound to the default tmux socket:

```go
package main

import (
    "fmt"

    "github.com/atomicstack/gotmuxcc/gotmuxcc"
)

func main() {
    tmux, err := gotmuxcc.DefaultTmux()
    if err != nil {
        panic(err)
    }
    defer tmux.Close()

    info, err := tmux.GetServerInformation()
    if err != nil {
        panic(err)
    }

    fmt.Printf("tmux version: %s\n", info.Version)
}
```

To point at a custom socket:

```go
tmux, err := gotmuxcc.NewTmux("/path/to/socket")
```

## Architecture

The library is organized into five layers (top to bottom):

| Layer | Package | Responsibility |
|-------|---------|----------------|
| Public API | `gotmuxcc/` | `Tmux`, `Session`, `Window`, `Pane` types that mirror gotmux |
| Query Builder | `gotmuxcc/query.go` | Fluent command assembly with `#{...}` format variables |
| Router | `gotmuxcc/router.go` | Concurrency-safe dispatch, frame parsing, event streaming |
| Control Bridge | `gotmuxcc/control_bridge.go` | Wires the transport interface to the concrete implementation |
| Control Transport | `internal/control/` | Launches and manages the `tmux -C` subprocess |

The `controlTransport` interface (defined in `gotmuxcc/tmux.go`) abstracts the
tmux pipe. In production it is satisfied by `internal/control.Transport`; in
tests it is replaced by fake or recording transports injected via
`WithDialer()`.

## Testing

Unit tests use fake control transports and do not require tmux:

```bash
make unit
```

Integration tests spin up isolated tmux servers with temporary sockets and
require tmux to be installed:

```bash
make integration
```

Other targets:

```bash
make cover   # Generate coverage report to coverage.out
make clean   # Remove local .gocache and .gomodcache
```

See `docs/testing.md` for details on running tests inside restricted sandboxes.

## Documentation

- `docs/api_inventory.md` — one-to-one API compatibility catalog with gotmux
- `docs/testing.md` — test prerequisites, integration test setup, coverage notes

## License

MIT License. See [LICENSE](LICENSE) for details.
