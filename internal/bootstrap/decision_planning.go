package bootstrap

import (
	"github.com/devrix/devrix/internal/layers/orchestration/decisionplanning"
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
)

// WireDecisionPlanning wires the production LLM Task Decomposer for S5
// (DecisionPlanning+Observe). Wraps decisionplanning.NewLLMDecomposer to keep
// the 6 S × WireFunc naming consistent in InitOrchestration.
func WireDecisionPlanning(llmInvoker orchtypes.LLMInvoker, defaultTier string) decisionplanning.LLMTaskDecomposer {
	return decisionplanning.NewLLMDecomposer(decisionplanning.LLMDecomposerDeps{
		LLM:         llmInvoker,
		DefaultTier: defaultTier,
	})
}
