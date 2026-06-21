package guard

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// mockGateway returns a canned judge verdict.
type mockGateway struct {
	response string
}

func (m *mockGateway) Stream(_ context.Context, _ *llmgateway.Request) (<-chan llmgateway.Chunk, error) {
	ch := make(chan llmgateway.Chunk, 1)
	ch <- llmgateway.Chunk{Content: m.response, Done: true}
	close(ch)
	return ch, nil
}
func (m *mockGateway) Close() error { return nil }
func (m *mockGateway) ResolveTier(tier string) string { return tier }

type mockAgentCtrl struct {
	agents map[string]multiagent.Agent
}

func (m *mockAgentCtrl) SessionAgent(sessionID string) multiagent.Agent {
	return m.agents[sessionID]
}
func (m *mockAgentCtrl) RegisterSessionAgent(sessionID string, ag multiagent.Agent) {
	if m.agents == nil {
		m.agents = make(map[string]multiagent.Agent)
	}
	m.agents[sessionID] = ag
}

type mockTaskCtrl struct {
	failErr error
}

func (m *mockTaskCtrl) Fail(_ string, _ string) error  { return m.failErr }
func (m *mockTaskCtrl) Complete(_ string) error         { return nil }

type mockAgentFactory struct{}

func (m *mockAgentFactory) Create(_ context.Context, _ multiagent.AgentConfig, _ *types.Session) (multiagent.Agent, error) {
	return &mockAgent{}, nil
}

type mockAgent struct {
	terminateErr error
	waitErr      error
}

func (m *mockAgent) ID() string                                        { return "mock-agent" }
func (m *mockAgent) State() multiagent.AgentState                      { return multiagent.AgentStateCreated }
func (m *mockAgent) Config() multiagent.AgentConfig                    { return multiagent.AgentConfig{} }
func (m *mockAgent) Run(_ context.Context) (*multiagent.AgentResult, error) { return nil, nil }
func (m *mockAgent) Fork(_ context.Context, _ multiagent.AgentConfig) (multiagent.Agent, error) { return nil, nil }
func (m *mockAgent) Join(_ context.Context, _ multiagent.Agent) error  { return nil }
func (m *mockAgent) Terminate(_ context.Context) error                  { return m.terminateErr }
func (m *mockAgent) Wait(_ context.Context) (*multiagent.AgentResult, error) { return nil, m.waitErr }
func (m *mockAgent) ResolvePermission(_ string, _ bool)                 {}
func (m *mockAgent) GetMessages() []types.Message                       { return nil }
func (m *mockAgent) SetAgentObserver(_ multiagent.AgentObserver)        {}
func (m *mockAgent) SetEngineEventSink(_ func(*contracts.EngineEvent))  {}

func session() *types.Session {
	return &types.Session{
		SessionID: "test-session",
		WorkDir:   "/tmp",
		State:     types.SessionStateIdle,
	}
}

func decisionRecord() DecisionRecord {
	return DecisionRecord{
		ID:        "test-decision-1",
		SessionID: "test-session",
		AgentID:   "agent-1",
		Category:  DecisionToolCall,
		RiskClass: RiskEvaluate,
		Timestamp: time.Now(),
		ToolName:  "bash",
	}
}

func newTestValidator(cfg OrchestrationConfig, gw llmgateway.IGateway) *RuntimeOrchestrationValidator {
	judge := NewRuntimeJudge(gw, cfg)
	exec := NewInterventionExecutor(&mockAgentCtrl{}, &mockTaskCtrl{}, &mockAgentFactory{})
	return NewRuntimeOrchestrationValidator(cfg, judge, exec)
}

func TestOnDecision_Disabled(t *testing.T) {
	v := NewRuntimeOrchestrationValidator(
		OrchestrationConfig{Enabled: false},
		nil, nil,
	)
	v.OnDecision(context.Background(), decisionRecord(), session())
}

func TestOnDecision_PreFilterSkipsTrustedTool(t *testing.T) {
	v := newTestValidator(
		OrchestrationConfig{
			Enabled:          true,
			PreFilterEnabled: true,
			TrustedToolAllowlist: []string{"read", "ls"},
		},
		&mockGateway{response: `{"valid":true,"confidence":0.9}`},
	)
	rec := decisionRecord()
	rec.ToolName = "read"
	v.OnDecision(context.Background(), rec, session())
}

