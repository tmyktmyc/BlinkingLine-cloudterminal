# CloudTerminal Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build CloudTerminal — a terminal multiplexer for Claude Code sessions with card-based queue navigation, Normal/Focus modes, and automated cross-platform releases.

**Architecture:** Bubbletea (Elm-architecture) TUI with goroutine-per-session subprocess management. State mutations from goroutines flow through Bubbletea messages to avoid data races. Platform-specific process tree management via build-tagged files. Config auto-created at OS-standard paths.

**Tech Stack:** Go 1.22+, Bubbletea, Lip Gloss, Bubbles (textarea/viewport), google/uuid, GoReleaser, GitHub Actions.

**Reference:** Full spec at `claude-deck-spec.md` in project root. All section references (e.g. "Spec Section 8") refer to this file.

---

### Task 1: Project Scaffolding

**Files:**
- Create: `go.mod`
- Create: `Makefile`
- Create: `internal/config/config.go` (placeholder)
- Create: `internal/session/session.go` (placeholder)
- Create: `internal/queue/queue.go` (placeholder)
- Create: `internal/ui/model.go` (placeholder)
- Create: `main.go` (placeholder)

**Step 1: Initialize Go module**

```bash
cd /Users/brunovisin/Documents/BlinkingLine/cloudterminal
go mod init github.com/BlinkingLine/cloudterminal
```

**Step 2: Create directory structure**

```bash
mkdir -p internal/config internal/session internal/queue internal/ui
```

**Step 3: Create Makefile**

```makefile
.PHONY: build run test clean mock

BINARY=cloudterminal

build:
	go build -o $(BINARY) .

run: build
	./$(BINARY)

mock: build
	./$(BINARY) --mock "demo:show me something cool" "test:write a hello world"

test:
	go test ./...

clean:
	rm -f $(BINARY)
```

**Step 4: Create placeholder main.go**

```go
package main

import "fmt"

func main() {
	fmt.Println("CloudTerminal — not yet implemented")
}
```

**Step 5: Add dependencies**

```bash
go get github.com/charmbracelet/bubbletea@latest
go get github.com/charmbracelet/lipgloss@latest
go get github.com/charmbracelet/bubbles@latest
go get github.com/google/uuid@latest
```

**Step 6: Verify build**

Run: `go build -o cloudterminal .`
Expected: Binary produced, no errors.

**Step 7: Commit**

```bash
git add go.mod go.sum main.go Makefile
git commit -m "scaffold: initialize Go module with dependencies"
```

---

### Task 2: Config Package

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

**Step 1: Write failing tests**

```go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := Default()
	if cfg.DefaultReply != "lgtm, continue" {
		t.Errorf("DefaultReply = %q, want %q", cfg.DefaultReply, "lgtm, continue")
	}
	if cfg.FocusThreshold != 3 {
		t.Errorf("FocusThreshold = %d, want 3", cfg.FocusThreshold)
	}
	if cfg.BellOnQueue != true {
		t.Error("BellOnQueue should default to true")
	}
	if cfg.MaxConcurrent != 5 {
		t.Errorf("MaxConcurrent = %d, want 5", cfg.MaxConcurrent)
	}
	if cfg.SubprocessTimeout.String() != "10m0s" {
		t.Errorf("SubprocessTimeout = %v, want 10m", cfg.SubprocessTimeout)
	}
	if len(cfg.AllowedTools) != 0 {
		t.Errorf("AllowedTools should be empty, got %v", cfg.AllowedTools)
	}
}

func TestLoadCreatesDefault(t *testing.T) {
	dir := t.TempDir()
	cfg, err := LoadFrom(dir)
	if err != nil {
		t.Fatalf("LoadFrom failed: %v", err)
	}
	if cfg.MaxConcurrent != 5 {
		t.Errorf("MaxConcurrent = %d, want 5", cfg.MaxConcurrent)
	}
	// File should have been created
	if _, err := os.Stat(filepath.Join(dir, "config.json")); os.IsNotExist(err) {
		t.Error("config.json was not created")
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "config.json"), []byte("{invalid"), 0600)
	cfg, err := LoadFrom(dir)
	if err != nil {
		t.Fatalf("LoadFrom should not error on invalid JSON, got: %v", err)
	}
	if cfg.MaxConcurrent != 5 {
		t.Error("Should fall back to defaults on invalid JSON")
	}
}

func TestLoadPartialOverride(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"max_concurrent": 10}`), 0600)
	cfg, err := LoadFrom(dir)
	if err != nil {
		t.Fatalf("LoadFrom failed: %v", err)
	}
	if cfg.MaxConcurrent != 10 {
		t.Errorf("MaxConcurrent = %d, want 10", cfg.MaxConcurrent)
	}
	if cfg.DefaultReply != "lgtm, continue" {
		t.Error("Unset fields should keep defaults")
	}
}

func TestValidation(t *testing.T) {
	tests := []struct {
		name  string
		input string
		check func(Config) bool
		desc  string
	}{
		{"max_concurrent too low", `{"max_concurrent": 0}`, func(c Config) bool { return c.MaxConcurrent == 5 }, "should use default"},
		{"max_concurrent too high", `{"max_concurrent": 100}`, func(c Config) bool { return c.MaxConcurrent == 5 }, "should use default"},
		{"focus_threshold too low", `{"focus_threshold": 0}`, func(c Config) bool { return c.FocusThreshold == 3 }, "should use default"},
		{"bad timeout", `{"subprocess_timeout": "abc"}`, func(c Config) bool { return c.SubprocessTimeout.String() == "10m0s" }, "should use default"},
		{"timeout too short", `{"subprocess_timeout": "5s"}`, func(c Config) bool { return c.SubprocessTimeout.String() == "10m0s" }, "should use default"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			os.WriteFile(filepath.Join(dir, "config.json"), []byte(tt.input), 0600)
			cfg, _ := LoadFrom(dir)
			if !tt.check(cfg) {
				t.Error(tt.desc)
			}
		})
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/config/ -v`
Expected: FAIL — types and functions don't exist yet.

**Step 3: Implement config.go**

```go
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const appName = "cloudterminal"

type Config struct {
	DefaultReply      string        `json:"default_reply"`
	FocusThreshold    int           `json:"focus_threshold"`
	BellOnQueue       bool          `json:"bell_on_queue"`
	MaxConcurrent     int           `json:"max_concurrent"`
	SubprocessTimeout time.Duration `json:"-"`
	TimeoutRaw        string        `json:"subprocess_timeout"`
	AllowedTools      []string      `json:"allowed_tools"`
}

func Default() Config {
	return Config{
		DefaultReply:      "lgtm, continue",
		FocusThreshold:    3,
		BellOnQueue:       true,
		MaxConcurrent:     5,
		SubprocessTimeout: 10 * time.Minute,
		TimeoutRaw:        "10m",
		AllowedTools:      []string{},
	}
}

// Dir returns the OS-standard config directory for CloudTerminal.
func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, appName), nil
}

