package kernel

import "github.com/devrix/devrix/internal/shared/contracts"

// SetPreparedTurnRunner wires D7 as the Process() turn loop (DM-20260618-010).
func (e *ContextEngine) SetPreparedTurnRunner(r contracts.PreparedTurnRunner) {
	if e == nil {
		return
	}
	e.preparedTurnRunner = r
}

// PreparedTurnRunner returns the wired D7 runner for tests.
func (e *ContextEngine) PreparedTurnRunner() contracts.PreparedTurnRunner {
	if e == nil {
		return nil
	}
	return e.preparedTurnRunner
}
