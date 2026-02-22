package ui

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/BlinkingLine/cloudterminal/internal/config"
	"github.com/BlinkingLine/cloudterminal/internal/queue"
	"github.com/BlinkingLine/cloudterminal/internal/session"
)

// ---------------------------------------------------------------------------
// New-flow types
// ---------------------------------------------------------------------------

// NewFlowStep tracks which field the inline session-creation flow is on.
type NewFlowStep int

const (
	NewFlowName   NewFlowStep = iota
	NewFlowDir
	NewFlowPrompt
)

// NewFlowState holds transient state for the inline /new session creation.
type NewFlowState struct {
	Step   NewFlowStep
	Name   string
	Dir    string
	Prompt string
	Error  string
}

// ---------------------------------------------------------------------------
// Model
// ---------------------------------------------------------------------------

// Model is the central Bubbletea model for CloudTerminal. It implements
// tea.Model (Init, Update, View) and coordinates all sessions, the queue,
// input handling, and rendering.
type Model struct {
	// Session management
	Sessions   []*session.Session
	Queue      queue.Queue
	ActiveID   string
	QueueIndex int

	// Deck state — what's currently shown
	Deck    DeckState
	NewFlow *NewFlowState // Non-nil during /new flow

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

// NewModel creates a fully initialised Model ready to be passed to
// tea.NewProgram.
func NewModel(
	cfg config.Config,
	sessions []*session.Session,
	runID string,
	mockMode, verbose bool,
	ctx context.Context,
	cancel context.CancelFunc,
) *Model {
	m := &Model{
		Sessions:     sessions,
		ActiveID:     "",
		Deck:         DeckFeed, // Start with feed
		RunID:        runID,
		Sem:          make(chan struct{}, cfg.MaxConcurrent),
		RootCtx:      ctx,
		RootCancel:   cancel,
		Config:       cfg,
		MockMode:     mockMode,
		Verbose:      verbose,
		Input:        NewInput(80), // default width; resized on first WindowSizeMsg
		Spinner:      newSpinner(),
		DefaultReply: cfg.DefaultReply,
	}
	if len(sessions) > 0 {
		m.ActiveID = sessions[0].ID
		m.Deck = DeckCard
	}
	return m
}

// SetProgram stores the tea.Program reference. main.go must call this after
// tea.NewProgram but before p.Run() so that goroutines can call
// m.program.Send().
func (m *Model) SetProgram(p *tea.Program) {
	m.program = p
}

// WaitForShutdown blocks until all session goroutines have finished. main.go
// should call this after p.Run() returns.
func (m *Model) WaitForShutdown() {
	m.ShutdownWg.Wait()
}

// ---------------------------------------------------------------------------
// tea.Model interface
// ---------------------------------------------------------------------------

// Init returns the initial command: a 1-second tick and dispatches any
// sessions that were created from CLI arguments (already in Working state).
func (m *Model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		tea.Tick(time.Second, func(t time.Time) tea.Msg { return TickMsg(t) }),
		m.Spinner.Tick,
	}
	// Dispatch sessions that start in Working state (from CLI args).
	for _, s := range m.Sessions {
		if s.State == session.Working && len(s.History) > 0 {
			prompt := s.History[0].Text
			m.handleSend(s, prompt)
		}
	}
	return tea.Batch(cmds...)
}

// Update is the core state machine. It processes every Bubbletea message and
// returns an updated model plus any commands to execute.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		return m.handleWindowSize(msg)

	case tea.KeyMsg:
		return m.handleKey(msg)

	case session.SessionDoneMsg:
		return m.handleSessionDone(msg)

	case session.SlotAcquiredMsg:
		return m.handleSlotAcquired(msg)

	case session.StatusHintMsg:
		return m.handleStatusHint(msg)

	case TickMsg:
		return m.handleTick()

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.Spinner, cmd = m.Spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

// View renders the entire UI via the single-pane deck renderer.
func (m *Model) View() string {
	if m.Width < 40 || m.Height < 10 {
		return "Terminal too small. Need at least 40×10."
	}
	return ViewDeck(m)
}

// newSpinner creates a MiniDot spinner styled with the Amber color.
func newSpinner() spinner.Model {
	s := spinner.New()
	s.Spinner = spinner.MiniDot
	s.Style = lipgloss.NewStyle().Foreground(Amber)
	return s
}

