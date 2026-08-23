# gotmuxcc phantom session bug response

**date:** 2026-03-20
**source:** ~/git_tree/tmux-popup-control/docs/gotmuxcc-phantom-session-bug.md

## summary

the reported bug was real. when `gotmuxcc.NewTmux()` connected to a tmux server
with no sessions, the constructor could launch bare `tmux -C`, which tmux
interpreted as an implicit `new-session`. that created an unwanted session as a
side effect of opening the control-mode connection.

the constructor path has been changed so it no longer falls through to bare
`tmux -C` in the empty-server case.

## changes made

the fix is implemented in `gotmuxcc/control_bridge.go`.

1. constructor startup now uses an explicit startup plan rather than a
   best-effort `initialAttachArgs()` fallback.
2. if an existing session is found, gotmuxcc still starts control mode with:

```text
tmux -C ... attach-session -t <session>
```

3. if no sessions exist, gotmuxcc now starts control mode with:

```text
tmux -C ... new-session -d -s __gotmuxcc_bootstrap_<pid>_<nsec>
```

4. the temporary bootstrap session name is explicit and unique, so gotmuxcc no
   longer steals the next numeric session name such as `0`.
5. unexpected `list-sessions` failures now surface as constructor errors
   instead of being silently treated as "no sessions".
6. when the `Tmux` handle is closed, gotmuxcc now does a best-effort
   `kill-session -t <bootstrap>` cleanup for the temporary session it created.

## tests added

unit coverage was added for:

- existing-session startup
- empty-server startup using a named bootstrap session
- propagation of unexpected discovery errors
- bootstrap-session cleanup on `Close()`

the focused test run used during this change was:

```bash
GOCACHE=$(pwd)/.gocache go test ./gotmuxcc ./internal/control
```

## information learned during implementation

the original proposed fix in the bug report was not sufficient as written.

### bare `tmux -C` really does create a session

this was reproduced against a real tmux server. if the server had no sessions,
running bare `tmux -C` caused tmux to create session `0`.

### `tmux -C new-session -d` still creates a phantom session

adding only `-d` changes the attachment behavior, but it does not avoid session
creation. it still creates a session, and without an explicit `-s` name it can
still consume the first numeric session name.

this means the real minimum fix is not "use `-d`"; it is "use an explicit
temporary session name".

### detached control-mode startup still emits the expected startup handshake

`tmux -C new-session -d -s <name>` still emits the initial `%begin/%end`
control-mode block, so the existing router readiness logic did not need to be
rewritten for this fix.

### immediate cleanup is not safe when the bootstrap session is the only session

if the temporary bootstrap session is killed immediately and it is the only
remaining session, tmux exits the server. that would break the control-mode
client and is worse than the original bug.

as a result, cleanup is currently tied to `Tmux.Close()` instead of happening
immediately after startup.

## caveats

the current implementation fixes the reported numeric phantom-session bug, but
it does not completely eliminate the existence of a bootstrap session.

while a `Tmux` handle remains open against a server that still has no real user
sessions, the named bootstrap session will remain present. this is intentional:
it keeps the control-mode client and server alive.

for short-lived startup callers, this is usually acceptable because the
bootstrap session is cleaned up when the handle closes. for long-lived callers,
the bootstrap session may remain visible in `list-sessions` and related APIs
until `Close()` is called.

## downstream impact

for downstream users such as tmux startup hooks:

- the constructor no longer steals the first numeric session name
- a temporary session with a `__gotmuxcc_bootstrap_...` name may exist briefly
- callers that only need startup-time work should still close the `Tmux` handle
  promptly so cleanup can run

## future work

the main follow-up improvement would be retiring the bootstrap session
automatically once a real non-bootstrap session exists and it is safe to move
or detach the control-mode client without collapsing the server.

that requires more care than this fix because tmux exits a server with no
sessions, and the control-mode client is still fundamentally attached to some
session for its lifetime.
