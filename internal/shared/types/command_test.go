package types

import "testing"

func TestParseCommand(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		prefix   string
		expected CommandType
	}{
		{
			name:     "parse new command",
			input:    "/new",
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
			name:     "parse help command",
			input:    "/help",
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
