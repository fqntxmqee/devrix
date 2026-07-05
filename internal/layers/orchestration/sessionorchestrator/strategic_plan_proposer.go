package sessionorchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
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


// StrategicPlanFrame is the LLM-view flat struct for the Plan user frame.
// It mirrors PlanUserFrame (16 fields, 1:1) so the prompttags reflection
// kernel can register and serialize it via pt struct tags. This is the
// conversion target of buildStrategicPlanFrame; nested DivergenceBudget
// is flattened here (DM-20260705-004 go-struct-driven M2).
//
// init() registers this struct with prompttags.MustRegisterFrame; any
// drift between struct / FrameSpec / i18n guide panics at process start.
type StrategicPlanFrame struct {
	// 1. work_item_id (control, omit_zero) - always emitted when non-empty.
	WorkItemID string `pt:"work_item_id,control,omit_empty"`

	// 2. directive (data, omit_empty) - always emitted when non-empty.
	Directive string `pt:"directive,data,omit_empty"`

	// 3. prior_parse_reject (control, omit_empty) - DM-20260705-002 cross-round feedback.
	PriorParseReject string `pt:"prior_parse_reject,control,omit_empty"`

	// 4. observation_ids (data, omit_empty) - prior Obs IDs (incremental).
	ObservationIDs []string `pt:"observation_ids,data,omit_empty"`

	// 5. observation_summary (data, omit_empty) - prior Obs summary.
	ObservationSummary string `pt:"observation_summary,data,omit_empty"`

	// 6-14. Budget 9-field flatten (control, no omit). 0 values are
	// emitted as literal "0" (matches the prior 35-line manual map
	// output; the Budget.MaxChildren>0 guard in buildStrategicPlanFrame
	// suppresses the entire block when no budget is set).
	Depth *int `pt:"depth,control"`
	MaxDepth *int `pt:"max_depth,control"`
	ExistingChildren *int `pt:"existing_children,control"`
	RemainingChildren *int `pt:"remaining_children,control"`
	MaxChildren *int `pt:"max_children,control"`
	DecomposeUsedToday *int `pt:"decompose_used_today,control"`
	RemainingDaily *int `pt:"remaining_daily,control"`
	MaxDaily *int `pt:"max_daily,control"`
	MaxIters *int `pt:"max_iters,control"`

	// 15. parent_scope_in (control, omit_empty) - parent in-paths.
	ParentScopeIn []string `pt:"parent_scope_in,control,omit_empty"`

	// 16. uncertainty_mean (control, omit_zero) - WorkItem uncertainty.
	UncertaintyMean float64 `pt:"uncertainty_mean,control,omit_zero"`
}

// init registers StrategicPlanFrame with the prompttags user-frame registry.
// Panics at process start on any drift between struct / FrameSpec / i18n guide.
func init() {
	prompttags.MustRegisterFrame[StrategicPlanFrame](prompttags.FramePlanUser)
}

// buildStrategicPlanFrame converts StrategicPlanInput (domain) to
// StrategicPlanFrame (LLM view), flattening nested DivergenceBudget
// and applying the same conditional guards as the prior 35-line
// manual map. 0 行为变化: guards + field order are identical.
//
// Guards retained for byte-equivalence with buildStrategicPlanUserPrompt v1:
//   - ObservationIDs: emit iff len > 0
//   - ReportSummary : emit iff TrimSpace != ""
//   - Budget 9 fields: emit iff Budget.MaxChildren > 0
//   - ParentScopeIn : emit iff len > 0
//   - UncertaintyMean: emit iff > 0
//   - PriorParseReject: emit iff TrimSpace != ""
func buildStrategicPlanFrame(in StrategicPlanInput) StrategicPlanFrame {
	frame := StrategicPlanFrame{
		WorkItemID: in.WorkItemID,
		Directive:  in.Directive,
	}
	if len(in.ObservationIDs) > 0 {
		frame.ObservationIDs = in.ObservationIDs
	}
	if s := strings.TrimSpace(in.ReportSummary); s != "" {
		frame.ObservationSummary = s
	}
	// RH-MUPS-07 (DM-20260701-001 T-P1-2): flatten Budget when MaxChildren > 0.
	if in.Budget.MaxChildren > 0 {
		b := in.Budget
		d, md := b.Depth, b.MaxDepth
		frame.Depth = &d
		frame.MaxDepth = &md
		ec, rc, mc := b.ExistingChildren, b.RemainingChildren(), b.MaxChildren
		frame.ExistingChildren = &ec
		frame.RemainingChildren = &rc
		frame.MaxChildren = &mc
		dut, rd, md2 := b.DecomposeUsedToday, b.RemainingDaily(), b.MaxDaily
		frame.DecomposeUsedToday = &dut
		frame.RemainingDaily = &rd
		frame.MaxDaily = &md2
		mi := b.MaxIters
		frame.MaxIters = &mi
	}
	if len(in.ParentScopeIn) > 0 {
		frame.ParentScopeIn = in.ParentScopeIn
	}
	if in.UncertaintyMean > 0 {
		frame.UncertaintyMean = in.UncertaintyMean
	}
	if s := strings.TrimSpace(in.PriorParseReject); s != "" {
		frame.PriorParseReject = s
	}
	return frame
}

