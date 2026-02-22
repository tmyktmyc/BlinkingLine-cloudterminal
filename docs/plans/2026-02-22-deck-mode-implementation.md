# Deck Mode UX Redesign — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task.

**Goal:** Replace Normal Mode + Focus Mode + Welcome Screen with a single "Deck Mode" that auto-switches between sessions across multiple projects.

**Architecture:** Collapse the two rendering paths (`view_normal.go`, `view_focus.go`) into a single deck renderer with four states: Card (session needing input), Feed (all working), Help (command reference), NewFlow (inline session creation). Add `Dir` field to sessions for multi-project support. Replace keybinding-based session creation with `/` command system.

**Tech Stack:** Go, Bubbletea (TUI framework), Lip Gloss (styling)

---

## Task 1: Add `Dir` Field to Session and Update `Send`

**Files:**
- Modify: `internal/session/session.go:42-54` (Session struct)
- Modify: `internal/session/session.go:81-90` (New function)
- Modify: `internal/session/send.go:71-74` (command creation)
- Modify: `internal/session/session_test.go`

**Why:** Every subsequent task depends on sessions having a working directory.

**Steps:**

1. In `internal/session/session.go`, add `Dir` field to `Session` struct (line 44, after Name):
```go
type Session struct {
	ID            string
	Name          string
	Dir           string // Working directory for this session's Claude subprocess.
	State         SessionState
	SlotAcquired  bool
	CompletedOnce bool
	CancelFunc    context.CancelFunc
	StatusHint    string
	History       []Message
	EnteredQueue  time.Time
	SkippedAt     time.Time
	StartedAt     time.Time
}
```

2. Update `New` function to accept `dir` parameter (line 81):
```go
func New(name, prompt, dir, runID string) *Session {
	shortID := uuid.New().String()[:8]
	s := &Session{
		ID:   fmt.Sprintf("ct-%s-%s-%s", runID, name, shortID),
		Name: name,
		Dir:  dir,
		History: []Message{
			{Role: "user", Text: prompt},
		},
		State:     Working,
		StartedAt: time.Now(),
	}
	return s
}
```

3. In `internal/session/send.go`, set `cmd.Dir` (around line 74, after creating the command):
```go
cmd := exec.CommandContext(ctx, "claude", args...)
if s.Dir != "" {
	cmd.Dir = s.Dir
}
```

4. Update all callers of `session.New` — currently only `main.go:197`. Add `""` as the dir argument for now (will update in Task 7):
```go
s := session.New(name, prompt, "", runID)
```

5. Update existing tests in `internal/session/session_test.go` to pass the new `dir` parameter (empty string).

6. Run tests: `go test ./...`

7. Commit: `feat: add Dir field to Session for multi-project support`

---

## Task 2: Add DeckState and Remove Mode Enum

**Files:**
- Modify: `internal/ui/messages.go` (replace Mode with DeckState)
- Modify: `internal/ui/model.go:47-97` (Model struct)

**Why:** The Model struct needs the new DeckState enum before we can change the rendering and key handling.

**Steps:**

1. In `internal/ui/messages.go`, replace the `Mode` type with `DeckState`:
```go
package ui

import "time"

// DeckState represents what the single-pane deck is currently showing.
type DeckState int

const (
	// DeckCard shows a single session (needing input or manually browsed to).
	DeckCard DeckState = iota
	// DeckFeed shows the live status feed of all working sessions.
	DeckFeed
	// DeckHelp shows the command reference (triggered by ?).
	DeckHelp
	// DeckNewFlow shows the inline step-by-step session creation.
	DeckNewFlow
)

// TickMsg fires every second for UI updates.
type TickMsg time.Time

// Notification is a transient message that auto-expires.
type Notification struct {
	Text      string
	CreatedAt time.Time
	IsError   bool
}
```

