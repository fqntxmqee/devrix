package i18n

import (
	"fmt"
	"strings"

	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/prompttags"
)

func localizeCondition(loc Locale, cond prompttags.SemanticCondition) string {
	if cond == "" || cond == prompttags.CondApplies {
		return ""
	}
	if loc == LocaleEN {
		if s, ok := semanticGlossaryEN[cond]; ok {
			return s
		}
	} else if s, ok := semanticGlossaryZH[cond]; ok {
		return s
	}
	return string(cond)
}

func semanticNodeRole(loc Locale, key string) string {
	if loc == LocaleEN {
		if s, ok := semanticNodeRoleEN[key]; ok {
			return s
		}
	} else if s, ok := semanticNodeRoleZH[key]; ok {
		return s
	}
	return key
}

// semanticText resolves a node-role key for tests and legacy call sites.
func semanticText(loc Locale, key string) string {
	return semanticNodeRole(loc, key)
}

func planeGuide(loc Locale) string {
	if loc == LocaleEN {
		return semanticPlaneGuideEN
	}
	return semanticPlaneGuideZH
}

// RenderSemanticAppendix returns locale-aware semantic appendix: machine JSON-lines + glossary.
// Observe/Plan phase appendices already declare node role in intro — node_role keys are
// omitted there. Execute keeps react/final section hints only (role lives in WorkItemExecuteIntro).
func RenderSemanticAppendix(phase contracts.MUPSPhase, loc Locale) string {
	block := prompttags.SemanticBlock(phase)
	if block == "" && phase != contracts.MUPSPhaseExecute {
		return ""
	}
	var b strings.Builder
	if block != "" {
		if loc == LocaleEN {
			b.WriteString("Semantic rules (machine-readable):\n")
		} else {
			b.WriteString("语义规则（机器可读）：\n")
		}
		b.WriteString(block)
		glossary := renderConditionGlossary(loc, prompttags.SemanticConditionsForPhase(phase))
		if glossary != "" {
			b.WriteString("\n")
			b.WriteString(glossary)
		}
	}
	if phase == contracts.MUPSPhaseExecute {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(semanticNodeRole(loc, "execute.output.section.react"))
		b.WriteString("\n")
		b.WriteString(semanticNodeRole(loc, "execute.output.section.final"))
	}
	return strings.TrimSpace(b.String())
}

func renderConditionGlossary(loc Locale, conds []prompttags.SemanticCondition) string {
	if len(conds) == 0 {
		return ""
	}
	var b strings.Builder
	if loc == LocaleEN {
		b.WriteString("Condition glossary:\n")
	} else {
		b.WriteString("条件说明：\n")
	}
	for _, c := range conds {
		if label := localizeCondition(loc, c); label != "" {
			fmt.Fprintf(&b, "- %s: %s\n", c, label)
		}
	}
	return strings.TrimSpace(b.String())
}

// RenderFrameFieldGuide returns a compact control/data plane guide for Observe/Plan user frames.
func RenderFrameFieldGuide(frame prompttags.FrameName, loc Locale) string {
	return RenderFrameFieldGuideForFields(frame, loc, nil)
}

// RenderFrameFieldGuideForFields documents only fields present in fields (all spec fields when nil).
func RenderFrameFieldGuideForFields(frame prompttags.FrameName, loc Locale, fields map[prompttags.TagName]any) string {
	spec, ok := prompttags.LookupLineFrame(frame)
	if !ok {
		return ""
	}
	var b strings.Builder
	if loc == LocaleEN {
		b.WriteString("User frame fields:\n")
	} else {
		b.WriteString("User 帧字段：\n")
	}
	present := fields
	for _, rule := range prompttags.InputRulesForFrame(frame) {
		name := prompttags.TagName(rule.Target)
		if present != nil {
			if _, ok := present[name]; !ok {
				continue
			}
		} else {
			var inSpec bool
			for _, f := range spec.Fields {
				if f == name {
					inSpec = true
					break
				}
			}
			if !inSpec {
				continue
			}
		}
		label := localizeCondition(loc, rule.WhenUse)
		if label == "" {
			label = string(rule.WhenUse)
		}
		fmt.Fprintf(&b, "- %s: %s\n", rule.Target, label)
	}
	return strings.TrimSpace(b.String())
}
