package compression

import (
	"context"
	"fmt"
	"strings"

	"github.com/devrix/devrix/internal/layers/contextengine/token"
	"github.com/devrix/devrix/internal/shared/errors"
	"github.com/devrix/devrix/internal/shared/types"
)

const (
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
	counter *token.Counter
	enabled bool
}

// NewPipeline creates a compression pipeline.
func NewPipeline(enabled bool) *Pipeline {
	return &Pipeline{counter: token.NewCounter(), enabled: enabled}
}

// Run compresses messages to fit within budget.
func (p *Pipeline) Run(ctx context.Context, msgs []types.Message, systemPrompt string, budget types.TokenBudget) ([]types.Message, types.CompressionReport, error) {
	_ = ctx
	report := types.CompressionReport{OriginalTokens: p.counter.CountMessages(msgs)}
	if !p.enabled {
		out := assemble(systemPrompt, msgs)
		report.CompressedTokens = p.counter.CountMessages(out)
		return out, report, nil
	}

	current := append([]types.Message(nil), msgs...)

	type namedStep struct {
		name string
		fn   func([]types.Message, types.TokenBudget) ([]types.Message, bool)
	}
	steps := []namedStep{
		{stepToolResultBudget, func(m []types.Message, b types.TokenBudget) ([]types.Message, bool) {
			before := p.counter.CountMessages(m)
			next := toolResultBudget(p.counter, m, b.ToolResultBudget)
			return next, before != p.counter.CountMessages(next)
		}},
		{stepSnip, func(m []types.Message, b types.TokenBudget) ([]types.Message, bool) {
			before := p.counter.CountMessages(m)
			next := snip(p.counter, m, b.CompressionTarget)
			return next, before != p.counter.CountMessages(next)
		}},
		{stepMicrocompact, microcompact},
		{stepCollapse, collapse},
	}

	for _, step := range steps {
		next, applied := step.fn(current, budget)
		if applied {
			report.StepsApplied = append(report.StepsApplied, step.name)
			report.Truncated = true
		}
		current = next
	}

	// Step 6: autocompact skipped in V1
	report.StepsApplied = append(report.StepsApplied, stepAutocompact+":skipped")

	current = assemble(systemPrompt, current)
	report.CompressedTokens = p.counter.CountMessages(current)

	if p.counter.CountMessages(current) > budget.MaxContextTokens-budget.ReservedOutput {
		report.StepsApplied = append(report.StepsApplied, stepTokenBlock)
		return nil, report, errors.NewContextExceededError()
	}
	return current, report, nil
}

func toolResultBudget(counter *token.Counter, msgs []types.Message, maxPerResult int) []types.Message {
	out := make([]types.Message, len(msgs))
	for i, m := range msgs {
		out[i] = m
		if m.Role == types.MessageRoleTool && counter.Count(m.Content) > maxPerResult {
			out[i].Content = counter.TruncateToTokens(m.Content, maxPerResult) + "\n...[truncated]"
		}
	}
	return out
}

func snip(counter *token.Counter, msgs []types.Message, target int) []types.Message {
	const minKeep = 4
	if len(msgs) <= minKeep {
		return msgs
	}
	current := append([]types.Message(nil), msgs...)
	for counter.CountMessages(current) > target && len(current) > minKeep {
		current = current[1:]
	}
	return current
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
		out = append(out, msgs[i])
	}
	return out, changed
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
	return p.counter.CountMessages(msgs) > budget.CompressionTarget
}

// CountMessages exposes token counting.
func (p *Pipeline) CountMessages(msgs []types.Message) int {
	return p.counter.CountMessages(msgs)
}

// AutocompactStub logs skip for V1.
func AutocompactStub() string {
	return stepAutocompact + ":skipped"
}

// TruncationMarker is appended to truncated tool results.
func TruncationMarker() string {
	return strings.TrimSpace("...[truncated]")
}
