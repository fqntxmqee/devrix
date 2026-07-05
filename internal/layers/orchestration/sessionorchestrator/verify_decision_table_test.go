package sessionorchestrator

import (
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/plan"
	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/types"
)

// T: D7-S10-A101-T01 (DM-20260705-005) — 12 detector 命名函数单测
// 覆盖每个 detector 的 fire true / fire false 1-2 case。

func TestDetectNilArtifact(t *testing.T) {
	if !detectNilArtifact(nil, &verifyContext{}) {
		t.Fatal("nil artifact should fire")
	}
	art := &wavescheduler.Artifact{TaskID: "x"}
	if detectNilArtifact(art, &verifyContext{}) {
		t.Fatal("non-nil artifact should not fire")
	}
}

func TestDetectMaxItersPartial(t *testing.T) {
	// fire: Error + max_iters + tool_calls > 0
	fire := &wavescheduler.Artifact{
		TaskID: "x", Error: "x",
		Metadata: map[string]any{"stop_reason": "max_iters", "tool_calls": 3},
	}
	if !detectMaxItersPartial(fire, &verifyContext{art: fire}) {
		t.Fatal("max_iters + tool_calls>0 should fire")
	}
	// not fire: max_iters + tool_calls == 0
	noFire := &wavescheduler.Artifact{
		TaskID: "x", Error: "x",
		Metadata: map[string]any{"stop_reason": "max_iters", "tool_calls": 0},
	}
	if detectMaxItersPartial(noFire, &verifyContext{art: noFire}) {
		t.Fatal("max_iters + tool_calls==0 should not fire")
	}
	// not fire: no Error
	clean := &wavescheduler.Artifact{TaskID: "x"}
	if detectMaxItersPartial(clean, &verifyContext{art: clean}) {
		t.Fatal("no Error should not fire")
	}
}

func TestDetectExecuteFail(t *testing.T) {
	// fire: Error 但不是 max_iters
	fire := &wavescheduler.Artifact{TaskID: "x", Error: "boom"}
	if !detectExecuteFail(fire, &verifyContext{art: fire}) {
		t.Fatal("Error alone should fire execute-fail")
	}
	// not fire: Error + max_iters + tool_calls > 0 (被 max-iters-partial 接管)
	noFire := &wavescheduler.Artifact{
		TaskID: "x", Error: "x",
		Metadata: map[string]any{"stop_reason": "max_iters", "tool_calls": 3},
	}
	if detectExecuteFail(noFire, &verifyContext{art: noFire}) {
		t.Fatal("max_iters+tool>0 should not fire execute-fail (delegated)")
	}
	// not fire: clean artifact
	clean := &wavescheduler.Artifact{TaskID: "x"}
	if detectExecuteFail(clean, &verifyContext{art: clean}) {
		t.Fatal("clean artifact should not fire execute-fail")
	}
}

func TestDetectSideEffectRolledBack(t *testing.T) {
	fire := &wavescheduler.Artifact{TaskID: "x", SideEffectStatus: types.SideEffectRolledBack}
	if !detectSideEffectRolledBack(fire, &verifyContext{art: fire}) {
		t.Fatal("RolledBack should fire")
	}
	noFire := &wavescheduler.Artifact{TaskID: "x", SideEffectStatus: types.SideEffectCommitted}
	if detectSideEffectRolledBack(noFire, &verifyContext{art: noFire}) {
		t.Fatal("Committed should not fire")
	}
}

func TestDetectSideEffectUncertain(t *testing.T) {
	for _, s := range []types.SideEffectStatus{types.SideEffectUnknown, types.SideEffectInflight} {
		art := &wavescheduler.Artifact{TaskID: "x", SideEffectStatus: s}
		if !detectSideEffectUncertain(art, &verifyContext{art: art}) {
			t.Fatalf("%v should fire", s)
		}
	}
	noFire := &wavescheduler.Artifact{TaskID: "x", SideEffectStatus: types.SideEffectCommitted}
	if detectSideEffectUncertain(noFire, &verifyContext{art: noFire}) {
		t.Fatal("Committed should not fire")
	}
}

func TestDetectUserGate(t *testing.T) {
	fire := &wavescheduler.Artifact{TaskID: "x", Summary: "I've sent scope questions. Awaiting your selection."}
	if !detectUserGate(fire, &verifyContext{art: fire, id: fire.TaskID}) {
		t.Fatal("user-gate phrase should fire")
	}
	noFire := &wavescheduler.Artifact{TaskID: "x", Summary: "P0: nil deref in foo.go:42"}
	if detectUserGate(noFire, &verifyContext{art: noFire, id: noFire.TaskID}) {
		t.Fatal("non-user-gate summary should not fire")
	}
}

