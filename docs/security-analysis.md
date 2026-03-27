# security analysis — gotmuxcc

**date:** 2026-03-27
**scope:** all go source on the `main` branch at commit `9111a04`
**threat model:** a go application uses gotmuxcc as a library and passes
user-controlled strings (session names, shell commands, format strings,
target identifiers) into its public API.

## executive summary

gotmuxcc communicates with tmux over a control-mode (`tmux -C`) stdin pipe.
commands are written as plain-text lines — one command per line. tmux's own
parser tokenises these lines (not `/bin/sh`), so classic shell injection
(`; && | $()`) does not apply. however, the line-oriented protocol creates a
different injection surface: **any embedded newline (`\n`) in a command string
splits it into two separate tmux commands**, enabling arbitrary tmux command
execution.

the most critical class of bug is **newline injection via unquoted or
insufficiently quoted positional arguments**. three `ShellCommand` sites, the
`SendKeys` API, and several rename/layout methods pass user-controlled strings
into `pargs()`, which applies **no quoting or sanitisation** before the string
reaches the transport layer.

## findings

### 1. newline injection via `ShellCommand` (high)

**files:**
- `gotmuxcc/session.go:137`
- `gotmuxcc/window.go:518`
- `gotmuxcc/pane.go:234`

all three sites wrap `ShellCommand` with manual single-quoting and pass through
`pargs`, which applies no quoting:

```go
q.pargs(fmt.Sprintf("'%s'", op.ShellCommand))
```

**problems:**
1. `pargs` appends arguments raw — `quoteArgument` is never called.
2. the manual `'%s'` wrapping does not escape embedded single quotes (compare to
   `quoteArgument` which correctly replaces `'` with `'\''`).
3. an embedded `\n` in `ShellCommand` terminates the current protocol line and
   injects an arbitrary second command.

**example:** `ShellCommand: "bash\nkill-server"` produces the wire bytes:

```
new-session ... 'bash
kill-server'
```

tmux reads the first line as `new-session ... 'bash` and the second line as a
new command `kill-server'` — executing it.

### 2. newline injection via `SendKeys` and other `pargs` call sites (high)

**files:**
- `gotmuxcc/pane.go:176` — `SendKeys(line)` → `pargs(line)` (highest risk)
- `gotmuxcc/session.go:282` — `Rename(name)` → `pargs(name)`
- `gotmuxcc/window.go:126` — `Rename(newName)` → `pargs(newName)`
- `gotmuxcc/window.go:151` — `SelectLayout(layout)` → `pargs(string(layout))`
- `gotmuxcc/window.go:701` — `Tmux.SelectLayout(target, layout)` → `pargs(layout)`

`pargs` in `query.build()` (query.go:76) appends positional arguments to the
command string **without any quoting or sanitisation**:

```go
parts = append(parts, q.posArgs...)
```

by contrast, `fargs` (query.go:58-60) runs each argument through
`quoteArgument`. this asymmetry means all positional arguments are injection
vectors if they contain newlines, and correctness bugs if they contain spaces.

### 3. `quoteArgument` does not reject control characters (medium)

**file:** `gotmuxcc/options.go:135-144`

`quoteArgument` detects `\n` in its trigger set and wraps the result in single
quotes. this is correct for shell quoting but **not for the control-mode
protocol**: single-quoting does not prevent the transport from splitting the
line on embedded newlines. the function needs to strip or reject `\n`, `\r`,
and `\x00` rather than just quoting them.

### 4. unquoted `windowID` interpolation in `refresh.go` (medium)

**file:** `gotmuxcc/refresh.go:22-35`

`SetWindowSize` and `ClearWindowSize` interpolate `windowID` directly into the
command string via `fmt.Sprintf` without `quoteArgument`:

```go
cmd := fmt.Sprintf("refresh-client -C %s:%dx%d", windowID, width, height)
```

a `windowID` containing spaces or newlines breaks the protocol. the expected
format is `@N` but nothing enforces this.

### 5. subscription name colon injection (medium)

**file:** `gotmuxcc/subscription.go:38`

`Subscribe` builds the `-B` argument by concatenating `name:target:format` with
colons. if `name` contains a colon, it shifts the delimiter boundaries —
tmux parses `sub:evil` as name=`sub`, target=`evil`, not name=`sub:evil`.

### 6. `checkSessionName` does not reject control characters (medium)

**file:** `gotmuxcc/helpers.go:8-19`

`checkSessionName` rejects `:` and `.` but allows newlines, carriage returns,
and null bytes. additionally, it is only called in `NewSession` — not in
`Rename`, `HasSession`, `Kill`, etc.

