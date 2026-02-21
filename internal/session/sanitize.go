package session

import (
	"regexp"
	"strings"
)

// ansiRe matches ANSI CSI sequences (\x1b[ followed by parameter bytes and a
// final letter) and OSC sequences (\x1b] ... \x07).
var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\x1b\][^\x07]*\x07`)

// Sanitize strips ANSI escape sequences and control characters from s.
//
// It removes:
//   - CSI sequences  (e.g. \x1b[31m, \x1b[0m, \x1b[1m)
//   - OSC sequences  (e.g. \x1b]0;title\x07)
//   - Control characters in the 0x00-0x1F range except \n (0x0A) and \t (0x09)
//
// The cleaned string is returned.
func Sanitize(s string) string {
	// Step 1: strip ANSI escape sequences using regex.
	s = ansiRe.ReplaceAllString(s, "")

	// Step 2: strip remaining control characters, keeping \n and \t.
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r >= 0x00 && r <= 0x1F && r != '\n' && r != '\t' {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