func TestOnDecision_ValidDecisionPasses(t *testing.T) {
	v := newTestValidator(
		OrchestrationConfig{
			Enabled:              true,
			PreFilterEnabled:     false,
			InterventionThreshold: 0.5,
		},
		&mockGateway{response: `{"valid":true,"confidence":0.95}`},
	)
	v.OnDecision(context.Background(), decisionRecord(), session())
}

func TestOnDecision_Hooks(t *testing.T) {
	var decisionHookCalled bool
	var validateHookCalled bool

	gw := &mockGateway{response: `{"valid":true,"confidence":0.95}`}
	v := newTestValidator(
		OrchestrationConfig{Enabled: true, PreFilterEnabled: false},
		gw,
	)
	v.OnDecisionHook(func(rec DecisionRecord) { decisionHookCalled = true })
	v.OnValidateHook(func(result ValidationResult) { validateHookCalled = true })

	v.OnDecision(context.Background(), decisionRecord(), session())

	if !decisionHookCalled {
		t.Error("OnDecisionHook not called")
	}
	if !validateHookCalled {
		t.Error("OnValidateHook not called")
	}
}

func TestNewOrchestrationObserver(t *testing.T) {
	gw := &mockGateway{response: `{"valid":true,"confidence":0.95}`}
	v := newTestValidator(config.DefaultOrchestrationConfig(), gw)

	obs := NewOrchestrationObserver(v, context.Background(), session())
	if obs == nil {
		t.Fatal("expected non-nil observer")
	}

	obs.EmitAgentEvent(multiagent.AgentEvent{
		EventType: "permission_required",
		SessionID: "test-session",
		AgentID:   "agent-1",
		Timestamp: time.Now(),
		Metadata:  map[string]any{"tool": "bash"},
	})
	obs.EmitAgentEvent(multiagent.AgentEvent{
		EventType: "agent.forked",
		SessionID: "test-session",
		AgentID:   "agent-1",
		ParentID:  "parent-1",
		Timestamp: time.Now(),
		Metadata:  map[string]any{"child_id": "child-1"},
	})
	obs.EmitAgentEvent(multiagent.AgentEvent{
		EventType: "unknown_event",
		SessionID: "test-session",
	})
}

func TestPreFilter_RateLimit(t *testing.T) {
	v := newTestValidator(
		OrchestrationConfig{
			Enabled:                true,
			PreFilterEnabled:       true,
			MinIntervalBetweenJudges: 1 * time.Hour,
		},
		&mockGateway{},
	)
	rec := decisionRecord()
	if !v.preFilter(rec) {
		t.Error("expected preFilter to block due to interval throttle")
	}
}

func TestPreFilter_TrustedToolAllowlist(t *testing.T) {
	v := newTestValidator(
		OrchestrationConfig{
			Enabled:          true,
			PreFilterEnabled: true,
			TrustedToolAllowlist: []string{"read", "ls"},
		},
		&mockGateway{},
	)
	rec := decisionRecord()
	rec.Category = DecisionToolCall
	rec.ToolName = "ls"
	if !v.preFilter(rec) {
		t.Error("expected preFilter to allow trusted tool 'ls'")
	}
}

func TestDecisionRecordJSON(t *testing.T) {
	rec := decisionRecord()
	data, err := json.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	var decoded DecisionRecord
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.ID != rec.ID {
		t.Errorf("json round-trip: got ID %q, want %q", decoded.ID, rec.ID)
	}
}

func TestInterventionExecutor_Execute(t *testing.T) {
	ctrl := &mockAgentCtrl{}
	taskCtrl := &mockTaskCtrl{}
	factory := &mockAgentFactory{}
	exec := NewInterventionExecutor(ctrl, taskCtrl, factory)

	iv := Intervention{
		DecisionID: "test-iv-1",
		Action:     "terminate",
		Reason:     "test termination",
	}
	err := exec.Execute(context.Background(), iv, session())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