### 7. trace log created world-readable (low)

**file:** `internal/trace/trace.go:105`

the trace log is opened with mode `0644`. when tracing is enabled, the log
contains all command strings and responses, which may include sensitive data
(session names, option values, pane titles). the file should use `0600`.

### 8. test temp directory created world-accessible (low)

**file:** `internal/testutil/tempdir.go:53`

`.testtmp/` base directory is created with mode `0755`. test tmux sockets
placed inside are discoverable by other local users. should use `0700`.

### 9. format string APIs accept untrusted input (low, by design)

**file:** `gotmuxcc/display.go`

`DisplayMessage`, `ListSessionsFormat`, `ListWindowsFormat`, and
`ListPanesFormat` pass caller-supplied format strings directly to tmux. a
format string containing `#{...}` expressions is evaluated by tmux, potentially
disclosing session/environment information. this is by-design for the API but
should be documented as a trust boundary.

### 10. `GOTMUXCC_TRACE_FILE` allows arbitrary path writes (low)

**file:** `internal/trace/trace.go:93-95`

when tracing is enabled, `GOTMUXCC_TRACE_FILE` specifies the log path with no
validation. an attacker who controls environment variables could write trace
data to arbitrary paths (limited by process permissions).

## summary table

| # | severity | category | files |
|---|----------|----------|-------|
| 1 | **high** | newline injection via ShellCommand | session.go, window.go, pane.go |
| 2 | **high** | newline injection via unquoted pargs | pane.go, session.go, window.go |
| 3 | **medium** | quoteArgument permits control chars | options.go |
| 4 | **medium** | unquoted windowID interpolation | refresh.go |
| 5 | **medium** | subscription name colon injection | subscription.go |
| 6 | **medium** | checkSessionName permits control chars | helpers.go |
| 7 | **low** | trace log world-readable | trace.go |
| 8 | **low** | test temp dir world-accessible | tempdir.go |
| 9 | **low** | format APIs trust boundary | display.go |
| 10 | **low** | trace file path injection | trace.go |

## fixes applied

this branch synthesises fixes from two independent security reviews:

- `security-hardening` branch (2026-03-27) — primary review, all fixes
- `codex-security-analysis-2026-03-24` branch (2026-03-24) — additional fixes

where both reviews identified the same issue, the `security-hardening` fix was
preferred. the codex review contributed two additional fixes not covered by the
primary review.

### from security-hardening (primary)

1. **strip control characters in `quoteArgument`** — `sanitizeControlArg`
   removes `\n`, `\r`, `\x00` before quoting (options.go).

2. **quote positional args in `query.build()`** — `quoteArgument` applied to
   `posArgs` the same way it is applied to `flagArgs` (query.go).

3. **remove manual `ShellCommand` quoting** — the three `'%s'` wrappers become
   plain `pargs(op.ShellCommand)` (session.go, window.go, pane.go).

4. **quote `windowID` in refresh.go** — `quoteArgument` applied to the composed
   `-C` argument in `SetWindowSize` and `ClearWindowSize`.

5. **validate subscription names** — reject colons in the `name` parameter
   (subscription.go).

6. **harden `checkSessionName`** — reject `\n`, `\r`, `\x00` in addition to
   `:` and `.` (helpers.go).

7. **defence-in-depth in `transport.Send()`** — reject commands containing
   embedded newlines at the transport layer (transport.go).

8. **tighten file permissions** — trace log to `0600` (trace.go), test temp dir
   to `0700` (tempdir.go).

### from codex review (additional)

9. **quote format strings via `quoteArgument`** — the `query.build()` format
   string was manually wrapped with `fmt.Sprintf("'%s'", format)`, bypassing
   `sanitizeControlArg`. now uses `quoteArgument(format)` so control characters
   are stripped consistently (query.go).

10. **route `SetClientSize` through query builder** — converted from raw
    `fmt.Sprintf`/`runCommand` to use the query builder, consistent with the
    other refresh-client helpers (refresh.go).

### remaining open items

- the control transport still uses `bufio.Scanner` with a 10 MiB line limit.
  a process inside a pane can force the client connection to drop by emitting a
  sufficiently long newline-free line (availability, not injection).
- format string APIs (`DisplayMessage`, `ListSessionsFormat`, etc.) pass
  caller-supplied format strings directly to tmux by design — documented as a
  trust boundary.
- `GOTMUXCC_TRACE_FILE` allows arbitrary path writes when tracing is enabled
  and an attacker controls environment variables.
