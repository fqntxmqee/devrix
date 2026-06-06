package commands

// HelpCommand implements the /help command
type HelpCommand struct{}

// NewHelpCommand creates a new HelpCommand
func NewHelpCommand() *HelpCommand {
	return &HelpCommand{}
}

// Execute runs the help command
func (c *HelpCommand) Execute(args []string) string {
	return `
Devrix CLI Commands
====================

/new          Start a new session
/stop         Stop current generation
/help         Show this help

Examples:
  /new
  /help

For more information, visit the documentation.
`
}

// Name returns the command name
func (c *HelpCommand) Name() string {
	return "help"
}

// Description returns the command description
func (c *HelpCommand) Description() string {
	return "Show help information"
}
