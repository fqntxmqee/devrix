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
// Architecture (W11 phase 2a): the main engine's diagnostic surface lives on
// the engine's surface list, NOT on the legacy toolRunner registry. Wires
// (NewContextEngine + WireDelegate) populate Surfaces, and the test
// enumerates via ce.Surfaces() (TOOL-SURFACE-1 SoT). The legacy registry
// still backs the BuiltinSurface so builtins (read_file, glob, grep, bash,
// edit_file) show up via the BuiltinSurface.Tools enumeration.

import (
	"context"
	"sort"
	"testing"

	llmbridge "github.com/devrix/devrix/internal/bridges/llm"
	"github.com/devrix/devrix/internal/layers/communication/capture"
	"github.com/devrix/devrix/internal/layers/contextengine/kernel"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/shared/config"
)

// collectToolNames flattens the engine's surface list into a name set.
func collectToolNames(t *testing.T, ce *kernel.ContextEngine) map[string]bool {
	t.Helper()
	got := map[string]bool{}
	for _, s := range ce.Surfaces() {
		for _, sp := range s.Tools(context.Background(), "", "") {
			got[sp.Name] = true
		}
	}
	return got
}

func TestMainEngine_RegistersDiagnosticToolSurface(t *testing.T) {
	obsBridge := observability.NewBridge(observability.NewNoOp())
	ctxCfg := config.DefaultContextEngineConfig()
	toolCfg := config.DefaultToolConfig()
	maCfg := config.DefaultMultiAgentConfig()
	maCfg.Delegate.Enabled = true
	permMgr := capture.NewPermissionManager(&config.PermissionConfig{})
	stack := llmbridge.ContextLLMStack{}

	ce := NewContextEngine(stack, permMgr, ctxCfg, toolCfg, maCfg, obsBridge, nil, nil)
	if ce == nil {
		t.Fatal("NewContextEngine returned nil")
	}
	// WireDelegate owns delegate_* registration on the main engine.
	WireDelegate(ctxCfg, maCfg, nil, nil, ce, ce.ToolRegistry(), nil, nil)

	got := collectToolNames(t, ce)
	names := make([]string, 0, len(got))
	for n := range got {
		names = append(names, n)
	}
	sort.Strings(names)

	wants := []string{
		"free_fork",
		"query_diagnostics",
		"verify_plan_execution",
		// DM-20260618-007 W3 — LSP 拆 5 个 typed spec 替代单 "lsp" spec。
		"lsp_go_to_definition",
		"lsp_find_references",
		"lsp_incoming_calls",
		"lsp_hover",
		"lsp_workspace_symbol",
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

	eng := SelectContextEngine("context", permMgr, ctxCfg, toolCfg, maCfg, obsBridge, stack, nil, nil)
	if eng == nil {
		t.Fatal("SelectContextEngine returned nil")
	}
	ce, ok := eng.(*kernel.ContextEngine)
	if !ok {
		t.Fatalf("SelectContextEngine returned %T, want *kernel.ContextEngine", eng)
	}
	// Mirror main.go:WireDelegate.
	WireDelegate(ctxCfg, maCfg, nil, nil, ce, ce.ToolRegistry(), nil, nil)
	got := collectToolNames(t, ce)
	for _, want := range []string{"free_fork", "query_diagnostics", "delegate_explore"} {
		if !got[want] {
			t.Errorf("SelectContextEngine → main engine missing %q", want)
		}
	}
}