2. In `internal/ui/model.go`, update the `Model` struct. Replace `Mode Mode` and `ShowStrip bool` with `DeckState DeckState`. Remove focus-tracking fields (`FocusCleared`, `FocusTotal`, `FocusIncoming`, `FocusSuggested`). Remove `Overlay *OverlayState`. Add new-flow state fields:
```go
type NewFlowStep int

const (
	NewFlowName NewFlowStep = iota
	NewFlowDir
	NewFlowPrompt
)

type NewFlowState struct {
	Step   NewFlowStep
	Name   string
	Dir    string
	Prompt string
	Error  string
}

type Model struct {
	// Session management
	Sessions   []*session.Session
	Queue      queue.Queue
	ActiveID   string
	QueueIndex int

	// Deck state — what's currently shown
	Deck     DeckState
	NewFlow  *NewFlowState // Non-nil during /new flow

	// Input & Display
	Input        textarea.Model
	Spinner      spinner.Model
	DefaultReply string

	// Notifications
	Notifs []Notification

	// Terminal
	Width  int
	Height int

	// Dismiss confirmation
	DismissConfirmID string
	DismissConfirmAt time.Time

	// Subprocess management
	RunID      string
	Sem        chan struct{}
	RootCtx    context.Context
	RootCancel context.CancelFunc
	ShutdownWg sync.WaitGroup

	// Config
	Config   config.Config
	MockMode bool
	Verbose  bool

	// Internal
	LastActiveID string
	LastCtrlC    time.Time
	program      *tea.Program
}
```

3. Update `NewModel` function to initialize `Deck` instead of `Mode`:
```go
func NewModel(cfg config.Config, sessions []*session.Session, runID string, mockMode, verbose bool, ctx context.Context, cancel context.CancelFunc) *Model {
	m := &Model{
		Sessions:     sessions,
		ActiveID:     "",
		Deck:         DeckFeed, // Start with feed (or card if sessions have items)
		Input:        NewInput(80),
		Spinner:      newSpinner(),
		DefaultReply: cfg.DefaultReply,
		RunID:        runID,
		Sem:          make(chan struct{}, cfg.MaxConcurrent),
		RootCtx:      ctx,
		RootCancel:   cancel,
		Config:       cfg,
		MockMode:     mockMode,
		Verbose:      verbose,
	}
	if len(sessions) > 0 {
		m.ActiveID = sessions[0].ID
		m.Deck = DeckCard
	}
	return m
}
```

4. Remove `OverlayStep`, `OverlayState` type definitions from `model.go` (lines 25-38).

5. Remove `enterFocusMode`, `exitFocusMode` functions from `model.go` (lines 711-726).

6. Remove `openOverlay` function from `model.go` (lines 797-801).

7. Run tests: `go test ./...` — expect compilation failures since view/update files still reference old types. That's OK — we'll fix them in subsequent tasks.

8. Commit: `refactor: replace Mode/Overlay with DeckState in model`

---

## Task 3: Implement Command Parser

**Files:**
- Create: `internal/ui/commands.go`
- Create: `internal/ui/commands_test.go`

**Why:** The `/` command system is used by the key handler (Task 5). Building and testing it first.

**Steps:**

1. Create `internal/ui/commands.go`:
```go
package ui

import "strings"

// Command represents a parsed user command.
type Command struct {
	Name string   // "new", "list", "skip", "dismiss", "go", "help"
	Args []string // Remaining arguments after command name
}

// ParseCommand checks if input is a command (starts with / or is "?").
// Returns nil if it's a regular message (reply to session).
func ParseCommand(input string) *Command {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil
	}
	if input == "?" {
		return &Command{Name: "help"}
	}
	if !strings.HasPrefix(input, "/") {
		return nil
	}
	parts := strings.Fields(input[1:]) // Strip leading /
	if len(parts) == 0 {
		return nil
	}
	return &Command{
		Name: strings.ToLower(parts[0]),
		Args: parts[1:],
	}
}
```

2. Create `internal/ui/commands_test.go`:
```go
package ui

import "testing"

func TestParseCommand(t *testing.T) {
	tests := []struct {
		input string
		name  string
		args  []string
		isNil bool
	}{
		{"", "", nil, true},
		{"hello world", "", nil, true},
		{"?", "help", nil, false},
		{"/new", "new", nil, false},
		{"/list", "list", nil, false},
		{"/skip", "skip", nil, false},
		{"/dismiss", "dismiss", nil, false},
		{"/go auth", "go", []string{"auth"}, false},
		{"/GO Auth", "go", []string{"Auth"}, false},
		{"  /new  ", "new", nil, false},
		{"/", "", nil, true},
	}
	for _, tt := range tests {
		cmd := ParseCommand(tt.input)
		if tt.isNil {
			if cmd != nil {
				t.Errorf("ParseCommand(%q) = %+v, want nil", tt.input, cmd)
			}
			continue
		}
		if cmd == nil {
			t.Errorf("ParseCommand(%q) = nil, want command", tt.input)
			continue
		}
		if cmd.Name != tt.name {
			t.Errorf("ParseCommand(%q).Name = %q, want %q", tt.input, cmd.Name, tt.name)
		}
		if len(cmd.Args) == 0 && len(tt.args) == 0 {
			continue
		}
		if len(cmd.Args) != len(tt.args) {
			t.Errorf("ParseCommand(%q).Args = %v, want %v", tt.input, cmd.Args, tt.args)
		}
	}
}
```

