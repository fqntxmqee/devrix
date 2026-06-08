package collaboration

import (
	"fmt"

	"github.com/devrix/devrix/internal/layers/multiagent"
)

// ValidateMode returns an error for unknown collaboration modes.
func ValidateMode(mode multiagent.CollaborationMode) error {
	switch mode {
	case multiagent.ModeDefault, multiagent.ModeChainOfThought, multiagent.ModeIterativeRefinement:
		return nil
	default:
		return fmt.Errorf("unknown collaboration mode: %q", mode)
	}
}