// planFrameToMap converts a StrategicPlanFrame to the map[TagName]any
// expected by i18n.RenderFrameFieldGuideForFields, applying the same
// omit_empty / omit_zero rules as the kernel. This preserves the v1
// behavior of only emitting when-use guides for fields actually present
// in the rendered user frame.
func planFrameToMap(frame StrategicPlanFrame) map[prompttags.TagName]any {
	out := map[prompttags.TagName]any{}
	v := reflect.ValueOf(frame)
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		raw, ok := f.Tag.Lookup("pt")
		if !ok || raw == "-" {
			continue
		}
		parts := strings.Split(raw, ",")
		if len(parts) < 2 {
			continue
		}
		name := prompttags.TagName(strings.TrimSpace(parts[0]))
		plane := strings.TrimSpace(parts[1])
		if plane != string(prompttags.PlaneData) && plane != string(prompttags.PlaneControl) {
			continue
		}
		oe, oz := false, false
		for _, p := range parts[2:] {
			p = strings.TrimSpace(p)
			switch p {
			case "omit_empty":
				oe = true
			case "omit_zero":
				oz = true
			}
		}
		fv := v.Field(i)
		if oe && isFrameEmptyValue(fv) {
			continue
		}
		if oz && fv.IsZero() {
			continue
		}
		// Unwrap pointer if non-nil; nil pointer -> field is absent.
		for fv.Kind() == reflect.Ptr {
			if fv.IsNil() {
				fv = reflect.Value{}
				break
			}
			fv = fv.Elem()
		}
		switch fv.Kind() {
		case reflect.String:
			out[name] = fv.String()
		case reflect.Float32, reflect.Float64:
			out[name] = fv.Float()
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			out[name] = fv.Int()
		case reflect.Slice, reflect.Array:
			if fv.Len() > 0 {
				ss := make([]string, 0, fv.Len())
				for j := 0; j < fv.Len(); j++ {
					if fv.Index(j).Kind() == reflect.String {
						ss = append(ss, fv.Index(j).String())
					}
				}
				if len(ss) > 0 {
					out[name] = ss
				}
			}
		}
	}
	return out
}

func isFrameEmptyValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.String:
		return v.String() == ""
	case reflect.Slice, reflect.Array, reflect.Map:
		return v.Len() == 0
	case reflect.Ptr, reflect.Interface:
		return v.IsNil()
	}
	return false
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

// buildStrategicPlanUserPrompt serializes StrategicPlanInput to the
// Plan user prompt via reflection-based BuildLineFrameFromStruct
// (DM-20260705-004 go-struct-driven M2). Schema is the sole
// responsibility of StrategicPlanFrame's pt struct tags; this function
// only chooses guide header + frame body. 0 行为变化 vs the prior
// 35-line manual map (T-P1-4): field order, plane, omit guards, and
// guide-only-when-set semantics are preserved.
func buildStrategicPlanUserPrompt(in StrategicPlanInput, loc i18n.Locale) string {
	frame := buildStrategicPlanFrame(in)
	userFrame := prompttags.BuildLineFrameFromStruct(prompttags.FramePlanUser, frame)
	fieldMap := planFrameToMap(frame)
	guide := i18n.RenderFrameFieldGuideForFields(prompttags.FramePlanUser, loc, fieldMap)
	if guide == "" {
		return userFrame
	}
	return guide + "\n\n" + userFrame
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
