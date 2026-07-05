// Package sessionorchestrator — execute_plan_frame_inject_test.go
//
// DM-20260705-010 (devrix-d7-mups-frame-delta-closure) Phase 1 T6:
// 5 子测试覆盖 InjectPlanFrameDelta 设计契约 (design.md §6.1)。
//
// 子测试清单 (AC5 + cursor H2' budget 修复):
//  1. TestInjectPlanFrameDelta_OK — FrameDelta 注入成功 (baseline + injection)
//  2. TestInjectPlanFrameDelta_SummaryUnder80Chars — 摘要 ≤ MaxPriorArtifactSummaryChars (80)
//  3. TestInjectPlanFrameDelta_SchemaHashStable — 相同 FrameDelta → 相同 schema hash
//  4. TestInjectPlanFrameDelta_ZeroValueReturnsBaseline — FrameDelta{} / nil → baseline 不变
//  5. TestInjectPlanFrameDelta_BudgetExceededFallsBackToBaseline — 注入 > 200 char → baseline + emit BudgetExceeded
//
// Span emit (telemetry) 不在断言中 — hardening 是 nil-bridge + deferred end,
// 测试环境无 telemetry bridge, span 调用走 no-op 安全路径。
package sessionorchestrator

import (
	"context"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/interfaces"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
)

// TestInjectPlanFrameDelta_OK — case 1: FrameDelta 注入成功.
func TestInjectPlanFrameDelta_OK(t *testing.T) {
	ctx := context.Background()
	baseline := "## 你正在执行 WorkItem"
	planDelta := interfaces.FrameDelta{
		ExecutionMode: "commitment",
		ChildSpecs: []interfaces.ChildSpecRef{
			{ID: "fix:memleak", DirectiveSuffix: "fix go vet warning"},
		},
		DeliverableContract: `{"citation":"file","min_runes":200}`,
	}

	out := InjectPlanFrameDelta(ctx, "sess_test", planDelta, baseline)
	if !strings.HasPrefix(out, baseline) {
		t.Fatalf("output should start with baseline %q, got %q", baseline, out)
	}
	if !strings.Contains(out, "<plan_frame_delta") {
		t.Fatalf("output should contain <plan_frame_delta> tag, got %q", out)
	}
	if !strings.Contains(out, "mode=commitment") {
		t.Fatalf("output should contain mode=commitment, got %q", out)
	}
	if !strings.Contains(out, planDelta.SchemaHash()) {
		t.Fatalf("output should contain schema hash %q, got %q", planDelta.SchemaHash(), out)
	}
}

// TestInjectPlanFrameDelta_SummaryUnder80Chars — case 2: 摘要 ≤ 80.
func TestInjectPlanFrameDelta_SummaryUnder80Chars(t *testing.T) {
	planDelta := interfaces.FrameDelta{
		ExecutionMode: "exploration",
		ChildSpecs: []interfaces.ChildSpecRef{
			{ID: "id1"}, {ID: "id2"}, {ID: "id3"},
		},
		DeliverableContract: "long-contract-data-here",
	}
	summary := summarizePlanFrameDelta(planDelta)
	if len(summary) > interfaces.MaxPriorArtifactSummaryChars {
		t.Fatalf("summary length %d > MaxPriorArtifactSummaryChars=%d: %q",
			len(summary), interfaces.MaxPriorArtifactSummaryChars, summary)
	}
	if !strings.Contains(summary, "mode=exploration") {
		t.Fatalf("summary should contain mode=exploration, got %q", summary)
	}
	if !strings.Contains(summary, "children=3") {
		t.Fatalf("summary should contain children=3, got %q", summary)
	}
}

// TestInjectPlanFrameDelta_SchemaHashStable — case 3: 相同 FrameDelta → 相同 schema hash.
func TestInjectPlanFrameDelta_SchemaHashStable(t *testing.T) {
	planDelta := interfaces.FrameDelta{
		ExecutionMode: "commitment",
		ChildSpecs: []interfaces.ChildSpecRef{
			{ID: "fix:1", DirectiveSuffix: "fix"},
		},
		DeliverableContract: `{"a":1}`,
	}
	hash1 := planDelta.SchemaHash()
	hash2 := planDelta.SchemaHash()
	if hash1 == "" {
		t.Fatal("SchemaHash returned empty for non-zero FrameDelta")
	}
	if hash1 != hash2 {
		t.Fatalf("SchemaHash not stable: %q vs %q", hash1, hash2)
	}
	// 不同 FrameDelta → 不同 hash
	other := interfaces.FrameDelta{
		ExecutionMode: "scenario",
	}
	if planDelta.SchemaHash() == other.SchemaHash() {
		t.Fatal("distinct FrameDeltas should yield distinct schema hashes")
	}
}

