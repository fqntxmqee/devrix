package prompttags

import (
	"encoding/json"
	"strings"
)

// SemanticRule documents LLM-facing guidance for one registry target (tag, kind, or field).
type SemanticRule struct {
	Target   string              `json:"target"`
	Plane    PromptPlane         `json:"plane,omitempty"`
	WhenUse  SemanticCondition   `json:"when"`
	WhenNot  SemanticCondition   `json:"when_not,omitempty"`
	Enforced bool                `json:"enforced,omitempty"`
	Gate     string              `json:"gate,omitempty"`
}

// FieldSemantic is the legacy alias kept for call-site compatibility during migration.
type FieldSemantic = SemanticRule

// MachineLine returns a compact JSON object line (same profile as ParseRejectRecord).
func (r SemanticRule) MachineLine() string {
	if r.Target == "" && r.WhenUse == "" {
		return ""
	}
	line := r
	if line.WhenUse == "" {
		line.WhenUse = CondApplies
	}
	b, err := json.Marshal(line)
	if err != nil {
		return ""
	}
	return string(b)
}

// ConditionsReferenced returns unique WhenUse/WhenNot codes for glossary rendering.
func (r SemanticRule) ConditionsReferenced() []SemanticCondition {
	var out []SemanticCondition
	if r.WhenUse != "" && r.WhenUse != CondApplies {
		out = append(out, r.WhenUse)
	}
	if r.WhenNot != "" {
		out = append(out, r.WhenNot)
	}
	return out
}

func uniqueConditions(rules []SemanticRule) []SemanticCondition {
	seen := map[SemanticCondition]struct{}{}
	var out []SemanticCondition
	for _, rule := range rules {
		for _, c := range rule.ConditionsReferenced() {
			if _, ok := seen[c]; ok {
				continue
			}
			seen[c] = struct{}{}
			out = append(out, c)
		}
	}
	return out
}

func inputSemanticsForFrame(frame FrameName) map[TagName]SemanticRule {
	switch frame {
	case FrameObserveUser:
		return observeInputSemantics
	case FramePlanUser:
		return planInputSemantics
	default:
		return nil
	}
}

// InputRulesForFrame derives input semantics from LineFrameRegistry field order.
func InputRulesForFrame(frame FrameName) []SemanticRule {
	spec, ok := LookupLineFrame(frame)
	if !ok {
		return nil
	}
	byTag := inputSemanticsForFrame(frame)
	var out []SemanticRule
	for _, name := range spec.Fields {
		base, ok := byTag[name]
		if !ok {
			continue
		}
		rule := base
		rule.Target = string(name)
		rule.Plane = FrameFieldPlane(frame, name)
		out = append(out, rule)
	}
	return out
}

var observeInputSemantics = map[TagName]SemanticRule{
	TagDirective:           {WhenUse: CondTaskDirective},
	TagPriorParseReject:    {WhenUse: CondPriorParseRejectFeedback, Enforced: true, Gate: "ParseRejectRecord"},
	TagPriorMean:           {WhenUse: CondPriorMeanControl},
	TagScopeGoal:           {WhenUse: CondScopeGoal},
	TagScopeOpenQuestion:   {WhenUse: CondScopeOpenQuestions},
	TagSignal:              {WhenUse: CondStructuredSignals},
	TagPriorObservationIDs: {WhenUse: CondPriorObservationIDs},
	TagIncrementalOnly:     {WhenUse: CondIncrementalRound, Enforced: true, Gate: "incremental_observe"},
	TagWorkItemID:          {WhenUse: CondWorkItemIdentifier},
}

var planInputSemantics = map[TagName]SemanticRule{
	TagDirective:           {WhenUse: CondTaskDirective},
	TagPriorParseReject:    {WhenUse: CondPriorParseRejectFeedback, Enforced: true, Gate: "ParseRejectRecord"},
	TagObservationIDs:      {WhenUse: CondObservationIDs},
	TagObservationSummary:  {WhenUse: CondObservationSummary},
	TagDepth:               {WhenUse: CondDepthControl},
	TagMaxDepth:            {WhenUse: CondMaxDepthControl},
	TagExistingChildren:    {WhenUse: CondExistingChildrenControl},
	TagRemainingChildren:   {WhenUse: CondRemainingChildrenBudget},
	TagMaxChildren:         {WhenUse: CondMaxChildrenControl},
	TagDecomposeUsedToday:  {WhenUse: CondDecomposeUsedToday},
	TagRemainingDaily:      {WhenUse: CondRemainingDaily},
	TagMaxDaily:            {WhenUse: CondMaxDailyControl},
	TagMaxIters:            {WhenUse: CondMaxItersControl},
	TagParentScopeIn:       {WhenUse: CondParentScopeInControl},
	TagUncertaintyMean:     {WhenUse: CondUncertaintyMeanControl, Enforced: true, Gate: "applySingleModeUncertaintyGate"},
	TagWorkItemID:          {WhenUse: CondWorkItemIdentifier, Enforced: true, Gate: "work_item_id"},
}

func inputRuleWhenKey(frame FrameName, name TagName) SemanticCondition {
	for _, rule := range InputRulesForFrame(frame) {
		if rule.Target == string(name) {
			return rule.WhenUse
		}
	}
	return ""
}

func trimSemanticBlock(s string) string {
	return strings.TrimSpace(s)
}