3. Run tests: `go test ./internal/ui/ -run TestParseCommand -v`

4. Commit: `feat: add command parser for / commands`

---

## Task 4: Implement Deck View Rendering

**Files:**
- Create: `internal/ui/view_deck.go` (replaces `view_normal.go` and `view_focus.go`)

**Why:** The single rendering path for all four deck states. This is the biggest visual change.

**Steps:**

1. Create `internal/ui/view_deck.go` with these functions:

```go
package ui

import (
	"fmt"
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
		// No active session — fall back to feed
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
		// Working with only the initial prompt — show spinner
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
```

Note: Add `"os"` to the import block for `shortenDir`.

2. Run: `go test ./internal/ui/...` to check compilation. Fix any import issues.

3. Commit: `feat: add deck view rendering (card, feed, help, new-flow)`

---

## Task 5: Implement Deck Key Handler

**Files:**
- Create: `internal/ui/update_deck.go` (replaces `handleNormalKey`, `handleFocusKey`, `handleOverlayKey`)

**Why:** Single key handler for all deck states. Routes to command execution or session reply.

**Steps:**

1. Create `internal/ui/update_deck.go`:

```go
package ui

import (
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/BlinkingLine/cloudterminal/internal/session"
)

// handleDeckKey is the unified key handler for all deck states.
func (m *Model) handleDeckKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Ctrl+C always works
	if msg.Type == tea.KeyCtrlC {
		return m.handleCtrlC()
	}

	switch m.Deck {
	case DeckHelp:
		// Any key exits help
		m.Deck = m.deckDefaultState()
		return m, nil

	case DeckNewFlow:
		return m.handleNewFlowKey(msg)

	case DeckFeed, DeckCard:
		return m.handleDeckInputKey(msg)

	default:
		return m, nil
	}
}

// handleDeckInputKey handles keys when viewing a card or the feed.
func (m *Model) handleDeckInputKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	inputEmpty := strings.TrimSpace(m.Input.Value()) == ""

	switch msg.Type {
	case tea.KeyEnter:
		text := strings.TrimSpace(m.Input.Value())
		if text == "" {
			return m, nil
		}

		// Check if it's a command
		cmd := ParseCommand(text)
		if cmd != nil {
			m.Input.Reset()
			return m.executeCommand(cmd)
		}

		// It's a reply to the current session
		s := m.activeSession()
		if s == nil || s.State != session.NeedsInput {
			return m, nil
		}
		m.handleSendAction()
		return m, nil

	case tea.KeyLeft:
		if inputEmpty {
			m.prevQueueItem()
			m.Deck = DeckCard
			return m, nil
		}

	case tea.KeyRight:
		if inputEmpty {
			m.nextQueueItem()
			m.Deck = DeckCard
			return m, nil
		}

	case tea.KeyCtrlW:
		m.handleDismiss()
		return m, nil

	case tea.KeyCtrlU:
		// Scroll up — handled by chat view
		return m, nil

	case tea.KeyCtrlD:
		// Scroll down — handled by chat view
		return m, nil
	}

	// Pass to textarea
	return m.updateTextarea(msg)
}

// handleNewFlowKey handles keys during /new session creation.
func (m *Model) handleNewFlowKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	nf := m.NewFlow
	if nf == nil {
		m.Deck = m.deckDefaultState()
		return m, nil
	}

	switch msg.Type {
	case tea.KeyEscape:
		m.NewFlow = nil
		m.Deck = m.deckDefaultState()
		m.Input.Reset()
		SetInputPlaceholder(&m.Input, "", false)
		return m, nil

	case tea.KeyEnter:
		text := strings.TrimSpace(m.Input.Value())
		switch nf.Step {
		case NewFlowName:
			if text == "" {
				nf.Error = "Name cannot be empty"
				return m, nil
			}
			validated, err := session.ValidateName(text)
			if err != nil {
				nf.Error = err.Error()
				return m, nil
			}
			// Check for duplicate name
			for _, s := range m.Sessions {
				if s.Name == validated {
					nf.Error = "Session '" + validated + "' already exists"
					return m, nil
				}
			}
			nf.Name = validated
			nf.Error = ""
			nf.Step = NewFlowDir
			m.Input.Reset()
			m.Input.Placeholder = "Enter = current directory"
			return m, nil

		case NewFlowDir:
			dir := text
			if dir == "" || dir == "." {
				cwd, _ := os.Getwd()
				dir = cwd
			}
			// Expand ~ to home dir
			if strings.HasPrefix(dir, "~/") {
				home, _ := os.UserHomeDir()
				dir = filepath.Join(home, dir[2:])
			}
			// Validate directory exists
			info, err := os.Stat(dir)
			if err != nil || !info.IsDir() {
				nf.Error = "Not a valid directory: " + dir
				return m, nil
			}
			nf.Dir = dir
			nf.Error = ""
			nf.Step = NewFlowPrompt
			m.Input.Reset()
			m.Input.Placeholder = "What should Claude do?"
			return m, nil

		case NewFlowPrompt:
			if text == "" {
				nf.Error = "Prompt cannot be empty"
				return m, nil
			}
			// Create the session
			s := session.New(nf.Name, text, nf.Dir, m.RunID)
			m.Sessions = append(m.Sessions, s)
			m.ActiveID = s.ID
			m.Deck = DeckCard

			// Dispatch the session
			m.handleSend(s, text)
			m.Queue.Rebuild(m.Sessions)

			// Clean up flow
			m.NewFlow = nil
			m.Input.Reset()
			SetInputPlaceholder(&m.Input, s.Name, true)
			m.addNotif("+ "+s.Name+" started", false)
			return m, nil
		}
	}

	// Pass to textarea for typing
	return m.updateTextarea(msg)
}

// executeCommand runs a parsed command.
func (m *Model) executeCommand(cmd *Command) (tea.Model, tea.Cmd) {
	switch cmd.Name {
	case "new":
		m.Deck = DeckNewFlow
		m.NewFlow = &NewFlowState{Step: NewFlowName}
		m.Input.Placeholder = "Session name (e.g. auth, tests)"
		return m, nil

	case "list":
		// Show sessions inline as notification-style
		if len(m.Sessions) == 0 {
			m.addNotif("No sessions", false)
			return m, nil
		}
		for _, s := range m.Sessions {
			state := "⟳"
			if s.State == session.NeedsInput {
				state = "●"
			}
			m.addNotif(state+" "+s.Name, false)
		}
		return m, nil

	case "skip":
		m.skipCard()
		return m, nil

	case "dismiss":
		m.handleDismiss()
		return m, nil

	case "go":
		if len(cmd.Args) == 0 {
			m.addNotif("Usage: /go <session-name>", true)
			return m, nil
		}
		name := strings.ToLower(cmd.Args[0])
		for _, s := range m.Sessions {
			if s.Name == name {
				m.ActiveID = s.ID
				m.Deck = DeckCard
				SetInputPlaceholder(&m.Input, s.Name, s.State == session.Working)
				return m, nil
			}
		}
		m.addNotif("Session '"+name+"' not found", true)
		return m, nil

	case "help":
		m.Deck = DeckHelp
		return m, nil

	default:
		m.addNotif("Unknown command: /"+cmd.Name, true)
		return m, nil
	}
}

// deckDefaultState returns DeckCard if there's an active session, otherwise DeckFeed.
func (m *Model) deckDefaultState() DeckState {
	if m.activeSession() != nil {
		return DeckCard
	}
	return DeckFeed
}
```

