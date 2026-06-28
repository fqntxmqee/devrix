package bootstrap

// DM-20260617-006 (devrix-tool-pipeline-permission): close the D2→D7 拆面 gap
// on tool permission. D7 turn adapter must (a) gate every tool call via
// IPermissionGate.Request, and (b) propagate the risk classification from
// the D2 ToolRegistry into the contextengine.ToolCall it dispatches.
//
// These tests pin both behaviors so a future refactor cannot silently
// regress them.

import (
	"context"
	"sync"
	"testing"

	"github.com/devrix/devrix/internal/layers/communication/capture"
	"github.com/devrix/devrix/internal/layers/contextengine"
	"github.com/devrix/devrix/internal/layers/contextengine/kernel"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce/tools"
	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/orchestration/sessionorchestrator"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// recordingPermission records every Request call so the test can assert on
// (sessionID, toolName, input, risk) tuples.
type recordingPermission struct {
	mu       sync.Mutex
	calls    []permCall
	decision bool // what Request returns for every call
}

type permCall struct {
	SessionID string
	ToolName  string
	Input     string
	Risk      types.RiskLevel
}

func (p *recordingPermission) Request(_ context.Context, sessionID, toolName, input string, risk types.RiskLevel) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls = append(p.calls, permCall{
		SessionID: sessionID,
		ToolName:  toolName,
		Input:     input,
		Risk:      risk,
	})
	return p.decision
}

func (p *recordingPermission) Calls() []permCall {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]permCall, len(p.calls))
	copy(out, p.calls)
	return out
}

// CheckPermission implements contracts.IPermissionGate. Returns Allow
// by default; tests can override via the permDecision field.
//
// TOOL-SURFACE-1-A01-F07 (DM-20260618-002).
func (p *recordingPermission) CheckPermission(_ context.Context, _ contracts.ToolSpec) contracts.Decision {
	return contracts.DecisionAllow
}

// recordingToolRunner captures every Execute call so the test can assert on
// the RiskLevel that the adapter passed through.
type recordingToolRunner struct {
	mu    sync.Mutex
	calls []tools.ToolCall
	out   string
}

func (r *recordingToolRunner) Execute(_ context.Context, call tools.ToolCall) (*tools.ToolResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, call)
	out := r.out
	if out == "" {
		out = "ok"
	}
	return &tools.ToolResult{Output: out}, nil
}

func (r *recordingToolRunner) Calls() []tools.ToolCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]tools.ToolCall, len(r.calls))
	copy(out, r.calls)
	return out
}

// riskFixedRegistry implements IToolRegistry by returning a fixed risk for
// every tool, with an empty runner set (we only care about risk lookup).
type riskFixedRegistry struct {
	risk types.RiskLevel
}

func (r *riskFixedRegistry) ListTools(context.Context, string) ([]tools.ToolSchema, error) {
	return nil, nil
}
func (r *riskFixedRegistry) RiskLevel(string) types.RiskLevel { return r.risk }

// T: D7-S2-A06-T03 (DM-20260617-006) — when IPermissionGate denies, the
// tool is NOT executed and the result carries "permission denied" with the
// original ToolCallID. The previous behavior (D7 path skipped perm.Request)
// let every call through, regardless of mode.
func TestExecuteRound_PermissionDenied_ToolNotCalled(t *testing.T) {
	store, err := capture.NewFileSessionStore(t.TempDir())
	if err != nil {
		t.Fatalf("session store: %v", err)
	}
	gw := capture.NewCommunicationGateway(store, nil, nil, nil, nil)

	perm := &recordingPermission{decision: false}
	tools := &recordingToolRunner{}
	reg := &riskFixedRegistry{risk: types.RiskLevelHigh}

	// Wire the adapter manually so we can inject our mocks (the production
	// constructor copies tools/reg/perm from a real *ContextEngine).
	adapter := &contextEngineAdapter{
		gw:       gw,
		engine:   &stubSessionEngine{},
		tools:    tools,
		toolsReg: reg,
		perm:     perm,
	}

	res, err := adapter.ExecuteRound(context.Background(), sessionorchestrator.ToolRoundRequest{
		SessionID: "sess-denied",
		ToolCalls: []llmgateway.ToolCall{{
			ID:    "call-1",
			Name:  "bash",
			Input: `{"command":"rm -rf /"}`,
		}},
	})
	if err != nil {
		t.Fatalf("ExecuteRound: %v", err)
	}
	if len(res.Results) != 1 {
		t.Fatalf("results len = %d, want 1", len(res.Results))
	}
	if res.Results[0].ToolCallID != "call-1" {
		t.Errorf("ToolCallID = %q, want %q", res.Results[0].ToolCallID, "call-1")
	}
	if res.Results[0].Error != "permission denied" {
		t.Errorf("Error = %q, want %q", res.Results[0].Error, "permission denied")
	}
	if res.Results[0].Output != "" {
		t.Errorf("Output = %q, want empty (tool must not run)", res.Results[0].Output)
	}
	if got := len(tools.Calls()); got != 0 {
		t.Errorf("tools.Execute called %d times, want 0 (perm denied)", got)
	}
	// perm.Request must have been called exactly once with the looked-up risk.
	calls := perm.Calls()
	if len(calls) != 1 {
		t.Fatalf("perm.Request called %d times, want 1", len(calls))
	}
	if calls[0].SessionID != "sess-denied" || calls[0].ToolName != "bash" {
		t.Errorf("perm.Request args = %+v, want sessionID=sess-denied name=bash", calls[0])
	}
	if calls[0].Risk != types.RiskLevelHigh {
		t.Errorf("perm.Request risk = %q, want HIGH (registry lookup)", calls[0].Risk)
	}
}

