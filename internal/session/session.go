package session

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// SessionState represents the current state of a Claude session.
type SessionState int

const (
	// Working means the Claude subprocess is running.
	Working SessionState = iota
	// NeedsInput means Claude has finished and it is the user's turn.
	NeedsInput
)

// String returns the human-readable name of the session state.
func (s SessionState) String() string {
	switch s {
	case Working:
		return "Working"
	case NeedsInput:
		return "NeedsInput"
	default:
		return fmt.Sprintf("SessionState(%d)", int(s))
	}
}

// Message represents a single exchange in the session history.
type Message struct {
	Role string // "user" or "claude"
	Text string
}

// Session tracks a single Claude conversation.
type Session struct {
	ID            string
	Name          string
	State         SessionState
	SlotAcquired  bool
	CompletedOnce bool
	CancelFunc    context.CancelFunc
	StatusHint    string
	History       []Message
	EnteredQueue  time.Time
	SkippedAt     time.Time
	StartedAt     time.Time
}

// SendResult holds the outcome of sending a prompt to a Claude session.
type SendResult struct {
	SessionID   string
	Output      string
	Err         error
	CompletedAt time.Time
}

// nameRegex validates session names: 1-32 chars, starts with alphanumeric,
// remainder may include lowercase alphanumeric, hyphens, and underscores.
var nameRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9\-_]{0,31}$`)

// ValidateName validates and normalises a session name.
// It lowercases the input and checks that it matches the allowed pattern.
func ValidateName(input string) (string, error) {
	cleaned := strings.ToLower(input)
	if !nameRegex.MatchString(cleaned) {
		return "", fmt.Errorf("invalid session name %q: must be 1-32 chars, start with alphanumeric, and contain only lowercase alphanumeric, hyphens, or underscores", input)
	}
	return cleaned, nil
}

// New creates a new Session in the Working state with the given name and
// initial prompt. The session ID is formatted as ct-{runID}-{name}-{shortID}
// where shortID is the first 8 characters of a UUIDv4.
func New(name, prompt, runID string) *Session {
	shortID := uuid.New().String()[:8]
	return &Session{
		ID:       fmt.Sprintf("ct-%s-%s-%s", runID, name, shortID),
		Name:     name,
		State:    Working,
		History:  []Message{{Role: "user", Text: prompt}},
		StartedAt: time.Now(),
	}
}
