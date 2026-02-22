package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"github.com/BlinkingLine/cloudterminal/internal/config"
	"github.com/BlinkingLine/cloudterminal/internal/session"
	"github.com/BlinkingLine/cloudterminal/internal/ui"
)

// version is set at build time via ldflags.
var version = "dev"

func main() {
	// -----------------------------------------------------------------------
	// 1. Parse CLI args: --mock flag, --verbose flag, remaining args are
	//    session specs in the form "name:prompt".
	// -----------------------------------------------------------------------
	// Check for --version before anything else.
	for _, arg := range os.Args[1:] {
		if arg == "--version" {
			fmt.Printf("cloudterminal %s\n", version)
			os.Exit(0)
		}
	}

	mockMode := false
	verbose := false
	var sessionArgs []string

	for _, arg := range os.Args[1:] {
		switch arg {
		case "--mock":
			mockMode = true
		case "--verbose":
			verbose = true
		default:
			sessionArgs = append(sessionArgs, arg)
		}
	}

	// -----------------------------------------------------------------------
	// 2. Check `claude` is in PATH (unless mock mode).
	// -----------------------------------------------------------------------
	if !mockMode {
		if _, err := exec.LookPath("claude"); err != nil {
			fmt.Fprintf(os.Stderr, "cloudterminal: warning: 'claude' not found in PATH — enabling mock mode\n")
			mockMode = true
		}
	}

	// -----------------------------------------------------------------------
	// 3. Check Claude CLI version (unless mock mode).
	// -----------------------------------------------------------------------
	if !mockMode {
		checkClaudeVersion()
	}

	// -----------------------------------------------------------------------
	// 4. Load config — warn on error, continue with defaults.
	// -----------------------------------------------------------------------
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cloudterminal: warning: config load error: %v\n", err)
	}

	// -----------------------------------------------------------------------
	// 5. Tool safety warning (real mode only, empty allowed_tools).
	// -----------------------------------------------------------------------
	if !mockMode && len(cfg.AllowedTools) == 0 {
		fmt.Fprintf(os.Stderr, "cloudterminal: warning: no allowed_tools configured — Claude will use its default tool policy\n")
		fmt.Fprintf(os.Stderr, "  Set allowed_tools in config to restrict which tools Claude can use.\n")
	}

	// -----------------------------------------------------------------------
	// 6. Generate run ID.
	// -----------------------------------------------------------------------
	runID := uuid.New().String()[:8]

	// -----------------------------------------------------------------------
	// 7. Parse session args.
	// -----------------------------------------------------------------------
	sessions := parseSessionArgs(sessionArgs, runID)

	// -----------------------------------------------------------------------
	// 8. Create root context.
	// -----------------------------------------------------------------------
	ctx, cancel := context.WithCancel(context.Background())

	// -----------------------------------------------------------------------
	// 9. Create Bubbletea model.
	// -----------------------------------------------------------------------
	model := ui.NewModel(cfg, sessions, runID, mockMode, verbose, ctx, cancel)

	// -----------------------------------------------------------------------
	// 10. Create tea.Program.
	// -----------------------------------------------------------------------
	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
		tea.WithContext(ctx),
	)

	// -----------------------------------------------------------------------
	// 11. Give model access to the program for goroutine messaging.
	// -----------------------------------------------------------------------
	model.SetProgram(p)

	// -----------------------------------------------------------------------
	// 12. Run the TUI.
	// -----------------------------------------------------------------------
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "cloudterminal: error: %v\n", err)
		cancel()
		os.Exit(1)
	}

	// -----------------------------------------------------------------------
	// 13. Cancel all subprocess contexts.
	// -----------------------------------------------------------------------
	cancel()

	// -----------------------------------------------------------------------
	// 14. Wait for goroutines with 3-second hard deadline.
	// -----------------------------------------------------------------------
	done := make(chan struct{})
	go func() {
		model.WaitForShutdown()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
	}
}

// parseSessionArgs parses CLI session specifications into Session objects.
// Formats supported:
//   - "prompt"              → auto name, cwd, prompt
//   - "name:prompt"         → name, cwd, prompt
//   - "name:dir:prompt"     → name, dir, prompt (split on first two colons)
//
// Empty prompts are skipped. Invalid names fall back to auto-generated names.
// A maximum of 20 sessions are accepted; extras are warned and dropped.
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
			first := strings.Index(arg, ":")
			second := strings.Index(arg[first+1:], ":") + first + 1
			name = strings.TrimSpace(arg[:first])
			dir = strings.TrimSpace(arg[first+1 : second])
			prompt = strings.TrimSpace(arg[second+1:])
		} else if colonCount == 1 {
			idx := strings.Index(arg, ":")
			name = strings.TrimSpace(arg[:idx])
			prompt = strings.TrimSpace(arg[idx+1:])
		} else {
			prompt = strings.TrimSpace(arg)
		}

		if prompt == "" {
			fmt.Fprintf(os.Stderr, "cloudterminal: warning: empty prompt for %q — skipping\n", arg)
			continue
		}

		// Validate or auto-generate name.
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

		// Resolve dir.
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

// checkClaudeVersion runs `claude --version`, extracts a semver string, and
// warns if the major version is less than 1.
func checkClaudeVersion() {
	out, err := exec.Command("claude", "--version").Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cloudterminal: warning: could not check claude version: %v\n", err)
		return
	}

	re := regexp.MustCompile(`(\d+\.\d+\.\d+)`)
	match := re.FindStringSubmatch(string(out))
	if len(match) < 2 {
		fmt.Fprintf(os.Stderr, "cloudterminal: warning: could not parse claude version from output: %s\n", strings.TrimSpace(string(out)))
		return
	}

	claudeVersion := match[1]
	parts := strings.SplitN(claudeVersion, ".", 3)
	if parts[0] == "0" {
		fmt.Fprintf(os.Stderr, "cloudterminal: warning: claude version %s detected — major version < 1, some features may not work\n", claudeVersion)
	}
}
