package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/BlinkingLine/cloudterminal/internal/session"
)

// ---------------------------------------------------------------------------
// ViewFocus — main compositor for Focus Mode
// ---------------------------------------------------------------------------

// ViewFocus renders the full Focus Mode UI. When the queue is empty (all
// sessions working or none exist), it shows a centered waiting screen.
// Otherwise it renders the focus header, progress bar, notifications,
// the current session card, input area, and help bar.
func ViewFocus(m *Model) string {
	// If the queue is empty, show the waiting screen.
	if m.Queue.Len() == 0 {
		return renderFocusWaiting(m)
	}

	// Resolve the active session from the queue.
	s := m.activeSession()
	if s == nil {
		return renderFocusWaiting(m)
	}

	// Update input placeholder.
	SetInputPlaceholder(&m.Input, s.Name, s.State == session.Working)

	// --- Render fixed regions ---
	header := renderFocusHeader(m)
	progress := renderProgressBar(m)
	notifs := renderNotifications(m)
	inputBar := renderFocusInput(m)
	helpBar := renderFocusHelp()

	// --- Calculate available height for the card ---
	usedHeight := 1 // header
	usedHeight++     // progress bar
	if notifs != "" {
		usedHeight += lipgloss.Height(notifs)
	}
	usedHeight += lipgloss.Height(inputBar)
	usedHeight += 1 // help bar

	cardHeight := m.Height - usedHeight
	if cardHeight < 1 {
		cardHeight = 1
	}

	card := renderFocusCard(m, s, cardHeight)

	// --- Stack vertically ---
	parts := []string{header, progress}
	if notifs != "" {
		parts = append(parts, notifs)
	}
	parts = append(parts, card)
	parts = append(parts, inputBar)
	parts = append(parts, helpBar)

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

// ---------------------------------------------------------------------------
// renderFocusWaiting — centered waiting screen when queue is empty
// ---------------------------------------------------------------------------

func renderFocusWaiting(m *Model) string {
	title := m.Spinner.View() + " " + BrandStyle.Render("Waiting for responses")

	var sessionLines []string
	for _, s := range m.Sessions {
		if s.State == session.Working {
			elapsed := formatDuration(time.Since(s.StartedAt))
			hint := ""
			if s.StatusHint != "" {
				hint = " — " + s.StatusHint
			}
			line := "  " + MutedStyle.Render(s.Name) + "  " +
				lipgloss.NewStyle().Foreground(Dim).Render(elapsed+hint)
			sessionLines = append(sessionLines, line)
		}
	}

	content := title
	if len(sessionLines) > 0 {
		content += "\n\n" + strings.Join(sessionLines, "\n")
	}

	return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center, content)
}

// ---------------------------------------------------------------------------
// renderFocusHeader — top bar for Focus Mode
// ---------------------------------------------------------------------------

