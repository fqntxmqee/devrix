package types

// CommandType represents the type of a CLI command
type CommandType string

const (
	CommandNew     CommandType = "new"
	CommandStop    CommandType = "stop"
	CommandHelp    CommandType = "help"
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
	switch parts[0] {
	case "new":
		cmdType = CommandNew
	case "stop":
		cmdType = CommandStop
	case "help":
		cmdType = CommandHelp
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