2. Run: `go test ./internal/ui/...` — check compilation.

3. Commit: `feat: add deck key handler with command execution`

---

## Task 6: Wire Up Deck Mode in Model (Update/View/Init)

**Files:**
- Modify: `internal/ui/model.go` (Update, View, Init, handleKey, handleSessionDone, handleTick, advanceQueue)
- Delete: `internal/ui/view_normal.go`
- Delete: `internal/ui/view_focus.go`
- Delete: `internal/ui/overlay.go`

**Why:** Connect the new deck rendering and key handling to the Bubbletea event loop. Remove old files.

**Steps:**

1. In `model.go`, update `View()` to use `ViewDeck`:
```go
func (m *Model) View() string {
	if m.Width < 40 || m.Height < 10 {
		return "Terminal too small. Need at least 40×10."
	}
	return ViewDeck(m)
}
```

2. Update `handleKey()` to use `handleDeckKey` instead of routing to normal/focus/overlay:
```go
func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	return m.handleDeckKey(msg)
}
```

3. Update `handleSessionDone()` — remove focus-mode-specific logic, replace with deck-aware auto-switching:
   - When a session becomes NeedsInput and user is on the feed, auto-switch to DeckCard
   - When user is typing (input not empty), just add notification

4. Update `handleTick()` — remove `FocusSuggested` logic, simplify to just expire notifications and update dismiss confirmation.

