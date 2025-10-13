# TODO

## Summary
- Build a concurrency-safe command router: serialize outgoing commands,
  correlate them with %begin/%end response frames, buffer output, and surface
  errors, while allowing asynchronous %event messages to be observed or
  discarded.
- Design a query layer atop the router: replicate the existing query builder
  semantics (command + flags + format variables) so higher-level code can stay
  mostly identical, but now emits control-mode commands and consumes structured
  responses.
- Re-implement server/client listing: port GetServerInformation, ListClients,
  and related helpers first to validate the transport, formatting, and parsing
  logic.
- Port session management APIs: migrate session creation, lookup,
  attach/detach, rename, and option handling, ensuring conversions to Session
  structs match current field expectations.
- Port window and pane features: implement listing, creation, movement, layout
  selection, splitting, resizing, and key sending, confirming control-mode
  commands match tmux semantics.
- Support utility APIs: handle options, raw command passthrough (Command),
  socket validation, and any remaining helpers so compatibility coverage is
  complete.
- Add verification and docs: create integration tests that spin up ephemeral
  tmux servers, document the control-mode architecture, and note any behavioral
  differences or required environment setup for users migrating from gotmux.
