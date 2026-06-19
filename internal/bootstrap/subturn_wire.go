package bootstrap

import "github.com/devrix/devrix/internal/shared/contracts"

var wiredSubTurn contracts.SubTurnExecutor

func setWiredSubTurn(st contracts.SubTurnExecutor) {
	wiredSubTurn = st
}

// WiredSubTurn returns the process-wide SubTurnExecutor wired by InitOrchestration.
func WiredSubTurn() contracts.SubTurnExecutor {
	return wiredSubTurn
}
