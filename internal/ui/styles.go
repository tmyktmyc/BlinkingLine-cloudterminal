package ui

import "github.com/charmbracelet/lipgloss"

// ---------------------------------------------------------------------------
// Color palette
// ---------------------------------------------------------------------------

var (
	Amber   = lipgloss.Color("#f59e0b")
	Blue    = lipgloss.Color("#3b82f6")
	Brand   = lipgloss.Color("#d4a574")
	Fg      = lipgloss.Color("#e5e5e5")
	Muted   = lipgloss.Color("#737373")
	Dim     = lipgloss.Color("#404040")
	Bg      = lipgloss.Color("#0a0a0a")
	Surface = lipgloss.Color("#151515")
	Red     = lipgloss.Color("#ef4444")
	Green   = lipgloss.Color("#22c55e")

	UserBlue = lipgloss.Color("#60a5fa")
	UserBg   = lipgloss.Color("#1e3a5f")
	ClaudeBg = lipgloss.Color("#2d1f0e")
)

// ---------------------------------------------------------------------------
// Reusable Lip Gloss styles
// ---------------------------------------------------------------------------

var (
	BrandStyle       = lipgloss.NewStyle().Foreground(Brand).Bold(true)
	CountStyle       = lipgloss.NewStyle().Foreground(Muted)
	BadgeStyle       = lipgloss.NewStyle().Foreground(Amber).Bold(true)
	SessionNameStyle = lipgloss.NewStyle().Foreground(Fg).Bold(true)
	WaitTimeStyle    = lipgloss.NewStyle().Foreground(Muted)

	UserRoleStyle   = lipgloss.NewStyle().Foreground(UserBlue).Bold(true)
	ClaudeRoleStyle = lipgloss.NewStyle().Foreground(Brand).Bold(true).Background(ClaudeBg)
	UserMsgStyle    = lipgloss.NewStyle().Background(UserBg)

	MutedStyle       = lipgloss.NewStyle().Foreground(Muted)
	InputBorderStyle = lipgloss.NewStyle().BorderForeground(Brand)
	PlaceholderStyle = lipgloss.NewStyle().Foreground(Muted).Italic(true)

	NotifStyle      = lipgloss.NewStyle().Foreground(Amber)
	ErrorNotifStyle = lipgloss.NewStyle().Foreground(Red)

	CardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Dim).
			Width(24).
			Padding(0, 1).
			Background(Surface)

	ActiveCardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Amber).
			Width(24).
			Padding(0, 1).
			Background(Surface)

	WorkingCardStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(Blue).
				Width(24).
				Padding(0, 1).
				Background(Surface)

	HelpStyle = lipgloss.NewStyle().Foreground(Muted)
	SepStyle  = lipgloss.NewStyle().Foreground(Dim)

	FocusSeparator = lipgloss.NewStyle().Foreground(Amber)
)

// ---------------------------------------------------------------------------
// Rendered constants (pre-rendered strings)
// ---------------------------------------------------------------------------

var (
	ActiveDot   = lipgloss.NewStyle().Foreground(Amber).Render("━━")
	InactiveDot = lipgloss.NewStyle().Foreground(Dim).Render("──")

	ProgressFilled = lipgloss.NewStyle().Foreground(Amber).Render("━")
	ProgressEmpty  = lipgloss.NewStyle().Foreground(Dim).Render("░")

	WorkingBadge = lipgloss.NewStyle().Foreground(Blue).Render("⟳")
	QueuedBadge  = lipgloss.NewStyle().Foreground(Muted).Render("⟳")
	NeedsYouBadge = lipgloss.NewStyle().Foreground(Amber).Render("●")
)
