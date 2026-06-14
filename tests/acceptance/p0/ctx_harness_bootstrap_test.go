//go:build acceptance && d2

package p0_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/communication/capture"
	"github.com/devrix/devrix/internal/layers/contextengine"
	mockctx "github.com/devrix/devrix/internal/layers/contextengine/mock"
	"github.com/devrix/devrix/internal/layers/contextengine/memory"
	"github.com/devrix/devrix/internal/layers/contextengine/prompt"
	"github.com/devrix/devrix/internal/layers/contextengine/registry"
	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

type harnessCaptureLLM struct {
	lastTools []contextengine.ToolSchema
	response  string
}

func (l *harnessCaptureLLM) ChatStream(_ context.Context, req *llmgateway.Request) (<-chan llmgateway.Chunk, error) {
	l.lastTools = make([]contextengine.ToolSchema, len(req.Tools))
	for i, t := range req.Tools {
		l.lastTools[i] = contextengine.ToolSchema{Name: t.Name, Description: t.Description, Parameters: t.Parameters}
	}
	ch := make(chan llmgateway.Chunk, 1)
	go func() {
		ch <- llmgateway.Chunk{Content: l.response, Done: true}
		close(ch)
	}()
	return ch, nil
}

// T: D2-S9-A01-T01, D2-S9-A01-T04, D2-S9-A03-T05, D2-S9-A02-T10, D2-S9-A02-T12, D2-S9-A03-T14
func TestAcceptance_HarnessBootstrapP0(t *testing.T) {
	workDir := t.TempDir()
	agentsPath := filepath.Join(workDir, "AGENTS.md")
	if err := os.WriteFile(agentsPath, []byte("Project agents rule"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Run("query loop uses core template not raw agents in system", func(t *testing.T) {
		ctxCfg := config.DefaultContextEngineConfig()
		ctxCfg.LongTerm.Enabled = false
		toolsReg, err := registry.NewBuiltinRegistry()
		if err != nil {
			t.Fatalf("NewBuiltinRegistry: %v", err)
		}
		engine := contextengine.NewContextEngine(contextengine.EngineDeps{
			LLM:        &mockctx.LLMGateway{Response: "ok"},
			Tools:      &mockctx.ToolRunner{},
			ToolsReg:   toolsReg,
			Permission: mockctx.AllowAllPermission{},
			Config:     ctxCfg,
		})
		session := types.NewSession("sess_p0_off", "cli", workDir)
		drainAcceptance(t, engine.Process(context.Background(), session, "hi"))
		sc, ok := engine.SessionContext(session.SessionID)
		if !ok {
			t.Fatal("missing session context")
		}
		if !strings.Contains(sc.SystemPrompt, "Devrix") {
			t.Fatal("system prompt must include embedded core template")
		}
		if strings.Contains(sc.SystemPrompt, "Project agents rule") {
			t.Fatal("AGENTS.md must not be in system prompt when user_context.mode=prepend")
		}
	})

	t.Run("enabled bootstrap and tool pool", func(t *testing.T) {
		ctxCfg := config.DefaultContextEngineConfig()
		ctxCfg.Harness.Enabled = true
		ctxCfg.Harness.Trusted = false
		ctxCfg.Harness.ToolPool.SimpleMode = true
		ctxCfg.Harness.Prefetch.Enabled = true
		ctxCfg.LongTerm.Enabled = false

		llm := &harnessCaptureLLM{response: "done"}
		toolsReg, err := registry.NewBuiltinRegistry()
		if err != nil {
			t.Fatalf("NewBuiltinRegistry: %v", err)
		}
		engine := contextengine.NewContextEngine(contextengine.EngineDeps{
			LLM:        llm,
			Tools:      &mockctx.ToolRunner{},
			ToolsReg:   toolsReg,
			Permission: mockctx.AllowAllPermission{},
			Config:     ctxCfg,
		})
		session := types.NewSession("sess_p0_on", "cli", workDir)
		drainAcceptance(t, engine.Process(context.Background(), session, "run tests"))
		sc, ok := engine.SessionContext(session.SessionID)
		if !ok {
			t.Fatal("missing session context")
		}
		if sc.Harness == nil {
			t.Fatal("missing harness state")
		}
		if sc.Harness.Report.DeferredInit.PluginInit {
			t.Fatal("untrusted session must not set deferred init flags")
		}
		if sc.Harness.Report.VisibleTools != 3 {
			t.Fatalf("simple_mode visible tools: got %d want 3", sc.Harness.Report.VisibleTools)
		}
		if !strings.Contains(sc.SystemPrompt, "<loaded_context>") {
			t.Fatal("harness path should include loaded_context when bootstrap ran")
		}
		if strings.Contains(sc.SystemPrompt, "Project agents rule") {
			t.Fatal("agents body must not be in system when user_context.mode=prepend")
		}
		if !strings.Contains(sc.SystemPrompt, "Devrix") {
			t.Fatal("assembled prompt missing core template")
		}
		if len(llm.lastTools) != 3 {
			t.Fatalf("visible tool count: got %d want 3", len(llm.lastTools))
		}
		for _, tool := range llm.lastTools {
			switch tool.Name {
			case "bash", "read_file", "write_file":
			default:
				t.Fatalf("unexpected visible tool: %s", tool.Name)
			}
		}
	})

	t.Run("assembler omits agents in prepend mode", func(t *testing.T) {
		assembler := prompt.NewSystemPromptAssembler(config.DefaultWorkspacePromptConfig())
		entries := []memory.MemoryEntry{{Topic: "bugs", Content: "race fix"}}
		built, _ := assembler.Build(prompt.SystemPromptBuildInput{
			HarnessEnabled:       false,
			AgentsRaw:            "Agents-only legacy body",
			OmitAgentsFromSystem: true,
			MemoryEntries:        entries,
		})
		if !strings.Contains(built, "Devrix") {
			preview := built
			if len(preview) > 80 {
				preview = preview[:80]
			}
			t.Fatalf("expected core template, got: %q", preview)
		}
		if strings.Contains(built, "Agents-only legacy body") {
			t.Fatal("agents raw should not appear in system when prepend mode")
		}
	})
}

func drainAcceptance(t *testing.T, ch <-chan *capture.EngineEvent) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
		case <-deadline:
			t.Fatal("timeout waiting for process")
		}
	}
}
