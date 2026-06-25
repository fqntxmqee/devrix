package orchtypes

import (
	"context"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/shared/types"
)

// LLMInvoker abstracts the LLM streaming interface used by both D7 RunTurn
// (sessionorchestrator/orchestrator.go) and D7 LLM Decomposer
// (decisionplanning/llm_decomposer.go).
//
// DM-20260626-004: 上提到 orchtypes 打破 sessionorchestrator ↔ decisionplanning
// import cycle (orchestrate_path.go 引 decisionplanning, llm_decomposer.go 之前
// 引 sessionorchestrator, turn 包合并到 sessionorchestrator 后 cycle 暴露)。
// sessionorchestrator.LLMInvoker / decisionplanning.LLMInvoker 都改为
// type alias = orchtypes.LLMInvoker, 保证 3 处类型一致 + 兼容。
type LLMInvoker interface {
	InvokeStream(ctx context.Context, req LLMInvokeRequest) (<-chan llmgateway.Chunk, error)
}

// LLMInvokeRequest is the input to LLMInvoker.InvokeStream.
//
// Fields:
//   - SessionID: D7 session id for tracing/reputation
//   - Tier: gateway tier override ("" → use invoker default)
//   - SystemPrompt: prepended as system role message
//   - Messages: LLM input messages (user/assistant/tool turns)
//   - Tools: tool schemas exposed to the LLM
type LLMInvokeRequest struct {
	SessionID    string
	Tier         string
	SystemPrompt string
	Messages     []types.Message
	Tools        []ToolSchema
}

// ToolSchema mirrors sessionorchestrator.ToolSchema for the D7→D3 contract
// boundary. Uplifted to orchtypes to break the sessionorchestrator ↔
// decisionplanning import cycle.
type ToolSchema struct {
	Name        string
	Description string
	Parameters  map[string]any
}