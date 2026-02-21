package ui

import "fmt"

// ViewNormal renders the normal mode UI.
// This is a placeholder — the full implementation is in Task 11.
func ViewNormal(m *Model) string {
	return fmt.Sprintf("Normal Mode - %d sessions - Queue: %d", len(m.Sessions), m.Queue.Len())
}
