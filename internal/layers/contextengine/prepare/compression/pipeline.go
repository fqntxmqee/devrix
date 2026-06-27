package compression

import (
	"context"
	"fmt"

	"github.com/devrix/devrix/internal/layers/contextengine/i18n"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/conversation"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/errors"
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
)

// Pipeline runs the seven-step compression chain.
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

	type namedStep struct {
		name string
		fn   func([]types.Message, types.TokenBudget) ([]types.Message, bool)
	}
	steps := []namedStep{
		{stepToolResultBudget, func(m []types.Message, b types.TokenBudget) ([]types.Message, bool) {
			before := p.counter.CountMessages(m)
			next := toolResultBudget(p.counter, m, b.ToolResultBudget)
			after := p.counter.CountMessages(next)
			p.emitStep(ctx, stepToolResultBudget, before, after)
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

	// Step 6: autocompact (before assembly, on message history only)
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

	if p.counter.CountMessages(current) > budget.MaxContextTokens-budget.ReservedOutput {
		report.StepsApplied = append(report.StepsApplied, stepTokenBlock)
		return nil, report, errors.NewContextExceededError()
	}
	return current, report, nil
}

func (p *Pipeline) emitStep(ctx context.Context, step string, before, after int) {
	if p.stepObserver != nil {
		p.stepObserver.OnStep(ctx, step, before, after)
	}
}

func toolResultBudget(counter contracts.ITokenCounter, msgs []types.Message, maxPerResult int) []types.Message {
	out := make([]types.Message, len(msgs))
	for i, m := range msgs {
		out[i] = m
		if m.Role == types.MessageRoleTool && counter.CountText(m.Content) > maxPerResult {
			out[i].Content = counter.TruncateToTokens(m.Content, maxPerResult) + "\n...[truncated]"
		}
	}
	return out
}

func (p *Pipeline) minKeepForAutocompact() int {
	const defaultMinKeep = 4
	if !p.autocompactCfg.Enabled {
		return defaultMinKeep
	}
	turns := p.autocompactCfg.PreserveHeadTurns + p.autocompactCfg.PreserveTailTurns + 1
	if turns < 3 {
		turns = 3
	}
	// Each turn needs at least one user message; keep enough messages for head+middle+tail turns.
	minKeep := turns * 2
	if minKeep < defaultMinKeep {
		return defaultMinKeep
	}
	return minKeep
}

func snip(counter contracts.ITokenCounter, msgs []types.Message, target, minKeep int) []types.Message {
	if minKeep <= 0 {
		minKeep = 4
	}
	if len(msgs) <= minKeep {
		return msgs
	}
	out := append([]types.Message(nil), msgs...)
	for counter.CountMessages(out) > target && len(out) > minKeep {
		out = out[1:]
	}
	return out
}

func assemble(systemPrompt string, msgs []types.Message) []types.Message {
	if systemPrompt == "" {
		return msgs
	}
	sys := types.Message{
		ID:      "system_prompt",
		Role:    types.MessageRoleSystem,
		Content: systemPrompt,
	}
	return append([]types.Message{sys}, msgs...)
}

// ShouldCompress returns true if compression should run.
func (p *Pipeline) ShouldCompress(msgs []types.Message, budget types.TokenBudget) bool {
	if p.maxMessages > 0 && len(msgs) > p.maxMessages {
		return true
	}
	return p.counter.CountMessages(msgs) > budget.CompressionTarget
}

// CountMessages exposes token counting.
func (p *Pipeline) CountMessages(msgs []types.Message) int {
	return p.counter.CountMessages(msgs)
}

func microcompact(msgs []types.Message, _ types.TokenBudget) ([]types.Message, bool) {
	if len(msgs) < 2 {
		return msgs, false
	}
	var out []types.Message
	changed := false
	for i := 0; i < len(msgs); i++ {
		if i+1 < len(msgs) && msgs[i].Role == msgs[i+1].Role {
			merged := msgs[i]
			merged.Content = msgs[i].Content + "\n---\n" + msgs[i+1].Content
			for i+2 < len(msgs) && msgs[i+2].Role == merged.Role {
				i++
				merged.Content += "\n---\n" + msgs[i].Content
			}
			out = append(out, merged)
			changed = true
			i++
			continue
		}
		out = append(out, msgs[i])
	}
	return out, changed
}

func collapse(msgs []types.Message, _ types.TokenBudget) ([]types.Message, bool) {
	const minLen = 20
	if len(msgs) < 3 {
		return msgs, false
	}
	var out []types.Message
	changed := false
	for i := 0; i < len(msgs); i++ {
		runStart := i
		for i+1 < len(msgs) && len(msgs[i].Content) < minLen && len(msgs[i+1].Content) < minLen {
			i++
		}
		if i-runStart >= 2 {
			out = append(out, msgs[runStart])
			folded := types.Message{
				ID:        msgs[runStart].ID + "_fold",
				SessionID: msgs[runStart].SessionID,
				Role:      msgs[runStart].Role,
				Content:   fmt.Sprintf("[折叠 %d 条消息]", i-runStart),
				Timestamp: msgs[i].Timestamp,
			}
			out = append(out, folded)
			out = append(out, msgs[i])
			changed = true
			continue
		}
		for j := runStart; j <= i; j++ {
			out = append(out, msgs[j])
		}
	}
	return out, changed
}
