package ui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/BlinkingLine/cloudterminal/internal/session"
)

// ViewDeck renders the single-pane deck view based on current DeckState.
func ViewDeck(m *Model) string {
	switch m.Deck {
	case DeckHelp:
		return renderDeckHelp(m)
	case DeckNewFlow:
		return renderDeckNewFlow(m)
	case DeckFeed:
		return renderDeckFeed(m)
	case DeckCard:
		return renderDeckCard(m)
	default:
		return renderDeckFeed(m)
	}
}

// renderDeckCard shows a single session: header, last Claude message, input bar.
func renderDeckCard(m *Model) string {
	s := m.activeSession()
	if s == nil {
		return renderDeckFeed(m)
	}

	var parts []string

	// Header: name, badge, dir, queue position
	parts = append(parts, renderDeckHeader(m, s))

	// Notifications (if any)
	if notifs := renderDeckNotifs(m); notifs != "" {
		parts = append(parts, notifs)
	}

	// Separator
	parts = append(parts, SepStyle.Render(strings.Repeat("─", m.Width)))

	// Chat view — fills remaining space
	headerHeight := len(parts)
	inputHeight := 3 // input + help hint
	chatHeight := m.Height - headerHeight - inputHeight - 1
	if chatHeight < 3 {
		chatHeight = 3
	}
	parts = append(parts, renderDeckChat(m, s, chatHeight))

	// Separator
	parts = append(parts, SepStyle.Render(strings.Repeat("─", m.Width)))

	// Input bar
	parts = append(parts, m.Input.View())

	// Help hint
	parts = append(parts, MutedStyle.Render("  ← → nav  Enter send  /skip  /dismiss  ? help"))

	return strings.Join(parts, "\n")
}

// renderDeckFeed shows the live status feed of all working sessions.
func renderDeckFeed(m *Model) string {
	var parts []string

	// Header
	workingCount := 0
	for _, s := range m.Sessions {
		if s.State == session.Working {
			workingCount++
		}
	}
	header := BrandStyle.Render("◆ CloudTerminal")
	if workingCount > 0 {
		header += "  " + MutedStyle.Render(fmt.Sprintf("%d working", workingCount))
	}
	parts = append(parts, header)
	parts = append(parts, "")

	// Notifications
	if notifs := renderDeckNotifs(m); notifs != "" {
		parts = append(parts, notifs)
	}

	if len(m.Sessions) == 0 {
		// Empty state
		parts = append(parts, MutedStyle.Render("  Type /new to start a session, ? for help"))
	} else {
		// Live status for each session
		for _, s := range m.Sessions {
			elapsed := formatDuration(time.Since(s.StartedAt))
			var line string
			if s.State == session.Working {
				hint := "thinking"
				if s.StatusHint != "" {
					hint = s.StatusHint
				}
				spinner := m.Spinner.View()
				line = fmt.Sprintf("  %-12s %s %s", s.Name, spinner, hint)
				line += MutedStyle.Render(fmt.Sprintf("  (%s)", elapsed))
			} else {
				line = fmt.Sprintf("  %-12s ", s.Name) + NeedsYouBadge + " needs input"
				line += MutedStyle.Render(fmt.Sprintf("  (%s)", formatDuration(time.Since(s.EnteredQueue))))
			}
			if s.Dir != "" {
				line += "  " + MutedStyle.Render(shortenDir(s.Dir))
			}
			parts = append(parts, line)
		}
	}

	// Fill to push input to bottom
	usedLines := len(parts) + 3 // +3 for separator, input, help
	for i := usedLines; i < m.Height-1; i++ {
		parts = append(parts, "")
	}

	// Separator
	parts = append(parts, SepStyle.Render(strings.Repeat("─", m.Width)))

	// Input bar
	parts = append(parts, m.Input.View())

	// Help hint
	if len(m.Sessions) == 0 {
		parts = append(parts, MutedStyle.Render("  /new to create a session  ? for help"))
	} else {
		parts = append(parts, MutedStyle.Render("  /new  /list  /skip  /dismiss  ? help"))
	}

	return strings.Join(parts, "\n")
}

// renderDeckHelp shows the command reference.
func renderDeckHelp(m *Model) string {
	help := `  Commands:
    /new              Create a new session
    /list             Show all sessions
    /skip             Skip current to back of queue
    /dismiss          Close current session
    /go <name>        Jump to session by name

  Navigation:
    ← →               Browse sessions
    Ctrl+U / Ctrl+D   Scroll chat
    Ctrl+C Ctrl+C     Quit

  Press any key to return.`

	var parts []string
	parts = append(parts, BrandStyle.Render("◆ CloudTerminal")+"  "+MutedStyle.Render("Help"))
	parts = append(parts, "")
	parts = append(parts, help)
	return strings.Join(parts, "\n")
}

