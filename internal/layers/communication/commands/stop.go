package commands

// StopCommand implements the /stop command
type StopCommand struct {
	stopper Stopper
}

// Stopper stops the current operation
type Stopper interface {
	Stop(sessionID string) error
}

// NewStopCommand creates a new StopCommand
func NewStopCommand(stopper Stopper) *StopCommand {
	return &StopCommand{stopper: stopper}
}

// Execute runs the stop command
func (c *StopCommand) Execute(sessionID string) (string, error) {
	if c.stopper == nil {
		return "No active operation to stop", nil
	}

	if err := c.stopper.Stop(sessionID); err != nil {
		return "", err
	}

	return "Operation stopped", nil
}

// Name returns the command name
func (c *StopCommand) Name() string {
	return "stop"
}

// Description returns the command description
func (c *StopCommand) Description() string {
	return "Stop current generation"
}
