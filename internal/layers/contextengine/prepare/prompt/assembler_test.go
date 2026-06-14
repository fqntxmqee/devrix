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
		HarnessEnabled:       true,
		AgentsRaw:            "Project agents content",
		OmitAgentsFromSystem: true,
	})
	if strings.Contains(prompt, "<agents_context>") && strings.Contains(prompt, "Project agents content") {
		t.Fatal("agents content should be omitted from system prompt in prepend mode")
	}
}

// T: D2-S9-A02-T10
func TestSystemPromptAssembler_should_build_xml_when_harness_enabled(t *testing.T) {
	assembler := NewSystemPromptAssembler(config.DefaultWorkspacePromptConfig())
	prompt, report := assembler.Build(SystemPromptBuildInput{
		HarnessEnabled: true,
		AgentsRaw:      "Project agents content",
		MemoryEntries: []memory.MemoryEntry{
			{Topic: "architecture", Content: "use DDD"},
		},
		Bootstrap: &types.BootstrapReport{
			Trusted:      true,
			ToolCount:    5,
			VisibleTools: 3,
			VisibleToolList: []types.VisibleTool{
				{Name: "bash"}, {Name: "read_file"}, {Name: "write_file"},
			},
			StagesApplied: []types.BootstrapStage{types.BootstrapStageToolPool},
		},
		Workspace: &types.WorkspaceContext{WorkDir: "/tmp/proj", GoFileCount: 10},
		Session:   types.NewSession("sess_1", "cli", "/tmp/proj"),
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
	if !strings.Contains(prompt, "Session ID: sess_1") {
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
		HarnessEnabled: true,
		AgentsRaw:      "# Agents\n",
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
	if !strings.Contains(staticPrefix, "Workspace Guidance") {
		t.Fatal("guidance should appear before boundary")
	}
	if !strings.Contains(staticPrefix, "You are an interactive agent") &&
		!strings.Contains(staticPrefix, "Devrix") {
		t.Fatal("core static content should appear before boundary")
	}
	dynamicSuffix := prompt[idx+len(DynamicBoundary):]
	if !strings.Contains(dynamicSuffix, "Session Context") {
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
	if !strings.Contains(prompt, "Session Context") {
		t.Fatal("session context should remain inline without boundary mode")
	}
}

func TestSystemPromptAssembler_should_use_core_when_harness_disabled(t *testing.T) {
	assembler := NewSystemPromptAssembler(config.DefaultWorkspacePromptConfig())
	prompt, report := assembler.Build(SystemPromptBuildInput{
		HarnessEnabled:       false,
		AgentsRaw:            "Project agents content",
		OmitAgentsFromSystem: true,
		MemoryEntries: []memory.MemoryEntry{
			{Topic: "architecture", Content: "use DDD"},
		},
		Session: types.NewSession("sess_1", "cli", "/tmp/proj"),
		Runtime: ProcessRuntimeContext{SessionID: "sess_1"},
	})
	if !strings.Contains(prompt, "Devrix") {
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
		HarnessEnabled:       false,
		AgentsRaw:            "Project agents content",
		OmitAgentsFromSystem: false,
	})
	if !strings.Contains(prompt, "<agents_context>") || !strings.Contains(prompt, "Project agents content") {
		t.Fatal("expected agents in system prompt when not omitted")
	}
}
