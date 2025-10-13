# Testing gotmuxcc

This project relies on integration-style tests that exercise tmux control mode.
To avoid interfering with user sessions, each test spins up a temporary tmux
server bound to a fresh socket under the Go test temporary directory.

## Prerequisites

- `tmux` must be available on `PATH`.
- The user running the tests must be allowed to create unix-domain sockets in
  the temporary directory (on macOS this may require running outside of a
  sandboxed filesystem).
- Tests clear the `TMUX` environment variable to ensure they are not attached to
  an existing tmux server.

If the environment does not satisfy these requirements (for example, the
filesystem disallows socket creation), the integration tests will skip and note
the reason.

## Running Tests

```bash
GOCACHE=$(pwd)/.gocache go test ./...
```

`GOCACHE` is set to a local directory to ensure compatibility with sandboxed
environments that disallow writing to the default Go build cache.

## Test Coverage

Current integration coverage includes:

- Fetching server information via control mode.
- Listing clients.
- Session lifecycle: creation, rename, option helpers, and teardown.

Future work may extend coverage to multi-window/multi-pane workflows once those
APIs are fully implemented.
