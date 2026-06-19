// Package prepare — D2-S15 PrepareExecutionContext orchestrator (v2.2 enhanced).
//
// PrepareOrchestrator coordinates the 4 A-level activities end-to-end:
//
//	A01 LoadSession    → snapshot restore / worker fork / model resolution
//	A02 RecallMemory   → long-term memory recall + context formatting
//	A03 CompressContext → token budget check + compression pipeline + autocompact
//	A04 AssemblePrompt → system prompt assembly (agents + memory + workspace + attachment + hub-spoke)
//
// Behavioral parity with the legacy facade runProcess path is enforced via:
//   - RepairToolMessageChain + MessagesAfterCompactBoundary (inside Compressor.Run)
//   - CompressPerTurn skip flag (configurable per call)
//   - workerLocal context (skips long-term recall)
//   - hooks.BeforeLoad / AfterLoad / AfterPrepare (facade injects permission init,
//     fork, tier resolution, view synthesis — scenario-orthogonal concerns)
//
// DSAFT: D2-S15 (PrepareExecutionContext)
package prepare

import (
	"context"
	"fmt"

	"github.com/devrix/devrix/internal/layers/contextengine/prepare/memory"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/prompt"
	"github.com/devrix/devrix/internal/layers/observability/instrument/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
	"github.com/devrix/devrix/internal/shared/types"
)

// PrepareHooks holds optional lifecycle hooks for the prepare orchestrator.
// All fields are nil-safe. Facade injects scenario-orthogonal behavior here
// (worker fork, permission init, tier resolution, view synthesis).
type PrepareHooks struct {
	// BeforeLoad runs after SessionLoader returns but before long-term recall.
	// Use this for session-context mutations that must precede A02.
	BeforeLoad func(ctx context.Context, in *PrepareInput, sc *types.SessionContext)

	// AfterLoad runs after SessionLoader but BEFORE memory recall + compression.
	// Use this for permission init / model tier resolution / fork prefix
	// (these mutate sc but don't depend on memory or prompt content).
	AfterLoad func(ctx context.Context, in *PrepareInput, sc *types.SessionContext)

	// AfterPrepare runs after AssemblePrompt completes. Use this to mutate
	// the final PrepareOutput (e.g. facade wraps the system prompt into a
	// System-role Message before the LLM view is built).
	AfterPrepare func(ctx context.Context, in *PrepareInput, out *PrepareOutput)
}

// PrepareDeps bundles dependencies for the prepare orchestration.
type PrepareDeps struct {
	SessionLoader   SessionLoader
	MemoryRecaller  MemoryRecaller
	Compressor      ContextCompressor
	PromptAssembler PromptAssembler
}

