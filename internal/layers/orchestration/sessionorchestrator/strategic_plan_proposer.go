package sessionorchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/devrix/devrix/internal/layers/contextengine/i18n"
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/prompttags"
	"github.com/devrix/devrix/internal/shared/types"
)

// StrategicPlanInput carries Plan-phase context for LLM strategic proposals.
type StrategicPlanInput struct {
	SessionID      string
	WorkItemID     string
	Directive      string
	ObservationIDs []string
	ReportSummary  string
	// Budget (RH-MUPS-07, T-P1-2): spawn-side limits the proposer must
	// see so it can self-bound its proposal. Built by
	// workmodel.StrategicPlanBudget.
	Budget workmodel.DivergenceBudget
	// ParentScopeIn (RH-MUPS-07, T-P1-2): the parent's in-scope paths,
	// copied from item.ScopeContract.InScope. The proposer should not
	// propose child scopes that are disjoint from this list (and the
	// post-validate gate enforces a real-subset invariant; see T-P2-2).
	ParentScopeIn []string
	// UncertaintyMean is the WorkItem's stored uncertainty at plan time (CC-U4).
	UncertaintyMean float64
	// PriorParseReject is compact JSON from the previous round's PlanParseReject field.
	PriorParseReject string
}

type rawStrategicChildSpec struct {
	Title           string   `json:"title"`
	DirectiveSuffix string   `json:"directive_suffix"`
	ExpectedReturn  string   `json:"expected_return"`
	ScopeIn         []string `json:"scope_in"`
}

type rawStrategicPlan struct {
	ExecutionMode       string                  `json:"execution_mode"`
	ScopeIn             []string                `json:"scope_in"`
	ChildSpecs          []rawStrategicChildSpec `json:"child_specs"`
	DeliverableContract workmodel.DeliverableContract `json:"deliverable_contract"`
	DeliverableSchema   string                  `json:"deliverable_schema"`
	ReactItersHint      int                     `json:"react_iters_hint"`
	Rationale           string                  `json:"rationale"`
}

// StrategicPlanProposal is a validated LLM strategic plan (DM-20260630-012).
type StrategicPlanProposal struct {
	ExecutionMode       string
	ScopeIn             []string
	ChildSpecs          []workmodel.ChildSpec
	DeliverableContract workmodel.DeliverableContract
	DeliverableSchema   workmodel.DeliverableSchema
	ReactItersHint      int
	QuantizedKind       string
	Rationale           string
}

// StrategicPlanProposer calls D2 MaterializeForMUPS then D3 for Plan strategy (DM-20260704-001).
type StrategicPlanProposer interface {
	ProposeStrategicPlan(ctx context.Context, in StrategicPlanInput) (*StrategicPlanProposal, error)
}

// LLMStrategicPlanProposer implements StrategicPlanProposer via D2→D3.
type LLMStrategicPlanProposer struct {
	LLM    orchtypes.LLMInvoker
	MUPS   contracts.IMUPSContextMaterializer
	Locale i18n.Locale
}

// NewLLMStrategicPlanProposer constructs a D2→D3 strategic plan proposer.
func NewLLMStrategicPlanProposer(llm orchtypes.LLMInvoker, mups contracts.IMUPSContextMaterializer, loc i18n.Locale) *LLMStrategicPlanProposer {
	if llm == nil || mups == nil {
		return nil
	}
	if loc == "" {
		loc = i18n.DefaultLocale
	}
	return &LLMStrategicPlanProposer{LLM: llm, MUPS: mups, Locale: loc}
}