// ---------------------------------------------------------------------------
// Message handlers
// ---------------------------------------------------------------------------

func (m *Model) handleWindowSize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.Width = msg.Width
	m.Height = msg.Height
	m.Input.SetWidth(msg.Width - 4)
	return m, nil
}

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	return m.handleDeckKey(msg)
}

func (m *Model) handleCtrlC() (tea.Model, tea.Cmd) {
	now := time.Now()
	if !m.LastCtrlC.IsZero() && now.Sub(m.LastCtrlC) < 500*time.Millisecond {
		// Double press: cancel all and quit.
		m.RootCancel()
		return m, tea.Quit
	}
	m.LastCtrlC = now
	m.addNotif("Press Ctrl+C again to quit", false)
	return m, nil
}

func (m *Model) handleSessionDone(msg session.SessionDoneMsg) (tea.Model, tea.Cmd) {
	s := m.findSession(msg.Result.SessionID)
	if s == nil {
		// Session was dismissed while working.
		return m, nil
	}

	s.State = session.NeedsInput
	s.SlotAcquired = false
	s.CancelFunc = nil
	s.StatusHint = ""

	if msg.Result.Err == nil {
		s.CompletedOnce = true
	}

	s.EnteredQueue = msg.Result.CompletedAt

	// Sanitize and append Claude's response to history.
	output := session.Sanitize(msg.Result.Output)
	if output != "" {
		s.History = append(s.History, session.Message{Role: "claude", Text: output})
	}

	// Error message appended too.
	if msg.Result.Err != nil {
		errText := fmt.Sprintf("[error: %s]", msg.Result.Err.Error())
		s.History = append(s.History, session.Message{Role: "claude", Text: errText})
	}

	m.Queue.Rebuild(m.Sessions)

	// Notification.
	if msg.Result.Err != nil {
		m.addNotif(fmt.Sprintf("! %s error: %s", s.Name, msg.Result.Err.Error()), true)
	} else {
		m.addNotif(fmt.Sprintf("+ %s needs you", s.Name), false)
	}

	// Auto-switch: if on feed or no active session, jump to this card.
	inputEmpty := strings.TrimSpace(m.Input.Value()) == ""
	if inputEmpty && (m.Deck == DeckFeed || m.ActiveID == "") {
		m.ActiveID = s.ID
		m.Deck = DeckCard
		idx := m.Queue.IndexOf(s.ID)
		if idx >= 0 {
			m.QueueIndex = idx
		}
		SetInputPlaceholder(&m.Input, s.Name, false)
	} else if m.ActiveID == "" || m.Queue.Len() == 1 {
		idx := m.Queue.IndexOf(s.ID)
		if idx >= 0 {
			m.QueueIndex = idx
			m.ActiveID = s.ID
		}
	}

	// Terminal bell.
	if m.Config.BellOnQueue {
		fmt.Print("\a")
	}

	return m, nil
}

func (m *Model) handleSlotAcquired(msg session.SlotAcquiredMsg) (tea.Model, tea.Cmd) {
	s := m.findSession(msg.SessionID)
	if s != nil {
		s.SlotAcquired = true
	}
	return m, nil
}

func (m *Model) handleStatusHint(msg session.StatusHintMsg) (tea.Model, tea.Cmd) {
	s := m.findSession(msg.SessionID)
	if s != nil {
		s.StatusHint = msg.Hint
	}
	return m, nil
}

func (m *Model) handleTick() (tea.Model, tea.Cmd) {
	now := time.Now()

	// Expire old notifications (>3s).
	alive := m.Notifs[:0]
	for _, n := range m.Notifs {
		if now.Sub(n.CreatedAt) < 3*time.Second {
			alive = append(alive, n)
		}
	}
	m.Notifs = alive

	// Expire dismiss confirmation (>2s).
	if m.DismissConfirmID != "" && now.Sub(m.DismissConfirmAt) >= 2*time.Second {
		m.DismissConfirmID = ""
	}

	cmd := tea.Tick(time.Second, func(t time.Time) tea.Msg { return TickMsg(t) })
	return m, cmd
}

// ---------------------------------------------------------------------------
// Actions
// ---------------------------------------------------------------------------