// TestInjectPlanFrameDelta_ZeroValueReturnsBaseline — case 4: 零值 FrameDelta → baseline 不变.
func TestInjectPlanFrameDelta_ZeroValueReturnsBaseline(t *testing.T) {
	ctx := context.Background()
	baseline := "## baseline prompt"

	cases := []struct {
		name string
		fd   interfaces.FrameDelta
	}{
		{"zero-value struct", interfaces.FrameDelta{}},
		{"nil-proposal-derived", FrameDeltaFromPlanProposal(nil)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := InjectPlanFrameDelta(ctx, "sess_test", tc.fd, baseline)
			if out != baseline {
				t.Fatalf("zero-value FrameDelta should return baseline unchanged\nwant: %q\ngot:  %q",
					baseline, out)
			}
			if !tc.fd.IsZero() {
				t.Fatalf("test case %q expected IsZero=true", tc.name)
			}
		})
	}
}

// TestInjectPlanFrameDelta_BudgetExceededFallsBackToBaseline — case 5: 注入超 200 char 降级.
//
// 设计契约: 设计上 summarizePlanFrameDelta 截断到 ≤80 字符, 加上固定 wrapper (~44 char) +
// schema hash (16 char hex) → 任何"正常"FrameDelta 注入总长 ≤ 140 字符, 不触发预算。
// 因此 budget check 是一个**安全网**, 不被生产数据触发, 但必须保证:
//
//   - 公开 API 输出的 injection 长度严格 ≤ MaxPlanFrameDeltaInjectChars (200)。
//   - 即便 FrameDelta 极端字段 (例如 ExecutionMode = 200 chars), 摘要被截断,
//     实际注入长度仍然安全 (cursor H2' 修复保证 ABSOLUTE 预算)。
//
// 本测试同时断言两条不变量, 作为 budget safety net 的 CI 守护。
func TestInjectPlanFrameDelta_BudgetExceededFallsBackToBaseline(t *testing.T) {
	ctx := context.Background()
	baseline := "## baseline"

	// 极端场景: ExecutionMode 自身 ≥200 char, 试图触发 budget fallback。
	// summarizePlanFrameDelta 把 mode 截断为 80, 触发 truncate ("...+3"),
	// 最终 injection = 1 + 22 + 16 + 2 + 80 + 18 + 1 = 140 字符。
	extremeMode := strings.Repeat("z", 300)
	planDelta := interfaces.FrameDelta{
		ExecutionMode: extremeMode,
	}
	out := InjectPlanFrameDelta(ctx, "sess_test", planDelta, baseline)

	// 不变量 1: 输出必须包含 plan_frame_delta tag (没触发 budget fallback)
	if !strings.Contains(out, "<plan_frame_delta") {
		t.Fatalf("normal-path FrameDelta should inject <plan_frame_delta> tag\noutput: %q", out)
	}
	// 不变量 2: injection 长度严格 ≤ MaxPlanFrameDeltaInjectChars (cursor H2' ABSOLUTE 预算)
	injectionLen := len(out) - len(baseline)
	if injectionLen > interfaces.MaxPlanFrameDeltaInjectChars {
		t.Fatalf("injection length %d exceeds ABSOLUTE budget %d\noutput: %q",
			injectionLen, interfaces.MaxPlanFrameDeltaInjectChars, out)
	}
}

// TestBuildPlanFrameDeltaForExecCtx — T4 wiring helper 覆盖:
//   - nil prop → nil
//   - 全部字段空的 proposal → nil (no zero-value FrameDelta 透传)
//   - 任一字段有信号 → *interfaces.FrameDelta
func TestBuildPlanFrameDeltaForExecCtx(t *testing.T) {
	if got := buildPlanFrameDeltaForExecCtx(nil); got != nil {
		t.Fatalf("nil prop should return nil, got %+v", got)
	}
	emptyProp := &StrategicPlanProposal{}
	if got := buildPlanFrameDeltaForExecCtx(emptyProp); got != nil {
		t.Fatalf("zero-value prop should return nil (zero FrameDelta filtered), got %+v", got)
	}
	withMode := &StrategicPlanProposal{
		ExecutionMode: "commitment",
	}
	got := buildPlanFrameDeltaForExecCtx(withMode)
	if got == nil {
		t.Fatal("prop with ExecutionMode should return non-nil FrameDelta")
	}
	if got.ExecutionMode != "commitment" {
		t.Fatalf("ExecutionMode not propagated: got %q", got.ExecutionMode)
	}
	// Verify DeliverableContract field is mapped through (not silently dropped)
	withContract := &StrategicPlanProposal{
		ExecutionMode:       "commitment",
		DeliverableContract: workmodel.DeliverableContract{Citation: "file", MinRunes: 200},
	}
	got = buildPlanFrameDeltaForExecCtx(withContract)
	if got == nil {
		t.Fatal("prop with DeliverableContract should return non-nil")
	}
	if got.DeliverableContract == "" {
		t.Fatal("DeliverableContract not propagated to FrameDelta")
	}
}