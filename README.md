gotmuxcc
========

`gotmuxcc` is a work-in-progress replacement for
[`github.com/GianlucaP106/gotmux`](https://github.com/GianlucaP106/gotmux).
It mirrors the public API of the original library while internally using a
persistent tmux control-mode connection instead of spawning a new `tmux`
process for every command. This greatly reduces overhead when issuing many
tmux operations from Go.

## Status

- Public structs and method signatures are kept compatible with gotmux.
- Session, window, pane, and option helpers are fully backed by the control
  transport.
- Control-mode router, query builder, and transport management are complete.
- Integration tests cover server info, client listing, and session lifecycle.

Remaining work includes enhanced documentation, expanded integration tests, and
final parity checks.

## Getting Started

Install the module in a Go project:

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
client, err := gotmuxcc.NewTmux("/path/to/socket")
```

## Testing

Integration tests require tmux to be installed and able to create UNIX socket
files in the test temporary directory. See `docs/testing.md` for details and for
instructions on running the test suite inside restricted sandboxes. The helper
scripts in `scripts/` can be used to separate fast and full runs:

```bash
# Fast unit tests only
scripts/test-unit.sh

# Full suite including integration specs (requires tmux)
scripts/test-integration.sh
```

## Documentation

- API inventory mirroring gotmux: `docs/api_inventory.md`
- Test guidance: `docs/testing.md`

## License

This project inherits the MIT-style licensing approach from gotmux; the final
text will be provided alongside the first public release.
