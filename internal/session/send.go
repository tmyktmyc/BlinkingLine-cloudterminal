package session

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// ---------------------------------------------------------------------------
// Bubbletea message types (exported for the ui package)
// ---------------------------------------------------------------------------

// SessionDoneMsg is sent when a session finishes (success or error).
type SessionDoneMsg struct{ Result SendResult }

// SlotAcquiredMsg is sent when a session acquires a semaphore slot.
type SlotAcquiredMsg struct{ SessionID string }

// StatusHintMsg is sent to update the UI with a tool/status hint.
type StatusHintMsg struct {
	SessionID string
	Hint      string
}

// maxOutputBytes is the maximum accumulated assistant output (1 MB).
const maxOutputBytes = 1 * 1024 * 1024

// Send executes the Claude subprocess for the session.
//
// It acquires a slot from sem, streams stdout line-by-line through the
// stream-json parser, and returns a SendResult. It communicates progress
// back to the Bubbletea program via p.Send.
//
// Send NEVER mutates session state directly — it only communicates via the
// return value and p.Send().
func (s *Session) Send(ctx context.Context, prompt string, sem chan struct{}, p *tea.Program, allowedTools []string) SendResult {
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

	// 2. Build args.
	args := []string{
		"--print",
		"--output-format", "stream-json",
		"--session-id", s.ID,
	}
	if s.CompletedOnce {
		args = append(args, "--resume")
	}
	if len(allowedTools) > 0 {
		args = append(args, "--allowedTools", strings.Join(allowedTools, ","))
	}
	args = append(args, prompt)

	// 3. Create command.
	cmd := exec.CommandContext(ctx, "claude", args...)

	// 4. Platform-specific process attributes and pipes.
	configureProcAttr(cmd)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return SendResult{
			SessionID:   s.ID,
			Err:         fmt.Errorf("stdout pipe: %w", err),
			CompletedAt: time.Now(),
		}
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	// 5. Start the process.
	if err := cmd.Start(); err != nil {
		return SendResult{
			SessionID:   s.ID,
			Err:         fmt.Errorf("start: %w", err),
			CompletedAt: time.Now(),
		}
	}

	var ps ProcState
	configureProcCancel(cmd, &ps)
	if err := afterStart(cmd, &ps); err != nil {
		return SendResult{
			SessionID:   s.ID,
			Err:         fmt.Errorf("after start: %w", err),
			CompletedAt: time.Now(),
		}
	}
	defer cleanupProc(&ps)

	// 6. Wait delay.
	cmd.WaitDelay = 2 * time.Second

	// 7. Read stdout line by line.
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	var output strings.Builder
	truncated := false

	// 8. Parse each line.
	for scanner.Scan() {
		line := scanner.Text()
		text, toolName, _ := ParseStreamLine(line)

		if text != "" && !truncated {
			if output.Len()+len(text) > maxOutputBytes {
				truncated = true
			} else {
				output.WriteString(text)
			}
		}

		if toolName != "" {
			p.Send(StatusHintMsg{SessionID: s.ID, Hint: toolName})
		}
	}

	// 9. Scanner error.
	if err := scanner.Err(); err != nil {
		output.WriteString("\n[output may be incomplete]")
	}

	// 10. Truncation notice.
	if truncated {
		output.WriteString("\n[output truncated at 1MB]")
	}

	// 11. Wait for process exit.
	if err := cmd.Wait(); err != nil {
		if ctx.Err() != nil {
			return SendResult{
				SessionID:   s.ID,
				Output:      output.String(),
				Err:         fmt.Errorf("Timed out"),
				CompletedAt: time.Now(),
			}
		}
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg != "" {
			return SendResult{
				SessionID:   s.ID,
				Output:      output.String(),
				Err:         fmt.Errorf("%s\n%w", errMsg, err),
				CompletedAt: time.Now(),
			}
		}
		return SendResult{
			SessionID:   s.ID,
			Output:      output.String(),
			Err:         err,
			CompletedAt: time.Now(),
		}
	}

	return SendResult{
		SessionID:   s.ID,
		Output:      output.String(),
		CompletedAt: time.Now(),
	}
}