// T: D7-S2-A06-T03 — when IPermissionGate approves, the tool IS executed
// and the RiskLevel looked up from the registry is propagated into the
// contextengine.ToolCall that the runner sees.
func TestExecuteRound_PermissionAllowed_PropagatesRisk(t *testing.T) {
	store, err := capture.NewFileSessionStore(t.TempDir())
	if err != nil {
		t.Fatalf("session store: %v", err)
	}
	gw := capture.NewCommunicationGateway(store, nil, nil, nil, nil)

	perm := &recordingPermission{decision: true}
	tools := &recordingToolRunner{out: "ran"}
	reg := &riskFixedRegistry{risk: types.RiskLevelMedium}

	adapter := &contextEngineAdapter{
		gw:       gw,
		engine:   &stubSessionEngine{},
		tools:    tools,
		toolsReg: reg,
		perm:     perm,
	}

	res, err := adapter.ExecuteRound(context.Background(), sessionorchestrator.ToolRoundRequest{
		SessionID: "sess-allowed",
		ToolCalls: []llmgateway.ToolCall{{
			ID:    "call-2",
			Name:  "write_file",
			Input: `{"path":"/tmp/x","content":"y"}`,
		}},
	})
	if err != nil {
		t.Fatalf("ExecuteRound: %v", err)
	}
	if len(res.Results) != 1 {
		t.Fatalf("results len = %d, want 1", len(res.Results))
	}
	if res.Results[0].Error != "" {
		t.Errorf("Error = %q, want empty", res.Results[0].Error)
	}
	if res.Results[0].Output != "ran" {
		t.Errorf("Output = %q, want %q", res.Results[0].Output, "ran")
	}
	// Risk level must reach the runner.
	tcalls := tools.Calls()
	if len(tcalls) != 1 {
		t.Fatalf("tools.Execute called %d times, want 1", len(tcalls))
	}
	if tcalls[0].RiskLevel != types.RiskLevelMedium {
		t.Errorf("runner saw RiskLevel = %q, want MEDIUM (registry lookup)", tcalls[0].RiskLevel)
	}
	if tcalls[0].ID != "call-2" || tcalls[0].Name != "write_file" {
		t.Errorf("runner saw call = %+v, want {ID:call-2 Name:write_file}", tcalls[0])
	}
}

// T: D7-S2-A06-T03 — when a.perm is nil (test/mock wiring), the gate is
// left open and the tool runs with the registry-provided risk. This pins
// the defensive default so a future refactor cannot regress to a nil-deref
// crash on the missing gate.
func TestExecuteRound_NilPermission_StillExecutes(t *testing.T) {
	store, err := capture.NewFileSessionStore(t.TempDir())
	if err != nil {
		t.Fatalf("session store: %v", err)
	}
	gw := capture.NewCommunicationGateway(store, nil, nil, nil, nil)

	tools := &recordingToolRunner{out: "ok"}
	reg := &riskFixedRegistry{risk: types.RiskLevelLow}

	adapter := &contextEngineAdapter{
		gw:       gw,
		engine:   &stubSessionEngine{},
		tools:    tools,
		toolsReg: reg,
		perm:     nil, // gate intentionally absent
	}

	res, err := adapter.ExecuteRound(context.Background(), sessionorchestrator.ToolRoundRequest{
		SessionID: "sess-noperm",
		ToolCalls: []llmgateway.ToolCall{{
			ID:    "call-3",
			Name:  "read_file",
			Input: `{"path":"/tmp/x"}`,
		}},
	})
	if err != nil {
		t.Fatalf("ExecuteRound: %v", err)
	}
	if len(res.Results) != 1 || res.Results[0].Error != "" {
		t.Fatalf("results = %+v, want 1 result with empty error", res.Results)
	}
	tcalls := tools.Calls()
	if len(tcalls) != 1 || tcalls[0].RiskLevel != types.RiskLevelLow {
		t.Errorf("runner calls = %+v, want 1 call with RiskLevel=LOW", tcalls)
	}
}

