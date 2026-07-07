package bootstrap

import (
	"fmt"
	"time"

	"github.com/devrix/devrix/internal/layers/contextengine/i18n"
	"github.com/devrix/devrix/internal/layers/orchestration/decisionplanning"
	"github.com/devrix/devrix/internal/layers/orchestration/mups/learn"
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"github.com/devrix/devrix/internal/layers/orchestration/sessionorchestrator"
	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/contracts"
)

// ItemPipelineWireDeps holds production wiring for per-WorkItem MUPS (Phase D).
//
// DM-20260626-009: per-WorkItem execution uses WorkItemExecutor
// (see sessionorchestrator.workitem_executor.go). The Executor drives a
// per-WorkItem ReAct loop (LLM ↔ Tool) and needs LLMInvoker for the LLM
// side and ContextPreparer for workspace context assembly.
type ItemPipelineWireDeps struct {
	ToolExec      sessionorchestrator.ToolRoundExecutor
	Tasks         *workmodel.TaskManager
	Classifier    decisionplanning.IntentClassifier
	LLMInvoker     orchtypes.LLMInvoker
	CtxPreparer    sessionorchestrator.ContextPreparer
	PromptLanguage string
	TrackMode      string
	// SemanticConvergence (DM-20260706-006) configures the LLM-driven
	// MUPS Verify override. Zero value uses orchtypes defaults
	// (Enabled=true production). Bootstrap wires the Enabled value
	// into ItemPipelineRunner.SemanticConfig and constructs a
	// DefaultSemanticVerifier when Enabled=true.
	SemanticConvergence orchtypes.SemanticConvergenceConfig
	// DAGExecutor (DM-20260707-001 PR-C) drives the multi-intent DAG
	// path when Plan emits pl.DAG + pl.IntentSegmentSet. nil →
	// ItemPipelineRunner.Run() falls back to the legacy single-WorkItem
	// path (defensive default so pre-PR-C callers continue to compile).
	DAGExecutor wavescheduler.DAGExecutor
	// StreamingEmitter (DM-20260707-001 PR-C) is the IM-side streaming
	// emit adapter (FeishuAdapter satisfies this via its EmitPartialCard
	// / EmitFinalCard methods). nil → DAG path still runs inner Execute +
	// Learn, but skips the IM streaming card emit.
	StreamingEmitter sessionorchestrator.StreamingEmitter
}

// WireDefaultMUPSLearner constructs the in-process LP-1 learner used by both
// ProcessMessage (prior injection) and ItemPipelineRunner (Learn node).
func WireDefaultMUPSLearner() learn.Learner {
	skill := learn.NewSkillMemory()
	feedback := learn.NewFeedbackMemory()
	scheduled := learn.NewScheduledMemory()
	rep := learn.NewInMemoryReputationStore()
	return learn.NewDefaultLearner(skill, feedback, scheduled, rep, learn.NewAssetBuilder())
}

// WireItemPipeline builds ItemPipelineRunner + shared Learner for bootstrap.
func WireItemPipeline(deps ItemPipelineWireDeps) (*sessionorchestrator.ItemPipelineRunner, learn.Learner, error) {
	if deps.Tasks == nil {
		return nil, nil, fmt.Errorf("wire item pipeline: TaskManager required")
	}
	if deps.ToolExec == nil {
		return nil, nil, fmt.Errorf("wire item pipeline: ToolRoundExecutor required")
	}
	if deps.LLMInvoker == nil {
		return nil, nil, fmt.Errorf("wire item pipeline: LLMInvoker required (WorkItemExecutor needs D7-S2-A07 to call the LLM)")
	}
	if deps.CtxPreparer == nil {
		return nil, nil, fmt.Errorf("wire item pipeline: ContextPreparer required (WorkItemExecutor needs D2-S15 to assemble SystemPrompt + Tools)")
	}
	learner := WireDefaultMUPSLearner()
	mups, ok := deps.CtxPreparer.(contracts.IMUPSContextMaterializer)
	if !ok {
		return nil, nil, fmt.Errorf("wire item pipeline: ContextPreparer must implement IMUPSContextMaterializer")
	}
	executor := sessionorchestrator.NewWorkItemExecutor(deps.LLMInvoker, mups, deps.ToolExec)
	if mat := newDefaultMaterializer(); mat != nil {
		executor.Materializer = mat
	}
	// DM-20260706-006: build the SemanticConfig + (optionally) the
	// SemanticVerifier so the per-WorkItem ItemPipelineRunner consults
	// the LLM when the code-based verify rubber-stamps Pass but the
	// artifact looks structurally identical to a prior round.
	semanticCfg := buildItemPipelineSemanticConfig(deps.SemanticConvergence)
	var semanticVerifier sessionorchestrator.SemanticVerifier
	if semanticCfg.Enabled && deps.LLMInvoker != nil {
		timeoutMs := deps.SemanticConvergence.TimeoutMs
		if timeoutMs <= 0 {
			timeoutMs = 8000
		}
		semanticVerifier = &sessionorchestrator.DefaultSemanticVerifier{
			LLM:       deps.LLMInvoker,
			ModelTier: deps.SemanticConvergence.ModelTier,
			Timeout:   time.Duration(timeoutMs) * time.Millisecond,
		}
	}
	runner, err := sessionorchestrator.NewItemPipelineRunner(sessionorchestrator.ItemPipelineDeps{
		Classifier: deps.Classifier,
		Executor:   executor,
		Learner:    learner,
		Tasks:      deps.Tasks,
		TrackMode:  deps.TrackMode,
		ObservationProposer: sessionorchestrator.NewLLMObservationProposer(
			deps.LLMInvoker,
			mups,
			i18n.ParseLanguage(deps.PromptLanguage),
		),
		StrategicPlanProposer: sessionorchestrator.NewLLMStrategicPlanProposer(
			deps.LLMInvoker,
			mups,
			i18n.ParseLanguage(deps.PromptLanguage),
		),
		// DM-20260706-006: pass through the wired semantic config +
		// verifier. The runner uses cfg.Enabled as the master switch
		// (looksLikeTemplateMimicry short-circuits on false). When
		// verifier is nil and cfg.Enabled is true, the cheap Jaccard
		// pre-check still runs but the LLM call is skipped (graceful
		// degradation when LLMInvoker is unavailable).
		SemanticConfig:    semanticCfg,
		SemanticVerifier:  semanticVerifier,
		// DM-20260707-001 PR-C: forward DAGExecutor + StreamingEmitter.
		// Both are optional — when nil, Run() falls back to the legacy
		// single-WorkItem path. The production bootstrap path doesn't
		// construct a WaveScheduler / FeishuAdapter here today; a
		// follow-up change (DM-20260707-002+) is responsible for
		// wiring those into WireItemPipeline's caller.
		DAGExecutor:      deps.DAGExecutor,
		StreamingEmitter: deps.StreamingEmitter,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("wire item pipeline: %w", err)
	}
	return runner, learner, nil
}

// buildItemPipelineSemanticConfig translates the runtime config shape
// (orchtypes.SemanticConvergenceConfig) into the package-internal
// sessionorchestrator.SemanticSimilarityConfig. Kept as a separate
// function so the field mapping is testable in isolation.
func buildItemPipelineSemanticConfig(in orchtypes.SemanticConvergenceConfig) sessionorchestrator.SemanticSimilarityConfig {
	return sessionorchestrator.SemanticSimilarityConfig{
		Enabled:                in.Enabled,
		MinSimilarityForVerify: in.MinSimilarity,
		MinArtifactChars:       100, // mirrors sessionorchestrator.DefaultSemanticSimilarityConfig
	}
}