package toolrunner_test

// W7 — D4-S11-A02 + D4-S13-A02 (alias G5) free_fork LLM tool 单元测试。
//
// AC5:
//   - 批量分叉 3 个 → spawned_count=3, agent_ids 长度 = 3
//   - factory 失败 → tool 返回 error（不返回 spawned_count）
//   - requests count > 5 → tool 拒绝

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce/toolrunner"
	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/layers/multiagent/provision/freefork"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// stubFreeforkFactory 满足 IAgentFactory 接口;可用于"factory 失败"用例。
type stubFreeforkFactory struct {
	mu       sync.Mutex
	created  int
	failOn   int // 第 N 次创建时失败；0 表示永不失败
}

func (s *stubFreeforkFactory) Create(_ context.Context, _ multiagent.AgentConfig, _ *types.Session) (multiagent.Agent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.created++
	if s.failOn > 0 && s.created == s.failOn {
		return nil, errors.New("stub: factory failed on request")
	}
	return &stubFFAgent{id: "agent-" + itoa(s.created)}, nil
}

func (s *stubFreeforkFactory) ReleaseSession(string) {}

type stubFFAgent struct{ id string }

func (a *stubFFAgent) ID() string                { return a.id }
func (a *stubFFAgent) State() multiagent.AgentState { return multiagent.AgentStateRunning }
func (a *stubFFAgent) Config() multiagent.AgentConfig { return multiagent.AgentConfig{} }
func (a *stubFFAgent) Run(_ context.Context) (*multiagent.AgentResult, error) {
	return &multiagent.AgentResult{ExitCode: 0}, nil
}
func (a *stubFFAgent) Fork(_ context.Context, _ multiagent.AgentConfig) (multiagent.Agent, error) {
	return nil, errors.New("nested fork not supported")
}
func (a *stubFFAgent) Join(_ context.Context, _ multiagent.Agent) error { return nil }
func (a *stubFFAgent) Terminate(_ context.Context) error                { return nil }
func (a *stubFFAgent) Wait(_ context.Context) (*multiagent.AgentResult, error) {
	return &multiagent.AgentResult{ExitCode: 0}, nil
}
func (a *stubFFAgent) ResolvePermission(string, bool)            {}
func (a *stubFFAgent) GetMessages() []types.Message              { return nil }
func (a *stubFFAgent) SetAgentObserver(multiagent.AgentObserver) {}
func (a *stubFFAgent) SetEngineEventSink(func(*contracts.EngineEvent)) {}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	digits := []byte{}
	for i > 0 {
		digits = append([]byte{byte('0' + i%10)}, digits...)
		i /= 10
	}
	return string(digits)
}

// setGlobalForkerForTest 注入 stub forker,测试结束时清理。
func setGlobalForkerForTest(t *testing.T, f freefork.Forker) {
	t.Helper()
	prev := freefork.GlobalForker()
	freefork.SetGlobalForker(f)
	t.Cleanup(func() { freefork.SetGlobalForker(prev) })
}

func TestFreeForkTool_BatchForkThree(t *testing.T) {
	factory := &stubFreeforkFactory{}
	setGlobalForkerForTest(t, freefork.NewDefaultForker(freefork.ForkerDeps{
		Factory:       factory,
		DefaultConfig: multiagent.AgentConfig{Mode: multiagent.ModeDefault},
	}))

	reg := toolrunner.NewToolRegistry()
	if err := toolrunner.RegisterFreeForkTool(reg); err != nil {
		t.Fatalf("RegisterFreeForkTool: %v", err)
	}

	input, _ := json.Marshal(map[string]any{
		"parent_session": "sess-1",
		"requests": []map[string]any{
			{"name": "r1", "prompt": "p1", "worktree": true},
			{"name": "r2", "prompt": "p2", "worktree": true},
			{"name": "r3", "prompt": "p3", "worktree": true},
		},
	})
	res, _ := reg.Execute(context.Background(), toolrunner.ToolCall{
		Name:  "free_fork",
		Input: string(input),
	})
	if res.Error != "" {
		t.Fatalf("tool error: %s", res.Error)
	}
	var out struct {
		SpawnedCount int      `json:"spawned_count"`
		AgentIDs     []string `json:"agent_ids"`
	}
	if err := json.Unmarshal([]byte(res.Output), &out); err != nil {
		t.Fatalf("unmarshal: %v, output=%s", err, res.Output)
	}
	if out.SpawnedCount != 3 {
		t.Errorf("spawned_count = %d, want 3", out.SpawnedCount)
	}
	if len(out.AgentIDs) != 3 {
		t.Errorf("agent_ids len = %d, want 3", len(out.AgentIDs))
	}
}

