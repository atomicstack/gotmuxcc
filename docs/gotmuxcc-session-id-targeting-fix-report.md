# gotmuxcc session id targeting fix report

**date:** 2026-03-31
**planned release:** `v0.1.3`
**bug report:** `~/git_tree/tmux-popup-control/docs/gotmuxcc-session-id-targeting.md`

## summary

the reported bug has been fixed in gotmuxcc.

the root cause was correct: several `Session` methods targeted tmux sessions by
session name, and tmux prefix-matches name-based targets. that could make
operations against `claude` also hit `claude2`.

gotmuxcc now targets sessions by `session_id` when an id is available, which
gives exact tmux matching. for compatibility, manually constructed
`Session{Name: ...}` values still fall back to name-based targeting when no id
is present.

## fixes included

- `Session.Detach()` now prefers `Session.Id` over `Session.Name`
- `Session.Kill()` now prefers `Session.Id` over `Session.Name`
- `Session.Rename()` now prefers `Session.Id` over `Session.Name`
- `Session.Rename()` now updates the in-memory `Session.Name` after success

the same id-first targeting was also applied to other session-scoped helpers
that send a session target to tmux:

- `Session.AttachSession()`
- `Session.SetOption()`
- `Session.Option()`
- `Session.Options()`
- `Session.DeleteOption()`
- `Session.ListPanes()`
- `Session.NewWindow()`
- `Session.NextWindow()`
- `Session.PreviousWindow()`

## compatibility

the public API is unchanged.

behavior after the fix:

- sessions returned by gotmuxcc APIs use exact id-based targeting
- callers that manually build `Session{Name: "..."}`
  still work because the library falls back to the session name if `Id` is empty

## validation

unit and integration coverage were added for this bug.

validated with:

- `GOCACHE=$(pwd)/.gocache go test ./...`
- `GOCACHE=$(pwd)/.gocache GOTMUXCC_INTEGRATION=1 go test ./... -tags integration -count=1 -v`

the integration suite includes a direct reproduction of the reported case:

1. create two sessions with overlapping names
2. kill the first session through gotmuxcc
3. verify the second session still exists

## outcome

the original workaround in downstream code should no longer be necessary for
session kill, detach, or rename operations when using this release.
