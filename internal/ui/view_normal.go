package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/BlinkingLine/cloudterminal/internal/session"
)

// ---------------------------------------------------------------------------
// ViewNormal — main compositor for Normal Mode
// ---------------------------------------------------------------------------

// ViewNormal renders the full Normal Mode UI. It stacks all regions vertically
// and calculates the remaining height for the scrollable chat view.
func ViewNormal(m *Model) string {
	// Resolve the active session (may be a queue item or last-active fallback).
	s := m.activeSession()
	if s == nil {
		return renderWelcome(m)
	}

	// Update input placeholder based on session state.
	SetInputPlaceholder(&m.Input, s.Name, s.State == session.Working)

	// --- Render fixed regions ---
	topBar := renderTopBar(m)

	var cardStrip string
	if m.ShowStrip {
		cardStrip = renderCardStrip(m)
	}

	var queueNav string
	if m.Queue.Len() > 0 {
		queueNav = renderQueueNav(m)
	}

	notifs := renderNotifications(m)

	sessionHeader := renderSessionHeader(m, s)

	inputBar := renderInputBar(m, s)

	helpBar := renderHelpBar()

	// --- Calculate available height for the chat view ---
	usedHeight := 1 // top bar
	if m.ShowStrip {
		usedHeight += lipgloss.Height(cardStrip)
	}
	if m.Queue.Len() > 0 {
		usedHeight += 1 // queue nav
	}
	if notifs != "" {
		usedHeight += lipgloss.Height(notifs)
	}
	usedHeight += lipgloss.Height(sessionHeader)
	usedHeight += lipgloss.Height(inputBar)
	usedHeight += 1 // help bar

	chatHeight := m.Height - usedHeight
	if chatHeight < 1 {
		chatHeight = 1
	}

	chatView := renderChatView(m, s, chatHeight)

	// --- Stack vertically ---
	parts := []string{topBar}
	if m.ShowStrip {
		parts = append(parts, cardStrip)
	}
	if m.Queue.Len() > 0 {
		parts = append(parts, queueNav)
	}
	if notifs != "" {
		parts = append(parts, notifs)
	}
	parts = append(parts, sessionHeader)
	parts = append(parts, chatView)
	parts = append(parts, inputBar)
	parts = append(parts, helpBar)

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

// ---------------------------------------------------------------------------
// renderWelcome — branded welcome screen shown when no sessions exist
// ---------------------------------------------------------------------------

func renderWelcome(m *Model) string {
	title := BrandStyle.Render("CloudTerminal")
	subtitle := MutedStyle.Render("Run multiple Claude Code sessions in parallel")

	steps := []string{
		lipgloss.NewStyle().Foreground(Amber).Render("1.") + " " + lipgloss.NewStyle().Foreground(Fg).Render("Press") + " " + lipgloss.NewStyle().Foreground(Brand).Bold(true).Render("Ctrl+N") + " " + lipgloss.NewStyle().Foreground(Fg).Render("to create a session"),
		lipgloss.NewStyle().Foreground(Amber).Render("2.") + " " + lipgloss.NewStyle().Foreground(Fg).Render("Give it a name and a prompt for Claude"),
		lipgloss.NewStyle().Foreground(Amber).Render("3.") + " " + lipgloss.NewStyle().Foreground(Fg).Render("Create more sessions — they run simultaneously"),
		lipgloss.NewStyle().Foreground(Amber).Render("4.") + " " + lipgloss.NewStyle().Foreground(Fg).Render("Reply when Claude needs you, skip between with") + " " + lipgloss.NewStyle().Foreground(Brand).Bold(true).Render("← →"),
	}

	stepsBlock := strings.Join(steps, "\n")

	hint := MutedStyle.Render("or start from the command line:")
	example := lipgloss.NewStyle().Foreground(Dim).Render("  cloudterminal \"auth:refactor auth\" \"tests:write tests\"")

	if m.MockMode {
		mockNote := lipgloss.NewStyle().Foreground(Amber).Render("mock mode") +
			MutedStyle.Render(" — responses are simulated (no claude CLI needed)")
		content := title + "\n" + subtitle + "\n\n" + stepsBlock + "\n\n" + hint + "\n" + example + "\n\n" + mockNote
		return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center, content)
	}

	content := title + "\n" + subtitle + "\n\n" + stepsBlock + "\n\n" + hint + "\n" + example
	return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center, content)
}

