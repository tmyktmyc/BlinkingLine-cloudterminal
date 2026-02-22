package main

import (
	"os"
	"testing"
)

func TestParseSessionArgs(t *testing.T) {
	// Test basic "name:prompt" parsing
	sessions := parseSessionArgs([]string{
		"auth:refactor the auth",
		"tests:write tests",
	}, "test1234")

	if len(sessions) != 2 {
		t.Fatalf("got %d sessions, want 2", len(sessions))
	}
	if sessions[0].Name != "auth" {
		t.Errorf("session 0 name = %q, want 'auth'", sessions[0].Name)
	}
	if sessions[1].Name != "tests" {
		t.Errorf("session 1 name = %q, want 'tests'", sessions[1].Name)
	}
}

func TestParseSessionArgsAutoName(t *testing.T) {
	// Test no colon → auto name
	sessions := parseSessionArgs([]string{"just a prompt"}, "test1234")
	if len(sessions) != 1 {
		t.Fatalf("got %d sessions, want 1", len(sessions))
	}
	if sessions[0].Name != "s1" {
		t.Errorf("auto name = %q, want 's1'", sessions[0].Name)
	}
}

func TestParseSessionArgsEmptyName(t *testing.T) {
	// Test ":prompt" → auto name (1 colon, empty name)
	sessions := parseSessionArgs([]string{":do something"}, "test1234")
	if len(sessions) != 1 {
		t.Fatalf("got %d sessions, want 1", len(sessions))
	}
	if sessions[0].Name != "s1" {
		t.Errorf("auto name = %q, want 's1'", sessions[0].Name)
	}
}

func TestParseSessionArgsEmptyPromptSkipped(t *testing.T) {
	// Test "name:" → skipped
	sessions := parseSessionArgs([]string{"empty:", "valid:prompt"}, "test1234")
	if len(sessions) != 1 {
		t.Fatalf("got %d sessions, want 1 (empty prompt should be skipped)", len(sessions))
	}
	if sessions[0].Name != "valid" {
		t.Errorf("name = %q, want 'valid'", sessions[0].Name)
	}
}

func TestParseSessionArgsLimit(t *testing.T) {
	// Test 25 args → only first 20 accepted
	var args []string
	for i := 0; i < 25; i++ {
		args = append(args, "s:prompt")
	}
	sessions := parseSessionArgs(args, "test1234")
	if len(sessions) > 20 {
		t.Errorf("should cap at 20, got %d", len(sessions))
	}
}

func TestParseSessionArgsInvalidName(t *testing.T) {
	// Test invalid name → falls back to auto name
	sessions := parseSessionArgs([]string{"bad name!:prompt"}, "test1234")
	if len(sessions) != 1 {
		t.Fatalf("got %d sessions, want 1", len(sessions))
	}
	if sessions[0].Name == "bad name!" {
		t.Error("invalid name should not be used directly")
	}
}

func TestParseSessionArgsNameDirPrompt(t *testing.T) {
	// Test "name:dir:prompt" format with a real directory
	home, _ := os.UserHomeDir()
	sessions := parseSessionArgs([]string{"auth:~/:refactor auth"}, "test1234")
	if len(sessions) != 1 {
		t.Fatalf("got %d sessions, want 1", len(sessions))
	}
	if sessions[0].Name != "auth" {
		t.Errorf("name = %q, want 'auth'", sessions[0].Name)
	}
	if sessions[0].Dir != home {
		t.Errorf("dir = %q, want %q", sessions[0].Dir, home)
	}
	if sessions[0].History[0].Text != "refactor auth" {
		t.Errorf("prompt = %q, want 'refactor auth'", sessions[0].History[0].Text)
	}
}

func TestParseSessionArgsDirDefaultsToCwd(t *testing.T) {
	// Test "name:prompt" → dir defaults to cwd
	cwd, _ := os.Getwd()
	sessions := parseSessionArgs([]string{"auth:refactor auth"}, "test1234")
	if len(sessions) != 1 {
		t.Fatalf("got %d sessions, want 1", len(sessions))
	}
	if sessions[0].Dir != cwd {
		t.Errorf("dir = %q, want cwd %q", sessions[0].Dir, cwd)
	}
}

func TestParseSessionArgsJustPromptDirDefaultsToCwd(t *testing.T) {
	// Test "just a prompt" → dir defaults to cwd
	cwd, _ := os.Getwd()
	sessions := parseSessionArgs([]string{"just a prompt"}, "test1234")
	if len(sessions) != 1 {
		t.Fatalf("got %d sessions, want 1", len(sessions))
	}
	if sessions[0].Name != "s1" {
		t.Errorf("name = %q, want 's1'", sessions[0].Name)
	}
	if sessions[0].Dir != cwd {
		t.Errorf("dir = %q, want cwd %q", sessions[0].Dir, cwd)
	}
}
