# Overview:
You're an expert Golang dev helping me produce a new module which provides an
API for interfacing with a running tmux session.

For some background: I am also working on a separate project which uses the
gotmux library to control and inspect a running tmux session. However, I
discovered the gotmux library does all its work by calling tmux via the shell.
I'd prefer to have a library which exposes an API similar to gotmux, so I don't
have to refactor my existing code (too much), but that replacement library
should spawn a Goroutine which has a persistent control mode connection open to
tmux that it can use to read and write from the tmux instance, instead of
invoking the tmux binary via exec (or its equivalent) and reading from its
stdout.

Therefore, the goal of this repository, gotmuxcc, is to implement a
public-facing API as close to the existing gotmux library as possible, but
which privately uses a persistent, open socket to tmux for all its reads and
writes, avoiding persistently invoking the tmux binary over and over when
making frequent calls to the API.

You can refer to a clone of the tmux source code itself at: `/Users/matt/git_tree/tmux`

## Design goals:

## Context updates:
