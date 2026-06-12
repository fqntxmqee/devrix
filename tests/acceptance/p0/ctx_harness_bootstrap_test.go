//go:build acceptance && d2

package p0_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/communication/gateway"
	"github.com/devrix/devrix/internal/layers/contextengine"
	mockctx "github.com/devrix/devrix/internal/layers/contextengine/mock"
	"github.com/devrix/devrix/internal/layers/contextengine/memory"
	"github.com/devrix/devrix/internal/layers/contextengine/prompt"
	"github.com/devrix/devrix/internal/layers/contextengine/registry"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

type harnessCaptureLLM struct {
	lastTools []contextengine.ToolSchema
	response  string
}

func (l *harnessCaptureLLM) ChatStream(_ context.Context, req *contextengine.LLMRequest) (<-chan contextengine.LLMChunk, error) {
	l.lastTools = append([]contextengine.ToolSchema(nil), req.Tools...)
	ch := make(chan contextengine.LLMChunk, 1)
	go func() {
		ch <- contextengine.LLMChunk{Content: l.response, Done: true}
		close(ch)
	}()
	return ch, nil
}

// Covers: L5-2-9-01, L5-2-9-04, L5-2-9-05, L5-2-9-10, L5-2-9-12, L5-2-9-14
func TestAcceptance_HarnessBootstrapP0(t *testing.T) {
	workDir := t.TempDir()
	agentsPath := filepath.Join(workDir, "AGENTS.md")
	if err := os.WriteFile(agentsPath, []byte("Project agents rule"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Run("disabled matches legacy system prompt", func(t *testing.T) {
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
		if strings.Contains(sc.SystemPrompt, "<loaded_context>") {
			t.Fatal("disabled path must not use harness XML assembly")
		}
		if !strings.Contains(sc.SystemPrompt, "Project agents rule") {
			t.Fatal("disabled path must still load AGENTS.md")
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
		if !strings.Contains(sc.SystemPrompt, "<agents_context>") {
			t.Fatal("assembled prompt missing agents_context")
		}
		if !strings.Contains(sc.SystemPrompt, "Project agents rule") {
			t.Fatal("agents body missing from assembled prompt")
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

	t.Run("build legacy equals disabled build path", func(t *testing.T) {
		assembler := prompt.NewSystemPromptAssembler(config.DefaultWorkspacePromptConfig())
		entries := []memory.MemoryEntry{{Topic: "bugs", Content: "race fix"}}
		appendix := memory.FormatLongTermAppendix(entries, 2000)
		legacy := assembler.BuildLegacy("Agents", appendix)
		disabled, _ := assembler.Build(prompt.SystemPromptBuildInput{
			HarnessEnabled: false,
			AgentsRaw:      "Agents",
			MemoryEntries:  entries,
		})
		if legacy != disabled {
			t.Fatalf("BuildLegacy drift: %q vs %q", legacy, disabled)
		}
	})
}

func drainAcceptance(t *testing.T, ch <-chan *gateway.EngineEvent) {
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
