package trace

import (
	"strings"
	"testing"
)

func TestFormatControlCommand(t *testing.T) {
	t.Parallel()

	if got := FormatControlCommand("  new-session -d  "); got != "new-session -d" {
		t.Fatalf("unexpected command formatting: %q", got)
	}

	if got := FormatControlCommand("   "); got != "<empty>" {
		t.Fatalf("expected empty placeholder, got %q", got)
	}

	long := strings.Repeat("a", controlCommandLimit+10)
	got := FormatControlCommand(long)
	if !strings.HasSuffix(got, " (truncated)") {
		t.Fatalf("expected truncation suffix, got %q", got)
	}
	expectedLen := controlCommandLimit + len(" (truncated)")
	if len(got) != expectedLen {
		t.Fatalf("expected length %d, got %d", expectedLen, len(got))
	}
}

func TestFormatControlLine(t *testing.T) {
	t.Parallel()

	input := "%begin 1697048557 42 2001"
	if got := FormatControlLine(input); got != input {
		t.Fatalf("line should remain unchanged, got %q", got)
	}

	if got := FormatControlLine(""); got != "<empty>" {
		t.Fatalf("expected empty placeholder, got %q", got)
	}
}

func TestSummariseControlLines(t *testing.T) {
	t.Parallel()

	if got := SummariseControlLines(nil); got != "lines=0" {
		t.Fatalf("expected lines=0, got %q", got)
	}

	lines := []string{"alpha", "beta", "gamma"}
	summary := SummariseControlLines(lines)
	if !strings.Contains(summary, "lines=3") || !strings.Contains(summary, "alpha | beta | gamma") {
		t.Fatalf("unexpected summary output: %q", summary)
	}
	if strings.Contains(summary, "(truncated)") {
		t.Fatalf("did not expect truncation notice in %q", summary)
	}

	longLine := strings.Repeat("x", controlOutputLineLimit+10)
	longSummary := SummariseControlLines([]string{longLine})
	if !strings.Contains(longSummary, "(truncated)") {
		t.Fatalf("expected truncation for long line, got %q", longSummary)
	}

	manyLines := []string{"l0", "l1", "l2", "l3", "l4", "l5", "l6"}
	manySummary := SummariseControlLines(manyLines)
	if !strings.Contains(manySummary, "(+2 more lines)") {
		t.Fatalf("expected preview note, got %q", manySummary)
	}
	if !strings.HasSuffix(manySummary, " (truncated)") {
		t.Fatalf("expected truncated suffix, got %q", manySummary)
	}
}
