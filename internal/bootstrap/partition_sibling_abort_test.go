// T: D7-S9-A50-T26 — BashSiblingAbortController 集成到 executeOneBatch (DM-20260702-009 PR-F).
//
// 验证 executeOneBatch 在 parallel 分支创建 per-batch controller,
// watched tool (bash) 失败时正确触发 AbortSiblings, 其它 watched
// siblings 在 select ctx.Done() 后返回 synthetic cancel result.
//
// 测试策略: 用 fake ExecuteFunc 模拟 bash 行为 — call_a 立即返错,
// call_b 阻塞在 ctx.Done(). 验证 call_b 看到 cancel 并提前返回.
package bootstrap

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/orchestration/sessionorchestrator"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// bashSurface 是一个 minimal ToolSurface, 用于让 partition 路径识别
// "bash" 为 watched tool (ConcurrencySafe=true). 实际 ExecuteFunc 由
// 测试自己提供, 这里 surface 只提供 spec.
type bashSurface struct{}

func (bashSurface) Name() string { return "bash-fixture" }

func (bashSurface) Tools(_ context.Context, _, _ string) []contracts.ToolSpec {
	return []contracts.ToolSpec{
		{Name: "bash", ConcurrencySafe: true, Risk: types.RiskLevelHigh},
	}
}

func (bashSurface) RiskLevel(_ string) types.RiskLevel { return types.RiskLevelHigh }

func (bashSurface) InterruptBehavior(_ string) contracts.InterruptMode {
	return contracts.InterruptBlock
}

func (bashSurface) CheckPermission(_ context.Context, _ contracts.ToolSpec, _ json.RawMessage) contracts.Decision {
	return contracts.DecisionAllow
}

func (bashSurface) IsConcurrencySafe(_ json.RawMessage) bool { return true }

func (bashSurface) ToAutoClassifierInput(_ json.RawMessage) string { return "" }

// Execute 满足 contracts.ToolSurface 接口 — 测试中不调用 (实际
// 执行走测试自带的 exec lambda), 这里只返一个占位 result.
func (bashSurface) Execute(_ context.Context, name, _, _ string) (*contracts.ToolResult, error) {
	return &contracts.ToolResult{Output: "bash-fixture:" + name}, nil
}

// executeOneBatch not exported; we test via ExecuteBatches with a
// bash-only call set so partitionToolCalls produces a single safe batch.

// AC12 核心: 同 batch 内 1 bash 失败 → 其它 bash siblings 收到 ctx.Done().
//
// 时序: 用 ready-barrier 让 A 等 B 注册后再返错. 否则 A 可能在 B
// Register 之前完成, AbortSiblings 看到 registry 里只有 A, 直接 no-op.
func TestExecuteBatches_BashSiblingAbort(t *testing.T) {
	lookup := BuildSurfaceLookup([]contracts.ToolSurface{bashSurface{}})

	bReady := make(chan struct{})
	var bSawCancel atomic.Bool

	exec := func(ctx context.Context, call llmgateway.ToolCall) sessionorchestrator.ToolResult {
		switch call.ID {
		case "call_a":
			// 等 B 进入 exec (B 已 Register) — 保证 AbortSiblings 时
			// B 在 registry 里.
			select {
			case <-bReady:
			case <-time.After(1 * time.Second):
				// B 没起来; 测试会失败, 不再 block.
			}
			// A 失败 — 触发 AbortSiblings("call_a", ...).
			return sessionorchestrator.ToolResult{
				ToolCallID: call.ID,
				Error:      "exit 1",
			}
		case "call_b":
			// Signal: 我已 Register (exec 入口 = Register 已完成).
			close(bReady)
			// B 模拟 bash exec.CommandContext 的 select ctx.Done().
			select {
			case <-ctx.Done():
				bSawCancel.Store(true)
				return sessionorchestrator.ToolResult{
					ToolCallID: call.ID,
					Error:      ctx.Err().Error(),
				}
			case <-time.After(3 * time.Second):
				return sessionorchestrator.ToolResult{
					ToolCallID: call.ID,
					Error:      "B did not see ctx.Done() within 3s",
				}
			}
		}
		return sessionorchestrator.ToolResult{ToolCallID: call.ID}
	}

	calls := []llmgateway.ToolCall{
		{ID: "call_a", Name: "bash", Input: `{"command":"false"}`},
		{ID: "call_b", Name: "bash", Input: `{"command":"sleep 5"}`},
	}

	results := ExecuteBatches(context.Background(), calls, lookup, exec, 0)

	if len(results) != 2 {
		t.Fatalf("len(results)=%d, want 2", len(results))
	}

	// A 的结果应该返 exit 1.
	if results[0].ToolCallID != "call_a" {
		t.Errorf("results[0].ToolCallID=%q, want %q (order broken)", results[0].ToolCallID, "call_a")
	}
	if results[0].Error != "exit 1" {
		t.Errorf("results[0].Error=%q, want %q", results[0].Error, "exit 1")
	}

	// B 的结果应该看到 ctx.Done() 并返 cancel error.
	if results[1].ToolCallID != "call_b" {
		t.Errorf("results[1].ToolCallID=%q, want %q (order broken)", results[1].ToolCallID, "call_b")
	}
	if results[1].Error == "" {
		t.Errorf("results[1].Error is empty; B should see cancel")
	}
	if !bSawCancel.Load() {
		t.Error("B goroutine must observe ctx.Done() (controller abort propagation broken)")
	}

	// 进一步: B 的 error 应该提及 cancel/deadline (context.Canceled 字符串).
	if !containsContextError(results[1].Error) {
		t.Errorf("results[1].Error=%q, want to contain context.Canceled or similar", results[1].Error)
	}
}