5. Update `advanceQueue()` — when queue is empty after a reply, switch to `DeckFeed` instead of trying to find a read-only session:
```go
func (m *Model) advanceQueue() {
	m.Queue.Rebuild(m.Sessions)
	if len(m.Queue.Items) > 0 {
		m.QueueIndex = 0
		m.ActiveID = m.Queue.Items[0].ID
		m.Deck = DeckCard
		s := m.activeSession()
		if s != nil {
			SetInputPlaceholder(&m.Input, s.Name, s.State == session.Working)
		}
	} else {
		m.Deck = DeckFeed
	}
}
```

6. Delete `internal/ui/view_normal.go`, `internal/ui/view_focus.go`, `internal/ui/overlay.go`.

7. Move `formatDuration` from the deleted `view_normal.go` into `view_deck.go` (if not already there).

8. Update `handleSendAction()` — remove focus-mode clearing logic:
```go
func (m *Model) handleSendAction() {
	s := m.activeSession()
	if s == nil || s.State != session.NeedsInput {
		return
	}
	prompt := strings.TrimSpace(m.Input.Value())
	if prompt == "" {
		return
	}
	s.History = append(s.History, session.Message{Role: "user", Text: prompt})
	m.Input.Reset()
	m.LastActiveID = s.ID
	m.handleSend(s, prompt)
	m.Queue.Rebuild(m.Sessions)
	m.advanceQueue()
}
```

9. Update mock mode first response text in `internal/session/mock.go` — replace references to Ctrl+N, F, and ← → with /new, /skip:
```go
var mockFirstResponse = "Welcome to CloudTerminal! Here's how this works:\n\n" +
	"You just sent me a prompt, and I (Claude) worked on it in the background. " +
	"While I was working, you could have created more sessions with /new — they all run in parallel.\n\n" +
	"Now I need your input. You can:\n" +
	"- Type a reply below and press Enter\n" +
	"- Use /skip to move to the next session\n" +
	"- Use ← → to browse sessions\n\n" +
	"This is mock mode — responses are simulated. " +
	"To use real Claude, run without --mock and make sure `claude` CLI is in your PATH."
```

10. Run: `go test ./...` — fix any remaining compilation errors.

11. Commit: `refactor: wire deck mode into model, remove normal/focus/overlay`

---

## Task 7: Update CLI Arg Parsing for Multi-Project

**Files:**
- Modify: `main.go:147-202` (parseSessionArgs)
- Modify: `main_test.go`

**Why:** Support `name:dir:prompt` format in CLI args.

**Steps:**

