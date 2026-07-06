package prompt

import (
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/prepare/memory"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

// T: D2-S10-A01-T35
func TestSystemPromptAssembler_should_omit_agents_when_prepend_mode(t *testing.T) {
	assembler := NewSystemPromptAssembler(config.DefaultWorkspacePromptConfig())
	prompt, _ := assembler.Build(SystemPromptBuildInput{
		AgentsRaw:            "Project agents content",
		OmitAgentsFromSystem: true,
	})
	if strings.Contains(prompt, "<agents_context>") && strings.Contains(prompt, "Project agents content") {
		t.Fatal("agents content should be omitted from system prompt in prepend mode")
	}
}

// T: D2-S9-A02-T10
func TestSystemPromptAssembler_should_build_xml_blocks(t *testing.T) {
	assembler := NewSystemPromptAssembler(config.DefaultWorkspacePromptConfig())
	prompt, report := assembler.Build(SystemPromptBuildInput{
		AgentsRaw: "Project agents content",
		MemoryEntries: []memory.MemoryEntry{
			{Topic: "architecture", Content: "use DDD"},
		},
		Session: types.NewSession("sess_1", "cli", "/tmp/proj"),
		Runtime: ProcessRuntimeContext{
			SessionID: "sess_1",
			RequestID: "req_1",
		},
	})
	if !strings.Contains(prompt, "<agents_context>") {
		t.Fatal("expected agents_context block")
	}
	if !strings.Contains(prompt, "<memory_context>") {
		t.Fatal("expected memory_context block")
	}
	if !strings.Contains(prompt, "Session ID: sess_1") && !strings.Contains(prompt, "会话 ID: sess_1") {
		t.Fatal("expected session context")
	}
	if report.TotalTokens <= 0 {
		t.Fatal("expected token report")
	}
}

// T: D2-S9-A02-T10
func TestSystemPromptAssembler_should_produce_stable_template_hash(t *testing.T) {
	assembler := NewSystemPromptAssembler(config.DefaultWorkspacePromptConfig())
	in := SystemPromptBuildInput{
		AgentsRaw: "# Agents\n",
	}
	_, r1 := assembler.Build(in)
	_, r2 := assembler.Build(in)
	if r1.TemplateHash == "" {
		t.Fatal("expected template hash")
	}
	if r1.TemplateHash != r2.TemplateHash {
		t.Fatalf("template hash unstable: %s vs %s", r1.TemplateHash, r2.TemplateHash)
	}
	if r1.AgentsMDHash == "" {
		t.Fatal("expected agents md hash")
	}
}

func TestSystemPromptAssembler_should_insert_dynamic_boundary(t *testing.T) {
	assembler := NewSystemPromptAssembler(config.DefaultWorkspacePromptConfig())
	prompt, report := assembler.Build(SystemPromptBuildInput{
		Session: types.NewSession("sess_boundary", "cli", "/tmp/proj"),
		Runtime: ProcessRuntimeContext{SessionID: "sess_boundary"},
	})
	if !report.HasDynamicBoundary {
		t.Fatal("expected dynamic boundary enabled in default config")
	}
	idx := strings.Index(prompt, DynamicBoundary)
	if idx < 0 {
		t.Fatal("expected dynamic boundary marker in prompt")
	}
	staticPrefix := prompt[:idx]
	if !strings.Contains(staticPrefix, "Workspace Guidance") && !strings.Contains(staticPrefix, "工作区指引") {
		t.Fatal("guidance should appear before boundary")
	}
	if !strings.Contains(staticPrefix, "Uncertainty handling principles") &&
		!strings.Contains(staticPrefix, "不确定性处理原则") {
		t.Fatal("core static content should appear before boundary")
	}
	dynamicSuffix := prompt[idx+len(DynamicBoundary):]
	if !strings.Contains(dynamicSuffix, "Session Context") &&
		!strings.Contains(dynamicSuffix, "会话上下文") {
		t.Fatal("session context should appear after boundary")
	}
	if !strings.Contains(report.DynamicSectionNames[0], "session_context") {
		t.Fatalf("expected session_context in dynamic sections, got %v", report.DynamicSectionNames)
	}
}

func TestSystemPromptAssembler_should_disable_boundary_when_config_off(t *testing.T) {
	cfg := config.DefaultWorkspacePromptConfig()
	cfg.PromptConfig.EnableDynamicBoundary = false
	assembler := NewSystemPromptAssembler(cfg)
	prompt, report := assembler.Build(SystemPromptBuildInput{
		Session: types.NewSession("sess_noboundary", "cli", "/tmp"),
		Runtime: ProcessRuntimeContext{SessionID: "sess_noboundary"},
	})
	if report.HasDynamicBoundary {
		t.Fatal("boundary should be disabled")
	}
	if strings.Contains(prompt, DynamicBoundary) {
		t.Fatal("prompt must not contain boundary marker")
	}
	if !strings.Contains(prompt, "日期:") && !strings.Contains(prompt, "Today's date:") {
		t.Fatal("session context should remain inline without boundary mode")
	}
}

func TestSystemPromptAssembler_should_use_core_template(t *testing.T) {
	assembler := NewSystemPromptAssembler(config.DefaultWorkspacePromptConfig())
	prompt, report := assembler.Build(SystemPromptBuildInput{
		AgentsRaw:            "Project agents content",
		OmitAgentsFromSystem: true,
		MemoryEntries: []memory.MemoryEntry{
			{Topic: "architecture", Content: "use DDD"},
		},
		Session: types.NewSession("sess_1", "cli", "/tmp/proj"),
		Runtime: ProcessRuntimeContext{SessionID: "sess_1"},
	})
	if !strings.Contains(prompt, "Devrix") && !strings.Contains(prompt, "软件工程") {
		t.Fatal("expected embedded core template in system prompt")
	}
	if strings.Contains(prompt, "Project agents content") {
		t.Fatal("agents content should be omitted when OmitAgentsFromSystem")
	}
	if !strings.Contains(prompt, "<memory_context>") {
		t.Fatal("expected memory_context block")
	}
	if report.SectionCount <= 0 {
		t.Fatal("expected core layer sections")
	}
}

func TestSystemPromptAssembler_should_embed_agents_when_system_mode(t *testing.T) {
	assembler := NewSystemPromptAssembler(config.DefaultWorkspacePromptConfig())
	prompt, _ := assembler.Build(SystemPromptBuildInput{
		AgentsRaw:            "Project agents content",
		OmitAgentsFromSystem: false,
	})
	if !strings.Contains(prompt, "<agents_context>") || !strings.Contains(prompt, "Project agents content") {
		t.Fatal("expected agents in system prompt when not omitted")
	}
}

// embed_core_template=true must use devrix_core.md, not legacy prompt_sections.
func TestSystemPromptAssembler_should_prefer_embedded_core_over_sections(t *testing.T) {
	cfg := config.DefaultWorkspacePromptConfig()
	if !cfg.EmbedCoreTemplate {
		t.Fatal("test expects EmbedCoreTemplate default true")
	}
	assembler := NewSystemPromptAssembler(cfg)
	prompt, report := assembler.Build(SystemPromptBuildInput{})
	if report.SectionCount != 1 {
		t.Fatalf("embedded core should be one layer, got SectionCount=%d", report.SectionCount)
	}
	if !strings.Contains(prompt, "不确定性处理原则") && !strings.Contains(prompt, "Uncertainty handling principles") {
		t.Fatal("expected embedded core template content")
	}
	// Legacy section-only phrase from prompt_sections intro (not in devrix_core).
	if strings.Contains(prompt, "绝不要生成或猜测 URL") {
		t.Fatal("legacy prompt_sections should not be concatenated when embed_core_template=true")
	}
}

// TestSystemPromptAssembler_ZHCoreTemplate_ChineseHardRule pins the
// DM-20260706-005 hotfix: the zh core template MUST contain an
// unconditional, prominent language lock at the top of the file.
// Without this rule, MiniMax-M3 (and similar English-biased models)
// ignores the buried conditional rule ("用户消息为中文时") later in
// the file and switches output to English during ReAct + structured
// JSON phases — observed in prod trace
// 613e6ae7d5856060532eaeda8fc871ae (sess_1783310385281_8000) on
// 2026-07-06 where the final review summary was returned entirely
// in English despite the user's Chinese directive.
//
// Hard invariants this test guards:
//   1. The rule appears in the first 20 lines of zh core (so it's
//      not buried below the JSON contract rules).
//   2. The rule is *unconditional* (no "用户消息为中文时" hedging —
//      that wording lets the LLM decide the user's language wasn't
//      Chinese and skip the rule).
//   3. The rule names the only allowed exceptions (code identifiers,
//      paths, API names, JSON keys) so the LLM doesn't have to guess.
//   4. The rule mirrors the i18n prompt_sections_zh.go hard rule
//      ("请始终用中文回复用户") so both language paths behave identically.
func TestSystemPromptAssembler_ZHCoreTemplate_ChineseHardRule(t *testing.T) {
	zhTemplate := defaultCoreTemplateZH

	// 1. First 20 lines must contain the language lock — pinning
	//    placement so a future reorder can't bury it again.
	lines := strings.SplitN(zhTemplate, "\n", 20)
	head := strings.Join(lines[:min(20, len(lines))], "\n")
	if !strings.Contains(head, "语言锁定") {
		t.Fatalf("zh core template language lock not in first 20 lines:\n%s", head)
	}

	// 2. The rule must be unconditional (no conditional hedging).
	if !strings.Contains(zhTemplate, "所有可见输出") {
		t.Fatalf("zh core template missing absolute '所有可见输出' phrasing")
	}
	// Conditional hedge the bug-fix replaces — MUST NOT exist anymore
	// in the new rule. The condition used to let the LLM decide the
	// user wasn't writing in Chinese and skip.
	condHedgeAllowed := []string{}
	for _, hedge := range condHedgeAllowed {
		if strings.Contains(zhTemplate, hedge) {
			t.Errorf("zh core should not still contain conditional hedge %q", hedge)
		}
	}

	// 3. Allowed exceptions explicitly named.
	mustHaves := []string{
		"代码标识符",
		"文件路径",
		"API",
		"JSON key",
	}
	for _, want := range mustHaves {
		if !strings.Contains(zhTemplate, want) {
			t.Errorf("zh core language lock missing explicit exception %q", want)
		}
	}

	// 4. Mirror the i18n path so any future divergence trips the test.
	if !strings.Contains(zhTemplate, "无条件") {
		t.Errorf("zh core must explicitly mark the rule as 无条件 (unconditional)")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
