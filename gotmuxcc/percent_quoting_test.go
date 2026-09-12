package gotmuxcc

import "testing"

// TestQuoteArgumentPercentRule pins gotmuxcc's copy of tmux's leading-"%"
// lexer rule (cmd-parse.y, yylex): a word starting with "%" is a conditional
// keyword unless every character of it is "%" or a digit.
//
// Quoting too little is a syntax error from tmux; quoting too much changes the
// wire format of every pane-targeted command, so both directions matter.
func TestQuoteArgumentPercentRule(t *testing.T) {
	tests := []struct {
		name string
		arg  string
		want string
	}{
		// All "%" or digits: a valid token, left alone so pane targets keep
		// their existing wire format.
		{"bare pane id", "%0", "%0"},
		{"multi digit pane id", "%1234", "%1234"},
		{"lone percent", "%", "%"},
		{"double percent", "%%", "%%"},

		// Anything else after a leading "%" must be quoted.
		{"pane output request", "%0:on", "'%0:on'"},
		{"pane colour report", "%5:rgb:1111/2222/3333", "'%5:rgb:1111/2222/3333'"},
		{"all panes subscription target", "%*", "'%*'"},
		{"conditional keyword", "%if", "'%if'"},
		{"hidden keyword", "%hidden", "'%hidden'"},

		// A "%" that is not the first byte is an ordinary character.
		{"trailing percent", "50%", "50%"},
		{"embedded percent", "a%b", "a%b"},

		// The pre-existing triggers must still fire.
		{"space", "two words", "'two words'"},
		{"format variable", "#{session_name}", "'#{session_name}'"},
		{"empty", "", "''"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := quoteArgument(tt.arg); got != tt.want {
				t.Errorf("quoteArgument(%q) = %q, want %q", tt.arg, got, tt.want)
			}
		})
	}
}