// Load loads config from the OS-standard directory.
func Load() (Config, error) {
	dir, err := Dir()
	if err != nil {
		return Default(), err
	}
	return LoadFrom(dir)
}

// LoadFrom loads config from a specific directory. Creates defaults if missing.
func LoadFrom(dir string) (Config, error) {
	cfg := Default()
	path := filepath.Join(dir, "config.json")

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		if mkErr := os.MkdirAll(dir, 0700); mkErr != nil {
			return cfg, fmt.Errorf("creating config dir: %w", mkErr)
		}
		if wErr := writeDefaults(path); wErr != nil {
			return cfg, fmt.Errorf("writing default config: %w", wErr)
		}
		return cfg, nil
	}
	if err != nil {
		return cfg, nil // permission denied etc — use defaults
	}

	// Parse JSON over defaults (unset fields keep default values)
	if err := json.Unmarshal(data, &cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: invalid config.json, using defaults: %v\n", err)
		return Default(), nil
	}

	cfg = validate(cfg)
	return cfg, nil
}

func validate(cfg Config) Config {
	d := Default()
	if cfg.MaxConcurrent < 1 || cfg.MaxConcurrent > 20 {
		cfg.MaxConcurrent = d.MaxConcurrent
	}
	if cfg.FocusThreshold < 1 || cfg.FocusThreshold > 50 {
		cfg.FocusThreshold = d.FocusThreshold
	}
	if strings.TrimSpace(cfg.DefaultReply) == "" || len(cfg.DefaultReply) > 500 {
		cfg.DefaultReply = d.DefaultReply
	}
	if cfg.TimeoutRaw != "" {
		dur, err := time.ParseDuration(cfg.TimeoutRaw)
		if err != nil || dur < 30*time.Second || dur > 30*time.Minute {
			cfg.SubprocessTimeout = d.SubprocessTimeout
		} else {
			cfg.SubprocessTimeout = dur
		}
	}
	if cfg.AllowedTools == nil {
		cfg.AllowedTools = []string{}
	}
	return cfg
}

func writeDefaults(path string) error {
	cfg := Default()
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/config/ -v`
Expected: All PASS.

**Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat: add config package with load, validate, and defaults"
```

---

### Task 3: Session Types and State

**Files:**
- Create: `internal/session/session.go`
- Create: `internal/session/session_test.go`

**Step 1: Write failing tests**

```go
package session

import (
	"testing"
	"time"
)

func TestNewSession(t *testing.T) {
	s := New("auth", "refactor the auth middleware", "abc12345")
	if s.Name != "auth" {
		t.Errorf("Name = %q, want %q", s.Name, "auth")
	}
	if s.State != Working {
		t.Errorf("State = %v, want Working", s.State)
	}
	if len(s.History) != 1 {
		t.Fatalf("History len = %d, want 1", len(s.History))
	}
	if s.History[0].Role != "user" {
		t.Errorf("First message role = %q, want 'user'", s.History[0].Role)
	}
	if s.History[0].Text != "refactor the auth middleware" {
		t.Errorf("First message text wrong")
	}
	if s.ID == "" {
		t.Error("ID should be set")
	}
}

func TestSessionStateString(t *testing.T) {
	if Working.String() != "Working" {
		t.Error("Working.String() wrong")
	}
	if NeedsInput.String() != "NeedsInput" {
		t.Error("NeedsInput.String() wrong")
	}
}

func TestValidateName(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"auth", true},
		{"my-task", true},
		{"task_1", true},
		{"123abc", true},
		{"-bad", false},
		{"_bad", false},
		{"has space", false},
		{"UPPER", true},      // auto-lowercased
		{"a!b", false},
		{"", false},
		{"abcdefghijklmnopqrstuvwxyz1234567", false}, // 33 chars
		{"abcdefghijklmnopqrstuvwxyz123456", true},   // 32 chars
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			_, err := ValidateName(tt.input)
			if tt.valid && err != nil {
				t.Errorf("ValidateName(%q) unexpected error: %v", tt.input, err)
			}
			if !tt.valid && err == nil {
				t.Errorf("ValidateName(%q) should have failed", tt.input)
			}
		})
	}
}

func TestValidateNameLowercases(t *testing.T) {
	name, err := ValidateName("MyTask")
	if err != nil {
		t.Fatal(err)
	}
	if name != "mytask" {
		t.Errorf("got %q, want %q", name, "mytask")
	}
}

func TestSessionID(t *testing.T) {
	s := New("auth", "prompt", "b7e2f1a0")
	// ID format: ct-{runid}-{name}-{shortid}
	if len(s.ID) == 0 {
		t.Error("ID should not be empty")
	}
	// Should start with ct-b7e2f1a0-auth-
	prefix := "ct-b7e2f1a0-auth-"
	if s.ID[:len(prefix)] != prefix {
		t.Errorf("ID prefix = %q, want %q", s.ID[:len(prefix)], prefix)
	}
}

func TestSendResult(t *testing.T) {
	r := SendResult{
		SessionID:   "test",
		Output:      "hello",
		Err:         nil,
		CompletedAt: time.Now(),
	}
	if r.SessionID != "test" {
		t.Error("SessionID wrong")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/session/ -v`
Expected: FAIL — types don't exist.

**Step 3: Implement session.go**

```go
package session

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

type SessionState int

const (
	Working    SessionState = iota
	NeedsInput
)

func (s SessionState) String() string {
	switch s {
	case Working:
		return "Working"
	case NeedsInput:
		return "NeedsInput"
	default:
		return "Unknown"
	}
}

type Message struct {
	Role string // "user" or "claude"
	Text string
}

type Session struct {
	ID            string
	Name          string
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

type SendResult struct {
	SessionID   string
	Output      string
	Err         error
	CompletedAt time.Time
}

var nameRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9\-_]{0,31}$`)

// ValidateName validates and lowercases a session name.
func ValidateName(input string) (string, error) {
	name := strings.ToLower(strings.TrimSpace(input))
	if name == "" {
		return "", fmt.Errorf("session name cannot be empty")
	}
	if !nameRegex.MatchString(name) {
		return "", fmt.Errorf("invalid session name %q: must be 1-32 chars, a-z 0-9 - _, start with alphanumeric", input)
	}
	return name, nil
}

