## Cross-session notes:

This is a section which can be used as a cross-session scratchpad to carry any
useful information needed about the work, or the execution environment, across
chat sessions.

- During agentic work, in the shell the python interpreter is named python3
  (though the use of Perl is preferable to other scripting languages)
- Integration tests require `GOTMUXCC_INTEGRATION=1` and a tmux-enabled
  environment; otherwise skip logic handles missing sockets/clients.
- Unit tests rely on fake control transports (`record`/`simple`) to avoid
  invoking tmux while validating command issuance.
- Coverage work uses `go test ./... -coverprofile=coverage.out` and `go tool
  cover -func=coverage.out`.
