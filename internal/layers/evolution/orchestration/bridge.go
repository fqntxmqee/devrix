// Package orchestration is a backward-compatibility bridge.
//
// Deprecated: use github.com/devrix/devrix/internal/layers/evolution/guard instead.
// This bridge will be removed in v2.1.
package orchestration

import "github.com/devrix/devrix/internal/layers/evolution/guard"

// Types

type (
	AgentController                = guard.AgentController
	TaskController                 = guard.TaskController
	AgentFactory                   = guard.AgentFactory
	InterventionExecutor           = guard.InterventionExecutor
	DecisionCategory               = guard.DecisionCategory
	RiskClass                      = guard.RiskClass
	DecisionRecord                 = guard.DecisionRecord
	ValidationResult               = guard.ValidationResult
	Intervention                   = guard.Intervention
	RuntimeJudge                   = guard.RuntimeJudge
	DecisionHook                   = guard.DecisionHook
	ValidationHook                 = guard.ValidationHook
	InterventionHook               = guard.InterventionHook
	RuntimeOrchestrationValidator  = guard.RuntimeOrchestrationValidator
	AgentObserver                  = guard.AgentObserver
	OrchestrationObserver          = guard.OrchestrationObserver
	OrchestrationConfig            = guard.OrchestrationConfig
)

// Functions

var (
	NewInterventionExecutor            = guard.NewInterventionExecutor
	NewRuntimeJudge                    = guard.NewRuntimeJudge
	NewRuntimeOrchestrationValidator   = guard.NewRuntimeOrchestrationValidator
	NewOrchestrationObserver           = guard.NewOrchestrationObserver
)
