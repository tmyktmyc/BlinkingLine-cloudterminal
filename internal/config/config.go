package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Config holds all user-configurable settings for CloudTerminal.
type Config struct {
	DefaultReply      string        `json:"default_reply"`
	FocusThreshold    int           `json:"focus_threshold"`
	BellOnQueue       bool          `json:"bell_on_queue"`
	MaxConcurrent     int           `json:"max_concurrent"`
	SubprocessTimeout time.Duration `json:"-"`
	TimeoutRaw        string        `json:"subprocess_timeout"`
	AllowedTools      []string      `json:"allowed_tools"`
}

// Default returns a Config populated with all default values.
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
		return "", fmt.Errorf("config dir: %w", err)
	}
	return filepath.Join(base, "cloudterminal"), nil
}

// Load loads the configuration from the OS-standard config directory.
func Load() (Config, error) {
	dir, err := Dir()
	if err != nil {
		return Default(), err
	}
	return LoadFrom(dir)
}

// LoadFrom loads configuration from the specified directory. If the config
// file does not exist, it creates one with default values. On any error
// (invalid JSON, permission denied, etc.) it falls back to defaults and
// never crashes.
func LoadFrom(dir string) (Config, error) {
	defaults := Default()
	path := filepath.Join(dir, "config.json")

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Create dir and write defaults
			if mkErr := os.MkdirAll(dir, 0700); mkErr != nil {
				fmt.Fprintf(os.Stderr, "cloudterminal: warning: could not create config dir: %v\n", mkErr)
				return defaults, nil
			}
			if wErr := writeConfig(path, defaults); wErr != nil {
				fmt.Fprintf(os.Stderr, "cloudterminal: warning: could not write default config: %v\n", wErr)
			}
			return defaults, nil
		}
		// Permission denied or other read error — fall back to defaults
		fmt.Fprintf(os.Stderr, "cloudterminal: warning: could not read config: %v\n", err)
		return defaults, nil
	}

	// Start from defaults so unset fields keep their default values.
	cfg := defaults
	if err := json.Unmarshal(data, &cfg); err != nil {
		fmt.Fprintf(os.Stderr, "cloudterminal: warning: invalid config JSON: %v\n", err)
		return defaults, nil
	}

	cfg = validate(cfg)
	return cfg, nil
}

// validate checks all fields and replaces invalid values with defaults.
func validate(cfg Config) Config {
	defaults := Default()

	// default_reply: non-whitespace, max 500 chars
	if strings.TrimSpace(cfg.DefaultReply) == "" || len(cfg.DefaultReply) > 500 {
		cfg.DefaultReply = defaults.DefaultReply
	}

	// focus_threshold: 1-50
	if cfg.FocusThreshold < 1 || cfg.FocusThreshold > 50 {
		cfg.FocusThreshold = defaults.FocusThreshold
	}

	// max_concurrent: 1-20
	if cfg.MaxConcurrent < 1 || cfg.MaxConcurrent > 20 {
		cfg.MaxConcurrent = defaults.MaxConcurrent
	}

	// subprocess_timeout: valid Go duration, 30s-30m
	if cfg.TimeoutRaw == "" {
		cfg.TimeoutRaw = defaults.TimeoutRaw
		cfg.SubprocessTimeout = defaults.SubprocessTimeout
	} else {
		d, err := time.ParseDuration(cfg.TimeoutRaw)
		if err != nil || d < 30*time.Second || d > 30*time.Minute {
			cfg.TimeoutRaw = defaults.TimeoutRaw
			cfg.SubprocessTimeout = defaults.SubprocessTimeout
		} else {
			cfg.SubprocessTimeout = d
		}
	}

	// allowed_tools: nil becomes empty slice
	if cfg.AllowedTools == nil {
		cfg.AllowedTools = []string{}
	}

	return cfg
}

// writeConfig serializes the config to JSON and writes it to path with
// mode 0600.
func writeConfig(path string, cfg Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0600)
}