// ---------------------------------------------------------------------------
// renderTopBar
// ---------------------------------------------------------------------------

func renderTopBar(m *Model) string {
	left := BrandStyle.Render("◆ CloudTerminal") + "  " +
		CountStyle.Render(fmt.Sprintf("%d sessions", len(m.Sessions)))

	var right string
	working := 0
	for _, s := range m.Sessions {
		if s.State == session.Working {
			working++
		}
	}
	if working > 0 {
		right = m.Spinner.View() + " " + MutedStyle.Render(fmt.Sprintf("%d working", working))
	}
	if m.Queue.Len() > 0 {
		if right != "" {
			right += "  "
		}
		right += BadgeStyle.Render(fmt.Sprintf("● %d waiting", m.Queue.Len()))
	}

	gap := m.Width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

// ---------------------------------------------------------------------------
// renderCardStrip
// ---------------------------------------------------------------------------

func renderCardStrip(m *Model) string {
	if len(m.Sessions) == 0 {
		return ""
	}

	maxVisible := (m.Width - 4) / 26
	if maxVisible < 1 {
		maxVisible = 1
	}

	visible := m.Sessions
	overflow := 0
	if len(visible) > maxVisible {
		overflow = len(visible) - maxVisible
		visible = visible[:maxVisible]
	}

	cards := make([]string, 0, len(visible))
	for _, s := range visible {
		cards = append(cards, renderCard(m, s))
	}

	row := lipgloss.JoinHorizontal(lipgloss.Top, cards...)

	if overflow > 0 {
		row += " " + MutedStyle.Render(fmt.Sprintf("+%d", overflow))
	}

	return row
}

func renderCard(m *Model, s *session.Session) string {
	// Truncate name to 14 chars.
	name := s.Name
	if len(name) > 14 {
		name = name[:14]
	}

	// State badge.
	var badge string
	switch s.State {
	case session.Working:
		badge = WorkingBadge
	case session.NeedsInput:
		badge = NeedsYouBadge
	}

	// Preview: last message truncated to 20 chars.
	var preview string
	if len(s.History) > 0 {
		last := s.History[len(s.History)-1]
		preview = last.Text
		if len(preview) > 20 {
			preview = preview[:20]
		}
		preview = MutedStyle.Render(preview)
	}

	// Wait time for NeedsInput sessions.
	var waitStr string
	if s.State == session.NeedsInput && !s.EnteredQueue.IsZero() {
		waitStr = MutedStyle.Render(formatDuration(time.Since(s.EnteredQueue)))
	}

	content := name + " " + badge
	if waitStr != "" {
		content += " " + waitStr
	}
	if preview != "" {
		content += "\n" + preview
	}

	// Choose card style.
	style := CardStyle
	if s.ID == m.ActiveID {
		style = ActiveCardStyle
	} else if s.State == session.Working {
		style = WorkingCardStyle
	}

	return style.Render(content)
}

// ---------------------------------------------------------------------------
// renderQueueNav
// ---------------------------------------------------------------------------

func renderQueueNav(m *Model) string {
	if m.Queue.Len() == 0 {
		return ""
	}
	nav := MutedStyle.Render("←") + "  " +
		lipgloss.NewStyle().Foreground(Fg).Render(fmt.Sprintf("%d of %d", m.QueueIndex+1, m.Queue.Len())) +
		"  " + MutedStyle.Render("→")
	return lipgloss.Place(m.Width, 1, lipgloss.Center, lipgloss.Top, nav)
}

// ---------------------------------------------------------------------------
// renderNotifications
// ---------------------------------------------------------------------------

func renderNotifications(m *Model) string {
	if len(m.Notifs) == 0 {
		return ""
	}

	maxShow := 4
	notifs := m.Notifs
	overflow := 0
	if len(notifs) > maxShow {
		overflow = len(notifs) - maxShow
		notifs = notifs[len(notifs)-maxShow:]
	}

	lines := make([]string, 0, len(notifs)+1)
	for _, n := range notifs {
		if n.IsError {
			lines = append(lines, ErrorNotifStyle.Render(n.Text))
		} else {
			lines = append(lines, NotifStyle.Render(n.Text))
		}
	}
	if overflow > 0 {
		lines = append(lines, MutedStyle.Render(fmt.Sprintf("  +%d more notifications", overflow)))
	}

	return strings.Join(lines, "\n")
}

// ---------------------------------------------------------------------------
// renderSessionHeader
// ---------------------------------------------------------------------------

func renderSessionHeader(m *Model, s *session.Session) string {
	name := SessionNameStyle.Render(s.Name)

	var badge string
	switch s.State {
	case session.Working:
		badge = m.Spinner.View() + " " + MutedStyle.Render("working")
	case session.NeedsInput:
		badge = NeedsYouBadge + " " + lipgloss.NewStyle().Foreground(Amber).Render("needs you")
	}

	var waitStr string
	if s.State == session.NeedsInput && !s.EnteredQueue.IsZero() {
		waitStr = MutedStyle.Render(formatDuration(time.Since(s.EnteredQueue)))
	}

	parts := "  " + name + "  " + badge
	if waitStr != "" {
		parts += "  " + waitStr
	}

	return parts + "\n"
}

// ---------------------------------------------------------------------------
// renderChatView
// ---------------------------------------------------------------------------

func renderChatView(m *Model, s *session.Session, height int) string {
	if height < 1 {
		return ""
	}

	wrapWidth := m.Width - 8
	if wrapWidth < 20 {
		wrapWidth = 20
	}

	wrapStyle := lipgloss.NewStyle().Width(wrapWidth)
	padding := "   "

	var lines []string

	for _, msg := range s.History {
		var rendered string
		switch msg.Role {
		case "user":
			role := UserRoleStyle.Render("you")
			text := wrapStyle.Render(msg.Text)
			rendered = padding + role + "  " + text
		case "claude":
			text := wrapStyle.Render(msg.Text)
			bordered := ClaudeMsgBorder.Render(text)
			rendered = padding + bordered
		}
		lines = append(lines, rendered)
		lines = append(lines, "")
	}

	// Working indicator at bottom.
	if s.State == session.Working {
		elapsed := time.Since(s.StartedAt).Truncate(time.Second)
		var indicator string
		if !s.SlotAcquired {
			indicator = MutedStyle.Render(fmt.Sprintf("waiting for slot... (%s)", elapsed))
		} else if s.StatusHint != "" {
			indicator = m.Spinner.View() + " " + MutedStyle.Render(fmt.Sprintf("%s (%s)", s.StatusHint, elapsed))
		} else {
			indicator = m.Spinner.View() + " " + MutedStyle.Render(fmt.Sprintf("thinking... (%s)", elapsed))
		}
		lines = append(lines, padding+indicator)
	}

	allLines := strings.Join(lines, "\n")
	split := strings.Split(allLines, "\n")

	if len(split) > height {
		split = split[len(split)-height:]
	}
	for len(split) < height {
		split = append([]string{""}, split...)
	}

	return strings.Join(split, "\n")
}

// ---------------------------------------------------------------------------
// renderInputBar
// ---------------------------------------------------------------------------

func renderInputBar(m *Model, s *session.Session) string {
	sep := SepStyle.Render(strings.Repeat("─", m.Width))

	inputView := m.Input.View()

	var hint string
	remaining := m.Queue.Len()
	// Don't count the active session if it's in the queue.
	if s.State == session.NeedsInput {
		remaining--
	}
	if remaining > 0 {
		hint = MutedStyle.Render(fmt.Sprintf("%d more waiting · press → after sending", remaining))
	}

	parts := []string{sep, inputView}
	if hint != "" {
		parts = append(parts, hint)
	}

	return strings.Join(parts, "\n")
}

// ---------------------------------------------------------------------------
// renderHelpBar
// ---------------------------------------------------------------------------

func renderHelpBar() string {
	return HelpStyle.Render("← → navigate  Enter send  F focus  Ctrl+N new session  Ctrl+W dismiss")
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

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
