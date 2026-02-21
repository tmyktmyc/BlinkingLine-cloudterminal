package ui

import "time"

// Mode represents the current UI mode.
type Mode int

const (
	// NormalMode is the default mode showing the session strip and chat pane.
	NormalMode Mode = iota
	// FocusMode is the queue-clearing mode showing one card at a time.
	FocusMode
)

// TickMsg is sent every second to drive time-based UI updates
// (notification expiry, dismiss confirmation expiry, focus auto-suggest).
type TickMsg time.Time

// Notification is a short-lived message shown at the top of the UI.
type Notification struct {
	Text      string
	CreatedAt time.Time
	IsError   bool
}
