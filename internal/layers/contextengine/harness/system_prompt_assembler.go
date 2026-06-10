package harness

import (
	_ "embed"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/devrix/devrix/internal/layers/contextengine/memory"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

//go:embed templates/devrix_core.zh.md
var defaultCoreTemplate string

//go:embed templates/workspace_guidance.zh.md
var defaultGuidanceTemplate string

// ProcessRuntimeContext carries per-process runtime metadata (not persisted).
type ProcessRuntimeContext struct {
	SessionID string
	UserID    string
	RequestID string
	Extra     map[string]string
}

// SystemPromptBuildInput holds inputs for system prompt assembly.
type SystemPromptBuildInput struct {
	WorkDir        string
	Session        *types.Session
	Runtime        ProcessRuntimeContext
	AgentsRaw      string
	MemoryEntries  []memory.MemoryEntry
	Bootstrap      *types.BootstrapReport
	Workspace      *types.WorkspaceContext
	Routing        *types.RoutingHint
	Preflight      *types.PreflightResult
	HarnessEnabled bool
}

// SystemPromptBuildReport describes assembly observability metadata.
type SystemPromptBuildReport struct {
	TotalTokens     int
	LayerTokens     [4]int
	MemoryTruncated bool
	BlocksIncluded  []string
}

// SystemPromptAssembler builds the final system prompt per §十 spec.
type SystemPromptAssembler struct {
	coreTemplate     string
	guidanceTemplate string
	cfg              config.WorkspacePromptConfig
}

// NewSystemPromptAssembler creates an assembler from workspace config.
func NewSystemPromptAssembler(cfg config.WorkspacePromptConfig) *SystemPromptAssembler {
	core := defaultCoreTemplate
	guidance := defaultGuidanceTemplate
	if !cfg.EmbedCoreTemplate {
		core = "You are Devrix, a multi-agent development assistant."
	}
	return &SystemPromptAssembler{
		coreTemplate:     core,
		guidanceTemplate: guidance,
		cfg:              cfg,
	}
}

// Build assembles the four-layer system prompt.
func (a *SystemPromptAssembler) Build(in SystemPromptBuildInput) (string, SystemPromptBuildReport) {
	if !in.HarnessEnabled {
		appendix := memory.FormatLongTermAppendix(in.MemoryEntries, 0)
		legacy := a.BuildLegacy(in.AgentsRaw, appendix)
		return legacy, SystemPromptBuildReport{TotalTokens: estimateTokens(legacy)}
	}

	report := SystemPromptBuildReport{}
	layer0 := strings.TrimSpace(a.coreTemplate)
	report.LayerTokens[0] = estimateTokens(layer0)

	layer1 := a.buildSessionContext(in)
	report.LayerTokens[1] = estimateTokens(layer1)

	layer2 := strings.TrimSpace(a.guidanceTemplate)
	report.LayerTokens[2] = estimateTokens(layer2)

	blocks, blockReport := a.buildLayer3Blocks(in)
	report.MemoryTruncated = blockReport.MemoryTruncated
	report.BlocksIncluded = blockReport.BlocksIncluded

	loaded := buildLoadedContext(blocks)
	layer3Header := "## Workspace Files (Injected)\nThe following <loaded_context> was loaded from workspace and harness runtime.\n\n"
	layer3 := layer3Header + loaded
	report.LayerTokens[3] = estimateTokens(layer3)

	parts := make([]string, 0, 4)
	for _, p := range []string{layer0, layer1, layer2, layer3} {
		if strings.TrimSpace(p) != "" {
			parts = append(parts, p)
		}
	}
	out := strings.Join(parts, "\n\n")
	report.TotalTokens = estimateTokens(out)
	return out, report
}

// BuildLegacy returns the V4 system prompt (agents + longterm appendix).
func (a *SystemPromptAssembler) BuildLegacy(agentsRaw string, memoryAppendix string) string {
	out := strings.TrimSpace(agentsRaw)
	if memoryAppendix != "" {
		if out != "" {
			out += memoryAppendix
		} else {
			out = strings.TrimPrefix(memoryAppendix, "\n\n")
		}
	}
	return out
}

func (a *SystemPromptAssembler) buildSessionContext(in SystemPromptBuildInput) string {
	agentName := a.cfg.AgentName
	if agentName == "" {
		agentName = "Devrix"
	}
	workDir := in.WorkDir
	if in.Session != nil && in.Session.WorkDir != "" {
		workDir = in.Session.WorkDir
	}
	sessionID := in.Runtime.SessionID
	requestID := in.Runtime.RequestID
	model := ""
	if in.Session != nil {
		if sessionID == "" {
			sessionID = in.Session.SessionID
		}
		if requestID == "" {
			requestID = in.Session.RequestID
		}
		model = in.Session.Model
	}
	return fmt.Sprintf(`## Session Context
Agent: %s
Today's date: %s
Operating system: %s
Workspace directory: %s
Session ID: %s
Request ID: %s
Model: %s
`, agentName, time.Now().Format("Monday Jan 2, 2006"), runtime.GOOS, workDir, sessionID, requestID, model)
}

type layer3BlockReport struct {
	MemoryTruncated bool
	BlocksIncluded  []string
}

func (a *SystemPromptAssembler) buildLayer3Blocks(in SystemPromptBuildInput) (map[string]string, layer3BlockReport) {
	report := layer3BlockReport{}
	blocks := make(map[string]string)

	budget := a.cfg.MaxContextTokens
	if budget <= 0 {
		budget = 8000
	}

	memoryRaw, truncated := memory.FormatMemoryContext(in.MemoryEntries, 0)
	report.MemoryTruncated = truncated

	harnessInit := ""
	if in.Bootstrap != nil {
		harnessInit = formatHarnessInit(*in.Bootstrap)
	}
	workspaceSnap := ""
	if in.Workspace != nil {
		workspaceSnap = formatWorkspaceSnapshot(*in.Workspace)
	}
	routing := formatRoutingHints(in.Routing)
	preflight := formatPreflightWarnings(in.Preflight)

	fixedTokens := estimateTokens(harnessInit) + estimateTokens(workspaceSnap) +
		estimateTokens(routing) + estimateTokens(preflight)
	agentsBudget := budget - fixedTokens
	if agentsBudget < 0 {
		agentsBudget = 0
	}
	agentsRaw := truncateToTokenBudget(strings.TrimSpace(in.AgentsRaw), agentsBudget)
	budget -= fixedTokens + estimateTokens(agentsRaw)
	if budget < 0 {
		budget = 0
	}
	memoryRaw = truncateToTokenBudget(memoryRaw, budget)

	blocks["agents_context"] = agentsRaw
	blocks["memory_context"] = memoryRaw
	if harnessInit != "" {
		blocks["harness_init"] = harnessInit
	}
	if workspaceSnap != "" {
		blocks["workspace_snapshot"] = workspaceSnap
	}
	if routing != "" {
		blocks["routing_hints"] = routing
	}
	if preflight != "" {
		blocks["preflight_warnings"] = preflight
	}

	for tag := range blocks {
		report.BlocksIncluded = append(report.BlocksIncluded, tag)
	}
	return blocks, report
}

func buildLoadedContext(blocks map[string]string) string {
	order := []string{
		"agents_context", "memory_context", "harness_init",
		"workspace_snapshot", "routing_hints", "preflight_warnings",
	}
	var b strings.Builder
	b.WriteString("<loaded_context>\n")
	for _, tag := range order {
		content, ok := blocks[tag]
		if !ok {
			continue
		}
		if tag == "routing_hints" && strings.TrimSpace(content) == "" {
			continue
		}
		if tag == "preflight_warnings" && strings.TrimSpace(content) == "" {
			continue
		}
		b.WriteString(buildXMLContext(tag, content))
	}
	b.WriteString("</loaded_context>\n")
	return b.String()
}

func buildXMLContext(tag, content string) string {
	if strings.TrimSpace(content) == "" {
		return fmt.Sprintf("<%s></%s>\n", tag, tag)
	}
	return fmt.Sprintf("<%s>\n%s\n</%s>\n", tag, strings.TrimSpace(content), tag)
}

func formatHarnessInit(report types.BootstrapReport) string {
	toolNames := make([]string, 0, len(report.VisibleToolList))
	for _, t := range report.VisibleToolList {
		toolNames = append(toolNames, t.Name)
	}
	stages := make([]string, 0, len(report.StagesApplied))
	for _, s := range report.StagesApplied {
		stages = append(stages, string(s))
	}
	return fmt.Sprintf(`Trusted: %t
Visible tools: %s (filtered from %d)
Deferred init: plugin=%t skill=%t mcp=%t hooks=%t
Bootstrap stages: %s`,
		report.Trusted,
		strings.Join(toolNames, ", "),
		report.ToolCount,
		report.DeferredInit.PluginInit,
		report.DeferredInit.SkillInit,
		report.DeferredInit.MCPPrefetch,
		report.DeferredInit.SessionHooks,
		strings.Join(stages, ", "),
	)
}

func formatWorkspaceSnapshot(ws types.WorkspaceContext) string {
	return fmt.Sprintf(`WorkDir: %s
AGENTS.md present: %t
Go source files: %d
Go test files: %d
Scanned at: %s`,
		ws.WorkDir, ws.AgentsMDPresent, ws.GoFileCount, ws.TestFileCount, ws.ScannedAt.Format(time.RFC3339))
}

func formatRoutingHints(hint *types.RoutingHint) string {
	if hint == nil || len(hint.Tools) == 0 {
		return ""
	}
	return fmt.Sprintf(`<routing_hints advisory="true">
Matched tools: %s
Note: These hints are advisory only; you may still choose other tools.
</routing_hints>`, strings.Join(hint.Tools, ", "))
}

func formatPreflightWarnings(result *types.PreflightResult) string {
	if result == nil || len(result.Warnings) == 0 {
		return ""
	}
	var b strings.Builder
	for _, w := range result.Warnings {
		b.WriteString("- ")
		b.WriteString(w)
		b.WriteString("\n")
	}
	return b.String()
}

func estimateTokens(s string) int {
	n := len(s) / 4
	if n <= 0 && s != "" {
		return 1
	}
	return n
}

func truncateToTokenBudget(text string, maxTokens int) string {
	if maxTokens <= 0 {
		return ""
	}
	maxChars := maxTokens * 4
	if len(text) <= maxChars {
		return text
	}
	return text[:maxChars] + memoryTruncationNoticeZH
}

const memoryTruncationNoticeZH = "\n... (记忆已截断 — 更多内容请依赖 LongTerm recall 或项目文档) ..."
