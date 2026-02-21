package ui

import (
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/lipgloss"
)

// NewInput creates a configured textarea for the input bar.
func NewInput(width int) textarea.Model {
	ta := textarea.New()
	ta.Placeholder = "Type a message..."
	ta.Prompt = "> "
	ta.CharLimit = 0 // unlimited
	ta.MaxHeight = 5
	ta.ShowLineNumbers = false
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.FocusedStyle.Base = lipgloss.NewStyle()
	ta.FocusedStyle.Prompt = lipgloss.NewStyle().Foreground(Brand)
	ta.FocusedStyle.Placeholder = lipgloss.NewStyle().Foreground(Muted)
	ta.BlurredStyle = ta.FocusedStyle
	ta.SetWidth(width - 4) // account for padding
	ta.Focus()
	return ta
}

// SetInputPlaceholder updates the textarea placeholder based on session state.
func SetInputPlaceholder(ta *textarea.Model, sessionName string, isWorking bool) {
	if isWorking {
		ta.Placeholder = "Claude is working..."
	} else if sessionName != "" {
		ta.Placeholder = "Reply to " + sessionName + "..."
	} else {
		ta.Placeholder = "Type a message..."
	}
}