func TestDetectScopeOnlyDeliverable(t *testing.T) {
	item := &workmodel.WorkItem{
		Kind: workmodel.WorkKindGoal,
		ScopeContract: &workmodel.ScopeContract{OpenQuestions: []string{"q1"}},
	}
	pl := &plan.Plan{Kind: plan.ExplorationPlan}
	art := &wavescheduler.Artifact{
		TaskID:  "x",
		Summary: "done\n<scope_contract>{}</scope_contract>", // no file:line
	}
	if !detectScopeOnlyDeliverable(art, &verifyContext{art: art, item: item, pl: pl, id: art.TaskID}) {
		t.Fatal("scope-only with open questions should fire")
	}
	// file:line citation → not fire
	withCitation := &wavescheduler.Artifact{
		TaskID:  "x",
		Summary: "done foo.go:42",
	}
	if detectScopeOnlyDeliverable(withCitation, &verifyContext{art: withCitation, item: item, pl: pl, id: withCitation.TaskID}) {
		t.Fatal("file:line citation should not fire scope-only")
	}
	// plan not ExplorationPlan → not fire
	plCommitment := &plan.Plan{Kind: plan.CommitmentPlan}
	if detectScopeOnlyDeliverable(art, &verifyContext{art: art, item: item, pl: plCommitment, id: art.TaskID}) {
		t.Fatal("CommitmentPlan should not fire scope-only")
	}
}

func TestDetectDeliverableIncomplete(t *testing.T) {
	// ContractApplicable + incomplete
	contract := workmodel.ExpandLegacySchemaToContract(workmodel.FirstRegisteredDeliverableSchema())
	art := &wavescheduler.Artifact{
		TaskID:   "x",
		Summary:  "Let me continue exploring.",
		Metadata: map[string]any{"stop_reason": "max_iters"},
	}
	if !detectDeliverableIncomplete(art, &verifyContext{art: art, contract: contract, id: art.TaskID}) {
		t.Fatal("incomplete deliverable should fire")
	}
	// Contract not applicable → not fire
	if detectDeliverableIncomplete(art, &verifyContext{art: art, contract: workmodel.DeliverableContract{}, id: art.TaskID}) {
		t.Fatal("not-applicable contract should not fire")
	}
}

func TestDetectRollupAllFailed(t *testing.T) {
	fire := &verifyContext{stats: workmodel.ChildOutcomeStats{Total: 3, Failed: 3}}
	if !detectRollupAllFailed(nil, fire) {
		t.Fatal("all-failed should fire")
	}
	noFire := &verifyContext{stats: workmodel.ChildOutcomeStats{Total: 3, Failed: 2}}
	if detectRollupAllFailed(nil, noFire) {
		t.Fatal("partial-failed should not fire all-failed")
	}
}

func TestDetectRollupMixedFailedRunning(t *testing.T) {
	fire := &verifyContext{stats: workmodel.ChildOutcomeStats{Total: 4, Failed: 2, Running: 1}}
	if !detectRollupMixedFailedRunning(nil, fire) {
		t.Fatal("failed+running should fire")
	}
	noFire := &verifyContext{stats: workmodel.ChildOutcomeStats{Total: 3, Failed: 3}} // all failed → 走 all-failed
	if detectRollupMixedFailedRunning(nil, noFire) {
		t.Fatal("all-failed should not fire mixed")
	}
}

func TestDetectRollupContractSatisfied(t *testing.T) {
	// valid rollup summary → fire
	art := &wavescheduler.Artifact{
		TaskID:  "x",
		Summary: validRollupSummary(),
	}
	if !detectRollupContractSatisfied(art, &verifyContext{art: art, id: art.TaskID}) {
		t.Fatal("valid rollup should fire contract-satisfied")
	}
	// empty summary → not fire (走 default-fail)
	emptyArt := &wavescheduler.Artifact{TaskID: "x", Summary: ""}
	if detectRollupContractSatisfied(emptyArt, &verifyContext{art: emptyArt, id: emptyArt.TaskID}) {
		t.Fatal("empty summary should not fire contract-satisfied")
	}
}

// T: D7-S10-A101-T02 (DM-20260705-005) — applyDecisionTable 行为

func TestApplyDecisionTable_FirstFiredReturns(t *testing.T) {
	// 构造一个 artifact 同时满足 trigger 1 (nil) — 但 table 的 fire 必须按顺序遍历
	// 这里测 happy path: 无 trigger fire → defaultVerdict
	art := &wavescheduler.Artifact{TaskID: "x", Summary: "ok"}
	v := applyDecisionTable(artifactDecisionTable, art, &verifyContext{art: art, id: "x"})
	if v.Kind != types.VerdictPass || v.Confidence != 0.9 {
		t.Fatalf("default: %+v", v)
	}
}

func TestApplyDecisionTable_NilArtifact(t *testing.T) {
	v := applyDecisionTable(artifactDecisionTable, nil, &verifyContext{art: nil, id: "artifact_unknown"})
	if v.Kind != types.VerdictIndeterminate {
		t.Fatalf("nil artifact: %+v", v)
	}
	if v.IndeterminateReason != "env_limited" {
		t.Fatalf("indeterminate reason: %s", v.IndeterminateReason)
	}
}

