package prompt

import (
	"fmt"
	"strings"

	"github.com/devrix/devrix/internal/layers/contextengine/i18n"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/memory"
)

// buildCoreLayer builds layer 0 from embedded sections or template.
// Workspace AGENTS.md is not Layer 0 (ClawCode: CLAUDE.md → user prepend / Layer 3).
//
// DM-20260629-002 devrix-d2-dsaft-restructuring PR-2: extracted from assembler.go
// Build into layers.go so the orchestrator only owns flow control.
func (a *SystemPromptAssembler) buildCoreLayer(in SystemPromptBuildInput) (string, int) {
	if a.promptLoader != nil && a.cfg.PromptConfig != nil {
		names := a.cfg.PromptConfig.GetStaticSections()
		sections := a.promptLoader.LoadStaticSections(names)
		if len(sections) > 0 {
			return strings.Join(sections, "\n\n"), len(sections)
		}
	}
	return a.coreTemplate(), 1
}

// buildLayer3Blocks assembles the layer-3 block map (agents_context +
// memory_context) honoring token budgets and OmitAgentsFromSystem.
//
// DM-20260629-002 PR-2: extracted from assembler.go.
func (a *SystemPromptAssembler) buildLayer3Blocks(in SystemPromptBuildInput) (map[string]string, layer3BlockReport) {
	report := layer3BlockReport{}
	blocks := make(map[string]string)

	budget := a.cfg.MaxContextTokens
	if budget <= 0 {
		budget = 8000
	}

	memoryBudget := in.RecallMaxTokens
	if memoryBudget <= 0 {
		memoryBudget = 2000
	}
	memoryRaw, truncated := memory.FormatMemoryContext(in.MemoryEntries, memoryBudget)
	report.MemoryTruncated = truncated

	agentsBudget := budget
	agentsRaw := a.truncateToTokenBudget(strings.TrimSpace(in.AgentsRaw), agentsBudget)
	if in.OmitAgentsFromSystem {
		agentsRaw = ""
	}
	budget -= estimateTokens(agentsRaw)
	if budget < 0 {
		budget = 0
	}
	memoryRaw = a.truncateToTokenBudget(memoryRaw, budget)

	if agentsRaw != "" {
		blocks["agents_context"] = agentsRaw
	}
	if memoryRaw != "" {
		blocks["memory_context"] = memoryRaw
	}

	for tag := range blocks {
		report.BlocksIncluded = append(report.BlocksIncluded, tag)
	}
	return blocks, report
}

// layer3BlockReport tracks which layer-3 blocks survived the token budget and
// whether memory had to be truncated.
type layer3BlockReport struct {
	MemoryTruncated bool
	BlocksIncluded  []string
}

// layer3HasContent returns true if any block has non-whitespace content.
//
// DM-20260629-002 PR-2: extracted from assembler.go.
func layer3HasContent(blocks map[string]string) bool {
	for _, v := range blocks {
		if strings.TrimSpace(v) != "" {
			return true
		}
	}
	return false
}

// buildLoadedContext serializes the layer-3 blocks into a <loaded_context> XML
// envelope in canonical order (agents_context first, then memory_context).
//
// DM-20260629-002 PR-2: extracted from assembler.go.
func buildLoadedContext(blocks map[string]string) string {
	order := []string{
		"agents_context", "memory_context",
	}
	var b strings.Builder
	b.WriteString("<loaded_context>\n")
	for _, tag := range order {
		content, ok := blocks[tag]
		if !ok {
			continue
		}
		b.WriteString(buildXMLContext(tag, content))
	}
	b.WriteString("</loaded_context>\n")
	return b.String()
}

// buildXMLContext emits an XML element wrapping the given content (or empty
// element when content is blank).
//
// DM-20260629-002 PR-2: extracted from assembler.go.
func buildXMLContext(tag, content string) string {
	if strings.TrimSpace(content) == "" {
		return fmt.Sprintf("<%s></%s>\n", tag, tag)
	}
	return fmt.Sprintf("<%s>\n%s\n</%s>\n", tag, strings.TrimSpace(content), tag)
}

// truncateToTokenBudget clips text to approximately maxTokens (using the
// 4-chars-per-token heuristic) and appends the locale-specific truncation
// notice.
//
// DM-20260629-002 PR-2: extracted from assembler.go.
func (a *SystemPromptAssembler) truncateToTokenBudget(text string, maxTokens int) string {
	if maxTokens <= 0 {
		return ""
	}
	maxChars := maxTokens * 4
	if len(text) <= maxChars {
		return text
	}
	return text[:maxChars] + i18n.MemoryTruncationNotice(a.locale)
}