// New creates a new session in Working state with the initial user message.
func New(name, prompt, runID string) *Session {
	shortID := uuid.New().String()[:8]
	return &Session{
		ID:    fmt.Sprintf("ct-%s-%s-%s", runID, name, shortID),
		Name:  name,
		State: Working,
		History: []Message{
			{Role: "user", Text: prompt},
		},
		StartedAt: time.Now(),
	}
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/session/ -v`
Expected: All PASS.

**Step 5: Commit**

```bash
git add internal/session/session.go internal/session/session_test.go
git commit -m "feat: add session types, state, and name validation"
```

---

### Task 4: ANSI Sanitization

**Files:**
- Create: `internal/session/sanitize.go`
- Create: `internal/session/sanitize_test.go`

**Step 1: Write failing tests**

```go
package session

import "testing"

func TestSanitize(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"plain text", "hello world", "hello world"},
		{"preserves newlines", "line1\nline2", "line1\nline2"},
		{"preserves tabs", "col1\tcol2", "col1\tcol2"},
		{"strips ANSI color", "\x1b[31mred\x1b[0m", "red"},
		{"strips ANSI bold", "\x1b[1mbold\x1b[0m", "bold"},
		{"strips OSC", "\x1b]0;title\x07text", "text"},
		{"strips null bytes", "hello\x00world", "helloworld"},
		{"strips bell", "hello\x07world", "helloworld"},
		{"strips backspace", "hello\x08world", "helloworld"},
		{"complex mix", "\x1b[1;32mGreen\x1b[0m\x00\n\ttab", "Green\n\ttab"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Sanitize(tt.input)
			if got != tt.want {
				t.Errorf("Sanitize(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/session/ -run TestSanitize -v`
Expected: FAIL.

**Step 3: Implement sanitize.go**

```go
package session

import (
	"regexp"
	"strings"
)

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\x1b\][^\x07]*\x07|\x1b\[[0-9;]*m`)

// Sanitize strips ANSI escape sequences and control characters from text,
// preserving newlines and tabs.
func Sanitize(s string) string {
	// Strip ANSI/OSC sequences
	s = ansiRegex.ReplaceAllString(s, "")
	// Strip control chars except \n (0x0a) and \t (0x09)
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r == '\n' || r == '\t' || r >= 0x20 {
			b.WriteRune(r)
		}
	}
	return b.String()
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/session/ -run TestSanitize -v`
Expected: All PASS.

**Step 5: Commit**

```bash
git add internal/session/sanitize.go internal/session/sanitize_test.go
git commit -m "feat: add ANSI/control character sanitization"
```

---

### Task 5: Queue Package

**Files:**
- Create: `internal/queue/queue.go`
- Create: `internal/queue/queue_test.go`

**Step 1: Write failing tests**

```go
package queue

import (
	"testing"
	"time"

	"github.com/BlinkingLine/cloudterminal/internal/session"
)

func TestEmptyQueue(t *testing.T) {
	q := &Queue{}
	q.Rebuild(nil)
	if q.Len() != 0 {
		t.Errorf("Len() = %d, want 0", q.Len())
	}
}

func TestRebuildFiltersWorking(t *testing.T) {
	sessions := []*session.Session{
		{ID: "a", State: session.Working},
		{ID: "b", State: session.NeedsInput, EnteredQueue: time.Now()},
		{ID: "c", State: session.Working},
	}
	q := &Queue{}
	q.Rebuild(sessions)
	if q.Len() != 1 {
		t.Fatalf("Len() = %d, want 1", q.Len())
	}
	if q.Items[0].ID != "b" {
		t.Errorf("Items[0].ID = %q, want 'b'", q.Items[0].ID)
	}
}

func TestRebuildFIFOOrder(t *testing.T) {
	t1 := time.Now()
	t2 := t1.Add(time.Second)
	t3 := t1.Add(2 * time.Second)
	sessions := []*session.Session{
		{ID: "c", State: session.NeedsInput, EnteredQueue: t3},
		{ID: "a", State: session.NeedsInput, EnteredQueue: t1},
		{ID: "b", State: session.NeedsInput, EnteredQueue: t2},
	}
	q := &Queue{}
	q.Rebuild(sessions)
	if q.Items[0].ID != "a" || q.Items[1].ID != "b" || q.Items[2].ID != "c" {
		t.Errorf("order wrong: %s %s %s", q.Items[0].ID, q.Items[1].ID, q.Items[2].ID)
	}
}

func TestSkippedGoesToEnd(t *testing.T) {
	t1 := time.Now()
	t2 := t1.Add(time.Second)
	skipTime := t1.Add(5 * time.Second)
	sessions := []*session.Session{
		{ID: "a", State: session.NeedsInput, EnteredQueue: t1, SkippedAt: skipTime},
		{ID: "b", State: session.NeedsInput, EnteredQueue: t2},
	}
	q := &Queue{}
	q.Rebuild(sessions)
	// "a" was skipped at t+5s, "b" entered at t+1s, so b should come first
	if q.Items[0].ID != "b" {
		t.Errorf("first item should be 'b' (not skipped), got %q", q.Items[0].ID)
	}
	if q.Items[1].ID != "a" {
		t.Errorf("second item should be 'a' (skipped), got %q", q.Items[1].ID)
	}
}

func TestIndexOf(t *testing.T) {
	sessions := []*session.Session{
		{ID: "a", State: session.NeedsInput, EnteredQueue: time.Now()},
		{ID: "b", State: session.NeedsInput, EnteredQueue: time.Now().Add(time.Second)},
	}
	q := &Queue{}
	q.Rebuild(sessions)
	if idx := q.IndexOf("b"); idx != 1 {
		t.Errorf("IndexOf('b') = %d, want 1", idx)
	}
	if idx := q.IndexOf("missing"); idx != -1 {
		t.Errorf("IndexOf('missing') = %d, want -1", idx)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/queue/ -v`
Expected: FAIL.

**Step 3: Implement queue.go**

```go
package queue

import (
	"slices"
	"time"

	"github.com/BlinkingLine/cloudterminal/internal/session"
)

type Queue struct {
	Items []*session.Session
}

func (q *Queue) Rebuild(sessions []*session.Session) {
	q.Items = q.Items[:0]
	for _, s := range sessions {
		if s.State == session.NeedsInput {
			q.Items = append(q.Items, s)
		}
	}
	slices.SortStableFunc(q.Items, func(a, b *session.Session) int {
		ta := sortKey(a)
		tb := sortKey(b)
		return ta.Compare(tb)
	})
}

func sortKey(s *session.Session) time.Time {
	if !s.SkippedAt.IsZero() {
		return s.SkippedAt
	}
	return s.EnteredQueue
}

func (q *Queue) Len() int {
	return len(q.Items)
}

func (q *Queue) IndexOf(id string) int {
	for i, s := range q.Items {
		if s.ID == id {
			return i
		}
	}
	return -1
}

// At returns the session at the given queue index, or nil if out of bounds.
func (q *Queue) At(index int) *session.Session {
	if index < 0 || index >= len(q.Items) {
		return nil
	}
	return q.Items[index]
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/queue/ -v`
Expected: All PASS.

**Step 5: Commit**

```bash
git add internal/queue/
git commit -m "feat: add FIFO queue with skip support"
```

---

### Task 6: Platform-Specific Process Management

**Files:**
- Create: `internal/session/proc_unix.go`
- Create: `internal/session/proc_windows.go`

**Step 1: Create proc_unix.go**

This file is taken almost directly from Spec Section 8. No unit test — this is syscall-level code tested via integration.

```go
//go:build !windows

package session

import (
	"os/exec"
	"syscall"
)

// ProcState holds platform-specific process management state.
type ProcState struct{}

func configureProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func configureProcCancel(cmd *exec.Cmd, ps *ProcState) {
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
	}
}

func afterStart(cmd *exec.Cmd, ps *ProcState) error {
	return nil
}

func cleanupProc(ps *ProcState) {}
```

**Step 2: Create proc_windows.go**

Full implementation from Spec Section 8 (the Windows Job Object code). This file will only compile on Windows.

```go
//go:build windows

package session

import (
	"os/exec"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

type ProcState struct {
	jobHandle windows.Handle
}

func configureProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.CREATE_SUSPENDED,
	}
}

func configureProcCancel(cmd *exec.Cmd, ps *ProcState) {
	cmd.Cancel = func() error {
		if ps.jobHandle != 0 {
			err := windows.TerminateJobObject(ps.jobHandle, 1)
			windows.CloseHandle(ps.jobHandle)
			ps.jobHandle = 0
			return err
		}
		return cmd.Process.Kill()
	}
}

func afterStart(cmd *exec.Cmd, ps *ProcState) error {
	job, err := assignJobAndResume(cmd)
	ps.jobHandle = job
	return err
}

func cleanupProc(ps *ProcState) {
	if ps.jobHandle != 0 {
		windows.CloseHandle(ps.jobHandle)
		ps.jobHandle = 0
	}
}

func assignJobAndResume(cmd *exec.Cmd) (windows.Handle, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		resumeProcess(cmd)
		return 0, err
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	_, err = windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	)
	if err != nil {
		windows.CloseHandle(job)
		resumeProcess(cmd)
		return 0, err
	}
	handle, err := windows.OpenProcess(windows.PROCESS_ALL_ACCESS, false, uint32(cmd.Process.Pid))
	if err != nil {
		windows.CloseHandle(job)
		resumeProcess(cmd)
		return 0, err
	}
	err = windows.AssignProcessToJobObject(job, handle)
	windows.CloseHandle(handle)
	if err != nil {
		windows.CloseHandle(job)
		resumeProcess(cmd)
		return 0, err
	}
	resumeProcess(cmd)
	return job, nil
}

func resumeProcess(cmd *exec.Cmd) {
	pid := uint32(cmd.Process.Pid)
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return
	}
	defer windows.CloseHandle(snapshot)
	var entry windows.ThreadEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	err = windows.Thread32First(snapshot, &entry)
	for err == nil {
		if entry.OwnerProcessID == pid {
			threadHandle, terr := windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, entry.ThreadID)
			if terr == nil {
				windows.ResumeThread(threadHandle)
				windows.CloseHandle(threadHandle)
			}
			return
		}
		err = windows.Thread32Next(snapshot, &entry)
	}
}
```

**Step 3: Add golang.org/x/sys dependency (needed for Windows build tag)**

```bash
go get golang.org/x/sys@latest
```

**Step 4: Verify build compiles on current platform**

Run: `go build ./internal/session/`
Expected: Success (proc_unix.go compiles on macOS/Linux).

**Step 5: Commit**

```bash
git add internal/session/proc_unix.go internal/session/proc_windows.go go.mod go.sum
git commit -m "feat: add platform-specific process tree management"
```

---

### Task 7: Session.Send() and Mock Mode

**Files:**
- Create: `internal/session/send.go`
- Create: `internal/session/stream.go`
- Create: `internal/session/mock.go`
- Create: `internal/session/mock_test.go`

**Step 1: Implement stream.go (stream-json parser)**

```go
package session

import "encoding/json"

// StreamEvent represents a parsed line from claude --output-format stream-json.
type StreamEvent struct {
	Type string `json:"type"`
	// For assistant text events
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content,omitempty"`
	// For tool_use events
	Tool struct {
		Name string `json:"name"`
	} `json:"tool,omitempty"`
	// Top-level text for result events
	Result struct {
		Text string `json:"text"`
	} `json:"result,omitempty"`
}

// ParseStreamLine parses a single line of stream-json output.
// Returns the event type, any assistant text extracted, any tool hint, and whether parsing succeeded.
func ParseStreamLine(line string) (assistantText string, toolHint string, ok bool) {
	var evt StreamEvent
	if err := json.Unmarshal([]byte(line), &evt); err != nil {
		return "", "", false
	}

	switch evt.Type {
	case "assistant":
		// Accumulate text from content blocks
		for _, c := range evt.Content {
			if c.Type == "text" {
				assistantText += c.Text
			}
		}
		return assistantText, "", true
	case "content_block_delta":
		// Streaming text delta
		var delta struct {
			Delta struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"delta"`
		}
		json.Unmarshal([]byte(line), &delta)
		if delta.Delta.Type == "text_delta" {
			return delta.Delta.Text, "", true
		}
		return "", "", true
	case "tool_use":
		return "", evt.Tool.Name, true
	case "result":
		return evt.Result.Text, "", true
	default:
		return "", "", true // known but unhandled event type
	}
}
```

**Step 2: Implement send.go**

```go
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

// Bubbletea message types — defined here so ui package can reference them.
type SessionDoneMsg struct{ Result SendResult }
type SlotAcquiredMsg struct{ SessionID string }
type StatusHintMsg struct{ SessionID string; Hint string }

const maxOutputBytes = 1_000_000 // 1MB cap on accumulated response text

// Send executes a claude CLI subprocess for this session.
// It blocks until the subprocess completes. All communication with the UI
// is via Bubbletea messages sent through p.Send(). Does NOT mutate session state.
func (s *Session) Send(ctx context.Context, prompt string, sem chan struct{}, p *tea.Program, allowedTools []string) SendResult {
	// Acquire semaphore slot
	select {
	case sem <- struct{}{}:
		defer func() { <-sem }()
		p.Send(SlotAcquiredMsg{SessionID: s.ID})
	case <-ctx.Done():
		return SendResult{SessionID: s.ID, Err: fmt.Errorf("timed out waiting for subprocess slot"), CompletedAt: time.Now()}
	}

	// Build args
	args := []string{"--print", "--output-format", "stream-json", "--session-id", s.ID}
	if s.CompletedOnce {
		args = append(args, "--resume")
	}
	if len(allowedTools) > 0 {
		args = append(args, "--allowedTools", strings.Join(allowedTools, ","))
	}
	args = append(args, prompt)

	cmd := exec.CommandContext(ctx, "claude", args...)
	var ps ProcState
	configureProcAttr(cmd)

	// Capture stderr
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	// Set up stdout pipe for stream-json parsing
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return SendResult{SessionID: s.ID, Err: fmt.Errorf("stdout pipe: %w", err), CompletedAt: time.Now()}
	}

	if err := cmd.Start(); err != nil {
		return SendResult{SessionID: s.ID, Err: fmt.Errorf("start: %w", err), CompletedAt: time.Now()}
	}

	configureProcCancel(cmd, &ps)
	if err := afterStart(cmd, &ps); err != nil {
		// Non-fatal: process runs but children may orphan on cancel
	}
	defer cleanupProc(&ps)

	cmd.WaitDelay = 2 * time.Second

	// Read stream-json line by line
	scanner := bufio.NewScanner(stdout)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 4*1024*1024)

	var output strings.Builder
	for scanner.Scan() {
		text, hint, _ := ParseStreamLine(scanner.Text())
		if text != "" && output.Len() < maxOutputBytes {
			output.WriteString(text)
		}
		if output.Len() >= maxOutputBytes {
			// Will stop accumulating, add notice after loop
		}
		if hint != "" {
			p.Send(StatusHintMsg{SessionID: s.ID, Hint: hint})
		}
	}

	if scanErr := scanner.Err(); scanErr != nil {
		if output.Len() > 0 {
			output.WriteString("\n\n[output may be incomplete — stream read error]")
		}
	}

	if output.Len() >= maxOutputBytes {
		output.WriteString("\n\n[output truncated at 1MB]")
	}

	waitErr := cmd.Wait()

	result := SendResult{
		SessionID:   s.ID,
		CompletedAt: time.Now(),
	}

	if waitErr != nil {
		stderrStr := strings.TrimSpace(stderr.String())
		if ctx.Err() != nil {
			result.Err = fmt.Errorf("timed out")
			result.Output = fmt.Sprintf("Timed out. Send another message to retry.")
		} else {
			errMsg := fmt.Sprintf("Process exited with error: %v", waitErr)
			if stderrStr != "" {
				result.Output = fmt.Sprintf("[stderr] %s\n\n%s", stderrStr, errMsg)
			} else {
				result.Output = errMsg
			}
			result.Err = waitErr
		}
	} else {
		result.Output = output.String()
	}

	return result
}
```

**Step 3: Implement mock.go**

```go
package session

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

var mockResponses = []string{
	"I've completed the refactoring. The main changes were:\n\n1. Extracted the authentication logic into a separate middleware\n2. Added proper error handling for token validation\n3. Updated the tests to cover edge cases\n\nShould I also update the API documentation?",
	"I found 3 issues in the code:\n\n- Missing null check on line 42 of handler.go\n- SQL injection vulnerability in the search query\n- Race condition in the cache invalidation\n\nI've fixed all three. Want me to run the tests?",
	"Done! The new endpoint is working. Here's what I added:\n\n- GET /api/v2/users with pagination support\n- Query params: page, limit, sort, filter\n- Response includes total count and next/prev links\n\nThe tests pass. Anything else?",
	"I've analyzed the database schema and here are my recommendations:\n\n1. Add an index on users.email (used in every login query)\n2. Normalize the address table (currently denormalized)\n3. Add a composite index on orders(user_id, created_at)\n\nShould I proceed with these changes?",
	"The integration tests are all passing now. I had to fix:\n\n- Mock server wasn't returning correct headers\n- Timeout was too short for the CI environment\n- One test was relying on insertion order\n\nAll 47 tests pass. Ready to merge?",
	"I've set up the new logging system:\n\n- Structured JSON logs via zerolog\n- Request ID propagation through context\n- Log levels configurable via environment variable\n- Automatic PII redaction for email and phone fields\n\nShould I add log rotation configuration too?",
}

var mockHints = []string{
	"reading main.go",
	"editing handler.go",
	"running tests",
	"searching codebase",
	"analyzing schema",
}

// MockSend simulates a claude subprocess for UI testing.
func (s *Session) MockSend(ctx context.Context, prompt string, sem chan struct{}, p *tea.Program) SendResult {
	// Acquire semaphore
	select {
	case sem <- struct{}{}:
		defer func() { <-sem }()
		p.Send(SlotAcquiredMsg{SessionID: s.ID})
	case <-ctx.Done():
		return SendResult{SessionID: s.ID, Err: fmt.Errorf("timed out waiting for slot"), CompletedAt: time.Now()}
	}

	// Simulate work with status hints
	duration := time.Duration(2+rand.Intn(6)) * time.Second
	hintInterval := duration / 3

	for i := 0; i < 2; i++ {
		select {
		case <-time.After(hintInterval):
			hint := mockHints[rand.Intn(len(mockHints))]
			p.Send(StatusHintMsg{SessionID: s.ID, Hint: hint})
		case <-ctx.Done():
			return SendResult{SessionID: s.ID, Err: ctx.Err(), CompletedAt: time.Now()}
		}
	}

	// Wait remaining time
	select {
	case <-time.After(hintInterval):
	case <-ctx.Done():
		return SendResult{SessionID: s.ID, Err: ctx.Err(), CompletedAt: time.Now()}
	}

	response := mockResponses[rand.Intn(len(mockResponses))]
	return SendResult{
		SessionID:   s.ID,
		Output:      response,
		CompletedAt: time.Now(),
	}
}
```

**Step 4: Write mock test**

```go
package session

import (
	"context"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestMockSendReturnsResponse(t *testing.T) {
	s := New("test", "hello", "abc12345")
	sem := make(chan struct{}, 5)
	p := tea.NewProgram(nil, tea.WithoutRenderer())

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result := s.MockSend(ctx, "hello", sem, p)
	if result.Output == "" {
		t.Error("MockSend should return a non-empty response")
	}
	if result.Err != nil {
		t.Errorf("MockSend should not error: %v", result.Err)
	}
	if result.SessionID != s.ID {
		t.Error("SessionID mismatch")
	}
}

func TestMockSendRespectsContext(t *testing.T) {
	s := New("test", "hello", "abc12345")
	sem := make(chan struct{}, 5)
	p := tea.NewProgram(nil, tea.WithoutRenderer())

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	result := s.MockSend(ctx, "hello", sem, p)
	if result.Err == nil {
		t.Error("MockSend should error when context is cancelled")
	}
}
```

**Step 5: Run tests**

Run: `go test ./internal/session/ -v`
Expected: All PASS.

**Step 6: Commit**

```bash
git add internal/session/send.go internal/session/stream.go internal/session/mock.go internal/session/mock_test.go
git commit -m "feat: add session Send, stream-json parser, and mock mode"
```

---

### Task 8: Lip Gloss Styles

**Files:**
- Create: `internal/ui/styles.go`

**Step 1: Implement styles.go**

All color constants and reusable styles from Spec Section 10.

```go
package ui

import "github.com/charmbracelet/lipgloss"

var (
	// Colors (Spec Section 10)
	Amber   = lipgloss.Color("#f59e0b")
	Blue    = lipgloss.Color("#3b82f6")
	Brand   = lipgloss.Color("#d4a574")
	Fg      = lipgloss.Color("#e5e5e5")
	Muted   = lipgloss.Color("#737373")
	Dim     = lipgloss.Color("#404040")
	Bg      = lipgloss.Color("#0a0a0a")
	Surface = lipgloss.Color("#151515")
	Red     = lipgloss.Color("#ef4444")
	Green   = lipgloss.Color("#22c55e")
	UserBlue = lipgloss.Color("#60a5fa")
	UserBg   = lipgloss.Color("#1e3a5f")
	ClaudeBg = lipgloss.Color("#2d1f0e")

	// Top bar
	BrandStyle = lipgloss.NewStyle().Foreground(Brand).Bold(true)
	CountStyle = lipgloss.NewStyle().Foreground(Muted)
	BadgeStyle = lipgloss.NewStyle().Foreground(Amber).Bold(true)

	// Session header
	SessionNameStyle = lipgloss.NewStyle().Foreground(Fg).Bold(true)
	WaitTimeStyle    = lipgloss.NewStyle().Foreground(Muted)

	// Chat
	UserRoleStyle   = lipgloss.NewStyle().Foreground(UserBlue).Bold(true)
	ClaudeRoleStyle = lipgloss.NewStyle().Foreground(Brand).Bold(true).Background(ClaudeBg)
	UserMsgStyle    = lipgloss.NewStyle().Background(UserBg)
	MutedStyle      = lipgloss.NewStyle().Foreground(Muted)

	// Input
	InputBorderStyle = lipgloss.NewStyle().BorderForeground(Brand)
	PlaceholderStyle = lipgloss.NewStyle().Foreground(Muted).Italic(true)

	// Notifications
	NotifStyle      = lipgloss.NewStyle().Foreground(Amber)
	ErrorNotifStyle = lipgloss.NewStyle().Foreground(Red)

	// Cards
	CardStyle        = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(Dim).Width(24).Padding(0, 1).Background(Surface)
	ActiveCardStyle  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(Amber).Width(24).Padding(0, 1).Background(Surface)
	WorkingCardStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(Blue).Width(24).Padding(0, 1).Background(Surface)

	// Queue nav
	ActiveDot  = lipgloss.NewStyle().Foreground(Amber).Render("━━")
	InactiveDot = lipgloss.NewStyle().Foreground(Dim).Render("──")

	// Focus mode
	FocusSeparator = lipgloss.NewStyle().Foreground(Amber)
	ProgressFilled = lipgloss.NewStyle().Foreground(Amber).Render("━")
	ProgressEmpty  = lipgloss.NewStyle().Foreground(Dim).Render("░")

	// Help bar
	HelpStyle = lipgloss.NewStyle().Foreground(Muted)

	// Separator
	SepStyle = lipgloss.NewStyle().Foreground(Dim)

	// Spinner badges
	WorkingBadge   = lipgloss.NewStyle().Foreground(Blue).Render("⟳")
	QueuedBadge    = lipgloss.NewStyle().Foreground(Muted).Render("⟳")
	NeedsYouBadge  = lipgloss.NewStyle().Foreground(Amber).Render("●")
)
```

**Step 2: Verify build**

Run: `go build ./internal/ui/`
Expected: Success (may need a placeholder model.go with `package ui` — create one if needed).

**Step 3: Commit**

```bash
git add internal/ui/styles.go
git commit -m "feat: add Lip Gloss style definitions"
```

---

### Task 9: Bubbletea Model Core (Init, Update, Messages)

**Files:**
- Create: `internal/ui/model.go`
- Create: `internal/ui/messages.go`

This is the largest task. It implements the core state machine from Spec Section 8.

**Step 1: Create messages.go**

```go
package ui

import "time"

type Mode int

const (
	NormalMode Mode = iota
	FocusMode
)

type TickMsg time.Time

type Notification struct {
	Text      string
	CreatedAt time.Time
	IsError   bool
}
```

**Step 2: Implement model.go**

This is the heart of the app. It implements `tea.Model` with Init, Update, View. The Update method is the state machine from the spec. The View method dispatches to view_normal.go or view_focus.go (created in subsequent tasks).

The model.go file will be large (~400-500 lines). Key responsibilities:
- `NewModel()` constructor
- `Init()` returns tick command
- `Update()` handles all messages: key presses, SessionDoneMsg, SlotAcquiredMsg, StatusHintMsg, TickMsg, WindowSizeMsg
- `handleSend()` — the prompt-advance loop
- `handleDismiss()` — Ctrl+W logic with Working confirmation
- `advanceQueue()` — move to next queue item
- `findSession()` — lookup by ID
- `View()` dispatches to view_normal or view_focus

Implementation is too large to include inline in the plan. The implementor should follow Spec Section 8 "Update Flow" exactly, translating the pseudocode into Go. Key patterns:

- Every key handler checks `m.Overlay != nil` first (keyboard isolation, Spec Section 6).
- Single-char keys (`F`, `V`, `S`) check `m.Input.Value() == ""` before dispatching.
- `handleSend()` creates `context.WithTimeout(m.RootCtx, timeout)`, stores cancel on session, launches goroutine.
- All `SessionDoneMsg`/`SlotAcquiredMsg`/`StatusHintMsg` handlers nil-check session lookup (dismissed sessions).
- TickMsg fires every 1 second via `tea.Tick`.

**Step 3: Verify build compiles**

Run: `go build ./internal/ui/`
Expected: Success.

**Step 4: Commit**

```bash
git add internal/ui/model.go internal/ui/messages.go
git commit -m "feat: add Bubbletea model with core state machine"
```

---

### Task 10: Text Input Component

**Files:**
- Create: `internal/ui/input.go`

**Step 1: Implement input.go**

Wraps Bubbletea textarea with 5-line max height, `> ` prompt, placeholder logic.

```go
package ui

import (
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/lipgloss"
)

func NewInput(width int) textarea.Model {
	ta := textarea.New()
	ta.Placeholder = "Type a message..."
	ta.Prompt = "> "
	ta.CharLimit = 0 // unlimited
	ta.MaxHeight = 5
	ta.ShowLineNumbers = false
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.FocusedStyle.Base = lipgloss.NewStyle()
	ta.FocusedStyle.Prompt = lipgloss.NewStyle().Foreground(Brand)
	ta.FocusedStyle.Placeholder = lipgloss.NewStyle().Foreground(Muted)
	ta.BlurredStyle = ta.FocusedStyle
	ta.SetWidth(width - 4) // account for padding
	ta.Focus()
	return ta
}

// SetInputPlaceholder updates the textarea placeholder based on session state.
func SetInputPlaceholder(ta *textarea.Model, sessionName string, isWorking bool) {
	if isWorking {
		ta.Placeholder = "Claude is working..."
	} else if sessionName != "" {
		ta.Placeholder = "Reply to " + sessionName + "..."
	} else {
		ta.Placeholder = "Type a message..."
	}
}
```

**Step 2: Verify build**

Run: `go build ./internal/ui/`
Expected: Success.

**Step 3: Commit**

```bash
git add internal/ui/input.go
git commit -m "feat: add text input component"
```

---

### Task 11: Normal Mode View

**Files:**
- Create: `internal/ui/view_normal.go`

**Step 1: Implement view_normal.go**

Renders all Normal Mode UI regions from Spec Section 3.1: top bar, card strip (if visible), queue nav, session header, chat view, input bar, hint, help bar.

Key rendering functions:
- `renderTopBar(m Model) string` — brand, session count, waiting badge
- `renderCardStrip(m Model) string` — horizontal cards with overflow
- `renderQueueNav(m Model) string` — dot indicators, position counter
- `renderSessionHeader(m Model, s *session.Session) string` — name, badge, wait time
- `renderChatView(m Model, s *session.Session, height int) string` — scrollable history
- `renderInputBar(m Model) string` — textarea + hint text
- `renderHelpBar(m Model) string` — keybinding hints
- `ViewNormal(m Model) string` — composes all regions

Chat view rendering:
- Each message: `you ›` or `claude ›` prefix + word-wrapped text
- Working indicator at bottom: `claude › thinking... (12s)` or `claude › {statusHint}... (12s)`
- Uses Bubbletea viewport for scrolling

The implementor should use `lipgloss.JoinVertical` to stack regions and `lipgloss.Place` for centering where needed. Word wrapping to `width - 6` (Spec Section 10).

**Step 2: Verify build**

Run: `go build ./internal/ui/`
Expected: Success.

**Step 3: Commit**

```bash
git add internal/ui/view_normal.go
git commit -m "feat: add Normal Mode rendering"
```

---

### Task 12: Focus Mode View

**Files:**
- Create: `internal/ui/view_focus.go`

**Step 1: Implement view_focus.go**

Renders Focus Mode UI from Spec Section 3.2: focus header, progress bar, compressed card view, focus input bar, focus help bar.

Key rendering functions:
- `renderFocusHeader(m Model) string` — `◆ focus` + working count
- `renderProgressBar(m Model, width int) string` — filled/empty segments + counter + incoming label
- `renderFocusCard(m Model, s *session.Session, height int) string` — name (muted) + last claude message only
- `renderFocusHelp() string` — focus-specific keybindings
- `renderFocusWaiting(m Model) string` — centered waiting screen
- `ViewFocus(m Model) string` — composes all focus regions

Progress bar: `━━━━━━━━━░░░░ 5/8  +2 incoming`
- Bar width = available width minus counter/label text
- Filled count = `m.FocusCleared`, total = `m.FocusTotal`

**Step 2: Verify build**

Run: `go build ./internal/ui/`
Expected: Success.

**Step 3: Commit**

```bash
git add internal/ui/view_focus.go
git commit -m "feat: add Focus Mode rendering"
```

---

### Task 13: New Session Overlay

**Files:**
- Create: `internal/ui/overlay.go`

**Step 1: Implement overlay.go**

Two-step overlay (Spec Section 6): name input → prompt input. Esc cancels. Backspace on empty prompt returns to name step. All other keybindings suppressed.

```go
package ui

type OverlayStep int

const (
	OverlayStepName   OverlayStep = iota
	OverlayStepPrompt
)

type OverlayState struct {
	Step       OverlayStep
	NameInput  string
	PromptInput string
	Error      string // validation error for name
}
```

The overlay rendering draws a centered box over the current view with the appropriate input field. The Update handler in model.go routes key events to the overlay when `m.Overlay != nil`.

**Step 2: Verify build**

Run: `go build ./internal/ui/`
Expected: Success.

**Step 3: Commit**

```bash
git add internal/ui/overlay.go
git commit -m "feat: add new session overlay"
```

---

### Task 14: main.go — CLI Entry Point

**Files:**
- Modify: `main.go`

**Step 1: Implement main.go**

Full implementation following Spec Sections 6, 8, 13:

```go
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/BlinkingLine/cloudterminal/internal/config"
	"github.com/BlinkingLine/cloudterminal/internal/session"
	"github.com/BlinkingLine/cloudterminal/internal/ui"
	"github.com/google/uuid"
)

func main() {
	// Parse flags
	args := os.Args[1:]
	mockMode := false
	verbose := false
	var sessionArgs []string

	for _, arg := range args {
		switch arg {
		case "--mock":
			mockMode = true
		case "--verbose":
			verbose = true
		default:
			sessionArgs = append(sessionArgs, arg)
		}
	}

	// Check claude CLI (unless mock mode)
	if !mockMode {
		if _, err := exec.LookPath("claude"); err != nil {
			fmt.Fprintln(os.Stderr, "Warning: claude CLI not found in PATH, running in mock mode")
			mockMode = true
		} else {
			checkClaudeVersion()
		}
	}

	// Load config
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: config load: %v\n", err)
	}

	// Tool safety warning
	if !mockMode && len(cfg.AllowedTools) == 0 {
		fmt.Fprintln(os.Stderr, "Warning: Sessions run with full tool access (--print mode auto-accepts all tool calls).")
		fmt.Fprintln(os.Stderr, "Configure \"allowed_tools\" in config to restrict.")
	}

	// Generate run ID
	runID := uuid.New().String()[:8]

	// Parse session arguments
	sessions := parseSessionArgs(sessionArgs, runID)

	// Create model
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	model := ui.NewModel(cfg, sessions, runID, mockMode, verbose, ctx, cancel)

	// Run Bubbletea
	p := tea.NewProgram(model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
		tea.WithContext(ctx),
	)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	}
	cancel()

	// Wait for goroutines
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

var semverRegex = regexp.MustCompile(`(\d+\.\d+\.\d+)`)

func checkClaudeVersion() {
	out, err := exec.Command("claude", "--version").Output()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Warning: could not check claude CLI version")
		return
	}
	matches := semverRegex.FindStringSubmatch(string(out))
	if len(matches) < 2 {
		fmt.Fprintf(os.Stderr, "Warning: could not parse claude version from: %s\n", strings.TrimSpace(string(out)))
		return
	}
	version := matches[1]
	parts := strings.Split(version, ".")
	if parts[0] == "0" {
		fmt.Fprintf(os.Stderr, "Warning: claude CLI version 1.0.0+ required (found: %s)\n", version)
	}
}

func parseSessionArgs(args []string, runID string) []*session.Session {
	var sessions []*session.Session
	autoNum := 1

	for i, arg := range args {
		if i >= 20 {
			fmt.Fprintf(os.Stderr, "Warning: session limit is 20, skipping %d extra arguments\n", len(args)-20)
			break
		}

		idx := strings.Index(arg, ":")
		var name, prompt string

		if idx == -1 || idx == 0 {
			// No colon or empty name — auto-generate name
			prompt = arg
			if idx == 0 {
				prompt = arg[1:] // skip leading colon
			}
			name = fmt.Sprintf("s%d", autoNum)
			autoNum++
		} else {
			name = arg[:idx]
			prompt = arg[idx+1:]
		}

		prompt = strings.TrimSpace(prompt)
		if prompt == "" {
			fmt.Fprintf(os.Stderr, "Warning: empty prompt for %q, skipping\n", name)
			continue
		}

		validName, err := session.ValidateName(name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: invalid session name %q, using s%d\n", name, autoNum)
			validName = fmt.Sprintf("s%d", autoNum)
			autoNum++
		}

		sessions = append(sessions, session.New(validName, prompt, runID))
	}

	return sessions
}
```

**Step 2: Verify build**

Run: `go build -o cloudterminal .`
Expected: Success.

**Step 3: Test mock mode**

Run: `./cloudterminal --mock "demo:hello world" "test:write tests"`
Expected: App launches in alt screen with 2 sessions.

**Step 4: Commit**

```bash
git add main.go
git commit -m "feat: add CLI entry point with arg parsing and mock detection"
```

---

### Task 15: Integration Testing — Mock Mode Smoke Test

**Files:**
- Create: `main_test.go`

**Step 1: Write a basic smoke test**

```go
package main

import (
	"testing"

	"github.com/BlinkingLine/cloudterminal/internal/session"
)

func TestParseSessionArgs(t *testing.T) {
	args := []string{
		"auth:refactor the auth",
		"db:normalize: the table",
		":do something",
		"empty:",
		"no-colon prompt",
	}
	sessions := parseSessionArgs(args, "test1234")

	// "empty:" should be skipped (empty prompt)
	// "no-colon prompt" gets auto name
	// ":do something" gets auto name
	if len(sessions) != 4 {
		t.Fatalf("got %d sessions, want 4", len(sessions))
	}
	if sessions[0].Name != "auth" {
		t.Errorf("session 0 name = %q, want 'auth'", sessions[0].Name)
	}
	if sessions[1].Name != "db" {
		t.Errorf("session 1 name = %q, want 'db'", sessions[1].Name)
	}
}

func TestParseSessionArgsLimit(t *testing.T) {
	var args []string
	for i := 0; i < 25; i++ {
		args = append(args, "s:prompt")
	}
	sessions := parseSessionArgs(args, "test1234")
	if len(sessions) > 20 {
		t.Errorf("should cap at 20, got %d", len(sessions))
	}
}

func TestSessionNameValidation(t *testing.T) {
	name, err := session.ValidateName("My-Task_123")
	if err != nil {
		t.Fatal(err)
	}
	if name != "my-task_123" {
		t.Errorf("got %q, want 'my-task_123'", name)
	}
}
```

**Step 2: Run tests**

Run: `go test ./... -v`
Expected: All PASS.

**Step 3: Commit**

```bash
git add main_test.go
git commit -m "test: add CLI argument parsing tests"
```

---

### Task 16: GoReleaser + GitHub Actions

**Files:**
- Create: `.goreleaser.yml`
- Create: `.github/workflows/release.yml`

**Step 1: Create .goreleaser.yml**

```yaml
version: 2

builds:
  - main: .
    binary: cloudterminal
    env:
      - CGO_ENABLED=0
    goos:
      - darwin
      - linux
      - windows
    goarch:
      - amd64
      - arm64
    ignore:
      - goos: windows
        goarch: arm64

archives:
  - format: tar.gz
    name_template: "cloudterminal_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
    format_overrides:
      - goos: windows
        format: zip

checksum:
  name_template: "checksums.txt"

changelog:
  sort: asc
  filters:
    exclude:
      - "^docs:"
      - "^test:"
```

**Step 2: Create .github/workflows/release.yml**

```yaml
name: Release

on:
  push:
    tags:
      - "v*"

permissions:
  contents: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - uses: actions/setup-go@v5
        with:
          go-version: "1.22"

      - uses: goreleaser/goreleaser-action@v6
        with:
          version: "~> v2"
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

**Step 3: Verify goreleaser config locally (optional)**

```bash
# If goreleaser is installed:
goreleaser check
```

**Step 4: Commit**

```bash
git add .goreleaser.yml .github/workflows/release.yml
git commit -m "ci: add GoReleaser config and GitHub Actions release workflow"
```

---

### Task 17: Final Verification

**Step 1: Run all tests**

```bash
go test ./... -v
```
Expected: All PASS.

**Step 2: Build binary**

```bash
go build -o cloudterminal .
```
Expected: Binary produced.

**Step 3: Test mock mode end-to-end**

```bash
./cloudterminal --mock "demo:show me something cool" "test:write hello world"
```
Expected: App launches, 2 sessions visible, mock responses arrive after 2-8 seconds, can navigate with arrow keys, enter Focus Mode with F, reply with Enter, dismiss with Ctrl+W.

**Step 4: Test empty state**

```bash
./cloudterminal --mock
```
Expected: Shows "Press Ctrl+N to create a session" centered. Ctrl+N opens overlay.

**Step 5: Commit any fixes from testing**

```bash
git add -A
git commit -m "fix: address issues found during end-to-end testing"
```

---

## Implementation Order Summary

| Task | Component | Depends On |
|------|-----------|------------|
| 1 | Project scaffolding | — |
| 2 | Config package | 1 |
| 3 | Session types | 1 |
| 4 | ANSI sanitization | 3 |
| 5 | Queue package | 3 |
| 6 | Platform proc management | 3 |
| 7 | Session.Send() + mock | 3, 4, 6 |
| 8 | Lip Gloss styles | 1 |
| 9 | Bubbletea model core | 2, 3, 5, 7, 8 |
| 10 | Text input component | 8 |
| 11 | Normal Mode view | 8, 9, 10 |
| 12 | Focus Mode view | 8, 9, 10 |
| 13 | New session overlay | 9 |
| 14 | main.go CLI entry | 2, 7, 9 |
| 15 | Integration tests | 14 |
| 16 | GoReleaser + CI | 14 |
| 17 | Final verification | All |

Tasks 2-8 can be parallelized across agents (they're independent). Tasks 9-13 are the UI core and must be sequential. Task 14 wires everything together.