// T: D7-S2-A06-T03 — multi-call round: perm denies one, allows the other.
// The denied call must NOT consume the slot in the runner, the allowed one
// must run with the correct risk. This guards against the for-loop short
// circuit leaking the denied result into the next iteration.
func TestExecuteRound_PermissionMixed_OnlyAllowedRuns(t *testing.T) {
	store, err := capture.NewFileSessionStore(t.TempDir())
	if err != nil {
		t.Fatalf("session store: %v", err)
	}
	gw := capture.NewCommunicationGateway(store, nil, nil, nil, nil)

	perm := &togglingPermission{
		deny: map[string]bool{"bash": true}, // deny bash, allow write_file
	}
	tools := &recordingToolRunner{out: "ran"}
	reg := &riskFixedRegistry{risk: types.RiskLevelHigh}

	adapter := &contextEngineAdapter{
		gw:       gw,
		engine:   &stubSessionEngine{},
		tools:    tools,
		toolsReg: reg,
		perm:     perm,
	}

	res, err := adapter.ExecuteRound(context.Background(), sessionorchestrator.ToolRoundRequest{
		SessionID: "sess-mixed",
		ToolCalls: []llmgateway.ToolCall{
			{ID: "c1", Name: "bash", Input: `{"command":"rm -rf /"}`},
			{ID: "c2", Name: "write_file", Input: `{"path":"/tmp/x","content":"y"}`},
		},
	})
	if err != nil {
		t.Fatalf("ExecuteRound: %v", err)
	}
	if len(res.Results) != 2 {
		t.Fatalf("results len = %d, want 2", len(res.Results))
	}
	if res.Results[0].Error != "permission denied" {
		t.Errorf("results[0].Error = %q, want %q", res.Results[0].Error, "permission denied")
	}
	if res.Results[1].Error != "" {
		t.Errorf("results[1].Error = %q, want empty (allowed)", res.Results[1].Error)
	}
	tcalls := tools.Calls()
	if len(tcalls) != 1 {
		t.Fatalf("runner calls = %d, want 1 (only write_file)", len(tcalls))
	}
	if tcalls[0].Name != "write_file" {
		t.Errorf("runner call name = %q, want write_file (bash denied)", tcalls[0].Name)
	}
}

// togglingPermission denies only the tool names in deny; everything else
// is allowed. Used by the mixed test to exercise the per-call decision.
type togglingPermission struct {
	deny map[string]bool
}

func (p *togglingPermission) Request(_ context.Context, _, toolName, _ string, _ types.RiskLevel) bool {
	return !p.deny[toolName]
}

// CheckPermission mirrors Request: denied tools → Deny; everything
// else → Allow.
//
// TOOL-SURFACE-1-A01-F07 (DM-20260618-002).
func (p *togglingPermission) CheckPermission(_ context.Context, spec contracts.ToolSpec) contracts.Decision {
	if p.deny[spec.Name] {
		return contracts.DecisionDeny
	}
	return contracts.DecisionAllow
}

// T: D7-S2-A06-T03 — integration: wire the adapter through the real
// ContextEngine constructor (as bootstrap.NewContextEngine does) and verify
// that IPermissionGate.DenyAllPermission blocks all tool calls under
// loop_first. This is the regression test for the gap noted in the DM.
func TestExecuteRound_RealEngine_DenyAllBlocksAll(t *testing.T) {
	store, err := capture.NewFileSessionStore(t.TempDir())
	if err != nil {
		t.Fatalf("session store: %v", err)
	}
	gw := capture.NewCommunicationGateway(store, nil, nil, nil, nil)

	cfg := config.DefaultContextEngineConfig()
	cfg.Compression.Autocompact.Enabled = false
	cfg.Snapshot.Enabled = false
	realReg, err := tools.NewBuiltinToolRegistry(nil)
	if err != nil {
		t.Fatalf("real reg: %v", err)
	}
	registryBuiltin := mustBuiltinRegistryForAdapter(t)
	engine := kernel.NewContextEngine(kernel.EngineDeps{
		PreparedTurnRunner: &contextengine.StaticPreparedTurnRunner{Response: "ok"},
		Summarizer:         &contextengine.StaticSummarizer{},
		Tools:              realReg,
		ToolsReg:           registryBuiltin,
		Permission:         enforce.DenyAllPermission{},
		Config:             cfg,
	})
	adapter := newContextEngineAdapter(gw, engine, nil)

	res, err := adapter.ExecuteRound(context.Background(), sessionorchestrator.ToolRoundRequest{
		SessionID: "sess-realdn",
		ToolCalls: []llmgateway.ToolCall{{
			ID:    "call-x",
			Name:  "read_file",
			Input: `{"path":"/etc/hostname"}`,
		}},
	})
	if err != nil {
		t.Fatalf("ExecuteRound: %v", err)
	}
	if len(res.Results) != 1 {
		t.Fatalf("results len = %d, want 1", len(res.Results))
	}
	if res.Results[0].Error != "permission denied" {
		t.Errorf("Error = %q, want %q (DenyAllPermission must block)", res.Results[0].Error, "permission denied")
	}
}
