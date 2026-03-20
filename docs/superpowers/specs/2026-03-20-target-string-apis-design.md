# target-string APIs and missing surfaces

**date:** 2026-03-20
**source:** ~/git_tree/tmux-popup-control/docs/gotmuxcc-feature-request.md

## context

tmux-popup-control's session restore creates sessions, windows, and panes in
bulk by target string (e.g. `"mysession:2"`, `"mysession:2.1"`). objects don't
exist yet during restore, so the object-based APIs can't be used. this spec
adds the missing fields and target-string methods to eliminate raw
`client.Command()` fallbacks.

## scope

7 items were requested. 3 are already implemented:

| # | requested feature | status |
|---|-------------------|--------|
| 1 | `SessionOptions.Command` | **done** — `ShellCommand` field already exists |
| 2 | `NewWindowOptions` index/detach/command | **this spec** |
| 3 | split-window detach/command + target method | **this spec** |
| 4 | `GlobalOption()` query | **this spec** |
| 5 | target-based `SelectLayout` | **this spec** |
| 6 | target-based `SelectPane` | **done** — `Tmux.SelectPane(target)` exists |
| 7 | target-based `SelectWindow` | **done** — `Tmux.SelectWindow(target)` exists |

## design

### item 2: NewWindowOptions — add Index, Detached, ShellCommand

**types.go** — extend `NewWindowOptions`:

```go
type NewWindowOptions struct {
    StartDirectory string
    WindowName     string
    DoNotAttach    bool
    Index          int    // new: window index within session (-t session:index)
    Detached       bool   // new: -d flag (don't move focus)
    ShellCommand   string // new: startup command (last positional arg)
}
```

note: `DoNotAttach` already maps to `-d`. `Detached` is a distinct field because
the feature request distinguishes them and callers may set both. however, their
tmux effect is identical (`-d`). to avoid confusion, `Detached` should be
treated as an alias — the implementation emits `-d` if either is true.

**window.go** — update `Session.NewWindow()`:

- when `Index > 0`: target becomes `sessionName:index` instead of `sessionName`
- when `Detached` is true: add `-d` flag (same as `DoNotAttach`)
- when `ShellCommand` is non-empty: append as positional arg via `pargs`

### item 3: SplitWindowOptions — add Detached; new Tmux.SplitWindow()

**types.go** — extend `SplitWindowOptions`:

```go
type SplitWindowOptions struct {
    SplitDirection PaneSplitDirection
    StartDirectory string
    ShellCommand   string
    Detached       bool   // new: -d flag
}
```

`ShellCommand` already exists for the startup command positional arg.

**pane.go** — two changes:

1. update existing `Pane.SplitWindow()` to honor `Detached` field (add `-d` flag)
2. new `Tmux.SplitWindow(target string, opts *SplitWindowOptions)` method that
   takes a raw target string instead of a `Pane` receiver. same command
   construction, just with caller-supplied target.

### item 4: Tmux.GlobalOption()

**options.go** — new method:

```go
func (t *Tmux) GlobalOption(key string) (string, error)
```

issues `show-option -gqv <key>`. the `-q` flag suppresses "unknown option"
errors — tmux returns empty output when the option doesn't exist. returns the
trimmed value string, or empty string if unset.

### item 5: Tmux.SelectLayout()

**window.go** — new method:

```go
func (t *Tmux) SelectLayout(target string, layout string) error
```

same pattern as existing `Tmux.SelectPane()` and `Tmux.SelectWindow()`: builds
`select-layout -t <target> <layout>` via the query builder.

## testing

each item gets unit tests using the existing recording/fake transport pattern:

- **item 2**: verify `Session.NewWindow()` emits correct command string with
  index targeting, `-d` flag, and positional shell command
- **item 3**: verify `Pane.SplitWindow()` emits `-d` when `Detached` is true;
  verify `Tmux.SplitWindow()` emits correct command with caller-supplied target
- **item 4**: verify `Tmux.GlobalOption()` emits `show-option -gqv <key>` and
  parses the single-line response; verify empty response for unset option
- **item 5**: verify `Tmux.SelectLayout()` emits `select-layout -t <target> <layout>`

integration tests are deferred — the unit tests with fake transports provide
sufficient confidence for these straightforward command-building changes.

## files changed

| file | change |
|------|--------|
| `gotmuxcc/types.go` | add fields to `NewWindowOptions`, `SplitWindowOptions` |
| `gotmuxcc/window.go` | update `Session.NewWindow()`; add `Tmux.SelectLayout()` |
| `gotmuxcc/pane.go` | update `Pane.SplitWindow()`; add `Tmux.SplitWindow()` |
| `gotmuxcc/options.go` | add `Tmux.GlobalOption()` |
| `gotmuxcc/window_test.go` | unit tests for items 2, 5 |
| `gotmuxcc/pane_test.go` | unit tests for item 3 |
| `gotmuxcc/options_test.go` | unit tests for item 4 |