1. Update `parseSessionArgs` in `main.go` to handle the `name:dir:prompt` format:
```go
func parseSessionArgs(args []string, runID string) []*session.Session {
	var sessions []*session.Session
	autoIndex := 1

	for _, arg := range args {
		if len(sessions) >= 20 {
			fmt.Fprintf(os.Stderr, "cloudterminal: warning: max 20 sessions — ignoring %q\n", arg)
			continue
		}

		var name, dir, prompt string

		// Count colons to determine format:
		// 0 colons: entire arg is prompt
		// 1 colon:  name:prompt
		// 2+ colons: name:dir:prompt (split on first two colons)
		colonCount := strings.Count(arg, ":")
		if colonCount >= 2 {
			// name:dir:prompt — split on first two colons
			first := strings.Index(arg, ":")
			second := strings.Index(arg[first+1:], ":") + first + 1
			name = strings.TrimSpace(arg[:first])
			dir = strings.TrimSpace(arg[first+1 : second])
			prompt = strings.TrimSpace(arg[second+1:])
		} else if colonCount == 1 {
			// name:prompt
			idx := strings.Index(arg, ":")
			name = strings.TrimSpace(arg[:idx])
			prompt = strings.TrimSpace(arg[idx+1:])
		} else {
			// Just prompt
			prompt = strings.TrimSpace(arg)
		}

		if prompt == "" {
			fmt.Fprintf(os.Stderr, "cloudterminal: warning: empty prompt for %q — skipping\n", arg)
			continue
		}

		// Validate or auto-generate name
		if name == "" {
			name = fmt.Sprintf("s%d", autoIndex)
			autoIndex++
		} else {
			validated, err := session.ValidateName(name)
			if err != nil {
				autoName := fmt.Sprintf("s%d", autoIndex)
				autoIndex++
				fmt.Fprintf(os.Stderr, "cloudterminal: warning: invalid name %q — using %q instead\n", name, autoName)
				name = autoName
			} else {
				name = validated
			}
		}

		// Resolve dir
		if dir == "" {
			cwd, _ := os.Getwd()
			dir = cwd
		} else if strings.HasPrefix(dir, "~/") {
			home, _ := os.UserHomeDir()
			if home != "" {
				dir = filepath.Join(home, dir[2:])
			}
		}

		s := session.New(name, prompt, dir, runID)
		sessions = append(sessions, s)
	}

	return sessions
}
```

Add `"path/filepath"` to main.go imports.

2. Update tests in `main_test.go` for the new format. Add test cases:
   - `"auth:~/code/backend:refactor auth"` → name="auth", dir expanded, prompt="refactor auth"
   - `"auth:refactor auth"` → name="auth", dir=cwd, prompt="refactor auth"
   - `"just a prompt"` → name="s1", dir=cwd, prompt="just a prompt"

3. Run: `go test ./... -v`

4. Commit: `feat: support name:dir:prompt format in CLI args`

---

## Task 8: Clean Up Styles and Remove Dead Code

**Files:**
- Modify: `internal/ui/styles.go` (remove unused styles)
- Modify: `internal/ui/input.go` (update placeholder logic)
- Remove any remaining references to `NormalMode`, `FocusMode`, `ShowStrip`

**Why:** Remove dead code from the mode collapse.

**Steps:**

1. In `styles.go`, remove styles only used by the old card strip and focus mode:
   - Remove `CardStyle`, `ActiveCardStyle`, `WorkingCardStyle` (strip-only)
   - Remove `FocusSeparator` (focus-only)
   - Remove `ProgressFilled`, `ProgressEmpty` (focus progress bar)
   - Keep all other styles (they're used by deck view)

2. In `input.go`, simplify `SetInputPlaceholder` — it no longer needs to know about modes.

3. Search for any remaining references to `NormalMode`, `FocusMode`, `ShowStrip`, `Overlay`, `FocusCleared`, `FocusTotal`, `FocusIncoming`, `FocusSuggested` and remove them.

4. Run: `go test ./...`

5. Commit: `refactor: remove dead styles and mode references`

---

## Task 9: End-to-End Verification

**Files:** None (verification only)

**Steps:**

1. Run full test suite: `go test ./... -v`
2. Build: `go build -ldflags "-X main.version=test" -o cloudterminal .`
3. Test mock mode: `./cloudterminal --mock "auth:refactor auth" "tests:write tests"`
   - Verify: single pane, sessions auto-switch, feed shows when all working
4. Test empty launch: `./cloudterminal --mock`
   - Verify: shows empty state with "/new" hint
   - Type `?` → help screen appears
   - Type `/new` → inline flow starts
   - Complete flow → session starts
5. Test multi-project: `./cloudterminal --mock "auth:~/Desktop:do something"`
   - Verify: dir shown in header
6. Verify `./cloudterminal --version` still works
7. Clean up: `rm cloudterminal`

---

## Verification Checklist

After all tasks:
1. Single pane always — no mode switching confusion
2. `/new` creates sessions inline (no overlay, no Ctrl+N)
3. Auto-switch to next session needing input after reply
4. Live status feed when all sessions working
5. `?` shows command reference
6. Arrow keys browse sessions manually
7. `/go name` jumps to session
8. Multi-project dirs work in CLI args and `/new`
9. Mock mode works for testing
10. All tests pass
