package prompt

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/devrix/devrix/internal/layers/contextengine/i18n"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/memory"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

//go:embed templates/devrix_core.zh.md
var defaultCoreTemplateZH string

//go:embed templates/devrix_core.en.md
var defaultCoreTemplateEN string

//go:embed templates/workspace_guidance.zh.md
var defaultGuidanceTemplateZH string

//go:embed templates/workspace_guidance.en.md
var defaultGuidanceTemplateEN string

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
	coreTemplateZH     string
	coreTemplateEN     string
	guidanceTemplateZH string
	guidanceTemplateEN string
	cfg                config.WorkspacePromptConfig
	promptLoader       *Loader
	locale             i18n.Locale
}

// NewSystemPromptAssembler creates an assembler from workspace config.
func NewSystemPromptAssembler(cfg config.WorkspacePromptConfig) *SystemPromptAssembler {
	locale := i18n.ParseLanguage(cfg.Language)

	var loader *Loader
	if cfg.PromptConfig != nil && cfg.PromptConfig.UseSections {
		loader = NewLoader(nil, locale)
	}

	coreZH, coreEN := defaultCoreTemplateZH, defaultCoreTemplateEN
	guidanceZH, guidanceEN := defaultGuidanceTemplateZH, defaultGuidanceTemplateEN
	if !cfg.EmbedCoreTemplate {
		fallback := i18n.CoreTemplateFallback(locale)
		coreZH, coreEN = fallback, fallback
	}
	return &SystemPromptAssembler{
		coreTemplateZH:     coreZH,
		coreTemplateEN:     coreEN,
		guidanceTemplateZH: guidanceZH,
		guidanceTemplateEN: guidanceEN,
		cfg:                cfg,
		promptLoader:       loader,
		locale:             locale,
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

	layer2 := strings.TrimSpace(a.guidanceTemplate())
	report.LayerTokens[2] = estimateTokens(layer2)

	blocks, blockReport := a.buildLayer3Blocks(in)
	report.MemoryTruncated = blockReport.MemoryTruncated
	report.BlocksIncluded = blockReport.BlocksIncluded

	layer3 := ""
	if layer3HasContent(blocks) {
		loaded := buildLoadedContext(blocks)
		layer3Header := i18n.Layer3Header(a.locale)
		layer3 = layer3Header + loaded
		report.LayerTokens[3] = estimateTokens(layer3)
	}

	sessionID := in.Runtime.SessionID
	if in.Session != nil && sessionID == "" {
		sessionID = in.Session.SessionID
	}

	// S4-Gate H-2 fix: drainTaskNotifications 必须每次 Build 都调, 因为它是消费性
	// 语义 (bus.Drain 一次性清空 pending). 如果放进 buildSessionContext, 而
	// buildSessionContext 会被 dynamic_sections 的 cache 命中, 第二次 build
	// 就拿不到新 event 了. 这里作为 top-level "live" section 拼接, 不进 cache.
	taskNotif := drainTaskNotifications(sessionID)

	if a.enableDynamicBoundary() {
		report.HasDynamicBoundary = true
		staticParts := joinNonEmptyParts(layer0, layer2)
		dynamicParts, dynNames := a.buildDynamicSections(sessionID, in, layer3, taskNotif)
		report.DynamicSectionNames = dynNames
		report.LayerTokens[1] = estimateTokens(strings.Join(dynamicParts, "\n\n"))

		out := joinNonEmptyParts(staticParts, DynamicBoundary, strings.Join(dynamicParts, "\n\n"))
		report.TotalTokens = estimateTokens(out)
		return out, report
	}

	layer1 := a.buildSessionContext(in) + taskNotif
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

func (a *SystemPromptAssembler) buildDynamicSections(sessionID string, in SystemPromptBuildInput, layer3, taskNotif string) ([]string, []string) {
	var parts []string
	var names []string

	sessionCtx := resolveCachedSection(sessionID, "session_context", false, func() string {
		return a.buildSessionContext(in)
	})
	if strings.TrimSpace(sessionCtx) != "" {
		parts = append(parts, sessionCtx)
		names = append(names, "session_context")
	}

	// S4-Gate H-2 fix: task_notifications 是 live section, 不进 cache,
	// 每次 Build 都会拿到 bus 里的最新完成事件. taskNotif 由 Build 顶层
	// drain 后传入, 这里只负责拼接 / 跳过空段.
	if strings.TrimSpace(taskNotif) != "" {
		parts = append(parts, taskNotif)
		names = append(names, "task_notifications")
	}

	if a.wantsDynamicSection("git_status") {
		workDir := in.WorkDir
		if in.Session != nil && in.Session.WorkDir != "" {
			workDir = in.Session.WorkDir
		}
		if gitCtx, ok := resolveGitStatusSection(sessionID, workDir, a.locale); ok {
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

func resolveGitStatusSection(sessionID, workDir string, loc i18n.Locale) (string, bool) {
	raw := resolveCachedSection(sessionID, "git_status", false, func() string {
		s, ok := computeGitStatus(workDir, loc)
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
	return i18n.EnvInfoHeader(a.locale) + "\n" + i18n.EnvInfoBody(a.locale, workDir, model)
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
	return a.coreTemplate(), 1
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
	// S4-Gate H-2 fix: 不在这里 drain — 移到 Build 顶层, 避免被 dynamic_sections
	// cache 命中导致第二次 build 拿不到新 event.
	now := time.Now()
	return i18n.SessionContextHeader(a.locale) + "\n" + i18n.SessionContextBody(
		a.locale, agentName, i18n.FormatSessionDate(a.locale, now), runtime.GOOS,
		workDir, sessionID, requestID, model,
	)
}

// drainTaskNotifications (D4-S12-A03 / G3) 在 prepare 阶段把 session 累积的
// 后台任务完成事件 drain 出来, 附加到 system reminder 段。bus 为 nil / 无事件时
// 返回空字符串。bus.Drain 一次性消费 + 清空 pending, 不会重复注入。
//
// S4-Gate H-3 fix: D2 Thin — prompt 包不 import orchestration/workmodel/notify,
// 用 function-based DI: 注入点 SetTaskNotifDrainer, 真实实现 (调用 bus.Drain +
// FormatReminder, bus 由 bootstrap 创建并传入) 放在 bootstrap 层.
// 默认实现返回空字符串, 单元测试不依赖 D7.
func drainTaskNotifications(sessionID string) string {
	if sessionID == "" {
		return ""
	}
	fn := getTaskNotifDrainer()
	if fn == nil {
		return ""
	}
	return fn(sessionID)
}

// TaskNotifDrainerFunc drain 出 session 累积的 task notification 文本.
// 返回 "" 表示无 event 可注入. 导出供 bootstrap / 测试注入.
type TaskNotifDrainerFunc func(sessionID string) string

var globalTaskNotifDrainer TaskNotifDrainerFunc

func getTaskNotifDrainer() TaskNotifDrainerFunc { return globalTaskNotifDrainer }

// SetTaskNotifDrainer 注入真实的 task notification drainer (通常在 bootstrap
// 阶段调用). nil 参数会保留原值, 避免误清空.
func SetTaskNotifDrainer(f TaskNotifDrainerFunc) {
	if f != nil {
		globalTaskNotifDrainer = f
	}
}

// SetTaskNotifDrainerForTest 同 SetTaskNotifDrainer, 但返回之前的函数, 便于
// t.Cleanup 恢复. nil 参数保留原值, 同 SetTaskNotifDrainer 语义.
func SetTaskNotifDrainerForTest(f TaskNotifDrainerFunc) TaskNotifDrainerFunc {
	prev := globalTaskNotifDrainer
	if f != nil {
		globalTaskNotifDrainer = f
	}
	return prev
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

func (a *SystemPromptAssembler) coreTemplate() string {
	if a.locale == i18n.LocaleEN {
		return a.coreTemplateEN
	}
	return a.coreTemplateZH
}

func (a *SystemPromptAssembler) guidanceTemplate() string {
	if a.locale == i18n.LocaleEN {
		return a.guidanceTemplateEN
	}
	return a.guidanceTemplateZH
}

func (a *SystemPromptAssembler) templateFingerprint() string {
	if a == nil {
		return ""
	}
	h := sha256.New()
	h.Write([]byte(a.coreTemplate()))
	h.Write([]byte{0})
	h.Write([]byte(a.guidanceTemplate()))
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
