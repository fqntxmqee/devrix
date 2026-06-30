package sessionorchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/devrix/devrix/internal/layers/contextengine/i18n"
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/types"
)

const (
	strategicPlanAppendixZH = `你是编排 Plan 节点的战略提案助手。仅返回 JSON 对象（不要 markdown）：
{"execution_mode":"single|decompose|parallel_probe","scope_in":["path/"],"child_specs":[],"deliverable_schema":"p0_p1_file_line|not_applicable","react_iters_hint":5,"rationale":"..."}

规则：
- 只能使用下方 directive 与 Obs 摘要；不要编造未提供的文件列表。
- 范围清晰且可一次完成时优先 execution_mode=single。
- decompose 时 child_specs 每项含 title、directive_suffix、expected_return；最多 2 项。
- react_iters_hint 范围 1-5。`

	strategicPlanAppendixEN = `You propose strategic execution plans for an orchestration Plan node.
Return ONLY a JSON object (no markdown):
{"execution_mode":"single|decompose|parallel_probe","scope_in":["path/"],"child_specs":[],"deliverable_schema":"p0_p1_file_line|not_applicable","react_iters_hint":5,"rationale":"..."}

Rules:
- Use ONLY the directive and observation summary below; do not invent files.
- Prefer execution_mode=single when scope is clear and small enough for one pass.
- For decompose, each child_specs entry needs title, directive_suffix, expected_return; max 2.
- react_iters_hint between 1 and 5.`
)

// StrategicPlanInput carries Plan-phase context for LLM strategic proposals.
type StrategicPlanInput struct {
	SessionID      string
	WorkItemID     string
	Directive      string
	ObservationIDs []string
	ReportSummary  string
}

type rawStrategicChildSpec struct {
	Title          string `json:"title"`
	DirectiveSuffix string `json:"directive_suffix"`
	ExpectedReturn string `json:"expected_return"`
	ScopeIn        []string `json:"scope_in"`
}

type rawStrategicPlan struct {
	ExecutionMode    string                  `json:"execution_mode"`
	ScopeIn          []string                `json:"scope_in"`
	ChildSpecs       []rawStrategicChildSpec `json:"child_specs"`
	DeliverableSchema string                 `json:"deliverable_schema"`
	ReactItersHint   int                     `json:"react_iters_hint"`
	Rationale        string                  `json:"rationale"`
}

// StrategicPlanProposal is a validated LLM strategic plan (DM-20260630-012).
type StrategicPlanProposal struct {
	ExecutionMode     string
	ScopeIn           []string
	ChildSpecs        []workmodel.ChildSpec
	DeliverableSchema workmodel.DeliverableSchema
	ReactItersHint    int
	QuantizedKind     string
	Rationale         string
}

// StrategicPlanProposer calls D2 Prepare then D3 for Plan strategy (G3).
type StrategicPlanProposer interface {
	ProposeStrategicPlan(ctx context.Context, in StrategicPlanInput) (*StrategicPlanProposal, error)
}

// LLMStrategicPlanProposer implements StrategicPlanProposer via D2→D3.
type LLMStrategicPlanProposer struct {
	LLM    orchtypes.LLMInvoker
	Ctx    ContextPreparer
	Locale i18n.Locale
}

// NewLLMStrategicPlanProposer constructs a D2→D3 strategic plan proposer.
func NewLLMStrategicPlanProposer(llm orchtypes.LLMInvoker, ctx ContextPreparer, loc i18n.Locale) *LLMStrategicPlanProposer {
	if llm == nil || ctx == nil {
		return nil
	}
	if loc == "" {
		loc = i18n.DefaultLocale
	}
	return &LLMStrategicPlanProposer{LLM: llm, Ctx: ctx, Locale: loc}
}

func (p *LLMStrategicPlanProposer) ProposeStrategicPlan(ctx context.Context, in StrategicPlanInput) (*StrategicPlanProposal, error) {
	if p == nil || p.LLM == nil || p.Ctx == nil {
		return nil, nil
	}
	prepared, err := p.Ctx.Prepare(ctx, PrepareRequest{
		SessionID: in.SessionID,
		Message: types.Message{
			SessionID: in.SessionID,
			Role:      types.MessageRoleUser,
			Content:   in.Directive,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("strategic plan proposer: d2 prepare: %w", err)
	}
	systemPrompt := strings.TrimSpace(prepared.SystemPrompt)
	if appendix := strategicPlanAppendix(p.Locale); appendix != "" {
		if systemPrompt != "" {
			systemPrompt += "\n\n"
		}
		systemPrompt += appendix
	}
	user := buildStrategicPlanUserPrompt(in)
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
	return parseStrategicPlanJSON(raw, in.Directive)
}

func strategicPlanAppendix(loc i18n.Locale) string {
	if loc == i18n.LocaleEN {
		return strategicPlanAppendixEN
	}
	return strategicPlanAppendixZH
}

func buildStrategicPlanUserPrompt(in StrategicPlanInput) string {
	var b strings.Builder
	fmt.Fprintf(&b, "work_item_id: %s\n", in.WorkItemID)
	fmt.Fprintf(&b, "directive: %s\n", in.Directive)
	if len(in.ObservationIDs) > 0 {
		fmt.Fprintf(&b, "observation_ids: %s\n", strings.Join(in.ObservationIDs, ","))
	}
	if s := strings.TrimSpace(in.ReportSummary); s != "" {
		fmt.Fprintf(&b, "observation_summary: %s\n", s)
	}
	return b.String()
}

func parseStrategicPlanJSON(raw, baseDirective string) (*StrategicPlanProposal, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("strategic plan: empty response")
	}
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		raw = raw[start : end+1]
	}
	var row rawStrategicPlan
	if err := json.Unmarshal([]byte(raw), &row); err != nil {
		return nil, fmt.Errorf("strategic plan: parse json: %w", err)
	}
	prop := &StrategicPlanProposal{
		ExecutionMode: strings.ToLower(strings.TrimSpace(row.ExecutionMode)),
		ScopeIn:       append([]string(nil), row.ScopeIn...),
		Rationale:     strings.TrimSpace(row.Rationale),
		ReactItersHint: clampReactIters(row.ReactItersHint),
	}
	prop.DeliverableSchema = mapDeliverableSchema(row.DeliverableSchema, baseDirective)
	prop.QuantizedKind = mapExecutionModeToQuantizedKind(prop.ExecutionMode)
	prop.ChildSpecs = mapRawChildSpecs(baseDirective, row.ChildSpecs, prop.ExecutionMode)
	if err := validateStrategicPlan(prop); err != nil {
		return nil, err
	}
	return prop, nil
}

func mapDeliverableSchema(s, directive string) workmodel.DeliverableSchema {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case string(workmodel.DeliverableSchemaP0P1FileLine):
		return workmodel.DeliverableSchemaP0P1FileLine
	case string(workmodel.DeliverableSchemaNotApplicable):
		return workmodel.DeliverableSchemaNotApplicable
	default:
		return workmodel.InferDeliverableSchema(nil, directive, "")
	}
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
	sc := item.ScopeContract
	if sc == nil {
		sc = &workmodel.ScopeContract{}
	}
	sc.GoalStatement = strings.TrimSpace(item.Directive)
	sc.InScope = append([]string(nil), prop.ScopeIn...)
	_ = tm.SetScopeContract(sessionID, item.ID, sc)
	item.ScopeContract = sc
}