// 不带 watched tool 的 batch: controller 被创建但没人 register,
// AbortSiblings 永远不被调用. 验证现有 AC15-AC21 invariant 不受影响.
func TestExecuteBatches_NonWatchedBatch_NoAbort(t *testing.T) {
	lookup := BuildSurfaceLookup([]contracts.ToolSurface{&fixedSurface{name: "s"}})

	hits := new(int32)
	s := &fixedSurface{name: "s", hits: hits}
	calls := []llmgateway.ToolCall{
		{ID: "a", Name: "safe_tool", Input: `{}`},
		{ID: "b", Name: "safe_tool", Input: `{}`},
	}
	exec := func(_ context.Context, call llmgateway.ToolCall) sessionorchestrator.ToolResult {
		res, _ := s.Execute(context.Background(), call.Name, call.Input, "")
		return sessionorchestrator.ToolResult{ToolCallID: call.ID, Output: res.Output, Error: res.Error}
	}
	results := ExecuteBatches(context.Background(), calls, lookup, exec, 0)

	if len(results) != 2 {
		t.Fatalf("len(results)=%d, want 2", len(results))
	}
	for i, r := range results {
		if r.Error != "" {
			t.Errorf("results[%d].Error=%q, want empty (no abort expected for non-watched)", i, r.Error)
		}
	}
}

// 自 abort: watched call AbortSiblings(self) — self 的 ctx 不被取消.
// 在 controller 单元测试已经覆盖; 这里只验证 integration path 不引入
// 副作用.
//
// 时序注意: call_a 的 exec 必须先 sleep 一段以保证 call_b 来得及
// register (否则 A 完成时 B 还没 register, AbortSiblings 看到 registry
// 里只有 A, 直接 no-op).
func TestExecuteBatches_SelfAbort_NotCancelsSelf(t *testing.T) {
	lookup := BuildSurfaceLookup([]contracts.ToolSurface{bashSurface{}})

	var bSawCancel atomic.Bool
	exec := func(ctx context.Context, call llmgateway.ToolCall) sessionorchestrator.ToolResult {
		switch call.ID {
		case "call_a":
			// A 先 sleep 让 B 注册, 然后失败 → AbortSiblings(self="call_a").
			time.Sleep(50 * time.Millisecond)
			return sessionorchestrator.ToolResult{
				ToolCallID: call.ID,
				Error:      "self-failure",
			}
		case "call_b":
			// B 是真正的 sibling, 会被 cancel.
			select {
			case <-ctx.Done():
				bSawCancel.Store(true)
				return sessionorchestrator.ToolResult{ToolCallID: call.ID, Error: ctx.Err().Error()}
			case <-time.After(3 * time.Second):
				return sessionorchestrator.ToolResult{ToolCallID: call.ID, Error: "B timeout"}
			}
		}
		return sessionorchestrator.ToolResult{ToolCallID: call.ID}
	}

	calls := []llmgateway.ToolCall{
		{ID: "call_a", Name: "bash", Input: `{}`},
		{ID: "call_b", Name: "bash", Input: `{}`},
	}
	results := ExecuteBatches(context.Background(), calls, lookup, exec, 0)

	if results[0].Error != "self-failure" {
		t.Errorf("results[0].Error=%q, want %q", results[0].Error, "self-failure")
	}
	if !bSawCancel.Load() {
		t.Error("B (real sibling) must see ctx.Done() even when self aborts")
	}
}

// containsContextError is a small helper to assert error string contains
// a context-related marker. We avoid errors.Is here because r.Error is a
// string (not an error type), so we test the rendered message instead.
func containsContextError(s string) bool {
	if s == "" {
		return false
	}
	// context.Canceled.Error() == "context canceled"
	// context.DeadlineExceeded.Error() == "context deadline exceeded"
	return contains(s, "context canceled") ||
		contains(s, "context deadline exceeded") ||
		contains(s, "canceled") ||
		contains(s, "cancelled")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > len(substr) && indexOf(s, substr) >= 0))
}

func indexOf(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// 抑制 unused import warning when refactors remove sync/sync/atomic.
var (
	_ = sync.WaitGroup{}
	_ atomic.Bool
)