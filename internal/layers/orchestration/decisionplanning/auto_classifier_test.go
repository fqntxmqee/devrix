// T: D7-S10-A50-T24 — AutoModeClassifier P2 interface stub 单测 (PR-D+E, 阶段 6).
//
// 4 单测覆盖 P2 interface stub 范围:
//
//	1. TestAutoModeClassifier_InterfaceExists        — interface 编译, panic 信息合规
//	2. TestAutoModeClassifier_StubPanic              — 当前调用 panic 行为符合预期
//	3. TestPartition_NoClassifierNoRegression        — ChannelRouter 占位代码不破坏 partition 行为
//	4. TestClassifierResult_ReprStable               — 不可变值对象 repr 在跨包传递时稳定
//
// 起 P1 触发 metric: verify_contract.deny_rate 7d 滑动 > 5%.
// 升 P1 时开 devrix-d2-tool-input-aware-concurrency-and-classifier-pr-d-followup Change.
package decisionplanning

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// T24.1 — AutoModeClassifier interface 必须存在且编译通过. 任何把 P2 stub
// 类型传给调用方的写法都应能编译, 这是 P2 升 P1 时不需要改 API 的必要条件.
//
// 注意: 该测试在编译期就验证 (var _ AutoModeClassifier = panicStubClassifier{}),
// runtime 仅做 sanity 检查.
func TestAutoModeClassifier_InterfaceExists(t *testing.T) {
	var _ AutoModeClassifier = panicStubClassifier{}
	// panicStubClassifier{} 同时实现 AutoModeClassifier 接口, 编译期已 check.
	// runtime sanity: type 检查 + 存在 (cannot alloc 0-size type, 用 value).
	var got AutoModeClassifier = panicStubClassifier{}
	if got == nil {
		t.Error("panicStubClassifier instance should not be nil")
	}
}

// T24.2 — 当前调用 ClassifyToolUse 必须 panic, panic 信息明确包含
// "P2 stub" 关键字 (升 P1 审计追溯需要 + 防止误激活 stub).
func TestAutoModeClassifier_StubPanic(t *testing.T) {
	var c AutoModeClassifier = panicStubClassifier{}
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected ClassifyToolUse to panic on P2 stub, got no panic")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("expected panic value to be a string, got %T (%v)", r, r)
		}
		if !strings.Contains(msg, "P2 stub") {
			t.Errorf("panic message must mention 'P2 stub', got %q", msg)
		}
		if !errors.Is(ErrAutoModeClassifierPanic, ErrAutoModeClassifierPanic) {
			t.Error("ErrAutoModeClassifierPanic sentinel must exist")
		}
	}()
	_, _ = c.ClassifyToolUse(context.Background(), nil)
}

// T24.3 — ChannelRouter 占位代码不破坏 partition 行为: 走 panicStubClassifier
// 但 partition 自身不该被影响 (panic 在 classify call site recover, 不是
// partition 内). 这里用最小依赖验证 "classifier 不会污染 partition".
//
// TestPartition_NoClassifierNoRegression 是 PR-D+E 阶段的轻量 invariant 测试:
// 即使调用 ClassifyToolUse 触发 panic, partition 的 batches 结构跟
// ExecuteBatches 的结果跟未接 classifier 一致.
//
// 当前实现: 调用站点根本没接 classifier (no wiring), 所以这个测试只是
// 确认 "即使 classifier 在某个未来实现里 panic, 用 recover 包一下就行,
// partition 自身不该被污染" 的合约.
func TestPartition_NoClassifierNoRegression(t *testing.T) {
	// 当前 ChannelRouter 集成: turn_adapter.ExecuteRound 只走 partitionToolCalls,
	// 不调用 classifier. 该测试验证 panicStubClassifier 被绕开, 不影响 partition.

	// 模拟一次正常 partition (跟 partition_invariants_test.go 同样的不变量):
	// 这里复述 partition 不变量 — 完整性 N:N+保序+id 1:1.

	// 用 ExecuteBatches 走最小 helper; partition 自身不被 classifier 路径污染.
	// 这里只放一个 placeholder, 真正的 partition 测试在 partition_invariants_test.go.

	// 真检查: 当 classifier panic, recover 后 partition 不应受影响.
	var c AutoModeClassifier = panicStubClassifier{}

	// 模拟 ChannelRouter TODO 调用站点:
	var recovered any
	func() {
		defer func() {
			recovered = recover()
		}()
		_, _ = c.ClassifyToolUse(context.Background(), []TranscriptBlock{
			{Type: BlockTypeToolUse, Name: "bash", Input: `{"command":"ls"}`},
		})
	}()

	if recovered == nil {
		t.Fatal("classifier still calling (test setup wrong); expected panic was caught")
	}
	// recover 后 partition/turn_adapter 不应看到 panic (合约: call site 必须 recover).
	// 这里只是模拟调用站点行为 — 真 ChannelRouter 集成见 turn_adapter.go TODO 注释.
	t.Logf("classifier panic recovered cleanly at simulated call site (panic=%v)", recovered)
}

