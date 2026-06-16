// Package prepare — D2-S15 PrepareExecutionContext orchestrator.
//
// PrepareOrchestrator coordinates the 4 A-level activities:
//
//	A01 LoadSession    → snapshot restore / worker fork / model resolution
//	A02 RecallMemory   → long-term memory recall + context formatting
//	A03 CompressContext → token budget check + compression pipeline + autocompact
//	A04 AssemblePrompt → system prompt assembly (agents + memory + workspace + attachment + hub-spoke)
package prepare

import (
	"context"

	"github.com/devrix/devrix/internal/layers/contextengine/prepare/memory"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/prompt"
	"github.com/devrix/devrix/internal/shared/types"
)

// PrepareDeps bundles dependencies for the prepare orchestration.
type PrepareDeps struct {
	SessionLoader  SessionLoader
	MemoryRecaller MemoryRecaller
	Compressor     ContextCompressor
	PromptAssembler PromptAssembler
}

// PrepareInput carries the inputs for a prepare orchestration run.
type PrepareInput struct {
	Session     *types.Session
	Model       string
	Message     string
	WorkerLocal bool
}

// PrepareOutput carries the outputs from a prepare orchestration run.
type PrepareOutput struct {
	SessionContext *types.SessionContext
	MemoryEntries  []memory.MemoryEntry
	Messages       []types.Message
	SystemPrompt   string
	IsNewSession   bool
}

// PrepareOrchestrator orchestrates the D2-S15 PrepareExecutionContext scenario.
//
// DSAFT: D2-S15 (PrepareExecutionContext)
type PrepareOrchestrator struct {
	deps PrepareDeps
}

// NewPrepareOrchestrator creates a prepare orchestrator.
func NewPrepareOrchestrator(deps PrepareDeps) *PrepareOrchestrator {
	return &PrepareOrchestrator{deps: deps}
}

// Prepare runs the full PrepareExecutionContext pipeline.
func (o *PrepareOrchestrator) Prepare(ctx context.Context, input PrepareInput) (*PrepareOutput, error) {
	sc, err := o.deps.SessionLoader.LoadOrInit(input.Session, input.Model)
	if err != nil {
		return nil, err
	}
	isNew := sc == nil
	if isNew {
		sc, err = o.deps.SessionLoader.LoadOrInit(input.Session, input.Model)
		if err != nil {
			return nil, err
		}
	}

	var memoryEntries []memory.MemoryEntry
	if !input.WorkerLocal && o.deps.MemoryRecaller != nil {
		memoryEntries, _ = o.deps.MemoryRecaller.RecallLongTermEntries(ctx, input.Message)
	}

	msgs := sc.Messages
	if o.deps.Compressor != nil && o.deps.Compressor.ShouldCompress(msgs, sc.TokenBudget) {
		compressed, _, err := o.deps.Compressor.Run(ctx, msgs, "", sc.TokenBudget)
		if err == nil {
			msgs = compressed
		}
	}

	var systemPrompt string
	if o.deps.PromptAssembler != nil {
		buildInput := prompt.SystemPromptBuildInput{
			Session: input.Session,
			WorkDir: sc.WorkDir,
		}
		if len(memoryEntries) > 0 {
			buildInput.MemoryEntries = memoryEntries
		}
		sp, _ := o.deps.PromptAssembler.Build(buildInput)
		systemPrompt = sp
	}

	return &PrepareOutput{
		SessionContext: sc,
		MemoryEntries:  memoryEntries,
		Messages:       msgs,
		SystemPrompt:   systemPrompt,
		IsNewSession:   isNew,
	}, nil
}
