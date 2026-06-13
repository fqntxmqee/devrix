package prompt

import (
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/memory"
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

func TestSystemPromptAssembler_BuildLegacy_should_match_v4_shape(t *testing.T) {
	assembler := NewSystemPromptAssembler(config.DefaultWorkspacePromptConfig())
	appendix := memory.FormatLongTermAppendix([]memory.MemoryEntry{
		{Topic: "bugs", Content: "fix race"},
	}, 2000)
	legacy := assembler.BuildLegacy("Agents body", appendix)
	disabled, _ := assembler.Build(SystemPromptBuildInput{
		HarnessEnabled: false,
		AgentsRaw:      "Agents body",
		MemoryEntries: []memory.MemoryEntry{
			{Topic: "bugs", Content: "fix race"},
		},
	})
	if legacy != disabled {
		t.Fatalf("legacy mismatch:\nlegacy=%q\ndisabled=%q", legacy, disabled)
	}
}