func (p *LLMStrategicPlanProposer) ProposeStrategicPlan(ctx context.Context, in StrategicPlanInput) (*StrategicPlanProposal, error) {
	if p == nil || p.LLM == nil || p.MUPS == nil {
		return nil, nil
	}
	req := buildPlanMUPSRequest(in, string(p.Locale))
	prepared, err := p.MUPS.MaterializeForMUPS(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("strategic plan proposer: d2 materialize: %w", err)
	}
	systemPrompt := mergeMUPSPreparedSystem(prepared)
	user := buildStrategicPlanUserPrompt(in, p.Locale)
	ch, err := p.LLM.InvokeStream(ctx, orchtypes.LLMInvokeRequest{
		SessionID:    in.SessionID,
		SystemPrompt: systemPrompt,
		Messages: []types.Message{{
			SessionID: in.SessionID,
			Role:      types.MessageRoleUser,
			Content:   user,
		}},
	})
	if err != nil {
		return nil, err
	}
	raw := collectLLMText(ch)
	prop, err := parseStrategicPlanJSON(raw, in.Directive)
	if err != nil {
		return nil, err
	}
	// RH-MUPS-07 (DM-20260701-001 T-P1-3): reject over-budget proposals
	// with a structured StrategicPlanReject so the next round's prompt
	// can pre-emptively size the proposal. Without this, CapChildSpecs
	// would silently truncate the LLM output and the next round would
	// re-propose the same too-large set forever.
	if budgetErr := applyBudgetCap(prop, in.Budget); budgetErr != nil {
		return nil, budgetErr
	}
	if gateErr := applySingleModeUncertaintyGate(prop, in); gateErr != nil {
		return nil, gateErr
	}
	return prop, nil
}

func buildStrategicPlanUserPrompt(in StrategicPlanInput, loc i18n.Locale) string {
	fields := map[prompttags.TagName]any{
		prompttags.TagWorkItemID: in.WorkItemID,
		prompttags.TagDirective:  in.Directive,
	}
	if len(in.ObservationIDs) > 0 {
		fields[prompttags.TagObservationIDs] = in.ObservationIDs
	}
	if s := strings.TrimSpace(in.ReportSummary); s != "" {
		fields[prompttags.TagObservationSummary] = s
	}
	// RH-MUPS-07 (DM-20260701-001 T-P1-2): inject the divergence budget so
	// the LLM can self-bound its proposal. Field order is fixed in
	// prompttags.PlanUserFrame; reordering breaks T-P1-4.
	if in.Budget.MaxChildren > 0 {
		budget := in.Budget
		fields[prompttags.TagDepth] = budget.Depth
		fields[prompttags.TagMaxDepth] = budget.MaxDepth
		fields[prompttags.TagExistingChildren] = budget.ExistingChildren
		fields[prompttags.TagRemainingChildren] = budget.RemainingChildren()
		fields[prompttags.TagMaxChildren] = budget.MaxChildren
		fields[prompttags.TagDecomposeUsedToday] = budget.DecomposeUsedToday
		fields[prompttags.TagRemainingDaily] = budget.RemainingDaily()
		fields[prompttags.TagMaxDaily] = budget.MaxDaily
		fields[prompttags.TagMaxIters] = budget.MaxIters
	}
	if len(in.ParentScopeIn) > 0 {
		fields[prompttags.TagParentScopeIn] = in.ParentScopeIn
	}
	if in.UncertaintyMean > 0 {
		fields[prompttags.TagUncertaintyMean] = in.UncertaintyMean
	}
	if s := strings.TrimSpace(in.PriorParseReject); s != "" {
		fields[prompttags.TagPriorParseReject] = s
	}
	frame := prompttags.BuildAnnotatedLineFrame(prompttags.FramePlanUser, prompttags.PlanUserFrame, fields)
	guide := i18n.RenderFrameFieldGuideForFields(prompttags.FramePlanUser, loc, fields)
	if guide == "" {
		return frame
	}
	return guide + "\n\n" + frame
}

