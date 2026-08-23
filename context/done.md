## Current state

Here’s what’s happened so far:
- Catalogued the original gotmux public API surface in docs/api_inventory.md for
  one-to-one compatibility tracking.
- Initialized the gotmuxcc Go module, mirrored exported type definitions, and
  added constructor options to inject a control-mode transport.
- Implemented the initial control-mode transport (internal/control) that launches
  `tmux -C`, handles stdio pipes, and exposes a line-oriented stream plus lifecycle
  management.
- Renamed the public package to `gotmuxcc`, updated constructors, and ensured the
  module builds cleanly under the new name.
- Built a concurrency-safe command router (gotmuxcc/router.go) that tracks pending
  commands, parses `%begin/%end/%error` frames, captures output, and surfaces
  asynchronous `%` events for observers.
- Implemented the query builder atop the control-mode transport (gotmuxcc/query.go),
  including format-variable handling and result collection helpers.
- Ported tmux server and client listing APIs using the new query layer, restoring
  `GetServerInformation` and `ListClients` compatibility.
- Added integration-style unit tests that bootstrap isolated tmux instances where
  permitted, covering server info, client listing, and session lifecycle behaviour.
- Ported session management APIs (session discovery, creation, rename, options,
  detach/switch helpers) to the control-mode backend.
- Ported window and pane APIs, including manipulation helpers and pane capture,
  to operate over the persistent control-mode transport.
- Completed utility parity by hardening option helpers, command passthrough
  quoting, and socket validation so existing gotmux integrations behave
  consistently on the control-mode backend.
- Documented local testing strategy and expanded README guidance for using
  gotmuxcc, including socket configuration notes.
- Added integration coverage for window/pane lifecycle to guard multi-pane
  workflows.
- Completed final API parity review and drafted an incremental plan for
  broadening automated test coverage ahead of release.
- Implemented tmux-level `capture-pane` support via the control-mode transport,
  exposing both direct and pane-scoped helpers.
- Added router unit tests with a fake control transport and query builder
  parsing tests to seed the upcoming coverage expansion.
- Added integration coverage for the control transport lifecycle, exercising
  stdout/stderr handling and process shutdown behaviour.
- Expanded session integration tests to cover detach/switch flows and session
  enumeration helpers under tmux control mode.
- Added integration coverage for window movement/layout updates and pane
  splits/key sending/capture workflows under control mode.
- Added unit tests for option helpers across session/window/pane scopes using
  a recording transport to validate issued tmux commands.
- Added unit coverage for raw command quoting to ensure control-mode commands
  escape arguments correctly.
- Added unit tests for socket validation by stubbing tmux interactions to cover
  success and failure paths without requiring tmux.
- Introduced `scripts/test-unit.sh` and `scripts/test-integration.sh` to
  separate fast unit runs from full integration suites for CI usage.
- Added conversion helper tests covering client/server/session/window/pane
  struct population and helper utility functions.
- Exercised query output helpers (run/collect/one/raw) through router-driven
  fakes to ensure control-mode responses are parsed correctly.
- Added constructor tests for Tmux (custom dialers, context propagation) to
  ensure entry points behave as expected without invoking tmux.
- Added session command tests using a recording transport to cover list/new/detach/
  switch/kill flows.
- Added window and pane command tests using a fake transport to exercise list/
  move/rename/split/choose-tree and related control-mode commands.
- Introduced a Makefile providing `make unit`, `make integration`, and `make clean`
  targets to streamline running unit and full integration test suites.
- Ensured integration tests create temporary directories under the repository
  root so filesystem writes stay within the sandbox.
- Added GOTMUXCC_TRACE tracing support (with optional GOTMUXCC_TRACE_FILE) across
  router and transport paths to help diagnose test hangs.
- Extended router unit tests to exercise enqueue error handling, stack trimming,
  orphan/unknown output events, and unexpected frame error emission for fuller
  coverage of edge behaviours.
- Expanded option and command unit tests to cover deletions, error propagation
  across scopes, multi-line command output, and raw command error wrapping to
  raise coverage.
- Normalised GOTMUXCC_TRACE logging to drop trace files in the repo root even
  when tests run from package subdirectories.
- Added control transport unit tests using scripted fake tmux binaries to cover
  successful command flow and stderr-driven failure propagation.
- Fixed ListAllWindows/ListAllPanes to fall back to per-session queries when
  control-mode "%all" queries return empty, matching gotmux behavior and added
  unit coverage for the regression scenario.
- Patched query result parsing to tolerate separator characters inside fields
  and enriched window records with owning session metadata so downstream
  consumers see complete session/window snapshots.
