package ui

import (
	"context"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"

	"github.com/BlinkingLine/cloudterminal/internal/config"
)

func TestNewSpinner(t *testing.T) {
	s := newSpinner()

	// The spinner should have MiniDot frames.
	if len(s.Spinner.Frames) != len(spinner.MiniDot.Frames) {
		t.Errorf("expected MiniDot frames (len %d), got len %d",
			len(spinner.MiniDot.Frames), len(s.Spinner.Frames))
	}
	for i, f := range s.Spinner.Frames {
		if f != spinner.MiniDot.Frames[i] {
			t.Errorf("frame %d: expected %q, got %q", i, spinner.MiniDot.Frames[i], f)
		}
	}
}

func TestModelHasSpinner(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := config.Config{
		MaxConcurrent: 1,
		DefaultReply:  "ok",
	}
	m := NewModel(cfg, nil, "test-run", true, false, ctx, cancel)

	// Spinner should be initialised (non-zero frames).
	if len(m.Spinner.Spinner.Frames) == 0 {
		t.Fatal("expected Spinner to be initialised with frames")
	}
}

func TestUpdateSpinnerTick(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := config.Config{
		MaxConcurrent: 1,
		DefaultReply:  "ok",
	}
	m := NewModel(cfg, nil, "test-run", true, false, ctx, cancel)

	// Send a spinner.TickMsg through Update.
	// We only set the ID field; the unexported tag field is zero-valued,
	// which is fine for verifying the Update routing.
	tick := spinner.TickMsg{ID: m.Spinner.ID()}
	updated, cmd := m.Update(tick)

	if updated == nil {
		t.Fatal("Update returned nil model")
	}
	// The spinner should return a continuation command.
	if cmd == nil {
		t.Fatal("expected a continuation command from spinner tick")
	}
}

func TestInitIncludesSpinnerTick(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := config.Config{
		MaxConcurrent: 1,
		DefaultReply:  "ok",
	}
	m := NewModel(cfg, nil, "test-run", true, false, ctx, cancel)

	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init returned nil command")
	}
}
