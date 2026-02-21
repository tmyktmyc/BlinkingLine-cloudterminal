package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultReturnsCorrectValues(t *testing.T) {
	cfg := Default()

	if cfg.DefaultReply != "lgtm, continue" {
		t.Errorf("DefaultReply = %q, want %q", cfg.DefaultReply, "lgtm, continue")
	}
	if cfg.FocusThreshold != 3 {
		t.Errorf("FocusThreshold = %d, want %d", cfg.FocusThreshold, 3)
	}
	if cfg.BellOnQueue != true {
		t.Errorf("BellOnQueue = %v, want %v", cfg.BellOnQueue, true)
	}
	if cfg.MaxConcurrent != 5 {
		t.Errorf("MaxConcurrent = %d, want %d", cfg.MaxConcurrent, 5)
	}
	if cfg.SubprocessTimeout != 10*time.Minute {
		t.Errorf("SubprocessTimeout = %v, want %v", cfg.SubprocessTimeout, 10*time.Minute)
	}
	if cfg.TimeoutRaw != "10m" {
		t.Errorf("TimeoutRaw = %q, want %q", cfg.TimeoutRaw, "10m")
	}
	if cfg.AllowedTools == nil {
		t.Error("AllowedTools is nil, want empty slice")
	}
	if len(cfg.AllowedTools) != 0 {
		t.Errorf("AllowedTools length = %d, want 0", len(cfg.AllowedTools))
	}
}

func TestLoadFromCreatesFileWhenMissing(t *testing.T) {
	dir := t.TempDir()

	cfg, err := LoadFrom(dir)
	if err != nil {
		t.Fatalf("LoadFrom() error = %v", err)
	}

	// Should return defaults
	if cfg.DefaultReply != "lgtm, continue" {
		t.Errorf("DefaultReply = %q, want %q", cfg.DefaultReply, "lgtm, continue")
	}

	// File should have been created
	path := filepath.Join(dir, "config.json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("config file not created: %v", err)
	}

	// Check file permissions (0600)
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("file permissions = %o, want %o", perm, 0600)
	}

	// File should contain valid JSON with defaults
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading config file: %v", err)
	}
	var written Config
	if err := json.Unmarshal(data, &written); err != nil {
		t.Fatalf("unmarshalling written config: %v", err)
	}
	if written.DefaultReply != "lgtm, continue" {
		t.Errorf("written DefaultReply = %q, want %q", written.DefaultReply, "lgtm, continue")
	}
}

func TestLoadFromInvalidJSONFallsBackToDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	// Write invalid JSON
	if err := os.WriteFile(path, []byte("{not valid json!!!"), 0600); err != nil {
		t.Fatalf("writing invalid config: %v", err)
	}

	cfg, err := LoadFrom(dir)
	if err != nil {
		t.Fatalf("LoadFrom() error = %v", err)
	}

	// Should fall back to defaults
	if cfg.DefaultReply != "lgtm, continue" {
		t.Errorf("DefaultReply = %q, want %q", cfg.DefaultReply, "lgtm, continue")
	}
	if cfg.MaxConcurrent != 5 {
		t.Errorf("MaxConcurrent = %d, want %d", cfg.MaxConcurrent, 5)
	}
}

func TestLoadFromPartialOverrideKeepsOtherDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	// Write partial config — only override default_reply
	partial := `{"default_reply": "sounds good"}`
	if err := os.WriteFile(path, []byte(partial), 0600); err != nil {
		t.Fatalf("writing partial config: %v", err)
	}

	cfg, err := LoadFrom(dir)
	if err != nil {
		t.Fatalf("LoadFrom() error = %v", err)
	}

	// Overridden field
	if cfg.DefaultReply != "sounds good" {
		t.Errorf("DefaultReply = %q, want %q", cfg.DefaultReply, "sounds good")
	}

	// Other fields should be defaults
	if cfg.FocusThreshold != 3 {
		t.Errorf("FocusThreshold = %d, want %d", cfg.FocusThreshold, 3)
	}
	if cfg.BellOnQueue != true {
		t.Errorf("BellOnQueue = %v, want %v", cfg.BellOnQueue, true)
	}
	if cfg.MaxConcurrent != 5 {
		t.Errorf("MaxConcurrent = %d, want %d", cfg.MaxConcurrent, 5)
	}
	if cfg.SubprocessTimeout != 10*time.Minute {
		t.Errorf("SubprocessTimeout = %v, want %v", cfg.SubprocessTimeout, 10*time.Minute)
	}
}

func TestValidateMaxConcurrentTooLow(t *testing.T) {
	cfg := Default()
	cfg.MaxConcurrent = 0
	result := validate(cfg)
	if result.MaxConcurrent != 5 {
		t.Errorf("MaxConcurrent = %d, want default 5", result.MaxConcurrent)
	}
}

func TestValidateMaxConcurrentTooHigh(t *testing.T) {
	cfg := Default()
	cfg.MaxConcurrent = 100
	result := validate(cfg)
	if result.MaxConcurrent != 5 {
		t.Errorf("MaxConcurrent = %d, want default 5", result.MaxConcurrent)
	}
}

func TestValidateFocusThresholdTooLow(t *testing.T) {
	cfg := Default()
	cfg.FocusThreshold = 0
	result := validate(cfg)
	if result.FocusThreshold != 3 {
		t.Errorf("FocusThreshold = %d, want default 3", result.FocusThreshold)
	}
}

