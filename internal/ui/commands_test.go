package ui

import "testing"

func TestParseCommand(t *testing.T) {
	tests := []struct {
		input string
		name  string
		args  []string
		isNil bool
	}{
		{"", "", nil, true},
		{"hello world", "", nil, true},
		{"?", "help", nil, false},
		{"/new", "new", nil, false},
		{"/list", "list", nil, false},
		{"/skip", "skip", nil, false},
		{"/dismiss", "dismiss", nil, false},
		{"/go auth", "go", []string{"auth"}, false},
		{"/GO Auth", "go", []string{"Auth"}, false},
		{"  /new  ", "new", nil, false},
		{"/", "", nil, true},
	}
	for _, tt := range tests {
		cmd := ParseCommand(tt.input)
		if tt.isNil {
			if cmd != nil {
				t.Errorf("ParseCommand(%q) = %+v, want nil", tt.input, cmd)
			}
			continue
		}
		if cmd == nil {
			t.Errorf("ParseCommand(%q) = nil, want command", tt.input)
			continue
		}
		if cmd.Name != tt.name {
			t.Errorf("ParseCommand(%q).Name = %q, want %q", tt.input, cmd.Name, tt.name)
		}
		if len(cmd.Args) == 0 && len(tt.args) == 0 {
			continue
		}
		if len(cmd.Args) != len(tt.args) {
			t.Errorf("ParseCommand(%q).Args = %v, want %v", tt.input, cmd.Args, tt.args)
		}
	}
}
