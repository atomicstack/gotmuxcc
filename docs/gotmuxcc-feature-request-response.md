# gotmuxcc feature request response

**date:** 2026-03-20
**source:** ~/git_tree/tmux-popup-control/docs/gotmuxcc-feature-request.md

## summary

all 7 items from the feature request have been addressed. 3 were already
implemented; 4 were added in this session.

## item status

| # | feature | status | notes |
|---|---------|--------|-------|
| 1 | `SessionOptions.Command` | **already done** | the existing `SessionOptions.ShellCommand` field does exactly this — it appends as the last positional arg to `new-session`. use `ShellCommand` instead of `Command`. |
| 2 | `NewWindowOptions` index/detach/cmd | **done** | added `Index *int` and `ShellCommand string` fields. `Index` is a pointer because index 0 is valid in tmux. the feature request's `Detached` field was not added separately — the existing `DoNotAttach bool` already maps to `-d` with identical semantics. |
| 3 | split-window detach/command | **done** | added `Detached bool` to `SplitWindowOptions`. added `Tmux.SplitWindow(target string, opts *SplitWindowOptions) error` for target-string-based splitting. existing `Pane.SplitWindow()` still works as before. `ShellCommand` was already present on `SplitWindowOptions`. |
| 4 | `GlobalOption()` query | **done** | added `Tmux.GlobalOption(key string) (string, error)`. uses `show-option -g -q -v <key>`. returns empty string (not error) for unset options. |
| 5 | target-based `SelectLayout` | **done** | added `Tmux.SelectLayout(target string, layout string) error`. takes raw `string` for layout (not the `WindowLayout` enum) so custom checksum layout strings work. |
| 6 | target-based `SelectPane` | **already done** | `Tmux.SelectPane(target string) error` already exists. |
| 7 | target-based `SelectWindow` | **already done** | `Tmux.SelectWindow(target string) error` already exists. |

## api usage examples

### creating a session with a startup command (item 1)

```go
sess, err := tmux.NewSession(&gotmuxcc.SessionOptions{
    Name:           "mysession",
    StartDirectory: "/tmp",
    ShellCommand:   "cat /tmp/pane.txt; exec bash",
})
```

### creating a window at a specific index (item 2)

```go
idx := 2
win, err := sess.NewWindow(&gotmuxcc.NewWindowOptions{
    WindowName:   "editor",
    StartDirectory: "/tmp",
    DoNotAttach:  true,       // -d flag (same as feature request's "Detached")
    Index:        &idx,       // -t session:2
    ShellCommand: "cat /tmp/pane.txt; exec bash",
})
```

note: `Index` is `*int`. use `&idx` or `intPtr(2)` helper. nil means "no index
specified" (tmux assigns next available). `&0` means "target index 0".

### splitting a pane by target string (item 3)

```go
err := tmux.SplitWindow("mysession:2", &gotmuxcc.SplitWindowOptions{
    SplitDirection: gotmuxcc.PaneSplitDirectionHorizontal,
    StartDirectory: "/tmp",
    Detached:       true,
    ShellCommand:   "cat /tmp/pane.txt; exec bash",
})
```

the existing `Pane.SplitWindow()` also supports the new `Detached` field.

### querying a global option (item 4)

```go
val, err := tmux.GlobalOption("@tmux-popup-control-session-storage-dir")
// val is "" if option is not set (no error)
```

### applying a layout by target string (item 5)

```go
err := tmux.SelectLayout("mysession:2", "bb62,80x24,0,0{40x24,0,0,1,39x24,41,0,2}")
```

### selecting a pane/window by target (items 6-7)

```go
err := tmux.SelectPane("mysession:2.1")
err := tmux.SelectWindow("mysession:2")
```

## commits

```
a5bb9f3 docs: update context files for target-string APIs
0738282 feat: add Tmux.SelectLayout() for target-based layout selection
ffbb3c9 feat: add Tmux.GlobalOption() for server-level option queries
7a1ce5f feat: add Detached to SplitWindowOptions; add Tmux.SplitWindow()
e429228 feat: add Index and ShellCommand fields to NewWindowOptions
```

## known limitations

`ShellCommand` values containing literal single quotes will produce malformed
command strings. this is a pre-existing limitation across all three option
structs (`SessionOptions`, `NewWindowOptions`, `SplitWindowOptions`) — they all
use manual single-quote wrapping via `fmt.Sprintf("'%s'", ...)` for the
positional arg. a hardening fix is tracked in gotmuxcc's todo.md.