// PrepareInput carries the inputs for a prepare orchestration run.
type PrepareInput struct {
	Session         *types.Session
	Model           string
	Message         string
	WorkerLocal     bool
	CompressPerTurn bool   // when false, skip per-turn compression entirely
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
type PrepareOrchestrator struct {
	deps  PrepareDeps
	hooks PrepareHooks
}

// NewPrepareOrchestrator creates a prepare orchestrator.
func NewPrepareOrchestrator(deps PrepareDeps) *PrepareOrchestrator {
	return &PrepareOrchestrator{deps: deps}
}

// WithHooks attaches lifecycle hooks. Returns the orchestrator for chaining.
func (o *PrepareOrchestrator) WithHooks(h PrepareHooks) *PrepareOrchestrator {
	o.hooks = h
	return o
}

// Prepare runs the full PrepareExecutionContext pipeline.
//
// Steps:
//
//  1. Start the D2-S15 prepare span (when ProcessSpan hook is set).
//  2. SessionLoader.LoadOrInit → obtain SessionContext.
//  3. IsNewSession = sc is freshly initialized (LoadOrInit returns false on
//     existing-session restore, true on first init).
//  4. hooks.BeforeLoad (facade may mutate input).
//  5. MemoryRecaller.RecallLongTermEntries (skipped when WorkerLocal).
//  6. Compressor.Run (skipped when !CompressPerTurn or budget allows).
//  7. PromptAssembler.Build → system prompt.
//  8. hooks.AfterPrepare (facade may wrap output).
//  9. End span and return.
func (o *PrepareOrchestrator) Prepare(ctx context.Context, input PrepareInput, processSpan SpanStarter) (*PrepareOutput, error) {
	ctx, span := startProcessSpan(ctx, processSpan, input)
	if span != nil {
		defer span.End()
	}

	sc, isNew, err := o.deps.SessionLoader.LoadOrInit(input.Session, input.Model)
	if err != nil {
		if span != nil {
			span.RecordError(err)
		}
		return nil, fmt.Errorf("prepare: load session: %w", err)
	}

	if o.hooks.BeforeLoad != nil {
		o.hooks.BeforeLoad(ctx, &input, sc)
	}

	var memoryEntries []memory.MemoryEntry
	if !input.WorkerLocal && o.deps.MemoryRecaller != nil {
		memoryEntries, _ = o.deps.MemoryRecaller.RecallLongTermEntries(ctx, input.Message)
	}

	msgs := sc.Messages
	if o.deps.Compressor != nil && input.CompressPerTurn {
		compressed, _, cerr := o.deps.Compressor.Run(ctx, msgs, "", sc.TokenBudget)
		if cerr == nil {
			msgs = compressed
		}
	}

	if o.hooks.AfterLoad != nil {
		o.hooks.AfterLoad(ctx, &input, sc)
	}

	var systemPrompt string
	if o.deps.PromptAssembler != nil {
		buildInput := input.toSystemPromptBuildInput(sc, memoryEntries)
		sp, _ := o.deps.PromptAssembler.Build(buildInput)
		systemPrompt = sp
	}

	out := &PrepareOutput{
		SessionContext: sc,
		MemoryEntries:  memoryEntries,
		Messages:       msgs,
		SystemPrompt:   systemPrompt,
		IsNewSession:   isNew,
	}

	if o.hooks.AfterPrepare != nil {
		o.hooks.AfterPrepare(ctx, &input, out)
	}

	if span != nil {
		span.SetAttributes(
			tracer.Attribute{Key: "session.is_new", Value: boolStr(isNew)},
			tracer.Attribute{Key: "session.worker_local", Value: boolStr(input.WorkerLocal)},
			tracer.Attribute{Key: "messages.count", Value: fmt.Sprintf("%d", len(msgs))},
			tracer.Attribute{Key: "memory.entries", Value: fmt.Sprintf("%d", len(memoryEntries))},
		)
	}

	return out, nil
}

// toSystemPromptBuildInput converts prepare state into the prompt.Assembler input.
// Centralized to avoid drift between orchestrator and facade paths.
func (in *PrepareInput) toSystemPromptBuildInput(sc *types.SessionContext, entries []memory.MemoryEntry) prompt.SystemPromptBuildInput {
	return prompt.SystemPromptBuildInput{
		WorkDir: sc.WorkDir,
		Session: in.Session,
		Runtime: prompt.ProcessRuntimeContext{
			SessionID: in.Session.SessionID,
			RequestID: in.Session.RequestID,
			UserID:    in.Session.UserID,
		},
		MemoryEntries: entries,
	}
}

// startProcessSpan is a small helper: invoke the supplied SpanStarter (if any),
// otherwise return ctx with no span. Kept here to centralize the op-name.
func startProcessSpan(ctx context.Context, starter SpanStarter, in PrepareInput) (context.Context, tracer.Span) {
	if starter == nil {
		return ctx, nil
	}
	return starter(ctx, telemetry.OpD2_S2_Context_Process, tracer.SpanKindInternal,
		tracer.Attribute{Key: "session.id", Value: in.Session.SessionID},
		tracer.Attribute{Key: "message.len", Value: fmt.Sprintf("%d", len(in.Message))},
	)
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}