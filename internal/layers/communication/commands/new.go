package commands

import (
	"fmt"

	"github.com/devrix/devrix/internal/shared/types"
)

// NewCommand implements the /new command
type NewCommand struct {
	sessionCreator SessionCreator
}

// SessionCreator creates a new session
type SessionCreator interface {
	CreateSession(chatID, workDir string) (*types.Session, error)
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

	session, err := c.sessionCreator.CreateSession("cli", workDir)
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}

	return fmt.Sprintf("New session started: %s", session.SessionID), nil
}

// Name returns the command name
func (c *NewCommand) Name() string {
	return "new"
}

// Description returns the command description
func (c *NewCommand) Description() string {
	return "Start a new session"
}