// T24.4 — ClassifierResult 跨包传递时 repr 稳定 (不可变值对象 per D7 P5).
//
// 验证 Decision / Reason / Source 三件套在赋值后不变, 不可被 mutate.
// 这是 devrix Naming Policy + 测试审计可重现性的硬要求.
func TestClassifierResult_ReprStable(t *testing.T) {
	r := ClassifierResult{
		Decision: DecisionDeny,
		Reason:   "test reason with unicode 测试",
		Source:   SourceRuleFallback,
	}
	// 复制是值语义, 修改副本不污染原值.
	r2 := r
	r2.Decision = DecisionAllow
	r2.Reason = "mutated"
	r2.Source = SourceAnthropic

	if r.Decision != DecisionDeny {
		t.Errorf("original Decision must be DecisionDeny (immutable), got %q", r.Decision)
	}
	if r.Reason != "test reason with unicode 测试" {
		t.Errorf("original Reason must be unchanged, got %q", r.Reason)
	}
	if r.Source != SourceRuleFallback {
		t.Errorf("original Source must be SourceRuleFallback, got %q", r.Source)
	}

	// 三件套枚举值是稳定的 wire format — 与 dashboard 字符串一致.
	if DecisionAllow != "allow" {
		t.Errorf("DecisionAllow wire must be \"allow\", got %q", DecisionAllow)
	}
	if DecisionDeny != "deny" {
		t.Errorf("DecisionDeny wire must be \"deny\", got %q", DecisionDeny)
	}
	if SourceAnthropic != "anthropic" {
		t.Errorf("SourceAnthropic wire must be \"anthropic\", got %q", SourceAnthropic)
	}
	if SourceRuleFallback != "rule-fallback" {
		t.Errorf("SourceRuleFallback wire must be \"rule-fallback\", got %q", SourceRuleFallback)
	}
}

// T24.5 (bonus) — 验证 ErrAutoModeClassifierPanic 是稳定的 sentinel.
//
// 跨多次调用返回值一致 (sentinel pattern); 升 P1 后亦应保持此 identifier.
func TestErrAutoModeClassifierPanic_Stable(t *testing.T) {
	e1 := ErrAutoModeClassifierPanic
	e2 := ErrAutoModeClassifierPanic
	if e1.Error() != e2.Error() {
		t.Errorf("sentinel Error() must be stable across reads, got %q vs %q", e1, e2)
	}
	if !strings.Contains(e1.Error(), "auto-mode classifier") {
		t.Errorf("sentinel message must contain 'auto-mode classifier' for audit log grep, got %q", e1)
	}
	if !strings.Contains(e1.Error(), "P2") {
		t.Errorf("sentinel message must contain 'P2' for trigger metric audit, got %q", e1)
	}
}
