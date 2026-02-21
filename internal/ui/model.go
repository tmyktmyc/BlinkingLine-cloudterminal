package ui

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/BlinkingLine/cloudterminal/internal/config"
	"github.com/BlinkingLine/cloudterminal/internal/queue"
	"github.com/BlinkingLine/cloudterminal/internal/session"
)

// ---------------------------------------------------------------------------
// Overlay types
// ---------------------------------------------------------------------------

// OverlayStep tracks which field the new-session overlay is on.
type OverlayStep int

const (
	OverlayStepName   OverlayStep = iota // entering the session name
	OverlayStepPrompt                    // entering the initial prompt
)

// OverlayState holds transient state for the new-session overlay dialog.
type OverlayState struct {
	Step        OverlayStep
	NameInput   string
	PromptInput string
	Error       string
}

// ---------------------------------------------------------------------------
// Model
// ---------------------------------------------------------------------------

// Model is the central Bubbletea model for CloudTerminal. It implements
// tea.Model (Init, Update, View) and coordinates all sessions, the queue,
// input handling, and rendering.
type Model struct {
	Sessions   []*session.Session
	Queue      queue.Queue
	ActiveID   string
	QueueIndex int

	Mode      Mode
	ShowStrip bool

	Input        textarea.Model
	DefaultReply string

	FocusCleared  int
	FocusTotal    int
	FocusIncoming int

	Notifs []Notification

	Width  int
	Height int

	Overlay *OverlayState

	DismissConfirmID string
	DismissConfirmAt time.Time

	RunID      string
	Sem        chan struct{}
	RootCtx    context.Context
	RootCancel context.CancelFunc
	ShutdownWg sync.WaitGroup

	Config   config.Config
	MockMode bool
	Verbose  bool

	// LastActiveID tracks the most recently interacted-with session so the
	// UI has something to show when the queue is empty.
	LastActiveID string
	// FocusSuggested tracks whether the focus auto-suggest notification has
	// already been fired for the current queue buildup.
	FocusSuggested bool
	// LastCtrlC records the time of the last Ctrl+C press for double-press
	// quit detection.
	LastCtrlC time.Time

	// program is a reference to the running tea.Program so that goroutines
	// can send messages back to the event loop via program.Send().
	program *tea.Program
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
		RunID:        runID,
		Sem:          make(chan struct{}, cfg.MaxConcurrent),
		RootCtx:      ctx,
		RootCancel:   cancel,
		Config:       cfg,
		MockMode:     mockMode,
		Verbose:      verbose,
		Input:        NewInput(80), // default width; resized on first WindowSizeMsg
		DefaultReply: cfg.DefaultReply,
		ShowStrip:    true,
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
	}

	return m, nil
}

