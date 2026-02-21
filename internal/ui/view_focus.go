package ui

import "fmt"

// ViewFocus renders the focus mode UI.
// This is a placeholder — the full implementation is in Task 12.
func ViewFocus(m *Model) string {
	return fmt.Sprintf("Focus Mode - %d/%d cleared", m.FocusCleared, m.FocusTotal)
}
