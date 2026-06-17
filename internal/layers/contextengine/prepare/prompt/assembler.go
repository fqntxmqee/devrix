package prompt

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/devrix/devrix/internal/layers/contextengine/prepare/memory"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel/notify"
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
	WorkDir              string
	Session              *types.Session
	Runtime              ProcessRuntimeContext
	AgentsRaw            string
	MemoryEntries        []memory.MemoryEntry
	OmitAgentsFromSystem bool
	RecallMaxTokens      int
}

// SystemPromptBuildReport describes assembly observability metadata.
type SystemPromptBuildReport struct {
	TotalTokens        int
	LayerTokens        [4]int
	MemoryTruncated    bool
	BlocksIncluded     []string
	TemplateHash       string
	AgentsMDHash       string
	SectionCount       int
	HasDynamicBoundary bool
	DynamicSectionNames []string
}

// SystemPromptAssembler builds the final system prompt per §十 spec.
// Supports both legacy mode and new section-based prompt system.
type SystemPromptAssembler struct {
	coreTemplate     string
	guidanceTemplate string
	cfg              config.WorkspacePromptConfig
	promptLoader     *Loader
}

// NewSystemPromptAssembler creates an assembler from workspace config.
func NewSystemPromptAssembler(cfg config.WorkspacePromptConfig) *SystemPromptAssembler {
	core := defaultCoreTemplate
	guidance := defaultGuidanceTemplate

	// Create prompt loader for section-based prompts
	var loader *Loader
	if cfg.PromptConfig != nil && cfg.PromptConfig.UseSections {
		loader = NewLoader(nil)
	}

	if !cfg.EmbedCoreTemplate {
		core = "你是 Devrix，多智能体开发助手。"
	}
	return &SystemPromptAssembler{
		coreTemplate:     core,
		guidanceTemplate: guidance,
		cfg:              cfg,
		promptLoader:     loader,
	}
}

// Build assembles the four-layer system prompt (ClawCode-aligned: core always
// in system; AGENTS.md via agents_context only when not omitted for prepend).
func (a *SystemPromptAssembler) Build(in SystemPromptBuildInput) (string, SystemPromptBuildReport) {
	report := SystemPromptBuildReport{
		TemplateHash: a.templateFingerprint(),
		AgentsMDHash: contentHash(in.AgentsRaw),
	}

	layer0, sectionCount := a.buildCoreLayer(in)
	report.LayerTokens[0] = estimateTokens(layer0)
	report.SectionCount = sectionCount

	layer2 := strings.TrimSpace(a.guidanceTemplate)
	report.LayerTokens[2] = estimateTokens(layer2)

	blocks, blockReport := a.buildLayer3Blocks(in)
	report.MemoryTruncated = blockReport.MemoryTruncated
	report.BlocksIncluded = blockReport.BlocksIncluded

	layer3 := ""
	if layer3HasContent(blocks) {
		loaded := buildLoadedContext(blocks)
		layer3Header := "## Workspace Files (Injected)\nThe following <loaded_context> was loaded from workspace.\n\n"
		layer3 = layer3Header + loaded
		report.LayerTokens[3] = estimateTokens(layer3)
	}

	sessionID := in.Runtime.SessionID
	if in.Session != nil && sessionID == "" {
		sessionID = in.Session.SessionID
	}

	if a.enableDynamicBoundary() {
		report.HasDynamicBoundary = true
		staticParts := joinNonEmptyParts(layer0, layer2)
		dynamicParts, dynNames := a.buildDynamicSections(sessionID, in, layer3)
		report.DynamicSectionNames = dynNames
		report.LayerTokens[1] = estimateTokens(strings.Join(dynamicParts, "\n\n"))

		out := joinNonEmptyParts(staticParts, DynamicBoundary, strings.Join(dynamicParts, "\n\n"))
		report.TotalTokens = estimateTokens(out)
		return out, report
	}

	layer1 := a.buildSessionContext(in)
	report.LayerTokens[1] = estimateTokens(layer1)

	out := joinNonEmptyParts(layer0, layer1, layer2, layer3)
	report.TotalTokens = estimateTokens(out)
	return out, report
}

func (a *SystemPromptAssembler) enableDynamicBoundary() bool {
	return a.cfg.PromptConfig != nil && a.cfg.PromptConfig.EnableDynamicBoundary
}

func (a *SystemPromptAssembler) wantsDynamicSection(name string) bool {
	if a.cfg.PromptConfig == nil {
		return false
	}
	for _, n := range a.cfg.PromptConfig.DynamicSections {
		if n == name {
			return true
		}
	}
	return false
}

