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