- Added a default 3Hz command rate limiter in the router with tests to cap tmux
  polling frequency and avoid flooding the backend with repeated queries.
- Added an end-to-end tmux integration scenario that renames windows, splits
  panes, sends commands, and asserts session/window/pane state through the API.
- Performed a gap analysis between gotmuxcc and tmux's control-mode API surface
  by reading the tmux C source (control.c, control-notify.c, cmd-refresh-client.c,
  etc.) and cataloguing every missing feature.
- Implemented tmux control-mode subscriptions (`refresh-client -B`): Subscribe()
  and Unsubscribe() methods with support for session, pane, window, all-panes,
  and all-windows subscription targets (subscription.go).
- Implemented control client window sizing (`refresh-client -C`): SetClientSize(),
  SetWindowSize(), and ClearWindowSize() methods (refresh.go).
- Implemented `%exit` notification handling in the router: the router now
  intercepts `%exit`, emits an exit event, and fails all pending/inflight
  commands with a typed ExitError that supports errors.Is(err, ErrServerExit).
- Implemented octal escape decoding for `%output` notification data: decodeOctal()
  handles tmux's \xxx encoding of non-printable bytes and backslashes (octal.go).
- Added 24 typed notification structs (notification.go) covering every tmux
  control-mode notification: OutputNotification (with octal decoding),
  ExtendedOutputNotification, LayoutChangeNotification,
  SubscriptionChangedNotification, SessionChangedNotification,
  SessionRenamedNotification, SessionWindowChangedNotification,
  WindowAddNotification, WindowCloseNotification, WindowRenamedNotification,
  WindowPaneChangedNotification, UnlinkedWindow* variants,
  PaneModeChangedNotification, ClientSessionChangedNotification,
  ClientDetachedNotification, SessionsChangedNotification,
  PasteBuffer{Changed,Deleted}Notification, Pause/ContinueNotification,
  ConfigErrorNotification, MessageNotification, ExitNotification.
  Event.Notification() parses from Raw to correctly handle notifications with
  spaces in data payloads and colon-separated values.
- Added missing pane operations: Pane.Rename(), Swap(), Move(), Break(), Join(),
  Resize() plus top-level Tmux.RenamePane(), SwapPanes(), MovePane(), BreakPane(),
  JoinPane(), ResizePane(), SelectPane() methods.
- Added missing window operations: Window.Unlink(), Link(), MoveToSession(),
  Swap() plus top-level Tmux.UnlinkWindow(), LinkWindow(), MoveWindowToSession(),
  SwapWindows(), SelectWindow() methods.
- Added Tmux.DisplayMessage(target, format) for arbitrary format string evaluation
  via `display-message -p`, enabling CurrentClientID and similar queries.
- Extended CaptureOptions with StartLine and EndLine fields mapping to
  capture-pane -S/-E flags for scrollback capture.
- Added custom format list methods: Tmux.ListSessionsFormat(),
  ListWindowsFormat(), ListPanesFormat() accepting arbitrary -F format strings
  and optional -f filters, returning raw []string results for consumer code that
  uses env-var-driven custom format strings.
- Implemented flow control (refresh-client -f/-A): SetControlFlags() for
  arbitrary flag strings, EnablePauseAfter()/DisablePauseAfter() convenience
  methods, and per-pane output control via SetPaneOutput(),
  EnablePaneOutput(), DisablePaneOutput(), PausePaneOutput(),
  ContinuePaneOutput(), SetMultiplePaneOutputs() for batch operations.
- Implemented pane color reports (refresh-client -r): ReportPaneColors() relays
  terminal color query responses (OSC 10/11) back to tmux for specific panes.
- tmux-popup-control now uses gotmuxcc as intended: a single long-lived control-mode connection shared across all 28 call sites, rather than spawning a new `tmux -C` subprocess per operation. This validates the persistent-connection design and exercises the library under sustained reuse.
- Added initial control-mode handshake handling in the router: tmux emits an
  implicit `%begin/%end` pair when a control-mode session starts (from
  `attach-session` or `new-session`). The router now absorbs this pair via
  `initialCmd` tracking and a `ready` channel, preventing the startup frames
  from being mismatched with the first user command. `NewTmuxWithOptions`
  blocks until the handshake completes so callers get a fully-settled router.
  Added `newRouterWithInit()` constructor for tests to opt out of waiting, a
  `seedInitialHandshake()` helper for constructor tests, and three new router
  tests covering absorbed initial pairs, initial output, and pre-handshake
  notification events.
