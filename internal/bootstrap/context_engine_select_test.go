package bootstrap

// DM-20260617-005: regression test for the missing-tool-list bug.
//
// Before the fix, NewContextEngine (the path used by devrix binary through
// SelectContextEngine) only registered QueryLoop/Task/Background tools and
// agent plugins. The leader LLM in the main engine therefore could not see
// free_fork / query_diagnostics / verify_plan_execution / lsp — it would
// respond with "unknown tool" even though the per-agent builder path
// (buildWithGate) had registered them.
//
// Architecture: the main engine's diagnostic surface is split between
//   - NewContextEngine:  registers free_fork / query_diagnostics /
//                        verify_plan_execution / lsp / todo_write
//   - WireDelegate:      registers delegate_* (after NewContextEngine,
//                        so it sees a populated engine)
//
// This test wires the same calls main.go does (NewContextEngine + WireDelegate)
// and asserts the leader LLM sees the full tool surface.

import (
	"context"
	"sort"
	"testing"

	llmbridge "github.com/devrix/devrix/internal/bridges/llm"
	"github.com/devrix/devrix/internal/layers/communication/capture"
	"github.com/devrix/devrix/internal/layers/contextengine"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/shared/config"
)

func TestMainEngine_RegistersDiagnosticToolSurface(t *testing.T) {
	obsBridge := observability.NewBridge(observability.NewNoOp())
	ctxCfg := config.DefaultContextEngineConfig()
	toolCfg := config.DefaultToolConfig()
	maCfg := config.DefaultMultiAgentConfig()
	maCfg.Delegate.Enabled = true
	permMgr := capture.NewPermissionManager(&config.PermissionConfig{})
	stack := llmbridge.ContextLLMStack{}

	ce := NewContextEngine(stack, permMgr, ctxCfg, toolCfg, maCfg, obsBridge, nil)
	if ce == nil {
		t.Fatal("NewContextEngine returned nil")
	}
	// WireDelegate owns delegate_* registration on the main engine.
	WireDelegate(ctxCfg, maCfg, nil, ce, ce.ToolRegistry())

	schemas, err := ce.ToolRegistry().ListTools(context.Background(), "")
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	got := make(map[string]bool, len(schemas))
	names := make([]string, 0, len(schemas))
	for _, s := range schemas {
		got[s.Name] = true
		names = append(names, s.Name)
	}
	sort.Strings(names)

	wants := []string{
		"free_fork",
		"query_diagnostics",
		"verify_plan_execution",
		"lsp",
		"delegate_explore",
		"delegate_plan",
		"delegate_implement",
		"delegate_status",
	}
	for _, w := range wants {
		if !got[w] {
			t.Errorf("main engine missing diagnostic tool %q; got names=%v", w, names)
		}
	}
}

func TestSelectContextEngine_ForwardsMultiAgentConfig(t *testing.T) {
	obsBridge := observability.NewBridge(observability.NewNoOp())
	ctxCfg := config.DefaultContextEngineConfig()
	toolCfg := config.DefaultToolConfig()
	maCfg := config.DefaultMultiAgentConfig()
	maCfg.Delegate.Enabled = true
	permMgr := capture.NewPermissionManager(&config.PermissionConfig{})
	stack := llmbridge.ContextLLMStack{}

	eng := SelectContextEngine("context", permMgr, ctxCfg, toolCfg, maCfg, obsBridge, stack, nil)
	if eng == nil {
		t.Fatal("SelectContextEngine returned nil")
	}
	ce, ok := eng.(*contextengine.ContextEngine)
	if !ok {
		t.Fatalf("SelectContextEngine returned %T, want *contextengine.ContextEngine", eng)
	}
	// Mirror main.go:WireDelegate.
	WireDelegate(ctxCfg, maCfg, nil, ce, ce.ToolRegistry())
	schemas, err := ce.ToolRegistry().ListTools(context.Background(), "")
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	for _, want := range []string{"free_fork", "query_diagnostics", "delegate_explore"} {
		found := false
		for _, s := range schemas {
			if s.Name == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("SelectContextEngine → main engine missing %q", want)
		}
	}
}


