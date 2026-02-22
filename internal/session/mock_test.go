package session

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// collectProgram returns a minimal *tea.Program that can receive messages
// without blocking. It uses a no-op model.
func collectProgram() *tea.Program {
	return tea.NewProgram(noopModel{}, tea.WithoutRenderer())
}

type noopModel struct{}

func (noopModel) Init() tea.Cmd                           { return nil }
func (m noopModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return m, nil }
func (m noopModel) View() string                           { return "" }

// ---------------------------------------------------------------------------
// MockSend — returns a non-empty response
// ---------------------------------------------------------------------------

func TestMockSendReturnsNonEmpty(t *testing.T) {
	s := New("mocktest", "hello", "", "run01")
	sem := make(chan struct{}, 1)
	p := collectProgram()
	go p.Run()
	defer p.Quit()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result := s.MockSend(ctx, "do something", sem, p)

	if result.Err != nil {
		t.Fatalf("MockSend returned error: %v", result.Err)
	}
	if strings.TrimSpace(result.Output) == "" {
		t.Error("MockSend returned empty output")
	}
	if result.SessionID != s.ID {
		t.Errorf("SessionID = %q, want %q", result.SessionID, s.ID)
	}
	if result.CompletedAt.IsZero() {
		t.Error("CompletedAt is zero")
	}
}

// ---------------------------------------------------------------------------
// MockSend — respects context cancellation
// ---------------------------------------------------------------------------

func TestMockSendRespectsCancel(t *testing.T) {
	s := New("mockcancel", "hello", "", "run01")
	sem := make(chan struct{}, 1)
	p := collectProgram()
	go p.Run()
	defer p.Quit()

	// Cancel immediately.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result := s.MockSend(ctx, "do something", sem, p)

	if result.Err == nil {
		t.Fatal("MockSend with cancelled context should return an error")
	}
}

// ---------------------------------------------------------------------------
// ParseStreamLine
// ---------------------------------------------------------------------------

func TestParseStreamLineAssistantText(t *testing.T) {
	line := `{"type":"assistant","content":[{"type":"text","text":"Hello world"}]}`
	text, tool, ok := ParseStreamLine(line)
	if !ok {
		t.Fatal("ParseStreamLine returned ok=false")
	}
	if text != "Hello world" {
		t.Errorf("text = %q, want %q", text, "Hello world")
	}
	if tool != "" {
		t.Errorf("tool = %q, want empty", tool)
	}
}

func TestParseStreamLineContentBlockDelta(t *testing.T) {
	line := `{"type":"content_block_delta","delta":{"type":"text_delta","text":"chunk"}}`
	text, tool, ok := ParseStreamLine(line)
	if !ok {
		t.Fatal("ParseStreamLine returned ok=false")
	}
	if text != "chunk" {
		t.Errorf("text = %q, want %q", text, "chunk")
	}
	if tool != "" {
		t.Errorf("tool = %q, want empty", tool)
	}
}

func TestParseStreamLineToolUse(t *testing.T) {
	line := `{"type":"tool_use","name":"Read"}`
	text, tool, ok := ParseStreamLine(line)
	if !ok {
		t.Fatal("ParseStreamLine returned ok=false")
	}
	if text != "" {
		t.Errorf("text = %q, want empty", text)
	}
	if tool != "Read" {
		t.Errorf("tool = %q, want %q", tool, "Read")
	}
}

func TestParseStreamLineResult(t *testing.T) {
	line := `{"type":"result","result":{"text":"final answer"}}`
	text, tool, ok := ParseStreamLine(line)
	if !ok {
		t.Fatal("ParseStreamLine returned ok=false")
	}
	if text != "final answer" {
		t.Errorf("text = %q, want %q", text, "final answer")
	}
	if tool != "" {
		t.Errorf("tool = %q, want empty", tool)
	}
}

func TestParseStreamLineUnknownType(t *testing.T) {
	line := `{"type":"ping"}`
	text, tool, ok := ParseStreamLine(line)
	if !ok {
		t.Fatal("ParseStreamLine returned ok=false for unknown type")
	}
	if text != "" {
		t.Errorf("text = %q, want empty", text)
	}
	if tool != "" {
		t.Errorf("tool = %q, want empty", tool)
	}
}

func TestParseStreamLineInvalidJSON(t *testing.T) {
	line := `not json at all`
	_, _, ok := ParseStreamLine(line)
	if ok {
		t.Error("ParseStreamLine returned ok=true for invalid JSON")
	}
}
