package gotmuxcc

import (
	"bytes"
	"testing"
)

func TestDecodeOctalPrintable(t *testing.T) {
	got := decodeOctal("hello world")
	if !bytes.Equal(got, []byte("hello world")) {
		t.Fatalf("expected 'hello world', got %q", got)
	}
}

func TestDecodeOctalNewline(t *testing.T) {
	got := decodeOctal(`hello\012world`)
	if !bytes.Equal(got, []byte("hello\nworld")) {
		t.Fatalf("expected newline, got %q", got)
	}
}

func TestDecodeOctalBackslash(t *testing.T) {
	got := decodeOctal(`path\134file`)
	if !bytes.Equal(got, []byte(`path\file`)) {
		t.Fatalf("expected backslash, got %q", got)
	}
}

func TestDecodeOctalTab(t *testing.T) {
	got := decodeOctal(`col1\011col2`)
	if !bytes.Equal(got, []byte("col1\tcol2")) {
		t.Fatalf("expected tab, got %q", got)
	}
}

func TestDecodeOctalMultiple(t *testing.T) {
	got := decodeOctal(`\033[31mred\033[0m`)
	expected := []byte("\033[31mred\033[0m")
	if !bytes.Equal(got, expected) {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestDecodeOctalEmpty(t *testing.T) {
	got := decodeOctal("")
	if len(got) != 0 {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestDecodeOctalTrailingBackslash(t *testing.T) {
	// Trailing backslash without enough digits should be literal.
	got := decodeOctal(`end\`)
	if !bytes.Equal(got, []byte(`end\`)) {
		t.Fatalf("expected literal trailing backslash, got %q", got)
	}
}

func TestDecodeOctalPartialEscape(t *testing.T) {
	// Backslash followed by fewer than 3 digits should be literal.
	got := decodeOctal(`\01x`)
	if !bytes.Equal(got, []byte(`\01x`)) {
		t.Fatalf("expected literal partial escape, got %q", got)
	}
}

func TestDecodeOctalNonOctalDigits(t *testing.T) {
	// \n is not an octal escape (n is not an octal digit).
	got := decodeOctal(`\nop`)
	if !bytes.Equal(got, []byte(`\nop`)) {
		t.Fatalf("expected literal, got %q", got)
	}
}

func TestDecodeOctalNullByte(t *testing.T) {
	got := decodeOctal(`\000`)
	if !bytes.Equal(got, []byte{0}) {
		t.Fatalf("expected null byte, got %q", got)
	}
}

func TestDecodeOctalMaxByte(t *testing.T) {
	got := decodeOctal(`\377`)
	if !bytes.Equal(got, []byte{0xff}) {
		t.Fatalf("expected 0xff, got %x", got)
	}
}