func (a *SystemPromptAssembler) buildDynamicSections(sessionID string, in SystemPromptBuildInput, layer3 string) ([]string, []string) {
	var parts []string
	var names []string

	sessionCtx := resolveCachedSection(sessionID, "session_context", false, func() string {
		return a.buildSessionContext(in)
	})
	if strings.TrimSpace(sessionCtx) != "" {
		parts = append(parts, sessionCtx)
		names = append(names, "session_context")
	}

	if a.wantsDynamicSection("git_status") {
		workDir := in.WorkDir
		if in.Session != nil && in.Session.WorkDir != "" {
			workDir = in.Session.WorkDir
		}
		if gitCtx, ok := resolveGitStatusSection(sessionID, workDir); ok {
			parts = append(parts, gitCtx)
			names = append(names, "git_status")
		}
	}

	if a.wantsDynamicSection("env_info") {
		if env := a.buildEnvInfo(in); env != "" {
			parts = append(parts, env)
			names = append(names, "env_info")
		}
	}

	if strings.TrimSpace(layer3) != "" {
		parts = append(parts, layer3)
		names = append(names, "loaded_context")
	}

	return parts, names
}

func resolveGitStatusSection(sessionID, workDir string) (string, bool) {
	raw := resolveCachedSection(sessionID, "git_status", false, func() string {
		s, ok := computeGitStatus(workDir)
		if !ok {
			return ""
		}
		return s
	})
	if strings.TrimSpace(raw) == "" {
		return "", false
	}
	return raw, true
}

func (a *SystemPromptAssembler) buildEnvInfo(in SystemPromptBuildInput) string {
	workDir := in.WorkDir
	model := ""
	if in.Session != nil {
		if in.Session.WorkDir != "" {
			workDir = in.Session.WorkDir
		}
		model = in.Session.Model
	}
	if workDir == "" && model == "" {
		return ""
	}
	return fmt.Sprintf(`## Environment
Workspace directory: %s
Model: %s
`, workDir, model)
}

func joinNonEmptyParts(parts ...string) string {
	nonEmpty := make([]string, 0, len(parts))
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			nonEmpty = append(nonEmpty, p)
		}
	}
	return strings.Join(nonEmpty, "\n\n")
}

// buildCoreLayer builds layer 0 from embedded sections or template.
// Workspace AGENTS.md is not Layer 0 (ClawCode: CLAUDE.md → user prepend / Layer 3).
func (a *SystemPromptAssembler) buildCoreLayer(in SystemPromptBuildInput) (string, int) {
	if a.promptLoader != nil && a.cfg.PromptConfig != nil {
		names := a.cfg.PromptConfig.GetStaticSections()
		sections := a.promptLoader.LoadStaticSections(names)
		if len(sections) > 0 {
			return strings.Join(sections, "\n\n"), len(sections)
		}
	}
	return a.coreTemplate, 1
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
`, agentName, time.Now().Format("Monday Jan 2, 2006"), runtime.GOOS, workDir, sessionID, requestID, model) + drainTaskNotifications(sessionID)
}

// drainTaskNotifications (D4-S12-A03 / G3) 在 prepare 阶段把 session 累积的
// 后台任务完成事件 drain 出来, 附加到 system reminder 段。bus 为 nil / 无事件时
// 返回空字符串。bus.Drain 一次性消费 + 清空 pending, 不会重复注入。
func drainTaskNotifications(sessionID string) string {
	if sessionID == "" {
		return ""
	}
	bus := notify.GlobalBus()
	if bus == nil {
		return ""
	}
	events := bus.Drain(sessionID)
	return notify.FormatReminder(events)
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

	memoryBudget := in.RecallMaxTokens
	if memoryBudget <= 0 {
		memoryBudget = 2000
	}
	memoryRaw, truncated := memory.FormatMemoryContext(in.MemoryEntries, memoryBudget)
	report.MemoryTruncated = truncated

	agentsBudget := budget
	agentsRaw := truncateToTokenBudget(strings.TrimSpace(in.AgentsRaw), agentsBudget)
	if in.OmitAgentsFromSystem {
		agentsRaw = ""
	}
	budget -= estimateTokens(agentsRaw)
	if budget < 0 {
		budget = 0
	}
	memoryRaw = truncateToTokenBudget(memoryRaw, budget)

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

func layer3HasContent(blocks map[string]string) bool {
	for _, v := range blocks {
		if strings.TrimSpace(v) != "" {
			return true
		}
	}
	return false
}

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

func buildXMLContext(tag, content string) string {
	if strings.TrimSpace(content) == "" {
		return fmt.Sprintf("<%s></%s>\n", tag, tag)
	}
	return fmt.Sprintf("<%s>\n%s\n</%s>\n", tag, strings.TrimSpace(content), tag)
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

func (a *SystemPromptAssembler) templateFingerprint() string {
	if a == nil {
		return ""
	}
	h := sha256.New()
	h.Write([]byte(a.coreTemplate))
	h.Write([]byte{0})
	h.Write([]byte(a.guidanceTemplate))
	h.Write([]byte{0})
	h.Write([]byte(fmt.Sprintf("%t", a.cfg.EmbedCoreTemplate)))
	return shortHash(h.Sum(nil))
}

func contentHash(s string) string {
	if strings.TrimSpace(s) == "" {
		return ""
	}
	h := sha256.Sum256([]byte(s))
	return shortHash(h[:])
}

func shortHash(sum []byte) string {
	return hex.EncodeToString(sum)[:12]
}
