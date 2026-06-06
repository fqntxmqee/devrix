package commands

import "fmt"

// NewCommand implements the /new command
type NewCommand struct {
	sessionCreator SessionCreator
}

// SessionCreator creates a new session
type SessionCreator interface {
	CreateSession(chatID, workDir string) (string, error)
}

// NewNewCommand creates a new NewCommand
func NewNewCommand(creator SessionCreator) *NewCommand {
	return &NewCommand{sessionCreator: creator}
}

// Execute runs the new command
func (c *NewCommand) Execute(args []string) (string, error) {
	// Get work directory from args or use current
	workDir := ""
	if len(args) > 0 {
		workDir = args[0]
	}

	sessionID, err := c.sessionCreator.CreateSession("cli", workDir)
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}

	return fmt.Sprintf("New session started: %s", sessionID), nil
}

// Name returns the command name
func (c *NewCommand) Name() string {
	return "new"
}

// Description returns the command description
func (c *NewCommand) Description() string {
	return "Start a new session"
}
