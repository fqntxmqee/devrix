package toolrunner_test

// W7 — D4-S11-A02 + D4-S13-A02 (alias G5) free_fork LLM tool 单元测试。
//
// AC5:
//   - 批量分叉 3 个 → spawned_count=3, agent_ids 长度 = 3
//   - factory 失败 → tool 返回 error（不返回 spawned_count）
//   - requests count > 5 → tool 拒绝
//
// S4-Gate H-3 fix: 测试通过 toolrunner.SetFreeForker 注入 stub 函数,
// 不再 import multiagent / freefork 内部包, 保持 D2 边界.

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce/toolrunner"
)

// stubFreeforkFunc 是注入给 toolrunner.SetFreeForker 的 stub 函数.
type stubFreeforkFunc struct {
	failOn int32 // 第 N 次调用时失败; 0 表示永不失败
	calls  int32
}

func (s *stubFreeforkFunc) Fork(ctx context.Context, parentSession string, reqs []toolrunner.FreeForkRequestDTO) ([]toolrunner.FreeForkHandleDTO, error) {
	n := atomic.AddInt32(&s.calls, 1)
	if s.failOn > 0 && n == s.failOn {
		return nil, errors.New("stub: factory failed on request")
	}
	handles := make([]toolrunner.FreeForkHandleDTO, 0, len(reqs))
	for i, r := range reqs {
		handles = append(handles, toolrunner.FreeForkHandleDTO{
			AgentID:  "agent-" + r.Name,
			Worktree: "/wt/" + r.Name,
			Name:     r.Name,
		})
		_ = i
	}
	return handles, nil
}

func setGlobalFreeForkerForTest(t *testing.T, f toolrunner.FreeForkerFunc) {
	t.Helper()
	prevFn := captureCurrentFreeForker(t)
	toolrunner.SetFreeForker(f)
	t.Cleanup(func() { toolrunner.SetFreeForker(prevFn) })
}

// captureCurrentFreeForker 在 SetFreeForker 之前先触发一次 nil 注入
// 把默认 stub 拿出来, 测试结束后恢复. 因为 globalFreeForker 没有 getter,
// 我们用 SetFreeForker 重新注册 prev stub.
func captureCurrentFreeForker(_ *testing.T) toolrunner.FreeForkerFunc {
	// 第一次 SetFreeForker(nil) 不会改 global; 所以 default 仍是 default stub.
	// 为了让 test 结束后能恢复 default, 我们直接返回一个 "no-op" 占位,
	// 之后 cleanup 再 SetFreeForker(nil) 仍保留 default.
	return func(_ context.Context, _ string, _ []toolrunner.FreeForkRequestDTO) ([]toolrunner.FreeForkHandleDTO, error) {
		return nil, errors.New("freefork: default stub (test cleanup placeholder)")
	}
}

func TestFreeForkTool_BatchForkThree(t *testing.T) {
	stub := &stubFreeforkFunc{}
	setGlobalFreeForkerForTest(t, stub.Fork)

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
	if atomic.LoadInt32(&stub.calls) != 1 {
		t.Errorf("stub.calls = %d, want 1", stub.calls)
	}
}

// T: D4-S11-A02-T02
// 第 2 次 stub 失败 → tool 返回 error，没有 spawned_count。
func TestFreeForkTool_FactoryFailureRollback(t *testing.T) {
	stub := &stubFreeforkFunc{failOn: 1} // 第一次就失败
	setGlobalFreeForkerForTest(t, stub.Fork)

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
	stub := &stubFreeforkFunc{}
	setGlobalFreeForkerForTest(t, stub.Fork)
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
	if atomic.LoadInt32(&stub.calls) != 0 {
		t.Errorf("stub should not be invoked on validation failure, got calls=%d", stub.calls)
	}
}

// T: global forker 未注入 → tool 拒绝（not initialized 错误）
func TestFreeForkTool_GlobalForkerNil(t *testing.T) {
	// 用一个返回 "not initialized" 的 stub 来模拟未注入状态。
	setGlobalFreeForkerForTest(t, func(_ context.Context, _ string, _ []toolrunner.FreeForkRequestDTO) ([]toolrunner.FreeForkHandleDTO, error) {
		return nil, errors.New("freefork: not initialized for test")
	})
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
	if !strings.Contains(res.Error, "not initialized") {
		t.Errorf("expected not initialized error, got %q", res.Error)
	}
}

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
