// AssemblerAdapter: D2-S15-A04 AssemblePrompt — wraps
// prompt.SystemPromptAssembler with span + template-hash hooks.
//
// Matches prepare.PromptAssembler:
//
//	Build(input prompt.SystemPromptBuildInput) (string, prompt.SystemPromptBuildReport)
//
// Replaces facade/engine.go::runProcess prompt-build block.
package adapters

import (
	"context"
	"fmt"

	"github.com/devrix/devrix/internal/layers/contextengine/prepare/prompt"
	"github.com/devrix/devrix/internal/layers/observability/instrument/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
	"github.com/devrix/devrix/internal/shared/buildinfo"
)

// AssemblerAdapter implements prepare.PromptAssembler.
type AssemblerAdapter struct {
	assembler *prompt.SystemPromptAssembler
	hooks     Hooks
}

// NewAssemblerAdapter constructs a PromptAssembler over a SystemPromptAssembler.
func NewAssemblerAdapter(assembler *prompt.SystemPromptAssembler, opts ...HooksOption) *AssemblerAdapter {
	return &AssemblerAdapter{assembler: assembler, hooks: applyHooks(opts)}
}

// Build assembles the system prompt, optionally emitting a span tagged with
// total_tokens + memory_truncated + template hashes.
func (a *AssemblerAdapter) Build(ctx context.Context, in prompt.SystemPromptBuildInput) (string, prompt.SystemPromptBuildReport) {
	ctx, span := a.hooks.startSpan(ctx, telemetry.OpD2_S5_Context_Harness_SystemPrompt_Build, tracer.SpanKindInternal)
	if span != nil {
		defer span.End()
	}

	promptText, report := a.assembler.Build(in)

	if span != nil {
		span.SetAttributes(
			tracer.Attribute{Key: "system_prompt.total_tokens", Value: fmt.Sprintf("%d", report.TotalTokens)},
			tracer.Attribute{Key: "system_prompt.memory_truncated", Value: boolStr(report.MemoryTruncated)},
		)
		span.SetAttributes(telemetry.GenAIPromptAttrs(
			buildinfo.Version,
			report.TemplateHash,
			report.AgentsMDHash,
		)...)
	}
	return promptText, report
}