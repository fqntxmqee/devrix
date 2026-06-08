//go:build acceptance

package p0

import (
	"testing"

	"github.com/devrix/devrix/internal/shared/types"
)

// Covers: L5-1-3-01, L5-1-3-02, L5-1-3-03, L5-0-1-07, L5-COMM-04, L5-COMM-05, L5-COMM-06
func TestL5_COMM_Commands_Parse(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected types.CommandType
		args     []string
	}{
		{name: "new command", input: "/new", expected: types.CommandNew},
		{name: "new with workdir", input: "/new /tmp/work", expected: types.CommandNew, args: []string{"/tmp/work"}},
		{name: "new uppercase", input: "/NEW", expected: types.CommandNew},
		{name: "new leading spaces", input: "  /new", expected: types.CommandNew},
		{name: "stop command", input: "/stop", expected: types.CommandStop},
		{name: "stop uppercase", input: "/STOP", expected: types.CommandStop},
		{name: "help command", input: "/help", expected: types.CommandHelp},
		{name: "help uppercase", input: "/HELP", expected: types.CommandHelp},
		{name: "unknown command", input: "/unknown", expected: types.CommandUnknown},
		{name: "regular message", input: "hello", expected: types.CommandUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := types.ParseCommand(tt.input, "/")
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