// View renders the entire UI. It delegates to ViewNormal or ViewFocus
// depending on the current mode. When the new-session overlay is active,
// it replaces the entire view with the centered overlay dialog.
func (m *Model) View() string {
	if m.Width < 60 || m.Height < 16 {
		return "Please resize terminal (min 60x16)"
	}

	// Overlay takes over the full screen when active.
	if m.Overlay != nil {
		return renderOverlay(m)
	}

	if len(m.Sessions) == 0 {
		return "No active sessions. Press Ctrl+N to create a session."
	}
	if m.Mode == FocusMode {
		return ViewFocus(m)
	}
	return ViewNormal(m)
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
	// Overlay takes priority.
	if m.Overlay != nil {
		return m.handleOverlayKey(msg)
	}

	// Ctrl+C: double-press quit.
	if msg.Type == tea.KeyCtrlC {
		return m.handleCtrlC()
	}

	if m.Mode == FocusMode {
		return m.handleFocusKey(msg)
	}
	return m.handleNormalKey(msg)
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

	if m.Mode == FocusMode {
		m.FocusTotal++
		m.FocusIncoming++
	}

	// Notification.
	if msg.Result.Err != nil {
		m.addNotif(fmt.Sprintf("! %s error: %s", s.Name, msg.Result.Err.Error()), true)
	} else {
		m.addNotif(fmt.Sprintf("+ %s needs you", s.Name), false)
	}

	// If no active session or queue was empty, set this as active.
	if m.ActiveID == "" || m.Queue.Len() == 1 {
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

	// Focus auto-suggest.
	if m.Mode == NormalMode && m.Queue.Len() >= m.Config.FocusThreshold && !m.FocusSuggested {
		m.addNotif(fmt.Sprintf("Queue has %d items — press F for focus mode", m.Queue.Len()), false)
		m.FocusSuggested = true
	}
	if m.Queue.Len() == 0 {
		m.FocusSuggested = false
	}

	cmd := tea.Tick(time.Second, func(t time.Time) tea.Msg { return TickMsg(t) })
	return m, cmd
}

// ---------------------------------------------------------------------------
// Key handlers — Normal Mode
// ---------------------------------------------------------------------------

func (m *Model) handleNormalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	inputEmpty := strings.TrimSpace(m.Input.Value()) == ""

	switch {
	case msg.Type == tea.KeyEnter:
		if !inputEmpty {
			active := m.activeSession()
			if active != nil && active.State == session.NeedsInput {
				m.handleSendAction()
			}
		}
		return m, nil

	case msg.Type == tea.KeyLeft || msg.Type == tea.KeyShiftTab:
		if inputEmpty {
			m.prevQueueItem()
		} else {
			return m.updateTextarea(msg)
		}
		return m, nil

	case msg.Type == tea.KeyRight || msg.Type == tea.KeyTab:
		if inputEmpty {
			m.nextQueueItem()
		} else {
			return m.updateTextarea(msg)
		}
		return m, nil

	case msg.Type == tea.KeyCtrlN:
		m.openOverlay()
		return m, nil

	case msg.Type == tea.KeyCtrlW:
		m.handleDismiss()
		return m, nil

	case msg.Type == tea.KeyCtrlU:
		// Scroll up — will be handled by view layer.
		return m, nil

	case msg.Type == tea.KeyCtrlD:
		// Scroll down — will be handled by view layer.
		return m, nil

	case msg.Type == tea.KeyRunes:
		r := string(msg.Runes)
		if inputEmpty {
			switch strings.ToUpper(r) {
			case "F":
				if m.Queue.Len() > 0 {
					m.enterFocusMode()
				}
				return m, nil
			case "V":
				m.ShowStrip = !m.ShowStrip
				return m, nil
			}
		}
		return m.updateTextarea(msg)
	}

	// Alt+1..9 and other keys pass to textarea.
	return m.updateTextarea(msg)
}

// ---------------------------------------------------------------------------
// Key handlers — Focus Mode
// ---------------------------------------------------------------------------

func (m *Model) handleFocusKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	inputEmpty := strings.TrimSpace(m.Input.Value()) == ""

	switch {
	case msg.Type == tea.KeyEnter:
		if !inputEmpty {
			m.handleSendAction()
		}
		return m, nil

	case msg.Type == tea.KeyCtrlJ: // Ctrl+Enter (often mapped as Ctrl+J)
		if inputEmpty {
			// Send default reply.
			m.Input.SetValue(m.DefaultReply)
			m.handleSendAction()
		}
		return m, nil

	case msg.Type == tea.KeyRunes:
		r := string(msg.Runes)
		if inputEmpty {
			switch strings.ToUpper(r) {
			case "S":
				if m.Queue.Len() > 1 {
					m.skipCard()
				}
				return m, nil
			}
		}
		return m.updateTextarea(msg)

	case msg.Type == tea.KeyCtrlW:
		m.handleDismissFocus()
		return m, nil

	case msg.Type == tea.KeyCtrlU:
		return m, nil

	case msg.Type == tea.KeyCtrlD:
		return m, nil

	case msg.Type == tea.KeyEsc:
		m.exitFocusMode()
		return m, nil
	}

	return m.updateTextarea(msg)
}

// ---------------------------------------------------------------------------
// Overlay key handler
// ---------------------------------------------------------------------------

