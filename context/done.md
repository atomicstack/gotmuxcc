## Current state

Here’s what’s happened so far:
- Catalogued the original gotmux public API surface in docs/api_inventory.md for
  one-to-one compatibility tracking.
- Initialized the gotmuxcc Go module, mirrored exported type definitions, and
  added constructor options to inject a control-mode transport.
- Implemented the initial control-mode transport (internal/control) that launches
  `tmux -C`, handles stdio pipes, and exposes a line-oriented stream plus lifecycle
  management.
