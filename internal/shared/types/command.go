package types

import "strings"

// CommandType represents the type of a CLI command
type CommandType string

const (
	CommandNew     CommandType = "new"
	CommandStop    CommandType = "stop"
	CommandHelp    CommandType = "help"
	CommandTask    CommandType = "task"
	CommandPlan    CommandType = "plan"
	CommandVerify  CommandType = "verify"
	CommandUnknown CommandType = "unknown"
)

// Command represents a parsed command from user input
type Command struct {
	Type    CommandType // 命令类型
	Raw     string      // 原始输入
	Args    []string   // 命令参数
}

// IsCommand checks if the input starts with the command prefix
func IsCommand(input string, prefix string) bool {
	return len(input) > 0 && input[0] == prefix[0]
}

// ParseCommand parses a command string into a Command struct
func ParseCommand(input string, prefix string) *Command {
	input = strings.TrimSpace(input)
	if !IsCommand(input, prefix) {
		return &Command{
			Type: CommandUnknown,
			Raw:  input,
		}
	}

	// Remove prefix and split by space
	cmdStr := input[1:] // Remove '/'
	parts := splitCommand(cmdStr)

	if len(parts) == 0 {
		return &Command{
			Type: CommandUnknown,
			Raw:  input,
		}
	}

	var cmdType CommandType
	switch {
	case strings.EqualFold(parts[0], "new"):
		cmdType = CommandNew
	case strings.EqualFold(parts[0], "stop"):
		cmdType = CommandStop
	case strings.EqualFold(parts[0], "help"):
		cmdType = CommandHelp
	case strings.EqualFold(parts[0], "task"):
		cmdType = CommandTask
	case strings.EqualFold(parts[0], "plan"):
		cmdType = CommandPlan
	case strings.EqualFold(parts[0], "verify"):
		cmdType = CommandVerify
	default:
		cmdType = CommandUnknown
	}

	return &Command{
		Type: cmdType,
		Raw:  input,
		Args: parts[1:],
	}
}

// splitCommand splits a command string by spaces
func splitCommand(s string) []string {
	var parts []string
	var current []byte
	for i := 0; i < len(s); i++ {
		if s[i] == ' ' {
			if len(current) > 0 {
				parts = append(parts, string(current))
				current = nil
			}
		} else {
			current = append(current, s[i])
		}
	}
	if len(current) > 0 {
		parts = append(parts, string(current))
	}
	return parts
}
