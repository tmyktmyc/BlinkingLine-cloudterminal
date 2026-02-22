package ui

import "github.com/charmbracelet/lipgloss"

// ---------------------------------------------------------------------------
// Color palette
// ---------------------------------------------------------------------------

var (
	Amber = lipgloss.Color("#f59e0b")
	Blue  = lipgloss.Color("#3b82f6")
	Brand = lipgloss.Color("#d4a574")
	Fg    = lipgloss.Color("#e5e5e5")
	Muted = lipgloss.Color("#737373")
	Dim   = lipgloss.Color("#404040")
	Red   = lipgloss.Color("#ef4444")

	UserBlue = lipgloss.Color("#60a5fa")
)

// ---------------------------------------------------------------------------
// Reusable Lip Gloss styles
// ---------------------------------------------------------------------------

var (
	BrandStyle       = lipgloss.NewStyle().Foreground(Brand).Bold(true)
	SessionNameStyle = lipgloss.NewStyle().Foreground(Fg).Bold(true)

	UserRoleStyle   = lipgloss.NewStyle().Foreground(UserBlue).Bold(true)
	ClaudeMsgBorder = lipgloss.NewStyle().
			BorderLeft(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(Brand).
			PaddingLeft(1)

	MutedStyle = lipgloss.NewStyle().Foreground(Muted)

	NotifStyle      = lipgloss.NewStyle().Foreground(Amber)
	ErrorNotifStyle = lipgloss.NewStyle().Foreground(Red)

	SepStyle = lipgloss.NewStyle().Foreground(Dim)
)

// ---------------------------------------------------------------------------
// Rendered constants (pre-rendered strings)
// ---------------------------------------------------------------------------

var (
	WorkingBadge  = lipgloss.NewStyle().Foreground(Blue).Render("⟳")
	NeedsYouBadge = lipgloss.NewStyle().Foreground(Amber).Render("●")
)
