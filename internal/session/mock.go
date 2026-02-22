package session

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// mockFirstResponse is an educational welcome message returned on the
// session's very first completion. It teaches the user how CloudTerminal
// works while they explore mock mode.
var mockFirstResponse = "Welcome to CloudTerminal! Here's how this works:\n\n" +
	"You just sent me a prompt, and I (Claude) worked on it in the background. " +
	"While I was working, you could have created more sessions with /new — they all run in parallel.\n\n" +
	"Now I need your input. You can:\n" +
	"- Type a reply below and press Enter\n" +
	"- Use /skip to move to the next session\n" +
	"- Use ← → to browse sessions\n\n" +
	"This is mock mode — responses are simulated. " +
	"To use real Claude, run without --mock and make sure `claude` CLI is in your PATH."

// mockFollowUpResponses are short, clearly labeled mock responses used
// after the first educational message.
var mockFollowUpResponses = []string{
	"[mock] Got it, I've made those changes. The tests are passing. Anything else you'd like me to adjust?",
	"[mock] Done. I updated the files and ran the test suite — all green. Want me to move on to the next part?",
	"[mock] I found a couple of issues and fixed them. Here's a summary of what changed. Let me know if you want to dig deeper.",
	"[mock] Finished the refactor. The code is cleaner now and the benchmarks look good. What's next?",
	"[mock] I've analyzed the problem and here's my recommendation. Should I go ahead and implement it?",
}

// mockHints is a set of realistic tool/status hints for MockSend.
var mockHints = []string{
	"Reading files",
	"Editing main.go",
	"Running tests",
	"Searching codebase",
	"Writing auth.go",
	"Analyzing logs",
	"Checking types",
	"Formatting code",
}

// MockSend simulates a Claude subprocess for testing without a real binary.
//
// It acquires a semaphore slot, waits 2-8 seconds (sending status hints
// during the wait), and returns a random mock response. Context cancellation
// is respected at each sleep point.
func (s *Session) MockSend(ctx context.Context, prompt string, sem chan struct{}, p *tea.Program) SendResult {
	// 1. Acquire semaphore slot.
	select {
	case sem <- struct{}{}:
		defer func() { <-sem }()
		p.Send(SlotAcquiredMsg{SessionID: s.ID})
	case <-ctx.Done():
		return SendResult{
			SessionID:   s.ID,
			Err:         fmt.Errorf("timed out waiting for subprocess slot"),
			CompletedAt: time.Now(),
		}
	}

	// Total simulated work time: 2-8 seconds.
	totalMs := 2000 + rand.Intn(6001) // 2000..8000
	third := time.Duration(totalMs/3) * time.Millisecond

	// Phase 1: first third of the wait, then send hint 1.
	select {
	case <-time.After(third):
	case <-ctx.Done():
		return SendResult{
			SessionID:   s.ID,
			Err:         fmt.Errorf("cancelled"),
			CompletedAt: time.Now(),
		}
	}
	hint1 := mockHints[rand.Intn(len(mockHints))]
	p.Send(StatusHintMsg{SessionID: s.ID, Hint: hint1})

	// Phase 2: second third of the wait, then send hint 2.
	select {
	case <-time.After(third):
	case <-ctx.Done():
		return SendResult{
			SessionID:   s.ID,
			Err:         fmt.Errorf("cancelled"),
			CompletedAt: time.Now(),
		}
	}
	hint2 := mockHints[rand.Intn(len(mockHints))]
	p.Send(StatusHintMsg{SessionID: s.ID, Hint: hint2})

	// Phase 3: final third of the wait.
	select {
	case <-time.After(third):
	case <-ctx.Done():
		return SendResult{
			SessionID:   s.ID,
			Err:         fmt.Errorf("cancelled"),
			CompletedAt: time.Now(),
		}
	}

	// 3. Return an educational first response or a random follow-up.
	var resp string
	if len(s.History) <= 1 {
		// First response for this session — educational
		resp = mockFirstResponse
	} else {
		resp = mockFollowUpResponses[rand.Intn(len(mockFollowUpResponses))]
	}
	return SendResult{
		SessionID:   s.ID,
		Output:      resp,
		CompletedAt: time.Now(),
	}
}
