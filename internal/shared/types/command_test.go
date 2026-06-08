package types

import "testing"

// Covers: L5-1-3-01, L5-1-3-02, L5-1-3-03
func TestParseCommand(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		prefix   string
		expected CommandType
		args     []string
	}{
		{
			name:     "parse new command",
			input:    "/new",
			prefix:   "/",
			expected: CommandNew,
		},
		{
			name:     "parse new with workdir",
			input:    "/new /tmp/work",
			prefix:   "/",
			expected: CommandNew,
			args:     []string{"/tmp/work"},
		},
		{
			name:     "parse new uppercase",
			input:    "/NEW",
			prefix:   "/",
			expected: CommandNew,
		},
		{
			name:     "parse new with leading spaces",
			input:    "  /new",
			prefix:   "/",
			expected: CommandNew,
		},
		{
			name:     "parse stop command",
			input:    "/stop",
			prefix:   "/",
			expected: CommandStop,
		},
		{
			name:     "parse stop uppercase",
			input:    "/STOP",
			prefix:   "/",
			expected: CommandStop,
		},
		{
			name:     "parse help command",
			input:    "/help",
			prefix:   "/",
			expected: CommandHelp,
		},
		{
			name:     "parse help with topic",
			input:    "/help new",
			prefix:   "/",
			expected: CommandHelp,
			args:     []string{"new"},
		},
		{
			name:     "parse help uppercase",
			input:    "/HELP",
			prefix:   "/",
			expected: CommandHelp,
		},
		{
			name:     "parse unknown command",
			input:    "/unknown",
			prefix:   "/",
			expected: CommandUnknown,
		},
		{
			name:     "parse regular message",
			input:    "hello world",
			prefix:   "/",
			expected: CommandUnknown,
		},
		{
			name:     "parse empty input",
			input:    "",
			prefix:   "/",
			expected: CommandUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := ParseCommand(tt.input, tt.prefix)
			if cmd.Type != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, cmd.Type)
			}
			if tt.args != nil {
				if len(cmd.Args) != len(tt.args) || (len(cmd.Args) > 0 && cmd.Args[0] != tt.args[0]) {
					t.Errorf("args = %v, want %v", cmd.Args, tt.args)
				}
			}
		})
	}
}

func TestIsCommand(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		prefix   string
		expected bool
	}{
		{
			name:     "is command",
			input:    "/new",
			prefix:   "/",
			expected: true,
		},
		{
			name:     "is not command",
			input:    "hello",
			prefix:   "/",
			expected: false,
		},
		{
			name:     "empty input",
			input:    "",
			prefix:   "/",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsCommand(tt.input, tt.prefix)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}
