package compression

import (
	"context"

	"github.com/devrix/devrix/internal/layers/contextengine/i18n"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/conversation"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

const (
	stepClearToolResults = "clear_tool_results"
	stepMessageBudget    = "message_budget"
	stepToolResultBudget = "tool_result_budget"
	stepSnip             = "snip"
	stepMicrocompact     = "microcompact"
	stepCollapse         = "context_collapse"
	stepAssembly         = "system_prompt_assembly"
	stepAutocompact      = "autocompact"
	stepTokenBlock       = "token_block"
	// DM-20260702-008 / D2-S15-A02-T14: per-message aggregate 200K cap
	stepPerMessageBudget = "per_message_budget"
)

// Pipeline runs the seven-step compression chain.
//
// DM-20260629-002 devrix-d2-dsaft-restructuring PR-2: RunForSession split into
//   - pipeline.go (this file: orchestrator + step loop)
//   - compression_steps.go (4 step helpers: toolResultBudget, snip, microcompact, collapse, assemble)
//   - budget.go (token validation: ShouldCompress, checkBudget, minKeepForAutocompact)
// Previously a 109-LOC god function; now ~85 LOC for the orchestrator alone.
type Pipeline struct {
	counter            contracts.ITokenCounter
	enabled            bool
	autocompactCfg     config.AutocompactConfig
	microcompactCfg    config.MicrocompactConfig
	maxMessages        int
	keepTailMessages   int
	preserveHeadTurns  int
	summarizer         Summarizer
	stepObserver       StepObserver
	asyncCompact       *AsyncAutocompacter
	sessionID          string
	projectDir         string // DM-20260702-008: T01 PersistToFile root
	// DM-20260702-008 / D2-S15-A02-T14: per-message budget state.
	// Thread-local ContentReplacementState for the aggregate 200K cap.
	// Per-conversation, set once and reused across turns.
	perMessageBudget  *PerMessageBudget
	skipAssembly       bool
	locale             i18n.Locale
}

// NewPipeline creates a compression pipeline with functional options.
func NewPipeline(opts ...Option) *Pipeline {
	p := defaultPipeline()
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// NewPipelineEnabled is a convenience constructor for tests (enabled/disabled only).
func NewPipelineEnabled(enabled bool) *Pipeline {
	return NewPipeline(WithEnabled(enabled))
}

// Run compresses messages to fit within budget.
func (p *Pipeline) Run(ctx context.Context, msgs []types.Message, systemPrompt string, budget types.TokenBudget) ([]types.Message, types.CompressionReport, error) {
	return p.RunForSession(ctx, p.sessionID, msgs, systemPrompt, budget)
}

// RunForSession compresses messages for a specific session (enables async autocompact).
//
// DM-20260629-002 PR-2: orchestrator-only after the god-fn split. The 4-step
// named loop is now applyCompressionSteps, the autocompact+assembly tail is
// applyAssemblyAndBudget.
func (p *Pipeline) RunForSession(ctx context.Context, sessionID string, msgs []types.Message, systemPrompt string, budget types.TokenBudget) ([]types.Message, types.CompressionReport, error) {
	report := types.CompressionReport{OriginalTokens: p.counter.CountMessages(msgs)}
	if !p.enabled {
		out := assemble(systemPrompt, msgs)
		report.CompressedTokens = p.counter.CountMessages(out)
		return out, report, nil
	}

	current := append([]types.Message(nil), msgs...)

	if cleared, applied := clearStaleToolResults(current, p.microcompactCfg.KeepRecentToolResults); applied {
		before := p.counter.CountMessages(current)
		current = cleared
		p.emitStep(ctx, stepClearToolResults, before, p.counter.CountMessages(current))
		report.StepsApplied = append(report.StepsApplied, stepClearToolResults)
		report.Truncated = true
	}

	if p.maxMessages > 0 && len(current) > p.maxMessages {
		beforeCount := len(current)
		headTurns := p.preserveHeadTurns
		if headTurns <= 0 {
			headTurns = 1
		}
		tail := p.keepTailMessages
		if tail <= 0 {
			tail = p.maxMessages - 2
		}
		current = conversation.HeadTailTrim(current, p.maxMessages, headTurns, tail)
		if len(current) != beforeCount {
			report.StepsApplied = append(report.StepsApplied, stepMessageBudget)
			report.Truncated = true
		}
	}

	current, report = p.applyCompressionSteps(ctx, current, budget, report)

	current, report, err := p.applyAssemblyAndBudget(ctx, sessionID, current, systemPrompt, budget, report)
	if err != nil {
		return nil, report, err
	}
	return current, report, nil
}

// applyCompressionSteps runs the 4 named token-budget steps (tool_result_budget,
// snip, microcompact, collapse) in order, recording each applied step into the report.
//
// DM-20260629-002 PR-2: extracted from RunForSession (was a 51-LOC inline loop).
func (p *Pipeline) applyCompressionSteps(ctx context.Context, current []types.Message, budget types.TokenBudget, report types.CompressionReport) ([]types.Message, types.CompressionReport) {
	type namedStep struct {
		name string
		fn   func([]types.Message, types.TokenBudget) ([]types.Message, bool)
	}
	steps := []namedStep{
		{stepToolResultBudget, func(m []types.Message, b types.TokenBudget) ([]types.Message, bool) {
			before := p.counter.CountMessages(m)
			next := toolResultBudget(p.counter, m, b.ToolResultBudget, p.projectDir, p.sessionID)
			after := p.counter.CountMessages(next)
			p.emitStep(ctx, stepToolResultBudget, before, after)
			return next, before != after
		}},
		// DM-20260702-008 / D2-S15-A02-T14: per-message aggregate 200K cap.
		// Runs AFTER stepToolResultBudget so individual results are
		// already bounded; this step catches the SUM-of-N case.
		{stepPerMessageBudget, func(m []types.Message, b types.TokenBudget) ([]types.Message, bool) {
			if p.perMessageBudget == nil {
				return m, false
			}
			before := p.counter.CountMessages(m)
			next := applyPerMessageBudget(p.perMessageBudget, m)
			after := p.counter.CountMessages(next)
			p.emitStep(ctx, stepPerMessageBudget, before, after)
			return next, before != after
		}},
		{stepSnip, func(m []types.Message, b types.TokenBudget) ([]types.Message, bool) {
			before := p.counter.CountMessages(m)
			snipTarget := b.SnipTarget
			if snipTarget <= 0 {
				snipTarget = b.CompressionTarget
			}
			next := snip(p.counter, m, snipTarget, p.minKeepForAutocompact())
			after := p.counter.CountMessages(next)
			p.emitStep(ctx, stepSnip, before, after)
			return next, before != after
		}},
		{stepMicrocompact, func(m []types.Message, b types.TokenBudget) ([]types.Message, bool) {
			before := p.counter.CountMessages(m)
			next, applied := microcompact(m, b)
			after := p.counter.CountMessages(next)
			if applied {
				p.emitStep(ctx, stepMicrocompact, before, after)
			}
			return next, applied
		}},
		{stepCollapse, func(m []types.Message, b types.TokenBudget) ([]types.Message, bool) {
			before := p.counter.CountMessages(m)
			next, applied := collapse(m, b)
			after := p.counter.CountMessages(next)
			if applied {
				p.emitStep(ctx, stepCollapse, before, after)
			}
			return next, applied
		}},
	}

	for _, step := range steps {
		next, applied := step.fn(current, budget)
		if applied {
			report.StepsApplied = append(report.StepsApplied, step.name)
			report.Truncated = true
		}
		current = next
	}
	return current, report
}

// applyAssemblyAndBudget runs autocompact (step 6), prepends the system prompt
// (step 7), and validates the final token budget (step 8 guard).
//
// DM-20260629-002 PR-2: extracted from RunForSession (was a 19-LOC tail block).
func (p *Pipeline) applyAssemblyAndBudget(ctx context.Context, sessionID string, current []types.Message, systemPrompt string, budget types.TokenBudget, report types.CompressionReport) ([]types.Message, types.CompressionReport, error) {
	next, stepLabel, _ := runAutocompact(ctx, sessionID, current, budget, p.counter, p.autocompactCfg, p.summarizer, p.stepObserver, p.asyncCompact, p.locale)
	current = next
	report.StepsApplied = append(report.StepsApplied, stepLabel)
	if stepLabel == stepAutocompact {
		report.Truncated = true
	}

	beforeAsm := p.counter.CountMessages(current)
	if !p.skipAssembly {
		current = assemble(systemPrompt, current)
	}
	afterAsm := p.counter.CountMessages(current)
	p.emitStep(ctx, stepAssembly, beforeAsm, afterAsm)
	report.CompressedTokens = afterAsm

	if err := p.checkBudget(current, budget, &report); err != nil {
		return nil, report, err
	}
	return current, report, nil
}

func (p *Pipeline) emitStep(ctx context.Context, step string, before, after int) {
	if p.stepObserver != nil {
		p.stepObserver.OnStep(ctx, step, before, after)
	}
}

// applyPerMessageBudget walks all user messages and runs the budget
// step on each. Returns a new slice (does not mutate input). When a
// message's tool_result aggregate exceeds the threshold, the largest
// fresh results are persisted to disk and replaced with
// <persisted-output> previews.
//
// DM-20260702-008 / D2-S15-A02-T14: simple per-message sweep; the more
// sophisticated "collectCandidatesByMessage + selectFreshToReplace"
// pattern from clawcode is implemented inside PerMessageBudget.Enforce
// for callers that need it. The pipeline-level step here is the
// simpler "run Enforce on every tool message" form which is correct
// for our single-threaded sync pipeline.
func applyPerMessageBudget(budget *PerMessageBudget, msgs []types.Message) []types.Message {
	out := make([]types.Message, len(msgs))
	for i, m := range msgs {
		out[i] = m
		if m.Role != types.MessageRoleTool {
			continue
		}
		// Only enforce when content actually exceeds threshold; the
		// per-message check is cheap and avoids unnecessary work.
		threshold := budget.Threshold
		if threshold <= 0 {
			threshold = MaxToolResultsPerMessageChars
		}
		if len(m.Content) <= threshold {
			continue
		}
		// Use the message ID as the toolUseID surrogate. Per-message
		// budget decisions are per-message (not per-result), and the
		// ContentReplacementState freezes the decision at the
		// message granularity. For finer per-result decisions, callers
		// use PerMessageBudget.Enforce directly with the actual result ID.
		out[i].Content = budget.Enforce(m.ID, m.Content)
	}
	return out
}
