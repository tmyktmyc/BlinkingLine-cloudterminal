package session

import "testing"

// ---------------------------------------------------------------------------
// Sanitize()
// ---------------------------------------------------------------------------

func TestSanitizePlainTextPassesThrough(t *testing.T) {
	input := "hello world"
	got := Sanitize(input)
	if got != input {
		t.Errorf("Sanitize(%q) = %q, want %q", input, got, input)
	}
}

func TestSanitizeNewlinesPreserved(t *testing.T) {
	input := "line1\nline2\nline3"
	got := Sanitize(input)
	if got != input {
		t.Errorf("Sanitize(%q) = %q, want %q", input, got, input)
	}
}

func TestSanitizeTabsPreserved(t *testing.T) {
	input := "col1\tcol2\tcol3"
	got := Sanitize(input)
	if got != input {
		t.Errorf("Sanitize(%q) = %q, want %q", input, got, input)
	}
}

func TestSanitizeANSIColorCodesStripped(t *testing.T) {
	input := "\x1b[31mred\x1b[0m"
	want := "red"
	got := Sanitize(input)
	if got != want {
		t.Errorf("Sanitize(%q) = %q, want %q", input, got, want)
	}
}

func TestSanitizeANSIBoldStripped(t *testing.T) {
	input := "\x1b[1mbold text\x1b[0m"
	want := "bold text"
	got := Sanitize(input)
	if got != want {
		t.Errorf("Sanitize(%q) = %q, want %q", input, got, want)
	}
}

func TestSanitizeOSCSequencesStripped(t *testing.T) {
	input := "\x1b]0;title\x07text"
	want := "text"
	got := Sanitize(input)
	if got != want {
		t.Errorf("Sanitize(%q) = %q, want %q", input, got, want)
	}
}

func TestSanitizeNullBytesStripped(t *testing.T) {
	input := "hello\x00world"
	want := "helloworld"
	got := Sanitize(input)
	if got != want {
		t.Errorf("Sanitize(%q) = %q, want %q", input, got, want)
	}
}

func TestSanitizeBellCharacterStripped(t *testing.T) {
	input := "alert\x07here"
	want := "alerthere"
	got := Sanitize(input)
	if got != want {
		t.Errorf("Sanitize(%q) = %q, want %q", input, got, want)
	}
}

func TestSanitizeBackspaceStripped(t *testing.T) {
	input := "back\x08space"
	want := "backspace"
	got := Sanitize(input)
	if got != want {
		t.Errorf("Sanitize(%q) = %q, want %q", input, got, want)
	}
}

func TestSanitizeComplexMix(t *testing.T) {
	// Mix of ANSI color, bold, OSC, null byte, bell, tab, newline, and plain text.
	input := "\x1b]0;mytitle\x07" + // OSC title (stripped)
		"\x1b[1m" + // bold on (stripped)
		"Hello" +
		"\x1b[0m" + // reset (stripped)
		"\x00" + // null byte (stripped)
		"\t" + // tab (kept)
		"\x1b[31m" + // red (stripped)
		"World" +
		"\x1b[0m" + // reset (stripped)
		"\x07" + // bell (stripped)
		"\n" + // newline (kept)
		"Done"

	want := "Hello\tWorld\nDone"
	got := Sanitize(input)
	if got != want {
		t.Errorf("Sanitize(complex) = %q, want %q", got, want)
	}
}