func parseStrategicPlanJSON(raw, baseDirective string) (*StrategicPlanProposal, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("strategic plan: empty response")
	}
	row, ok := prompttags.ParseWholeBody[rawStrategicPlan](raw)
	if !ok {
		start := strings.Index(raw, "{")
		end := strings.LastIndex(raw, "}")
		candidate := raw
		if start >= 0 && end > start {
			candidate = raw[start : end+1]
		}
		if err := json.Unmarshal([]byte(candidate), &row); err != nil {
			return nil, fmt.Errorf("strategic plan: parse json: %w", err)
		}
	}
	prop := &StrategicPlanProposal{
		ExecutionMode:  strings.ToLower(strings.TrimSpace(row.ExecutionMode)),
		ScopeIn:        append([]string(nil), row.ScopeIn...),
		Rationale:      strings.TrimSpace(row.Rationale),
		ReactItersHint: clampReactIters(row.ReactItersHint),
	}
	prop.DeliverableContract = mapDeliverableContract(row.DeliverableContract, row.DeliverableSchema, baseDirective)
	if prop.DeliverableContract.ContractApplicable() {
		prop.DeliverableSchema = workmodel.FirstRegisteredDeliverableSchema()
	} else {
		prop.DeliverableSchema = workmodel.DeliverableSchemaNotApplicable
	}
	prop.QuantizedKind = mapExecutionModeToQuantizedKind(prop.ExecutionMode)
	prop.ChildSpecs = mapRawChildSpecs(baseDirective, row.ChildSpecs, prop.ExecutionMode)
	if err := validateStrategicPlan(prop); err != nil {
		return nil, err
	}
	return prop, nil
}

func mapDeliverableContract(raw workmodel.DeliverableContract, schemaField, directive string) workmodel.DeliverableContract {
	if raw.ContractApplicable() {
		return workmodel.ClampDeliverableContract(raw)
	}
	if schema, ok := workmodel.LookupRegisteredDeliverableSchema(schemaField); ok {
		return workmodel.ExpandLegacySchemaToContract(schema)
	}
	if tag := workmodel.ParseDeliverableSchemaTag(directive); tag != workmodel.DeliverableSchemaNotApplicable {
		return workmodel.ExpandLegacySchemaToContract(tag)
	}
	return workmodel.DeliverableContract{}
}

func mapExecutionModeToQuantizedKind(mode string) string {
	switch mode {
	case "single":
		return "intent_command"
	case "parallel_probe":
		return "intent_fast"
	default:
		return "intent_orchestrate"
	}
}

func mapRawChildSpecs(base string, raw []rawStrategicChildSpec, mode string) []workmodel.ChildSpec {
	if mode != "decompose" || len(raw) == 0 {
		return nil
	}
	out := make([]workmodel.ChildSpec, 0, len(raw))
	for _, r := range raw {
		title := strings.TrimSpace(r.Title)
		if title == "" {
			continue
		}
		suffix := strings.TrimSpace(r.DirectiveSuffix)
		directive := base
		if suffix != "" {
			directive = strings.TrimSpace(base) + "\n\n" + suffix
		}
		expected := strings.TrimSpace(r.ExpectedReturn)
		if expected == "" {
			expected = workmodel.DefaultChildExpectedReturn(nil, base)
		}
		out = append(out, workmodel.ChildSpec{
			Kind:           workmodel.WorkKindExplore,
			Title:          title,
			Directive:      directive,
			ScopeIn:        append([]string(nil), r.ScopeIn...),
			ExpectedReturn: expected,
		})
	}
	return workmodel.CapChildSpecs(out)
}

func validateStrategicPlan(p *StrategicPlanProposal) error {
	if p == nil {
		return fmt.Errorf("strategic plan: nil proposal")
	}
	switch p.ExecutionMode {
	case "single", "decompose", "parallel_probe", "":
	default:
		return fmt.Errorf("strategic plan: unknown execution_mode %q", p.ExecutionMode)
	}
	if p.ExecutionMode == "decompose" && len(p.ChildSpecs) == 0 {
		return fmt.Errorf("strategic plan: decompose requires child_specs")
	}
	if p.ExecutionMode == "single" && len(p.ChildSpecs) > 0 {
		p.ChildSpecs = nil
	}
	return nil
}

