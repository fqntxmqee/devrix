//go:build legacy_verify
// +build legacy_verify

// Legacy verify implementations — 仅在 `-tags legacy_verify` 时编译。
// 用于 M4 (DM-20260705-005) byte-equivalent 测试对比新旧实现的输出。
// 生产代码永远是新的 `verifyArtifact` / `verifyRollupArtifact` (in item_verify.go / rollup_verify.go)。
//
// 下个 change (`mups-cleanup-legacy`) 必须删除本文件 + 相关 build tag。
package sessionorchestrator

import (
	"fmt"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/plan"
	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/types"
)

// verifyArtifactLegacy 旧版 Phase B 决策 (重构前 49 行版本)。
func verifyArtifactLegacy(art *wavescheduler.Artifact) workmodel.Verdict {
	if art == nil {
		return workmodel.Verdict{
			Kind:       types.VerdictIndeterminate,
			Reason:     "missing artifact",
			Confidence: 0,
		}.WithIndeterminateReason("env_limited")
	}
	id := art.TaskID
	if id == "" {
		id = "artifact_unknown"
	}
	if art.Error != "" || art.ExitCode != 0 {
		if reason, _ := art.Metadata["stop_reason"].(string); reason == "max_iters" {
			if calls, _ := art.Metadata["tool_calls"].(int); calls > 0 {
				return workmodel.Verdict{
					Kind:       types.VerdictPartial,
					Reason:     "iteration cap with partial progress",
					SourceID:   id,
					Confidence: 0.55,
				}
			}
		}
		return workmodel.Verdict{
			Kind:       types.VerdictFail,
			Reason:     fmt.Sprintf("execute failed: %s", art.Error),
			SourceID:   id,
			Confidence: 0.9,
		}
	}
	switch art.SideEffectStatus {
	case types.SideEffectRolledBack:
		return workmodel.Verdict{
			Kind:       types.VerdictFail,
			Reason:     "side effect rolled back",
			SourceID:   id,
			Confidence: 0.85,
		}
	case types.SideEffectUnknown, types.SideEffectInflight:
		return workmodel.Verdict{
			Kind:       types.VerdictPartial,
			Reason:     "side effect uncertain",
			SourceID:   id,
			Confidence: 0.6,
		}
	default:
		return workmodel.Verdict{
			Kind:       types.VerdictPass,
			Reason:     art.Summary,
			SourceID:   id,
			Confidence: 0.9,
		}
	}
}

// verifyArtifactForWorkItemWithContractLegacy 旧版 4 overlay (重构前 54 行版本)。
func verifyArtifactForWorkItemWithContractLegacy(
	art *wavescheduler.Artifact,
	item *workmodel.WorkItem,
	pl *plan.Plan,
	contract workmodel.DeliverableContract,
) WorkItemVerifyOutcome {
	v := verifyArtifactLegacy(art)
	schema := workmodel.DeliverableSchemaNotApplicable
	if contract.ContractApplicable() {
		schema = workmodel.DeliverableSchema("legacy_contract")
	}
	if art == nil {
		return WorkItemVerifyOutcome{Verdict: v, DeliverableContract: contract, DeliverableSchema: schema}
	}
	id := art.TaskID
	if id == "" {
		id = "artifact_unknown"
	}
	if artifactAwaitingUserGate(art) {
		v = workmodel.Verdict{
			Kind:       types.VerdictPartial,
			Reason:     "interactive user gate not allowed in pipeline execute",
			SourceID:   id,
			Confidence: 0.85,
		}
	}
	if item != nil && workmodel.CanDecompose(item.Kind) && pl != nil && pl.Kind == plan.ExplorationPlan {
		if isScopeOnlyDeliverable(art, item) {
			v = workmodel.Verdict{
				Kind:       types.VerdictPartial,
				Reason:     "scope contract emitted without deliverable; decompose required",
				SourceID:   id,
				Confidence: 0.8,
			}
		}
	}
	deliverable := VerifyDeliverableContract(contract, art)
	if contract.ContractApplicable() {
		if deliverable.Status == workmodel.DeliverableStatusIncomplete {
			if v.Kind == types.VerdictPass {
				v = workmodel.Verdict{
					Kind:       types.VerdictPartial,
					Reason:     deliverableReason(deliverable),
					SourceID:   id,
					Confidence: 0.65,
				}
			}
		}
	}
	return WorkItemVerifyOutcome{
		Verdict:             v,
		Deliverable:         deliverable,
		DeliverableContract: contract,
		DeliverableSchema:   schema,
	}
}