// renderDeckNewFlow shows the inline step-by-step session creation.
func renderDeckNewFlow(m *Model) string {
	nf := m.NewFlow
	if nf == nil {
		return renderDeckFeed(m)
	}

	var parts []string
	parts = append(parts, BrandStyle.Render("◆ CloudTerminal")+"  "+MutedStyle.Render("New Session"))
	parts = append(parts, "")

	switch nf.Step {
	case NewFlowName:
		parts = append(parts, "  Name:")
		parts = append(parts, "")
	case NewFlowDir:
		parts = append(parts, "  Name: "+SessionNameStyle.Render(nf.Name))
		parts = append(parts, "  Dir [.]:")
		parts = append(parts, "")
	case NewFlowPrompt:
		parts = append(parts, "  Name: "+SessionNameStyle.Render(nf.Name))
		parts = append(parts, "  Dir: "+MutedStyle.Render(nf.Dir))
		parts = append(parts, "  Prompt:")
		parts = append(parts, "")
	}

	if nf.Error != "" {
		parts = append(parts, "  "+ErrorNotifStyle.Render(nf.Error))
		parts = append(parts, "")
	}

	// Fill to push input to bottom
	usedLines := len(parts) + 3
	for i := usedLines; i < m.Height-1; i++ {
		parts = append(parts, "")
	}

	parts = append(parts, SepStyle.Render(strings.Repeat("─", m.Width)))
	parts = append(parts, m.Input.View())
	parts = append(parts, MutedStyle.Render("  Enter confirm  Esc cancel"))

	return strings.Join(parts, "\n")
}

// renderDeckHeader renders the top line for a card view.
func renderDeckHeader(m *Model, s *session.Session) string {
	left := "  " + SessionNameStyle.Render(s.Name) + "  "
	if s.State == session.NeedsInput {
		left += NeedsYouBadge
	} else {
		left += WorkingBadge
	}
	if s.Dir != "" {
		left += "  " + MutedStyle.Render(shortenDir(s.Dir))
	}

	right := ""
	queueLen := len(m.Queue.Items)
	if queueLen > 0 {
		right = MutedStyle.Render(fmt.Sprintf("%d/%d waiting", m.QueueIndex+1, queueLen))
	}

	gap := m.Width - lipgloss.Width(left) - lipgloss.Width(right) - 2
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

// renderDeckNotifs renders notification lines.
func renderDeckNotifs(m *Model) string {
	if len(m.Notifs) == 0 {
		return ""
	}
	var lines []string
	max := 4
	if len(m.Notifs) < max {
		max = len(m.Notifs)
	}
	for i := 0; i < max; i++ {
		n := m.Notifs[i]
		style := NotifStyle
		if n.IsError {
			style = ErrorNotifStyle
		}
		lines = append(lines, "  "+style.Render(n.Text))
	}
	if len(m.Notifs) > 4 {
		lines = append(lines, MutedStyle.Render(fmt.Sprintf("  +%d more", len(m.Notifs)-4)))
	}
	return strings.Join(lines, "\n")
}

// renderDeckChat renders the conversation history for a session.
func renderDeckChat(m *Model, s *session.Session, height int) string {
	if s.State == session.Working && len(s.History) <= 1 {
		hint := "thinking"
		if s.StatusHint != "" {
			hint = s.StatusHint
		}
		elapsed := formatDuration(time.Since(s.StartedAt))
		return "  " + m.Spinner.View() + " " + hint + MutedStyle.Render(fmt.Sprintf("  (%s)", elapsed))
	}

	maxWidth := m.Width - 8
	if maxWidth < 20 {
		maxWidth = 20
	}

	var lines []string
	for _, msg := range s.History {
		if msg.Role == "user" {
			prefix := UserRoleStyle.Render("you ›") + " "
			wrapped := wrapText(msg.Text, maxWidth)
			for i, line := range strings.Split(wrapped, "\n") {
				if i == 0 {
					lines = append(lines, "  "+prefix+line)
				} else {
					lines = append(lines, "        "+line)
				}
			}
		} else {
			border := ClaudeMsgBorder.Render("│")
			wrapped := wrapText(msg.Text, maxWidth)
			for _, line := range strings.Split(wrapped, "\n") {
				lines = append(lines, "  "+border+" "+line)
			}
		}
		lines = append(lines, "")
	}

	// If session is working, show spinner at bottom
	if s.State == session.Working {
		hint := "thinking"
		if s.StatusHint != "" {
			hint = s.StatusHint
		}
		elapsed := formatDuration(time.Since(s.StartedAt))
		lines = append(lines, "  "+m.Spinner.View()+" "+hint+MutedStyle.Render(fmt.Sprintf("  (%s)", elapsed)))
	}

	// Auto-scroll: show last N lines
	if len(lines) > height {
		lines = lines[len(lines)-height:]
	}

	return strings.Join(lines, "\n")
}

// formatDuration returns a human-readable short duration string.
func formatDuration(d time.Duration) string {
	d = d.Truncate(time.Second)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%dh", int(d.Hours()))
}

// shortenDir shortens a directory path for display.
func shortenDir(dir string) string {
	home, _ := os.UserHomeDir()
	if home != "" && strings.HasPrefix(dir, home) {
		return "~" + dir[len(home):]
	}
	return dir
}

// wrapText wraps text to the given width.
func wrapText(text string, width int) string {
	if width <= 0 {
		return text
	}
	var result strings.Builder
	for _, line := range strings.Split(text, "\n") {
		if lipgloss.Width(line) <= width {
			if result.Len() > 0 {
				result.WriteByte('\n')
			}
			result.WriteString(line)
			continue
		}
		words := strings.Fields(line)
		current := ""
		for _, word := range words {
			if current == "" {
				current = word
			} else if lipgloss.Width(current+" "+word) <= width {
				current += " " + word
			} else {
				if result.Len() > 0 {
					result.WriteByte('\n')
				}
				result.WriteString(current)
				current = word
			}
		}
		if current != "" {
			if result.Len() > 0 {
				result.WriteByte('\n')
			}
			result.WriteString(current)
		}
	}
	return result.String()
}