- Fixed router.enqueue() concurrency bug: moved transport.Send() inside the
  mutex so the pending queue append and send are atomic, preventing response
  mismatch when multiple goroutines call runCommand() concurrently. Added
  TestRouterEnqueueOrderMatchesSendOrder to verify ordering under contention.
- Fixed CapturePane returning empty output in control mode: modified
  router.handleLine() so that when inside a command response (stack non-empty),
  lines starting with % that don't match frame markers (%begin, %end, %error,
  %exit) are treated as command output via appendOutput() instead of being
  consumed as events. tmux's capture-pane -p sends raw pane content between
  %begin/%end without escaping %-prefixed lines. Added unit tests verifying
  %-prefixed lines are captured as output inside commands and still emitted as
  events outside commands.
- Fixed `%error` frame handler to extract error text from accumulated output
  lines (between `%begin` and `%error`) instead of looking for text in the
  `%error` frame itself. tmux's `cmdq_guard` emits `%error <time> <number>
  <flags>` with no trailing text; error messages are written as output via
  `cmdq_error` → `cmdq_print` before the `%error` guard. Previously all
  errors were reported as "tmux reported an error" when `rest` was empty.
  Updated all test fixtures to match the real tmux frame format.
- Fixed `fargs` values not being quoted in query command strings. Applied
  `quoteArgument()` to all flagArgs in `query.build()` so values containing
  spaces or special characters are properly shell-quoted. Removed redundant
  `quoteArgument()` calls from `display.go` callers that were manually
  quoting before passing to `fargs`.
- Fixed `quoteArgument()` to also quote strings containing tmux parser special
  characters (`#`, `;`, `{`, `}`, `~`). In tmux's command parser (cmd-parse.y),
  unquoted `#` triggers comment handling — everything from `#` to end-of-line
  is ignored. This caused `display-message -p #{client_name}` to lose its
  format argument, falling back to `DISPLAY_MESSAGE_TEMPLATE` (the default
  status line format). Now format variables like `#{client_name}` are properly
  single-quoted when passed as fargs values.
- Added `Index *int` and `ShellCommand string` fields to `NewWindowOptions` for
  creating windows at specific indices with startup commands.
- Added `Detached bool` field to `SplitWindowOptions` and new
  `Tmux.SplitWindow(target, opts)` method for target-string-based pane splitting.
- Added `Tmux.GlobalOption(key)` method for querying server-level global options
  via `show-option -gqv`.
- Added `Tmux.SelectLayout(target, layout)` method for applying layout strings
  to windows by target string.
- Hardened the control transport against orphaning `tmux -C` on abnormal consumer
  exit (crash/signal/os.Exit without Close). added build-tagged `sysProcAttr()`
  helpers: linux sets `Pdeathsig: SIGKILL` (full parent-death cleanup) plus
  `Setpgid`; darwin/bsd set `Setpgid` only (no pdeathsig equivalent — Close
  remains mandatory); non-unix returns nil. `New` now also derives an internal
  cancelable lifetime context that `Close()` cancels, giving a second
  CommandContext-based kill path. exposed `NewTmuxContext(ctx, socket, ...)` so
  consumers can bind a client's lifetime to e.g. signal.NotifyContext. documented
  the macOS Close() requirement on `(*Tmux).Close`. tests cover the platform
  helpers and constructor threading.
- Hardened the router's control-mode framing so pane content can never be parsed
  as protocol (`gotmuxcc/router.go`). While a `%begin` block is open every line is
  now command output until the exact `%end`/`%error` guard for that block arrives
  — matching the contract tmux asserts in `regress/control-notify-guard.sh` after
  upstream `6db5175e` queued notifications out of guard blocks. Previously a
  `capture-pane -p` line starting `%exit` tore the whole connection down, a
  captured `%begin` mis-bound the next queued request, and a captured `%end` whose
  command number collided completed a command early. Guards are now matched as
  whole keywords (so `%beginning`/`%errors`/`%exited` stay notifications) and must
  carry exactly three decimal fields; `%exit` is only honoured at depth 0. A
  `%begin` whose flags field is not `1` is never paired with a queued request,
  since tmux sets that field only for commands the control client typed.
- Added `pane_flags`, `pane_floating_flag`, `pane_modal_flag`, `pane_x`, `pane_y`,
  `pane_z`, `pane_unzoomed_width`, `pane_unzoomed_height` to `paneVars()` and
  `window_modal_pane`, `window_manual_width`, `window_manual_height` to
  `windowVars()`, with matching `Pane`/`Window` fields. tmux 3.8 keeps floating
  panes in the window's ordinary pane list, so without these they are
  indistinguishable from tiled panes in `list-panes` output.
