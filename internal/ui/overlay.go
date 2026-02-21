package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderOverlay draws a centered "New Session" dialog box over the terminal.
// It is called from View() when m.Overlay is non-nil.
func renderOverlay(m *Model) string {
	ov := m.Overlay

	// --- Box dimensions ---
	boxWidth := 50
	if m.Width-4 < boxWidth {
		boxWidth = m.Width - 4
	}
	// Inner content width (subtract border + padding: 1 border + 1 pad each side = 4).
	innerWidth := boxWidth - 4
	if innerWidth < 10 {
		innerWidth = 10
	}

	// --- Styles ---
	titleStyle := lipgloss.NewStyle().
		Foreground(Brand).
		Bold(true).
		Width(innerWidth).
		Align(lipgloss.Center)

	labelStyle := lipgloss.NewStyle().
		Foreground(Fg).
		Bold(true)

	mutedLabelStyle := lipgloss.NewStyle().
		Foreground(Muted)

	inputStyle := lipgloss.NewStyle().
		Foreground(Fg)

	errorStyle := lipgloss.NewStyle().
		Foreground(Red)

	hintStyle := lipgloss.NewStyle().
		Foreground(Muted).
		Width(innerWidth).
		Align(lipgloss.Center)

	cursor := lipgloss.NewStyle().Foreground(Brand).Render("\u2588")

	// --- Build content lines ---
	var lines []string

	// Title.
	lines = append(lines, titleStyle.Render("New Session"))
	lines = append(lines, "")

	switch ov.Step {
	case OverlayStepName:
		// Name input with cursor.
		nameDisplay := inputStyle.Render(ov.NameInput) + cursor
		lines = append(lines, labelStyle.Render("Name:"))
		lines = append(lines, nameDisplay)

	case OverlayStepPrompt:
		// Show confirmed name (muted).
		lines = append(lines, mutedLabelStyle.Render("Name: "+ov.NameInput))
		lines = append(lines, "")
		// Prompt input with cursor.
		promptDisplay := inputStyle.Render(ov.PromptInput) + cursor
		lines = append(lines, labelStyle.Render("Prompt:"))
		lines = append(lines, promptDisplay)
	}

	// Error line.
	if ov.Error != "" {
		lines = append(lines, "")
		lines = append(lines, errorStyle.Render(ov.Error))
	}

	// Pad to reach minimum height for visual consistency.
	minContentLines := 8
	for len(lines) < minContentLines {
		lines = append(lines, "")
	}

	// Hint line.
	switch ov.Step {
	case OverlayStepName:
		lines = append(lines, hintStyle.Render("Enter to continue \u00b7 Esc to cancel"))
	case OverlayStepPrompt:
		lines = append(lines, hintStyle.Render("Enter to create \u00b7 Backspace to go back \u00b7 Esc to cancel"))
	}

	content := strings.Join(lines, "\n")

	// --- Box style ---
	boxStyle := lipgloss.NewStyle().
		Background(Surface).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Brand).
		Padding(1, 1).
		Width(boxWidth)

	box := boxStyle.Render(content)

	// --- Center on screen ---
	return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center, box)
}
