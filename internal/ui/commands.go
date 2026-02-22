package ui

import "strings"

// Command represents a parsed user command.
type Command struct {
	Name string   // "new", "list", "skip", "dismiss", "go", "help"
	Args []string // Remaining arguments after command name
}

// ParseCommand checks if input is a command (starts with / or is "?").
// Returns nil if it's a regular message (reply to session).
func ParseCommand(input string) *Command {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil
	}
	if input == "?" {
		return &Command{Name: "help"}
	}
	if !strings.HasPrefix(input, "/") {
		return nil
	}
	parts := strings.Fields(input[1:]) // Strip leading /
	if len(parts) == 0 {
		return nil
	}
	return &Command{
		Name: strings.ToLower(parts[0]),
		Args: parts[1:],
	}
}