// T: D4-S11-A02-T02
// 第 2 次 factory.Create 失败 → tool 返回 error，没有 spawned_count。
func TestFreeForkTool_FactoryFailureRollback(t *testing.T) {
	factory := &stubFreeforkFactory{failOn: 2}
	setGlobalForkerForTest(t, freefork.NewDefaultForker(freefork.ForkerDeps{
		Factory:       factory,
		DefaultConfig: multiagent.AgentConfig{Mode: multiagent.ModeDefault},
	}))

	reg := toolrunner.NewToolRegistry()
	if err := toolrunner.RegisterFreeForkTool(reg); err != nil {
		t.Fatalf("RegisterFreeForkTool: %v", err)
	}

	input, _ := json.Marshal(map[string]any{
		"parent_session": "sess-1",
		"requests": []map[string]any{
			{"name": "r1", "prompt": "p1"},
			{"name": "r2", "prompt": "p2"},
			{"name": "r3", "prompt": "p3"},
		},
	})
	res, _ := reg.Execute(context.Background(), toolrunner.ToolCall{
		Name:  "free_fork",
		Input: string(input),
	})
	if !strings.Contains(res.Error, "factory failed on request") {
		t.Errorf("expected factory failure error, got %q", res.Error)
	}
	if strings.Contains(res.Output, "spawned_count") {
		t.Errorf("expected no spawned_count on failure, got %s", res.Output)
	}
}

// T: D4-S11-A02-T01 边界
// requests count > 5 → tool 拒绝
func TestFreeForkTool_MaxRequestsLimit(t *testing.T) {
	factory := &stubFreeforkFactory{}
	setGlobalForkerForTest(t, freefork.NewDefaultForker(freefork.ForkerDeps{
		Factory:       factory,
		DefaultConfig: multiagent.AgentConfig{Mode: multiagent.ModeDefault},
	}))
	reg := toolrunner.NewToolRegistry()
	if err := toolrunner.RegisterFreeForkTool(reg); err != nil {
		t.Fatalf("RegisterFreeForkTool: %v", err)
	}

	requests := make([]map[string]any, 6)
	for i := range requests {
		requests[i] = map[string]any{"name": "r" + itoa(i+1), "prompt": "p"}
	}
	input, _ := json.Marshal(map[string]any{
		"parent_session": "sess-1",
		"requests":       requests,
	})
	res, _ := reg.Execute(context.Background(), toolrunner.ToolCall{
		Name:  "free_fork",
		Input: string(input),
	})
	if !strings.Contains(res.Error, "requests count must be in [1,5]") {
		t.Errorf("expected count limit error, got %q", res.Error)
	}
}

// T: global forker 未注入 → tool 拒绝（not initialized 错误）
func TestFreeForkTool_GlobalForkerNil(t *testing.T) {
	setGlobalForkerForTest(t, nil)
	reg := toolrunner.NewToolRegistry()
	if err := toolrunner.RegisterFreeForkTool(reg); err != nil {
		t.Fatalf("RegisterFreeForkTool: %v", err)
	}
	input, _ := json.Marshal(map[string]any{
		"parent_session": "sess-1",
		"requests":       []map[string]any{{"name": "r1"}},
	})
	res, _ := reg.Execute(context.Background(), toolrunner.ToolCall{
		Name:  "free_fork",
		Input: string(input),
	})
	if !strings.Contains(res.Error, "global forker not initialized") {
		t.Errorf("expected not initialized error, got %q", res.Error)
	}
}
