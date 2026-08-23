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

- **`%begin` flags hardening is deliberately soft**: `handleBegin` refuses to pair
  a `%begin` whose flags field is not `1` with a queued request only while the
  initial attach block is still outstanding. tmux sets that field from
  `!!(state->flags & CMDQ_STATE_CONTROL)` (verified back to 3.0a), so a strict
  "flags must be 1" rule would be more defensive — but it would make gotmuxcc hang
  on any tmux that ever emitted `0` for a typed command, and it would require
  rewriting all 112 `%begin … 0` fixtures in the unit tests. Revisit if a minimum
  tmux version is ever declared.

- **Upstream, not gotmuxcc — `capture-pane -p` truncates at NUL**: tmux writes the
  buffer with `control_write(c, "%.*s", (int)len, buf)` (`cmd-capture-pane.c:441`)
  while `control_write_line` does `strlen(line)` (`control.c:398`), so captured
  content containing a NUL byte is silently truncated. Nothing gotmuxcc can do;
  recorded so it isn't re-diagnosed as a library bug.

- **`gofmt` reports `gotmuxcc/conversion_test.go`**: four map entries in the
  window-conversion fixture are indented with one tab instead of two. Pre-existing
  (present at `440c9d0`), whitespace only.

## Remaining tmux control-mode feature gaps
These were identified by comparing gotmuxcc against the tmux C source and have
not yet been implemented:

- **Clipboard request** (`refresh-client -l`): request terminal clipboard via
  xterm escape sequences.
