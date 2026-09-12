package gotmuxcc

import (
	"context"
	"fmt"
	"strings"
)

func (t *Tmux) SetOption(target, key, value, level string) error {
	q := t.query().
		cmd("set-option")

	if level != "" {
		q.fargs(level)
	}

	q.fargs("-t", target).
		pargs(key, value)

	if _, err := q.run(); err != nil {
		return fmt.Errorf("failed to set option: %w", err)
	}

	return nil
}

func (t *Tmux) Option(target, key, level string) (*Option, error) {
	q := t.query().
		cmd("show-option")
	if level != "" {
		q.fargs(level)
	}
	q.fargs("-t", target).
		fargs("-v", key)

	output, err := q.run()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve option: %w", err)
	}

	raw := strings.TrimSpace(output.raw())
	return newOption(key, raw), nil
}

// GlobalOption queries a server-level (global) option by key.
// Returns the value, or empty string if the option is not set.
// The -q flag suppresses errors for unknown options.
func (t *Tmux) GlobalOption(key string) (string, error) {
	q := t.query().
		cmd("show-option").
		fargs("-g", "-q", "-v", key)

	output, err := q.run()
	if err != nil {
		return "", fmt.Errorf("failed to retrieve global option: %w", err)
	}

	return strings.TrimSpace(output.raw()), nil
}

func (t *Tmux) Options(target, level string) ([]*Option, error) {
	q := t.query().
		cmd("show-options")
	if level != "" {
		q.fargs(level)
	}
	q.fargs("-t", target)

	output, err := q.run()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve options: %w", err)
	}

	return output.toOptions(), nil
}

func (t *Tmux) DeleteOption(target, key, level string) error {
	q := t.query().
		cmd("set-option")
	if level != "" {
		q.fargs(level)
	}
	q.fargs("-t", target).
		fargs("-u", key)

	if _, err := q.run(); err != nil {
		return fmt.Errorf("failed to delete option: %w", err)
	}
	return nil
}

func (t *Tmux) Command(parts ...string) (string, error) {
	return t.CommandContext(context.Background(), parts...)
}

// CommandContext is Command with cancellation. It is the raw escape hatch, so a
// context here also covers the helpers that route through it. Cancellation can
// abandon a blocked send or response wait. Once writing starts, the command may
// still execute; cancellation does not remove its response slot or replay it.
func (t *Tmux) CommandContext(ctx context.Context, parts ...string) (string, error) {
	command, err := buildCommand(parts)
	if err != nil {
		return "", err
	}
	result, err := t.runCommandContext(ctx, command)
	if err != nil {
		return "", fmt.Errorf("failed to run command: %w", err)
	}
	return strings.Join(result.Lines, "\n"), nil
}

func newOption(key, value string) *Option {
	return &Option{Key: key, Value: value}
}

func (o *queryOutput) toOptions() []*Option {
	lines := o.result.Lines
	options := make([]*Option, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			continue
		}
		options = append(options, newOption(parts[0], parts[1]))
	}
	return options
}

func buildCommand(parts []string) (string, error) {
	if len(parts) == 0 {
		return "", errEmptyCommand
	}
	escaped := make([]string, 0, len(parts))
	for _, part := range parts {
		escaped = append(escaped, quoteArgument(part))
	}
	return strings.Join(escaped, " "), nil
}

// needsPercentQuoting reports whether tmux's parser would reject arg because
// it begins with "%".
//
// tmux lexes a word starting with "%" as a conditional keyword unless every
// character of that word is "%" or a digit (cmd-parse.y, yylex): the scan
// breaks at the first other character, and what follows must then be one of
// %hidden/%if/%else/%elif/%endif or the token is a syntax error. So a bare
// pane target such as "%0" parses fine, while "%0:on" — the argument shape
// refresh-client -A and -r take — does not, and must be quoted.
//
// A "%" anywhere but the first byte is an ordinary character and needs no
// quoting, so this deliberately does not fire on, say, "50%".
func needsPercentQuoting(arg string) bool {
	if !strings.HasPrefix(arg, "%") {
		return false
	}
	for _, r := range arg {
		if r != '%' && (r < '0' || r > '9') {
			return true
		}
	}
	return false
}

func quoteArgument(arg string) string {
	if arg == "" {
		return "''"
	}
	// strip control characters that could inject commands into the
	// line-oriented control-mode protocol.
	arg = sanitizeControlArg(arg)
	if needsPercentQuoting(arg) || strings.ContainsAny(arg, " \t'\"\\#;{}~") {
		escaped := strings.ReplaceAll(arg, "'", "'\\''")
		return "'" + escaped + "'"
	}
	return arg
}

// sanitizeControlArg removes characters that are unsafe in the tmux
// control-mode wire protocol: newlines (which split a single command
// into two), carriage returns, and null bytes.
func sanitizeControlArg(s string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '\n', '\r', '\x00':
			return -1
		default:
			return r
		}
	}, s)
}