func TestValidateFocusThresholdTooHigh(t *testing.T) {
	cfg := Default()
	cfg.FocusThreshold = 51
	result := validate(cfg)
	if result.FocusThreshold != 3 {
		t.Errorf("FocusThreshold = %d, want default 3", result.FocusThreshold)
	}
}

func TestValidateBadTimeoutString(t *testing.T) {
	cfg := Default()
	cfg.TimeoutRaw = "not-a-duration"
	result := validate(cfg)
	if result.SubprocessTimeout != 10*time.Minute {
		t.Errorf("SubprocessTimeout = %v, want default 10m", result.SubprocessTimeout)
	}
	if result.TimeoutRaw != "10m" {
		t.Errorf("TimeoutRaw = %q, want default %q", result.TimeoutRaw, "10m")
	}
}

func TestValidateTimeoutTooShort(t *testing.T) {
	cfg := Default()
	cfg.TimeoutRaw = "5s"
	result := validate(cfg)
	if result.SubprocessTimeout != 10*time.Minute {
		t.Errorf("SubprocessTimeout = %v, want default 10m", result.SubprocessTimeout)
	}
	if result.TimeoutRaw != "10m" {
		t.Errorf("TimeoutRaw = %q, want default %q", result.TimeoutRaw, "10m")
	}
}

func TestValidateTimeoutTooLong(t *testing.T) {
	cfg := Default()
	cfg.TimeoutRaw = "1h"
	result := validate(cfg)
	if result.SubprocessTimeout != 10*time.Minute {
		t.Errorf("SubprocessTimeout = %v, want default 10m", result.SubprocessTimeout)
	}
	if result.TimeoutRaw != "10m" {
		t.Errorf("TimeoutRaw = %q, want default %q", result.TimeoutRaw, "10m")
	}
}

func TestValidateDefaultReplyWhitespace(t *testing.T) {
	cfg := Default()
	cfg.DefaultReply = "   "
	result := validate(cfg)
	if result.DefaultReply != "lgtm, continue" {
		t.Errorf("DefaultReply = %q, want default %q", result.DefaultReply, "lgtm, continue")
	}
}

func TestValidateDefaultReplyTooLong(t *testing.T) {
	cfg := Default()
	cfg.DefaultReply = string(make([]byte, 501))
	result := validate(cfg)
	if result.DefaultReply != "lgtm, continue" {
		t.Errorf("DefaultReply = %q, want default", result.DefaultReply)
	}
}

func TestValidateAllowedToolsNilBecomesEmptySlice(t *testing.T) {
	cfg := Default()
	cfg.AllowedTools = nil
	result := validate(cfg)
	if result.AllowedTools == nil {
		t.Error("AllowedTools is nil, want empty slice")
	}
	if len(result.AllowedTools) != 0 {
		t.Errorf("AllowedTools length = %d, want 0", len(result.AllowedTools))
	}
}

func TestValidateValidTimeoutInRange(t *testing.T) {
	cfg := Default()
	cfg.TimeoutRaw = "5m"
	result := validate(cfg)
	if result.SubprocessTimeout != 5*time.Minute {
		t.Errorf("SubprocessTimeout = %v, want 5m", result.SubprocessTimeout)
	}
	if result.TimeoutRaw != "5m" {
		t.Errorf("TimeoutRaw = %q, want %q", result.TimeoutRaw, "5m")
	}
}

func TestValidateValidFields(t *testing.T) {
	cfg := Config{
		DefaultReply:    "ok",
		FocusThreshold:  10,
		BellOnQueue:     false,
		MaxConcurrent:   8,
		TimeoutRaw:      "2m",
		AllowedTools:    []string{"bash", "read"},
	}
	result := validate(cfg)
	if result.DefaultReply != "ok" {
		t.Errorf("DefaultReply = %q, want %q", result.DefaultReply, "ok")
	}
	if result.FocusThreshold != 10 {
		t.Errorf("FocusThreshold = %d, want %d", result.FocusThreshold, 10)
	}
	if result.BellOnQueue != false {
		t.Errorf("BellOnQueue = %v, want %v", result.BellOnQueue, false)
	}
	if result.MaxConcurrent != 8 {
		t.Errorf("MaxConcurrent = %d, want %d", result.MaxConcurrent, 8)
	}
	if result.SubprocessTimeout != 2*time.Minute {
		t.Errorf("SubprocessTimeout = %v, want %v", result.SubprocessTimeout, 2*time.Minute)
	}
	if len(result.AllowedTools) != 2 {
		t.Errorf("AllowedTools length = %d, want 2", len(result.AllowedTools))
	}
}

func TestLoadFromPermissionDeniedFallsBackToDefaults(t *testing.T) {
	dir := t.TempDir()

	// Create a config file that's not readable
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{"default_reply":"custom"}`), 0000); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := LoadFrom(dir)
	if err != nil {
		t.Fatalf("LoadFrom() error = %v", err)
	}

	// Should fall back to defaults since file is unreadable
	if cfg.DefaultReply != "lgtm, continue" {
		t.Errorf("DefaultReply = %q, want default %q", cfg.DefaultReply, "lgtm, continue")
	}
}

func TestDirReturnsSuffix(t *testing.T) {
	d, err := Dir()
	if err != nil {
		t.Fatalf("Dir() error = %v", err)
	}
	if filepath.Base(d) != "cloudterminal" {
		t.Errorf("Dir() base = %q, want %q", filepath.Base(d), "cloudterminal")
	}
}
