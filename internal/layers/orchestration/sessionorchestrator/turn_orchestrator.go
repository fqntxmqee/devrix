package sessionorchestrator

import (
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/persist"
	"github.com/devrix/devrix/internal/layers/observability"
)

// OrchestratorDeps holds the dependencies for the TurnOrchestrator.
type OrchestratorDeps struct {
	LLM     LLMInvoker
	Context ContextPreparer
	Tools   ToolRoundExecutor
	Persist SessionPersister
	// MaxTurns is an *optional safety net*, not the expected loop bound.
	// 0 / negative → unbounded: the loop only terminates on LLM natural
	// finish or one of the deterministic exit reasons below. The agent
	// matches claude-code semantics: the main conversation has no hard
	// turn limit; child agents (compact, extract-memories, etc.) set
	// their own MaxTurns based on expected workload.
	MaxTurns         int
	DefaultModel     string
	MaxContextTokens int
	ObsBridge        *observability.Bridge
	FocusHint        FocusHintProvider
	ResolveAwait     ResolveAwaiter
	// ToolResultStore persists oversized tool results to disk so they do
	// not blow up the LLM context budget (DM-20260620-001 / AC1). Nil
	// disables the cap (legacy behaviour).
	ToolResultStore *persist.ToolResultStore
	// MaxToolResultChars is the soft cap above which a tool result is
	// persisted. 0 → persist.DefaultMaxChars (12000).
	MaxToolResultChars int
	// MaxAssistantChars is the soft cap above which an assistant
	// message is folded head/tail (DM-20260620-001 / AC2). 0 →
	// persist.DefaultMaxAssistantChars (8000).
	MaxAssistantChars int
	// PromptLanguage controls LLM-facing compression prompts (zh-CN | en-US).
	PromptLanguage string
	// FallbackModel is the optional secondary model used when the primary
	// model returns RateLimit/ServerError ≥ 2 consecutive times.
	//
	// DM-20260628-001 (FR-13, AC3 partial): field reservation only. Empty =
	// fallback disabled. Full retry-loop wiring is the P0-2 follow-up
	// (`devrix-streaming-fallback`); S4 only logs fallback_trigger_candidate
	// + fallback_model_set_but_not_yet_wired for observability.
	FallbackModel string
}

// verify.ExitReason is defined in exit_reason.go.

// Deterministic-exit thresholds. Aligned with clawcode's hard-coded
// constants in src/query/tokenBudget.ts and src/query.ts.
const (
	repeatedToolLookback           = 5
	repeatedToolThreshold          = 3
	consecutiveToolErrorThreshold  = 3
	tokenBudgetCompletionThreshold = 0.9
	tokenBudgetDiminishingDelta    = 500
	tokenBudgetDiminishingChecks   = 2
)

// Metadata key on the final complete event carrying the exit reason.
const metadataKeyExitReason = "exit_reason"

// DefaultOrchestrator implements TurnOrchestrator with the canonical
// prepare→llm→tools→persist state machine (design.md §3).
type DefaultOrchestrator struct {
	llm              LLMInvoker
	context          ContextPreparer
	tools            ToolRoundExecutor
	persist          SessionPersister
	maxTurns         int
	defaultModel     string
	maxContextTokens int
	obsBridge        *observability.Bridge
	focusHint        FocusHintProvider
	resolveAwait     ResolveAwaiter
	toolResultStore  *persist.ToolResultStore
	maxToolResultCh  int
	maxAssistantCh   int
	promptLanguage   string
	// fallbackModel — DM-20260628-001 (FR-13). Empty = fallback disabled.
	fallbackModel string
	// consecutiveServerErrors counts consecutive APICodeRateLimit/ServerError
	// responses from the primary model; reset on success or non-retryable error.
	consecutiveServerErrors int
}

// NewOrchestrator creates a DefaultOrchestrator.
//
// MaxTurns ≤ 0 means *unbounded* — the loop runs until the LLM naturally
// finishes or one of the deterministic exit reasons fires. Match
// clawcode semantics where the main conversation has no hard turn
// ceiling; child agents that need a bound set it explicitly (see
// internal/layers/orchestration/delegatetools/builtin_agents.go).
func NewOrchestrator(deps OrchestratorDeps) *DefaultOrchestrator {
	// Leave deps.MaxTurns at 0 / negative — the orchestrator treats those
	// as "no safety net" rather than substituting a magic default. See
	// OrchestratorDeps.MaxTurns doc for the rationale.
	maxChars := deps.MaxToolResultChars
	if maxChars == 0 && deps.ToolResultStore != nil {
		maxChars = persist.DefaultMaxChars
	}
	assistChars := deps.MaxAssistantChars
	if assistChars == 0 && deps.ToolResultStore != nil {
		assistChars = persist.DefaultMaxAssistantChars
	}
	return &DefaultOrchestrator{
		llm:              deps.LLM,
		context:          deps.Context,
		tools:            deps.Tools,
		persist:          deps.Persist,
		maxTurns:         deps.MaxTurns,
		defaultModel:     deps.DefaultModel,
		maxContextTokens: deps.MaxContextTokens,
		obsBridge:        deps.ObsBridge,
		focusHint:        deps.FocusHint,
		resolveAwait:     deps.ResolveAwait,
		toolResultStore:  deps.ToolResultStore,
		maxToolResultCh:  maxChars,
		maxAssistantCh:   assistChars,
		promptLanguage:   deps.PromptLanguage,
		fallbackModel:    deps.FallbackModel,
	}
}

// MaxTurns returns the orchestrator-level MaxTurns bound (0 = unbounded).
// Surfaced for diagnostics in the D7 bootstrap wiring log so the actual
// bound is observable in startup logs (the previous hardcoded 8 was
// misleading once the main conversation switched to unbounded).
func (o *DefaultOrchestrator) MaxTurns() int {
	return o.maxTurns
}