// verifyRollupArtifactLegacy 旧版 rollup-specific gates (重构前 47 行版本)。
func verifyRollupArtifactLegacy(art *wavescheduler.Artifact, stats workmodel.ChildOutcomeStats) workmodel.Verdict {
	if art == nil || art.Error != "" || art.ExitCode != 0 {
		return verifyArtifactLegacy(art)
	}
	if stats.Total > 0 && stats.Failed == stats.Total {
		return workmodel.Verdict{
			Kind:       types.VerdictFail,
			Reason:     fmt.Sprintf("all %d rollup children failed; refusing Pass", stats.Failed),
			SourceID:   art.TaskID,
			Confidence: 0.95,
		}
	}
	if stats.Total > 0 && stats.Failed > 0 && stats.Running > 0 {
		return workmodel.Verdict{
			Kind:       types.VerdictPartial,
			Reason:     fmt.Sprintf("rollup synthesized with %d failed + %d running children", stats.Failed, stats.Running),
			SourceID:   art.TaskID,
			Confidence: 0.8,
		}
	}
	summary := strings.TrimSpace(art.Summary)
	if summary == "" {
		return workmodel.Verdict{
			Kind:       types.VerdictFail,
			Reason:     "rollup summary empty",
			SourceID:   art.TaskID,
			Confidence: 0.9,
		}
	}
	contract := workmodel.RollupDeliverableContract()
	got := workmodel.VerifyDeliverableContract(contract, summary, "")
	switch got.Status {
	case workmodel.DeliverableStatusComplete:
		return workmodel.Verdict{
			Kind:       types.VerdictPass,
			Reason:     summary,
			SourceID:   art.TaskID,
			Confidence: 0.9,
		}
	default:
		reason := got.Reason
		if reason == "" {
			reason = "rollup deliverable contract not satisfied"
		}
		return workmodel.Verdict{
			Kind:       types.VerdictFail,
			Reason:     reason,
			SourceID:   art.TaskID,
			Confidence: 0.85,
		}
	}
}

// ---- byte-equivalent 测试 ----

// T: D7-S10-A101-T03 (DM-20260705-005) — verifyArtifact 7 组合字节级等价
func TestVerifyArtifactRefactor_ByteEquivalent_OldVsNew(t *testing.T) {
	cases := []*wavescheduler.Artifact{
		nil,
		{TaskID: "wi_1", Error: "x", Metadata: map[string]any{"stop_reason": "max_iters", "tool_calls": 3}},
		{TaskID: "wi_1", Error: "x", Metadata: map[string]any{"stop_reason": "max_iters", "tool_calls": 0}},
		{TaskID: "wi_1", ExitCode: 1, Error: "boom"},
		{TaskID: "wi_1", SideEffectStatus: types.SideEffectRolledBack},
		{TaskID: "wi_1", SideEffectStatus: types.SideEffectInflight},
		{TaskID: "wi_1", Summary: "ok"},
	}
	for i, art := range cases {
		oldV := verifyArtifactLegacy(art)
		newV := verifyArtifact(art)
		if !verdictEqual(oldV, newV) {
			t.Errorf("case %d: old=%+v new=%+v", i, oldV, newV)
		}
	}
}