// renderFocusHeader renders the focus header line.
// Left: "diamond focus" in brand style. Right: "N working" in muted style.
func renderFocusHeader(m *Model) string {
	left := BrandStyle.Render("◆ focus")

	working := 0
	for _, s := range m.Sessions {
		if s.State == session.Working {
			working++
		}
	}

	var right string
	if working > 0 {
		right = m.Spinner.View() + " " + MutedStyle.Render(fmt.Sprintf("%d working", working))
	}

	gap := m.Width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

// ---------------------------------------------------------------------------
// renderProgressBar — visual progress bar with counters
// ---------------------------------------------------------------------------

// renderProgressBar renders the progress bar showing cleared/total with
// optional incoming count.
func renderProgressBar(m *Model) string {
	// Build the counter and label suffix first so we know how wide the bar can be.
	pct := 0
	if m.FocusTotal > 0 {
		pct = (m.FocusCleared * 100) / m.FocusTotal
	}
	counter := fmt.Sprintf(" %d/%d (%d%%)", m.FocusCleared, m.FocusTotal, pct)

	var incoming string
	if m.FocusIncoming > 0 {
		incoming = fmt.Sprintf("  +%d incoming", m.FocusIncoming)
	}

	suffix := counter + incoming

	// Suffix is rendered in muted; measure its visual width.
	suffixRendered := MutedStyle.Render(suffix)
	suffixWidth := lipgloss.Width(suffixRendered)

	// Bar width fills the rest of the terminal width.
	barWidth := m.Width - suffixWidth
	if barWidth < 1 {
		barWidth = 1
	}

	filled := m.FocusCleared
	total := m.FocusTotal
	if total < 1 {
		total = 1
	}
	if filled > total {
		filled = total
	}

	// Scale segments to fit bar width.
	filledSegments := 0
	if total > 0 {
		filledSegments = (filled * barWidth) / total
	}
	emptySegments := barWidth - filledSegments

	bar := strings.Repeat("\u2501", filledSegments)
	empty := strings.Repeat("\u2591", emptySegments)

	barRendered := lipgloss.NewStyle().Foreground(Amber).Render(bar) +
		lipgloss.NewStyle().Foreground(Dim).Render(empty)

	return barRendered + suffixRendered
}

// ---------------------------------------------------------------------------
// renderFocusCard — compressed card showing Claude's last message
// ---------------------------------------------------------------------------

// renderFocusCard renders the current session's card in focus mode.
// It shows the session name at the top (muted), followed by Claude's last
// message word-wrapped to width-6. If no Claude message exists, the user's
// prompt is shown in muted text.
func renderFocusCard(m *Model, s *session.Session, height int) string {
	if height < 1 {
		return ""
	}

	wrapWidth := m.Width - 6
	if wrapWidth < 20 {
		wrapWidth = 20
	}

	// Session name header (muted).
	nameHeader := MutedStyle.Render(s.Name)

	// Find Claude's last message.
	var content string
	var isMuted bool

	lastClaude := ""
	for i := len(s.History) - 1; i >= 0; i-- {
		if s.History[i].Role == "claude" {
			lastClaude = s.History[i].Text
			break
		}
	}

	if lastClaude != "" {
		content = lastClaude
		isMuted = false
	} else {
		// No Claude message yet — show user's prompt in muted text.
		if len(s.History) > 0 {
			content = s.History[0].Text
		}
		isMuted = true
	}

	// Word-wrap the content.
	wrapStyle := lipgloss.NewStyle().Width(wrapWidth)
	wrapped := wrapStyle.Render(content)

	if isMuted {
		wrapped = MutedStyle.Render(wrapped)
	}

	// Build the full card content: name + blank line + message.
	cardLines := []string{nameHeader, ""}
	msgLines := strings.Split(wrapped, "\n")
	cardLines = append(cardLines, msgLines...)

	// Pad left with 3 spaces for consistent indentation.
	padding := "   "
	for i, line := range cardLines {
		cardLines[i] = padding + line
	}

	// Auto-scroll: if content exceeds height, show the last `height` lines.
	if len(cardLines) > height {
		cardLines = cardLines[len(cardLines)-height:]
	}

	// Pad with empty lines if fewer lines than available height.
	for len(cardLines) < height {
		cardLines = append(cardLines, "")
	}

	return strings.Join(cardLines, "\n")
}

// ---------------------------------------------------------------------------
// renderFocusInput — amber separator + textarea
// ---------------------------------------------------------------------------

// renderFocusInput renders the input section for focus mode: an amber
// separator line followed by the textarea.
func renderFocusInput(m *Model) string {
	sep := FocusSeparator.Render(strings.Repeat("\u2501", m.Width))
	inputView := m.Input.View()
	return sep + "\n" + inputView
}

// ---------------------------------------------------------------------------
// renderFocusHelp — focus-mode key bindings
// ---------------------------------------------------------------------------

// renderFocusHelp renders the help bar for focus mode.
func renderFocusHelp() string {
	return HelpStyle.Render("Enter send  S skip  Ctrl+W dismiss  Ctrl+Enter default reply  Esc exit")
}