// StrategicPlanReject is the structured over-budget rejection for
// RH-MUPS-07 (T-P1-3). The runner (item_pipeline.go) inspects this
// type to record the rejection in the round's SpawnRationale so the
// next round's LLM prompt can self-correct.
//
// Without this, the prior `CapChildSpecs` truncated silently — the LLM
// saw a 4-child proposal produce a 1-child CapChildSpecs output and
// had no idea why. With this, the rejection carries a Reason (one of
// the BudgetField* constants) + MaxAllowed so the next prompt can
// pre-emptively size its proposal.
type StrategicPlanReject struct {
	Reason     string // one of BudgetField*
	MaxAllowed int
	Requested  int
	Field      string // "children" or "daily" — for the runner's logging
}

// Error implements error.
func (e *StrategicPlanReject) Error() string {
	return fmt.Sprintf("strategic plan: over budget (%s): requested=%d max=%d",
		e.Reason, e.Requested, e.MaxAllowed)
}

// BudgetFieldScope rejects scope proposals outside parent bounds.
const (
	BudgetFieldChildren = "children"
	BudgetFieldDaily    = "daily"
	BudgetFieldScope       = "scope"
	BudgetFieldUncertainty = "uncertainty"
)

// applyBudgetCap implements T-P1-3: when the LLM proposes more children
// than the budget allows, return a structured StrategicPlanReject so
// the runner can surface the rejection to the next round. Replaces the
// silent truncation in CapChildSpecs (kept in spawn_apply.go as a
// last-resort safety net, never the primary path).
func applyBudgetCap(prop *StrategicPlanProposal, budget workmodel.DivergenceBudget) error {
	if prop == nil || prop.ExecutionMode != "decompose" {
		return nil
	}
	n := len(prop.ChildSpecs)
	if n == 0 {
		return nil
	}
	if remaining := budget.RemainingChildren(); n > remaining {
		return &StrategicPlanReject{
			Reason:     BudgetFieldChildren,
			MaxAllowed: budget.MaxChildren,
			Requested:  n,
			Field:      BudgetFieldChildren,
		}
	}
	if remaining := budget.RemainingDaily(); n > remaining {
		return &StrategicPlanReject{
			Reason:     BudgetFieldDaily,
			MaxAllowed: budget.MaxDaily,
			Requested:  n,
			Field:      BudgetFieldDaily,
		}
	}
	return nil
}

func applySingleModeUncertaintyGate(prop *StrategicPlanProposal, in StrategicPlanInput) error {
	if prop == nil || prop.ExecutionMode != "single" {
		return nil
	}
	if in.UncertaintyMean < workmodel.SingleModeUncertaintyThreshold {
		return nil
	}
	return &StrategicPlanReject{
		Reason:     BudgetFieldUncertainty,
		MaxAllowed: int(workmodel.SingleModeUncertaintyThreshold * 100),
		Requested:  int(in.UncertaintyMean * 100),
		Field:      BudgetFieldUncertainty,
	}
}

func clampReactIters(n int) int {
	if n <= 0 {
		return 0
	}
	if n > DefaultWorkItemMaxIters {
		return DefaultWorkItemMaxIters
	}
	return n
}

func applyStrategicScope(sessionID string, item *workmodel.WorkItem, prop *StrategicPlanProposal, tm *workmodel.TaskManager) {
	if prop == nil || item == nil || tm == nil || len(prop.ScopeIn) == 0 {
		return
	}
	// RH-D7-05: decomposed child items carry authoritative scope via
	// ChildDownlink at decompose time. Strategic plan LLM output must not
	// overwrite persisted ScopeContract with hallucinated paths (e.g.
	// internal/layers/d2/kernel/ instead of contextengine/kernel/).
	if item.Kind != workmodel.WorkKindGoal {
		if dl, ok := tm.ChildDownlinkFor(sessionID, item.ID); ok && len(dl.ScopeIn) > 0 {
			return
		}
	}
	sc := item.ScopeContract
	if sc == nil {
		sc = &workmodel.ScopeContract{}
	}
	sc.GoalStatement = strings.TrimSpace(item.Directive)
	sc.InScope = append([]string(nil), prop.ScopeIn...)
	_ = tm.SetScopeContract(sessionID, item.ID, sc)
	item.ScopeContract = sc
}
