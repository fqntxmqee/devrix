//go:build acceptance

package p0

import (
	"testing"

	"github.com/devrix/devrix/internal/shared/types"
)

// Covers: L5-COMM-04, L5-COMM-05, L5-COMM-06
func TestL5_COMM_Commands_Parse(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected types.CommandType
	}{
		{name: "new command", input: "/new", expected: types.CommandNew},
		{name: "stop command", input: "/stop", expected: types.CommandStop},
		{name: "help command", input: "/help", expected: types.CommandHelp},
		{name: "unknown command", input: "/unknown", expected: types.CommandUnknown},
		{name: "regular message", input: "hello", expected: types.CommandUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := types.ParseCommand(tt.input, "/")
			if cmd.Type != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, cmd.Type)
			}
		})
	}
}