// T: D7-S10-A101-T04 (DM-20260705-005) — verifyArtifactForWorkItemWithContract 4 overlay 字节级等价
func TestVerifyArtifactForWorkItemWithContractRefactor_ByteEquivalent_OldVsNew(t *testing.T) {
	contract := workmodel.ExpandLegacySchemaToContract(workmodel.FirstRegisteredDeliverableSchema())
	item := &workmodel.WorkItem{Kind: workmodel.WorkKindExplore}
	pl := &plan.Plan{Kind: plan.ExplorationPlan}
	plCommitment := &plan.Plan{Kind: plan.CommitmentPlan}
	itemGoal := &workmodel.WorkItem{
		Kind:          workmodel.WorkKindGoal,
		ScopeContract: &workmodel.ScopeContract{OpenQuestions: []string{"q1"}},
	}
	cases := []struct {
		name     string
		art      *wavescheduler.Artifact
		item     *workmodel.WorkItem
		pl       *plan.Plan
		contract workmodel.DeliverableContract
	}{
		{
			"user-gate",
			&wavescheduler.Artifact{TaskID: "wi_goal", Summary: "I've sent scope. Awaiting your selection.", ExitCode: 0},
			itemGoal, pl, workmodel.DeliverableContract{},
		},
		{
			"scope-only",
			&wavescheduler.Artifact{
				TaskID:   "wi_goal",
				Summary:  "done\n<scope_contract>{\"open_questions\":[\"q1\"]}</scope_contract>",
				ExitCode: 0,
			},
			itemGoal, pl, workmodel.DeliverableContract{},
		},
		{
			"deliverable-incomplete-pass-downgrade",
			&wavescheduler.Artifact{
				TaskID:   "wi_1",
				Summary:  "Let me read the next file.",
				ExitCode: 0,
				Metadata: map[string]any{"stop_reason": "max_iters"},
			},
			item, nil, contract,
		},
		{
			"deliverable-incomplete-fail-no-downgrade",
			&wavescheduler.Artifact{
				TaskID:   "wi_1",
				Summary:  "Let me read the next file.",
				ExitCode: 0,
				Metadata: map[string]any{"stop_reason": "max_iters"},
			},
			item, plCommitment, contract,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			oldV := verifyArtifactForWorkItemWithContractLegacy(c.art, c.item, c.pl, c.contract)
			newV := verifyArtifactForWorkItemWithContract(c.art, c.item, c.pl, c.contract)
			if !verdictEqual(oldV.Verdict, newV.Verdict) {
				t.Errorf("verdict: old=%+v new=%+v", oldV.Verdict, newV.Verdict)
			}
		})
	}
}

// T: D7-S10-A101-T05 (DM-20260705-005) — verifyRollupArtifact 6 rollup 组合字节级等价
func TestVerifyRollupArtifactRefactor_ByteEquivalent_OldVsNew(t *testing.T) {
	cases := []struct {
		name  string
		art   *wavescheduler.Artifact
		stats workmodel.ChildOutcomeStats
	}{
		{"pass", &wavescheduler.Artifact{TaskID: "x", Summary: validRollupSummary(), ExitCode: 0}, workmodel.ChildOutcomeStats{Total: 3, Completed: 3}},
		{"too-short", &wavescheduler.Artifact{TaskID: "x", Summary: "P0: short P1: short", ExitCode: 0}, workmodel.ChildOutcomeStats{Total: 3, Completed: 3}},
		{"planning-denylist", &wavescheduler.Artifact{TaskID: "x", Summary: validRollupSummary() + "\n我将要 parallel explore", ExitCode: 0}, workmodel.ChildOutcomeStats{Total: 3, Completed: 3}},
		{"all-failed", &wavescheduler.Artifact{TaskID: "x", Summary: validRollupSummary(), ExitCode: 0}, workmodel.ChildOutcomeStats{Total: 3, Failed: 3}},
		{"failed+running", &wavescheduler.Artifact{TaskID: "x", Summary: validRollupSummary(), ExitCode: 0}, workmodel.ChildOutcomeStats{Total: 4, Completed: 1, Failed: 2, Running: 1}},
		{"mixed-failure-passes", &wavescheduler.Artifact{TaskID: "x", Summary: validRollupSummary(), ExitCode: 0}, workmodel.ChildOutcomeStats{Total: 4, Completed: 2, Failed: 2}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			oldV := verifyRollupArtifactLegacy(c.art, c.stats)
			newV := verifyRollupArtifact(c.art, c.stats)
			if !verdictEqual(oldV, newV) {
				t.Errorf("verdict: old=%+v new=%+v", oldV, newV)
			}
		})
	}
}

// verdictEqual 比较两个 Verdict 的 Kind/Confidence/Reason/SourceID/IndeterminateReason 5 字段。
func verdictEqual(a, b workmodel.Verdict) bool {
	return a.Kind == b.Kind &&
		a.Confidence == b.Confidence &&
		a.Reason == b.Reason &&
		a.SourceID == b.SourceID &&
		a.IndeterminateReason == b.IndeterminateReason
}
