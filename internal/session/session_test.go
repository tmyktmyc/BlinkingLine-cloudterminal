package session

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// SessionState.String()
// ---------------------------------------------------------------------------

func TestSessionStateStringWorking(t *testing.T) {
	if s := Working.String(); s != "Working" {
		t.Errorf("Working.String() = %q, want %q", s, "Working")
	}
}

func TestSessionStateStringNeedsInput(t *testing.T) {
	if s := NeedsInput.String(); s != "NeedsInput" {
		t.Errorf("NeedsInput.String() = %q, want %q", s, "NeedsInput")
	}
}

// ---------------------------------------------------------------------------
// New()
// ---------------------------------------------------------------------------

func TestNewCreatesSessionWithCorrectName(t *testing.T) {
	s := New("myproject", "do something", "", "run01")
	if s.Name != "myproject" {
		t.Errorf("Name = %q, want %q", s.Name, "myproject")
	}
}

func TestNewCreatesSessionInWorkingState(t *testing.T) {
	s := New("myproject", "do something", "", "run01")
	if s.State != Working {
		t.Errorf("State = %v, want Working", s.State)
	}
}

func TestNewCreatesSessionWithInitialHistory(t *testing.T) {
	s := New("myproject", "do something", "", "run01")
	if len(s.History) != 1 {
		t.Fatalf("History length = %d, want 1", len(s.History))
	}
	if s.History[0].Role != "user" {
		t.Errorf("History[0].Role = %q, want %q", s.History[0].Role, "user")
	}
	if s.History[0].Text != "do something" {
		t.Errorf("History[0].Text = %q, want %q", s.History[0].Text, "do something")
	}
}

func TestNewSetsStartedAt(t *testing.T) {
	before := time.Now()
	s := New("myproject", "do something", "", "run01")
	after := time.Now()

	if s.StartedAt.Before(before) || s.StartedAt.After(after) {
		t.Errorf("StartedAt = %v, want between %v and %v", s.StartedAt, before, after)
	}
}

func TestNewIDFormat(t *testing.T) {
	s := New("myproject", "do something", "", "run01")

	// ID must start with ct-{runID}-{name}-
	prefix := "ct-run01-myproject-"
	if !strings.HasPrefix(s.ID, prefix) {
		t.Errorf("ID = %q, want prefix %q", s.ID, prefix)
	}

	// shortID is the remaining 8 hex chars
	shortID := strings.TrimPrefix(s.ID, prefix)
	if len(shortID) != 8 {
		t.Errorf("shortID length = %d, want 8 (ID = %q)", len(shortID), s.ID)
	}
}

func TestNewIDsAreUnique(t *testing.T) {
	s1 := New("proj", "prompt", "", "run01")
	s2 := New("proj", "prompt", "", "run01")
	if s1.ID == s2.ID {
		t.Errorf("two calls to New() produced the same ID: %q", s1.ID)
	}
}

func TestNewDefaultFieldValues(t *testing.T) {
	s := New("proj", "prompt", "", "run01")

	if s.SlotAcquired {
		t.Error("SlotAcquired should be false by default")
	}
	if s.CompletedOnce {
		t.Error("CompletedOnce should be false by default")
	}
	if s.CancelFunc != nil {
		t.Error("CancelFunc should be nil by default")
	}
	if s.StatusHint != "" {
		t.Errorf("StatusHint = %q, want empty", s.StatusHint)
	}
}

// ---------------------------------------------------------------------------
// ValidateName()
// ---------------------------------------------------------------------------

func TestValidateNameValidInputs(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"myproject", "myproject"},
		{"a", "a"},
		{"project1", "project1"},
		{"my-project", "my-project"},
		{"my_project", "my_project"},
		{"0leading", "0leading"},
		{"a1b2c3", "a1b2c3"},
		{"ab-cd_ef", "ab-cd_ef"},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got, err := ValidateName(tc.input)
			if err != nil {
				t.Fatalf("ValidateName(%q) error = %v", tc.input, err)
			}
			if got != tc.want {
				t.Errorf("ValidateName(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestValidateNameLowercasesInput(t *testing.T) {
	got, err := ValidateName("MyProject")
	if err != nil {
		t.Fatalf("ValidateName(\"MyProject\") error = %v", err)
	}
	if got != "myproject" {
		t.Errorf("ValidateName(\"MyProject\") = %q, want %q", got, "myproject")
	}
}

func TestValidateNameMixedCaseLowercase(t *testing.T) {
	got, err := ValidateName("FoO-BaR")
	if err != nil {
		t.Fatalf("ValidateName(\"FoO-BaR\") error = %v", err)
	}
	if got != "foo-bar" {
		t.Errorf("ValidateName(\"FoO-BaR\") = %q, want %q", got, "foo-bar")
	}
}

func TestValidateNameInvalidInputs(t *testing.T) {
	cases := []string{
		"",                                   // empty
		"-starts-with-dash",                  // starts with dash
		"_starts-with-underscore",            // starts with underscore
		"has spaces",                         // spaces
		"hello!world",                        // special char
		"a.b",                                // dot
		"name@place",                         // at sign
		"abcdefghijklmnopqrstuvwxyz0123456",  // 33 chars — too long
	}

	for _, input := range cases {
		t.Run(input, func(t *testing.T) {
			_, err := ValidateName(input)
			if err == nil {
				t.Errorf("ValidateName(%q) expected error, got nil", input)
			}
		})
	}
}

func TestValidateNameMaxLength(t *testing.T) {
	// Exactly 32 chars starting with alphanumeric should be valid
	name := "a" + strings.Repeat("b", 31) // 32 chars
	got, err := ValidateName(name)
	if err != nil {
		t.Fatalf("ValidateName(32 chars) error = %v", err)
	}
	if got != name {
		t.Errorf("ValidateName(32 chars) = %q, want %q", got, name)
	}
}

// ---------------------------------------------------------------------------
// SendResult
// ---------------------------------------------------------------------------

func TestSendResultHoldsValues(t *testing.T) {
	now := time.Now()
	testErr := errors.New("something went wrong")

	r := SendResult{
		SessionID:   "ct-run01-proj-abcd1234",
		Output:      "task completed",
		Err:         testErr,
		CompletedAt: now,
	}

	if r.SessionID != "ct-run01-proj-abcd1234" {
		t.Errorf("SessionID = %q, want %q", r.SessionID, "ct-run01-proj-abcd1234")
	}
	if r.Output != "task completed" {
		t.Errorf("Output = %q, want %q", r.Output, "task completed")
	}
	if r.Err != testErr {
		t.Errorf("Err = %v, want %v", r.Err, testErr)
	}
	if !r.CompletedAt.Equal(now) {
		t.Errorf("CompletedAt = %v, want %v", r.CompletedAt, now)
	}
}

func TestSendResultZeroValue(t *testing.T) {
	var r SendResult

	if r.SessionID != "" {
		t.Errorf("SessionID = %q, want empty", r.SessionID)
	}
	if r.Output != "" {
		t.Errorf("Output = %q, want empty", r.Output)
	}
	if r.Err != nil {
		t.Errorf("Err = %v, want nil", r.Err)
	}
	if !r.CompletedAt.IsZero() {
		t.Errorf("CompletedAt should be zero value")
	}
}