func TestApplyDecisionTable_MaxItersBeatsExecuteFail(t *testing.T) {
	// Error + max_iters + tool_calls=3 → 应返回 max-iters-partial (Partial 0.55)
	// 而非 execute-fail (Fail 0.9)
	art := &wavescheduler.Artifact{
		TaskID: "x", Error: "x",
		Metadata: map[string]any{"stop_reason": "max_iters", "tool_calls": 3},
	}
	v := applyDecisionTable(artifactDecisionTable, art, &verifyContext{art: art, id: "x"})
	if v.Kind != types.VerdictPartial {
		t.Fatalf("max_iters should return Partial, got %v", v.Kind)
	}
	if v.Confidence != 0.55 {
		t.Fatalf("confidence = %f, want 0.55", v.Confidence)
	}
}

// T: D7-S10-A101-T07 (DM-20260705-005) — detector 顺序锁定

func TestVerifyArtifact_DetectorOrder(t *testing.T) {
	// 构造一个同时可能命中 trigger 1 (nil) 但实际非 nil + trigger 2 (max-iters-partial) 的 artifact
	// 期望：trigger 1 not fire → trigger 2 fire → return Partial 0.55
	art := &wavescheduler.Artifact{
		TaskID: "x", Error: "x",
		Metadata: map[string]any{"stop_reason": "max_iters", "tool_calls": 3},
	}
	// 1) nil-artifact: false (art != nil)
	if detectNilArtifact(art, &verifyContext{art: art, id: "x"}) {
		t.Fatal("order: nil-artifact should not fire")
	}
	// 2) max-iters-partial: true (first match)
	if !detectMaxItersPartial(art, &verifyContext{art: art, id: "x"}) {
		t.Fatal("order: max-iters-partial should fire")
	}
	// 3) execute-fail: should NOT be evaluated (被 trigger 2 短路)
	// 通过 verifyArtifact 间接验证：final verdict = Partial 0.55
	v := verifyArtifact(art)
	if v.Kind != types.VerdictPartial || v.Confidence != 0.55 {
		t.Fatalf("order: final = %+v, want Partial(0.55)", v)
	}
	// 4) side-effect-rolledback: not fire (SideEffectStatus empty)
	if detectSideEffectRolledBack(art, &verifyContext{art: art, id: "x"}) {
		t.Fatal("order: rolledback should not fire")
	}
	// 5) side-effect-uncertain: not fire
	if detectSideEffectUncertain(art, &verifyContext{art: art, id: "x"}) {
		t.Fatal("order: uncertain should not fire")
	}
}

func TestApplyDecisionTable_SourceIDFromContext(t *testing.T) {
	art := &wavescheduler.Artifact{TaskID: "wi_42", Summary: "ok"}
	v := applyDecisionTable(artifactDecisionTable, art, &verifyContext{art: art, id: "wi_42"})
	if v.SourceID != "wi_42" {
		t.Fatalf("SourceID = %q, want wi_42", v.SourceID)
	}
}

// T: D7-S10-A101-T06 (DM-20260705-005) — 现有 13 测试 0 修改 PASS
// (covered by `go test ./...` of the package; this is a smoke test.)

func TestVerifyArtifact_TableHasFiveTriggers(t *testing.T) {
	if got := len(artifactDecisionTable); got != 5 {
		t.Fatalf("artifactDecisionTable len = %d, want 5", got)
	}
}

func TestVerifyRollupArtifact_TableHasFourTriggers(t *testing.T) {
	// 3 explicit + 1 catch-all default-fail
	if got := len(rollupDecisionTable); got != 4 {
		t.Fatalf("rollupDecisionTable len = %d, want 4", got)
	}
}

// T: D7-S10-A101-T05 (DM-20260705-005) — verifyRollupArtifact byte-equivalent smoke
func TestVerifyRollupArtifact_DefaultFailOnEmptySummary(t *testing.T) {
	art := &wavescheduler.Artifact{TaskID: "x", Summary: ""}
	v := verifyRollupArtifact(art, workmodel.ChildOutcomeStats{Total: 3, Completed: 3})
	if v.Kind != types.VerdictFail {
		t.Fatalf("empty summary: kind = %v, want Fail", v.Kind)
	}
	if !strings.Contains(v.Reason, "rollup") {
		t.Fatalf("reason should mention rollup, got %q", v.Reason)
	}
}

func TestVerifyRollupArtifact_AllChildrenFailed(t *testing.T) {
	art := &wavescheduler.Artifact{TaskID: "x", Summary: validRollupSummary()}
	v := verifyRollupArtifact(art, workmodel.ChildOutcomeStats{Total: 3, Failed: 3})
	if v.Kind != types.VerdictFail {
		t.Fatalf("all-failed: kind = %v, want Fail", v.Kind)
	}
	if v.Confidence != 0.95 {
		t.Fatalf("all-failed confidence = %f, want 0.95", v.Confidence)
	}
}

func TestVerifyRollupArtifact_MixedRunning(t *testing.T) {
	art := &wavescheduler.Artifact{TaskID: "x", Summary: validRollupSummary()}
	v := verifyRollupArtifact(art, workmodel.ChildOutcomeStats{Total: 4, Completed: 1, Failed: 2, Running: 1})
	if v.Kind != types.VerdictPartial {
		t.Fatalf("mixed running: kind = %v, want Partial", v.Kind)
	}
}
