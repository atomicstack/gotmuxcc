# TODO

## Summary
- Automate coverage reporting in CI (ensure scripts enforce minimum threshold and fail below 100%).

## Remaining tmux control-mode feature gaps
These were identified by comparing gotmuxcc against the tmux C source and have
not yet been implemented:

- **Clipboard request** (`refresh-client -l`): request terminal clipboard via
  xterm escape sequences.
