# TODO

## Summary
- Automate coverage reporting in CI (ensure scripts enforce minimum threshold and fail below 100%).

## Code hardening

- **ShellCommand quoting**: `SessionOptions.ShellCommand`, `NewWindowOptions.ShellCommand`,
  and `SplitWindowOptions.ShellCommand` all use manual single-quote wrapping via
  `fmt.Sprintf("'%s'", op.ShellCommand)` when appending positional args. If the value
  contains a literal single quote (e.g. `echo 'hello'`), the resulting command string
  is malformed. Fix: route through `quoteArgument()` instead, which already handles
  embedded single quotes via `'\''` escaping. Affected files: `session.go:137`,
  `window.go:518`, `pane.go:234`.

- **pargs quoting asymmetry**: `query.build()` applies `quoteArgument()` to `flagArgs`
  but not to `posArgs`. This is why shell commands need manual pre-quoting. Consider
  either documenting this in a comment on `pargs()`/`build()`, or applying
  `quoteArgument()` to posArgs too (would require removing the manual quoting at all
  call sites).

## Remaining tmux control-mode feature gaps
These were identified by comparing gotmuxcc against the tmux C source and have
not yet been implemented:

- **Clipboard request** (`refresh-client -l`): request terminal clipboard via
  xterm escape sequences.
