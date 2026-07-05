package sessionorchestrator

import (
	"regexp"
	"strings"

	"github.com/devrix/devrix/internal/layers/orchestration/executionflow/verify"
	"github.com/devrix/devrix/internal/layers/orchestration/plan"
	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/types"
)

var (
	fileLineCitationRE = regexp.MustCompile(`\w[\w./-]*\.(go|py|ts|tsx|js|rs):\d+`)
	userGatePhrases    = []string{
		"awaiting your",
		"awaiting user",
		"reply with your",
		"等待您的",
		"等待你的",
		"before i proceed, i need to clarify",
	}
	userGateToolRE = regexp.MustCompile(`ask_user_question\s*[\({]`)
)

// M4 (mups-verify-table-driven, DM-20260705-005) — verifyArtifact 走决策表。
// 49 行 → 5 行：建 ctx + applyDecisionTable。
func verifyArtifact(art *wavescheduler.Artifact) workmodel.Verdict {
	id := ""
	if art != nil {
		id = art.TaskID
		if id == "" {
			id = "artifact_unknown"
		}
	}
	ctx := &verifyContext{art: art, id: id}
	return applyDecisionTable(artifactDecisionTable, art, ctx)
}

// verifyArtifactForWorkItem applies pipeline-aware checks so autonomous rounds
// do not Pass on user-gate or scope-only output (which would block decompose).
func verifyArtifactForWorkItem(art *wavescheduler.Artifact, item *workmodel.WorkItem, pl *plan.Plan) workmodel.Verdict {
	return verifyArtifactForWorkItemWithSchema(art, item, pl, workmodel.DeliverableSchemaNotApplicable).Verdict
}

// WorkItemVerifyOutcome bundles verdict with deliverable verification (DM-20260630-012).
type WorkItemVerifyOutcome struct {
	Verdict             workmodel.Verdict
	Deliverable         DeliverableVerifyResult
	DeliverableContract workmodel.DeliverableContract
	DeliverableSchema   workmodel.DeliverableSchema
}

func verifyArtifactForWorkItemWithSchema(
	art *wavescheduler.Artifact,
	item *workmodel.WorkItem,
	pl *plan.Plan,
	schema workmodel.DeliverableSchema,
) WorkItemVerifyOutcome {
	return verifyArtifactForWorkItemWithContract(art, item, pl, workmodel.ExpandLegacySchemaToContract(schema))
}

// verifyArtifactForWorkItemWithContract M4 重构：先走 artifactDecisionTable 拿 base verdict，
// 再叠加 3 overlay detector (user_gate / scope_only / deliverable_incomplete) 形成最终 verdict。
// 54 行 → ~35 行。
func verifyArtifactForWorkItemWithContract(
	art *wavescheduler.Artifact,
	item *workmodel.WorkItem,
	pl *plan.Plan,
	contract workmodel.DeliverableContract,
) WorkItemVerifyOutcome {
	schema := workmodel.DeliverableSchemaNotApplicable
	if contract.ContractApplicable() {
		schema = workmodel.DeliverableSchema("legacy_contract")
	}
	id := "artifact_unknown"
	if art != nil && art.TaskID != "" {
		id = art.TaskID
	}
	ctx := &verifyContext{art: art, item: item, pl: pl, contract: contract, id: id}
	v := applyDecisionTable(artifactDecisionTable, art, ctx)
	if detectUserGate(art, ctx) {
		v = workmodel.Verdict{
			Kind:       types.VerdictPartial,
			Reason:     "interactive user gate not allowed in pipeline execute",
			SourceID:   id,
			Confidence: 0.85,
		}
	}
	if detectScopeOnlyDeliverable(art, ctx) {
		v = workmodel.Verdict{
			Kind:       types.VerdictPartial,
			Reason:     "scope contract emitted without deliverable; decompose required",
			SourceID:   id,
			Confidence: 0.8,
		}
	}
	deliverable := VerifyDeliverableContract(contract, art)
	if contract.ContractApplicable() && deliverable.Status == workmodel.DeliverableStatusIncomplete && v.Kind == types.VerdictPass {
		v = workmodel.Verdict{
			Kind:       types.VerdictPartial,
			Reason:     deliverableReason(deliverable),
			SourceID:   id,
			Confidence: 0.65,
		}
	}
	return WorkItemVerifyOutcome{
		Verdict:             v,
		Deliverable:         deliverable,
		DeliverableContract: contract,
		DeliverableSchema:   schema,
	}
}

func deliverableReason(r DeliverableVerifyResult) string {
	if r.Reason != "" {
		return r.Reason
	}
	return "deliverable schema not satisfied"
}

func artifactAwaitingUserGate(art *wavescheduler.Artifact) bool {
	if used, _ := art.Metadata["used_ask_user_question"].(bool); used {
		return true
	}
	lower := strings.ToLower(strings.TrimSpace(art.Summary))
	if userGateToolRE.MatchString(lower) {
		return true
	}
	for _, phrase := range userGatePhrases {
		if strings.Contains(lower, phrase) {
			return true
		}
	}
	return false
}

func isScopeOnlyDeliverable(art *wavescheduler.Artifact, item *workmodel.WorkItem) bool {
	if art == nil || item == nil || item.ScopeContract == nil {
		return false
	}
	if fileLineCitationRE.MatchString(art.Summary) {
		return false
	}
	// Scope contract persisted from execute with unresolved questions means
	// the round converged scope only — decompose must continue downstream.
	return item.ScopeContract.HasOpenQuestions()
}

func exitReasonForVerdict(v workmodel.Verdict, sessionID string) string {
	return string(verify.VerdictToExitReason(v, sessionID))
}
