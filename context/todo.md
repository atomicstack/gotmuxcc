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

- **`%begin` flags hardening — DONE (2026-09-12)**: the soft rule was the bug, not
  the safe option. A command run by a hook is inserted into this client's own queue
  and guarded like any other, so it arrives as an unrequested `%begin/%end` pair with
  flags `0`; pairing that with `pending[0]` handed the request the hook's output and
  put every later reply off by one. `handleBegin` now never pairs a non-`1` block with
  a queued request and tracks it under a sentinel label instead. The 222 `%begin … 0`
  fixtures were corrected to `1` — real tmux never sends `0` for a typed command — and
  the handshake/initial-attach fixtures, which genuinely carry `0`, were left alone.

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
  xterm escape sequences. Note the semantics changed upstream in `322adfbd`: the
  forward-to-pane ability was removed from `-l`, and forwarding is now controlled by
  a `get-clipboard` server option (`options-table.c:437`), off by default.