func (m *Model) handleOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	ov := m.Overlay

	switch msg.Type {
	case tea.KeyEsc:
		m.Overlay = nil
		return m, nil

	case tea.KeyEnter:
		switch ov.Step {
		case OverlayStepName:
			name, err := session.ValidateName(ov.NameInput)
			if err != nil {
				ov.Error = err.Error()
				return m, nil
			}
			ov.NameInput = name
			ov.Error = ""
			ov.Step = OverlayStepPrompt
			return m, nil

		case OverlayStepPrompt:
			prompt := strings.TrimSpace(ov.PromptInput)
			if prompt == "" {
				ov.Error = "prompt cannot be empty"
				return m, nil
			}
			// Create and dispatch session.
			s := session.New(ov.NameInput, prompt, m.RunID)
			m.Sessions = append(m.Sessions, s)
			m.handleSend(s, prompt)
			m.ActiveID = s.ID
			m.Overlay = nil
			m.addNotif(fmt.Sprintf("Created session %s", s.Name), false)
			return m, nil
		}

	case tea.KeyBackspace:
		switch ov.Step {
		case OverlayStepName:
			if len(ov.NameInput) > 0 {
				ov.NameInput = ov.NameInput[:len(ov.NameInput)-1]
			}
		case OverlayStepPrompt:
			if len(ov.PromptInput) > 0 {
				ov.PromptInput = ov.PromptInput[:len(ov.PromptInput)-1]
			} else {
				// Backspace on empty prompt goes back to name.
				ov.Step = OverlayStepName
			}
		}
		return m, nil

	case tea.KeyRunes:
		switch ov.Step {
		case OverlayStepName:
			ov.NameInput += string(msg.Runes)
		case OverlayStepPrompt:
			ov.PromptInput += string(msg.Runes)
		}
		return m, nil
	}

	return m, nil
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

	if m.Mode == FocusMode {
		m.FocusCleared++
	}

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

// handleDismissFocus is the focus-mode variant of dismiss.
func (m *Model) handleDismissFocus() {
	s := m.activeSession()
	if s == nil {
		return
	}

	if s.State == session.NeedsInput {
		m.FocusTotal--
		m.removeSession(s.ID)
	} else if s.State == session.Working {
		now := time.Now()
		if m.DismissConfirmID == s.ID && now.Sub(m.DismissConfirmAt) < 2*time.Second {
			if s.CancelFunc != nil {
				s.CancelFunc()
			}
			m.FocusTotal--
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
// Mode transitions
// ---------------------------------------------------------------------------

func (m *Model) enterFocusMode() {
	m.Mode = FocusMode
	m.FocusCleared = 0
	m.FocusTotal = m.Queue.Len()
	m.FocusIncoming = 0
	if m.Queue.Len() > 0 {
		m.QueueIndex = 0
		m.ActiveID = m.Queue.Items[0].ID
	}
	m.addNotif("Entered focus mode", false)
}

func (m *Model) exitFocusMode() {
	m.Mode = NormalMode
	m.addNotif("Exited focus mode", false)
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

// advanceQueue sets the active session after a queue change. If the queue is
// empty it falls back to LastActiveID or the first session.
func (m *Model) advanceQueue() {
	if m.Queue.Len() == 0 {
		if m.Mode == FocusMode {
			// In focus mode, keep ActiveID (waiting screen will show).
			return
		}
		// Normal mode: show last active or first session.
		if m.LastActiveID != "" {
			if m.findSession(m.LastActiveID) != nil {
				m.ActiveID = m.LastActiveID
				return
			}
		}
		if len(m.Sessions) > 0 {
			m.ActiveID = m.Sessions[0].ID
		} else {
			m.ActiveID = ""
		}
		return
	}

	// Clamp QueueIndex.
	if m.QueueIndex >= m.Queue.Len() {
		m.QueueIndex = m.Queue.Len() - 1
	}
	if m.QueueIndex < 0 {
		m.QueueIndex = 0
	}
	if s := m.Queue.At(m.QueueIndex); s != nil {
		m.ActiveID = s.ID
	}
}

// ---------------------------------------------------------------------------
// Overlay
// ---------------------------------------------------------------------------

func (m *Model) openOverlay() {
	m.Overlay = &OverlayState{
		Step: OverlayStepName,
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
