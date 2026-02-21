package session

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// mockResponses is a set of realistic Claude-style outputs used by MockSend.
var mockResponses = []string{
	"I've updated the `main.go` file to add the HTTP handler for `/api/health`. The endpoint returns a JSON object with `{\"status\": \"ok\"}` and a 200 status code. Would you like me to add tests for this endpoint?",

	"Here's what I found:\n\n1. The `Config` struct is missing the `Timeout` field\n2. The `LoadConfig()` function doesn't validate the YAML schema\n3. There's a race condition in `startWorker()` — two goroutines write to `shared.count` without a mutex\n\nShould I fix all three issues, or would you like to prioritise one?",

	"Done. I've created the following files:\n\n- `internal/auth/jwt.go` — token generation and validation\n- `internal/auth/middleware.go` — HTTP middleware that checks the Authorization header\n- `internal/auth/auth_test.go` — unit tests (all passing)\n\nRun `go test ./internal/auth/` to verify.",

	"The test failure is caused by a nil pointer dereference in `processItem()` at line 47 of `worker.go`. The `item.Metadata` field is nil when the database returns a row with a NULL metadata column. I've added a nil check and a fallback default. All 23 tests pass now.",

	"I've refactored the database layer:\n\n```go\ntype Repository interface {\n    FindByID(ctx context.Context, id string) (*Entity, error)\n    Save(ctx context.Context, e *Entity) error\n    Delete(ctx context.Context, id string) error\n}\n```\n\nThe concrete `PostgresRepository` implements this interface. I also updated all callers in `service.go` to use the interface instead of the concrete type.",

	"Looking at the error logs, the issue is that the TLS certificate expired on 2025-12-01. The application falls back to plaintext HTTP when the handshake fails, which triggers the `ERR_INSECURE_RESPONSE` in the browser.\n\nTo fix this:\n1. Renew the certificate with `certbot renew`\n2. Restart the reverse proxy: `systemctl restart nginx`\n\nWould you like me to write a cron job to auto-renew the certificate?",
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

	// 3. Return a random mock response.
	resp := mockResponses[rand.Intn(len(mockResponses))]
	return SendResult{
		SessionID:   s.ID,
		Output:      resp,
		CompletedAt: time.Now(),
	}
}
