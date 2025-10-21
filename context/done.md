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
