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
	TotalTokens         int
	LayerTokens         [4]int
	MemoryTruncated     bool
	BlocksIncluded      []string
	TemplateHash        string
	AgentsMDHash        string
	SectionCount        int
	HasDynamicBoundary  bool
	DynamicSectionNames []string
}

// SystemPromptAssembler builds the final system prompt per §十 spec.
// Supports both legacy mode and new section-based prompt system.
//
// DM-20260629-002 devrix-d2-dsaft-restructuring PR-2: Build (was 55 LOC)
// split into:
//   - assembler.go (this file: orchestrator + report + NewSystemPromptAssembler)
//   - layers.go (buildCoreLayer, buildLayer3Blocks, buildLoadedContext,
//     buildXMLContext, truncateToTokenBudget)
//   - dynamic_sections.go (in pkg prompt: buildDynamicSections, wantsDynamicSection,
//     resolveGitStatusSection, buildEnvInfo, coreTemplate, guidanceTemplate)
//
// The orchestrator Build is now ~55 LOC of flow control only.
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
//
// DM-20260629-002 PR-2: orchestrator-only after the god-fn split. Each layer's
// body lives in layers.go; dynamic-boundary logic in dynamic_sections.go.
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

	// S4-Gate H-2 fix: drainTaskNotifications must be called on every Build,
	// because the underlying bus.Drain is one-shot. Putting it in
	// buildSessionContext would mean dynamic_sections cache hits skip the
	// drain. So we drain here at the top, then splice.
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

// buildSessionContext formats the per-session runtime context block (agent
// name, date, OS, workdir, session id, request id, model).
//
// DM-20260629-002 PR-2: kept here because the H-2 fix comment is more
// discoverable when paired with the drainTaskNotifications comment.
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
	// S4-Gate H-2 fix: drain moved to Build top-level — here we only format.
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

// joinNonEmptyParts concatenates non-empty parts with double newlines.
//
// DM-20260629-002 PR-2: kept in assembler.go because both static and dynamic
// paths in Build need it (and it's 9 LOC).
func joinNonEmptyParts(parts ...string) string {
	nonEmpty := make([]string, 0, len(parts))
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			nonEmpty = append(nonEmpty, p)
		}
	}
	return strings.Join(nonEmpty, "\n\n")
}

// estimateTokens uses a 4-chars-per-token heuristic for the system prompt
// layer accounting.
//
// DM-20260629-002 PR-2: kept in assembler.go because both Build and
// buildLayer3Blocks use it.
func estimateTokens(s string) int {
	n := len(s) / 4
	if n <= 0 && s != "" {
		return 1
	}
	return n
}

// templateFingerprint hashes the active templates + embed flag so report
// consumers can detect prompt-version drift.
//
// DM-20260629-002 PR-2: kept in assembler.go (Build calls it inline).
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

// contentHash hashes non-blank content (used for AGENTS.md provenance).
//
// DM-20260629-002 PR-2: kept in assembler.go.
func contentHash(s string) string {
	if strings.TrimSpace(s) == "" {
		return ""
	}
	h := sha256.Sum256([]byte(s))
	return shortHash(h[:])
}

// shortHash returns the first 12 hex chars of a digest (compact form).
//
// DM-20260629-002 PR-2: kept in assembler.go.
func shortHash(sum []byte) string {
	return hex.EncodeToString(sum)[:12]
}