package main

import (
	"testing"
)

func TestParseSessionArgs(t *testing.T) {
	// Test basic "name:prompt" parsing
	sessions := parseSessionArgs([]string{
		"auth:refactor the auth",
		"db:normalize: the table",
	}, "test1234")

	if len(sessions) != 2 {
		t.Fatalf("got %d sessions, want 2", len(sessions))
	}
	if sessions[0].Name != "auth" {
		t.Errorf("session 0 name = %q, want 'auth'", sessions[0].Name)
	}
	if sessions[1].Name != "db" {
		t.Errorf("session 1 name = %q, want 'db'", sessions[1].Name)
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
	// Test ":prompt" → auto name
	sessions := parseSessionArgs([]string{":do something"}, "test1234")
	if len(sessions) != 1 {
		t.Fatalf("got %d sessions, want 1", len(sessions))
	}
	// Should get auto name since name portion is empty
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
	// Should get auto name since "bad name!" is invalid
	if sessions[0].Name == "bad name!" {
		t.Error("invalid name should not be used directly")
	}
}

func TestParseSessionArgsColonInPrompt(t *testing.T) {
	// Test "db:normalize: the table" → name="db", prompt="normalize: the table"
	sessions := parseSessionArgs([]string{"db:normalize: the table"}, "test1234")
	if len(sessions) != 1 {
		t.Fatalf("got %d sessions, want 1", len(sessions))
	}
	if sessions[0].Name != "db" {
		t.Errorf("name = %q, want 'db'", sessions[0].Name)
	}
	// First message should contain the full prompt after first colon
	if len(sessions[0].History) < 1 {
		t.Fatal("expected at least 1 history message")
	}
	if sessions[0].History[0].Text != "normalize: the table" {
		t.Errorf("prompt = %q, want 'normalize: the table'", sessions[0].History[0].Text)
	}
}