// handleSendAction takes the current input, appends to history, dispatches
// the session, and advances the queue.
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

// handleSend spawns the goroutine that runs the Claude subprocess (real or mock).
func (m *Model) handleSend(s *session.Session, prompt string) {
	timeout := m.Config.SubprocessTimeout
	ctx, cancel := context.WithTimeout(m.RootCtx, timeout)
	s.CancelFunc = cancel
	s.State = session.Working
	s.SlotAcquired = false
	s.StatusHint = ""
	s.StartedAt = time.Now()

	sem := m.Sem
	mockMode := m.MockMode
	allowedTools := m.Config.AllowedTools
	p := m.program

	m.ShutdownWg.Add(1)
	go func() {
		defer m.ShutdownWg.Done()
		defer cancel()
		var result session.SendResult
		if mockMode {
			result = s.MockSend(ctx, prompt, sem, p)
		} else {
			result = s.Send(ctx, prompt, sem, p, allowedTools)
		}
		p.Send(session.SessionDoneMsg{Result: result})
	}()
}

// handleDismiss removes a NeedsInput session, or initiates double-press
// confirmation for Working sessions.
func (m *Model) handleDismiss() {
	s := m.activeSession()
	if s == nil {
		return
	}

	switch s.State {
	case session.NeedsInput:
		m.removeSession(s.ID)

	case session.Working:
		now := time.Now()
		if m.DismissConfirmID == s.ID && now.Sub(m.DismissConfirmAt) < 2*time.Second {
			// Double press confirmed: cancel and remove.
			if s.CancelFunc != nil {
				s.CancelFunc()
			}
			m.removeSession(s.ID)
			m.DismissConfirmID = ""
		} else {
			m.DismissConfirmID = s.ID
			m.DismissConfirmAt = now
			m.addNotif(fmt.Sprintf("Press Ctrl+W again to dismiss %s (working)", s.Name), false)
		}
	}
}

// removeSession removes a session by ID, rebuilds the queue, and adjusts
// the active selection.
func (m *Model) removeSession(id string) {
	for i, s := range m.Sessions {
		if s.ID == id {
			m.Sessions = append(m.Sessions[:i], m.Sessions[i+1:]...)
			break
		}
	}
	m.Queue.Rebuild(m.Sessions)
	m.advanceQueue()

	if len(m.Sessions) == 0 {
		m.ActiveID = ""
		m.QueueIndex = 0
	}
}

// skipCard marks the current queue item as skipped and advances to the next.
func (m *Model) skipCard() {
	s := m.activeSession()
	if s == nil {
		return
	}
	s.SkippedAt = time.Now()
	m.Queue.Rebuild(m.Sessions)
	m.advanceQueue()
}

// ---------------------------------------------------------------------------
// Queue navigation
// ---------------------------------------------------------------------------

func (m *Model) prevQueueItem() {
	if m.Queue.Len() == 0 {
		return
	}
	m.QueueIndex--
	if m.QueueIndex < 0 {
		m.QueueIndex = m.Queue.Len() - 1
	}
	if s := m.Queue.At(m.QueueIndex); s != nil {
		m.ActiveID = s.ID
	}
}

func (m *Model) nextQueueItem() {
	if m.Queue.Len() == 0 {
		return
	}
	m.QueueIndex++
	if m.QueueIndex >= m.Queue.Len() {
		m.QueueIndex = 0
	}
	if s := m.Queue.At(m.QueueIndex); s != nil {
		m.ActiveID = s.ID
	}
}

// advanceQueue sets the active session after a queue change.
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

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// activeSession returns the session matching ActiveID, or nil.
func (m *Model) activeSession() *session.Session {
	return m.findSession(m.ActiveID)
}

// findSession performs a linear scan for the session with the given ID.
func (m *Model) findSession(id string) *session.Session {
	for _, s := range m.Sessions {
		if s.ID == id {
			return s
		}
	}
	return nil
}

// addNotif appends a notification.
func (m *Model) addNotif(text string, isError bool) {
	m.Notifs = append(m.Notifs, Notification{
		Text:      text,
		CreatedAt: time.Now(),
		IsError:   isError,
	})
}

// updateTextarea forwards the key to the textarea component.
func (m *Model) updateTextarea(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.Input, cmd = m.Input.Update(msg)
	return m, cmd
}
