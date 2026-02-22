package ui

import "time"

// DeckState represents what the single-pane deck is currently showing.
type DeckState int

const (
	// DeckCard shows a single session (needing input or manually browsed to).
	DeckCard DeckState = iota
	// DeckFeed shows the live status feed of all working sessions.
	DeckFeed
	// DeckHelp shows the command reference (triggered by ?).
	DeckHelp
	// DeckNewFlow shows the inline step-by-step session creation.
	DeckNewFlow
)

// TickMsg fires every second for UI updates.
type TickMsg time.Time

// Notification is a transient message that auto-expires.
type Notification struct {
	Text      string
	CreatedAt time.Time
	IsError   bool
}
