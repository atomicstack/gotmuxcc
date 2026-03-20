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
| 2 | `NewWindowOptions` index/command | **this spec** |
| 3 | split-window detach/command + target method | **this spec** |
| 4 | `GlobalOption()` query | **this spec** |
| 5 | target-based `SelectLayout` | **this spec** |
| 6 | target-based `SelectPane` | **done** — `Tmux.SelectPane(target)` exists |
| 7 | target-based `SelectWindow` | **done** — `Tmux.SelectWindow(target)` exists |

## design

### item 2: NewWindowOptions — add Index, ShellCommand

**types.go** — extend `NewWindowOptions`:

```go
type NewWindowOptions struct {
    StartDirectory string
    WindowName     string
    DoNotAttach    bool
    Index          *int   // new: window index within session (-t session:index)
    ShellCommand   string // new: startup command (last positional arg)
}
```

`Index` is `*int` (pointer) because tmux window index 0 is valid and common
(the default `base-index`). a nil pointer means "no index specified" while
`&0` means "target index 0". this avoids zero-value ambiguity.

the feature request's `Detached` field is not added — the existing `DoNotAttach`
field already maps to `-d` with identical semantics.

**window.go** — update `Session.NewWindow()`:

- when `Index != nil`: target becomes `sessionName:*index` instead of
  `sessionName`
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

1. update existing `Pane.SplitWindow()` to honor `Detached` field (add `-d`
   flag when true)
2. new `Tmux.SplitWindow(target string, opts *SplitWindowOptions) error` method
   that takes a raw target string instead of a `Pane` receiver. same command
   construction, just with caller-supplied target. returns `error` only (no pane
   output), matching the existing `Pane.SplitWindow()` return signature. the
   restore use case addresses panes by target string and doesn't need a
   materialized `Pane` object back.

### item 4: Tmux.GlobalOption()

**options.go** — new method:

```go
func (t *Tmux) GlobalOption(key string) (string, error)
```

issues `show-option` with flags `-g`, `-q`, `-v` and the key as a flag
argument. this is a distinct code path from the existing `Option()` method —
`Option()` always includes `-t <target>`, but global options use `-g` with no
target.

the `-q` flag suppresses "unknown option" errors — tmux returns empty output
when the option doesn't exist. returns the trimmed value string, or empty
string if unset.

### item 5: Tmux.SelectLayout()

**window.go** — new method:

```go
func (t *Tmux) SelectLayout(target string, layout string) error
```

same pattern as existing `Tmux.SelectPane()` and `Tmux.SelectWindow()`: builds
`select-layout -t <target> <layout>` via the query builder.

the `layout` parameter is `string` (not `WindowLayout`) because the restore use
case passes custom layout strings with checksums (e.g.
`"bb62,80x24,0,0{40x24,0,0,1,39x24,41,0,2}"`) that don't map to the
predefined `WindowLayout` enum values.

## testing

each item gets unit tests using the existing recording/fake transport pattern:

- **item 2**: verify `Session.NewWindow()` emits correct command string with
  index targeting, `-d` flag, and positional shell command
- **item 3**: verify `Pane.SplitWindow()` emits `-d` when `Detached` is true;
  verify `Tmux.SplitWindow()` emits correct command with caller-supplied target
- **item 4**: verify `Tmux.GlobalOption()` emits `show-option -g -q -v <key>`
  and parses the single-line response; verify empty response for unset option
- **item 5**: verify `Tmux.SelectLayout()` emits
  `select-layout -t <target> <layout>`

integration tests are deferred — the unit tests with fake transports provide
sufficient confidence for these straightforward command-building changes.

## files changed

| file | change |
|------|--------|
| `gotmuxcc/types.go` | add fields to `NewWindowOptions`, `SplitWindowOptions` |
| `gotmuxcc/window.go` | update `Session.NewWindow()`; add `Tmux.SelectLayout()` |
| `gotmuxcc/pane.go` | update `Pane.SplitWindow()`; add `Tmux.SplitWindow()` |
| `gotmuxcc/options.go` | add `Tmux.GlobalOption()` |
| `gotmuxcc/window_ops_test.go` | unit tests for items 2, 5 |
| `gotmuxcc/pane_ops_test.go` | unit tests for item 3 |
| `gotmuxcc/options_test.go` | unit tests for item 4 |
