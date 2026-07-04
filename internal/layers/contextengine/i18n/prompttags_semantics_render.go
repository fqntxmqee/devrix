package i18n

import (
	"fmt"
	"strings"

	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/prompttags"
)

func semanticText(loc Locale, key string) string {
	if loc == LocaleEN {
		if s, ok := prompttagsSemanticsEN[key]; ok {
			return s
		}
	} else if s, ok := prompttagsSemanticsZH[key]; ok {
		return s
	}
	return key
}

// RenderSemanticAppendix returns locale-aware semantic bullets for phase (no schema duplication).
func RenderSemanticAppendix(phase contracts.MUPSPhase, loc Locale) string {
	sem := prompttags.SemanticsForPhase(phase)
	if sem.NodeRoleKey == "" && len(sem.OutputRules) == 0 {
		return ""
	}
	var b strings.Builder
	if sem.NodeRoleKey != "" {
		b.WriteString(semanticText(loc, sem.NodeRoleKey))
	}
	if len(sem.OutputRules) > 0 {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		if loc == LocaleEN {
			b.WriteString("Semantics:\n")
		} else {
			b.WriteString("语义：\n")
		}
		for _, rule := range sem.OutputRules {
			writeSemanticBullet(&b, loc, rule)
		}
	}
	if phase == contracts.MUPSPhaseExecute {
		b.WriteString("\n")
		b.WriteString(semanticText(loc, "execute.output.section.react"))
		b.WriteString("\n")
		b.WriteString(semanticText(loc, "execute.output.section.final"))
	}
	return strings.TrimSpace(b.String())
}

func writeSemanticBullet(b *strings.Builder, loc Locale, rule prompttags.FieldSemantic) {
	line := fmt.Sprintf("- %s: %s", rule.Name, semanticText(loc, rule.WhenUse))
	if rule.WhenNot != "" {
		line += "; " + semanticText(loc, rule.WhenNot)
	}
	if rule.Enforced {
		if loc == LocaleEN {
			line += " [enforced]"
		} else {
			line += " [enforce]"
		}
	}
	b.WriteString(line)
	b.WriteString("\n")
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
	b.WriteString(semanticText(loc, "plane.guide"))
	b.WriteString("\n")
	for _, name := range spec.Fields {
		if fields != nil {
			if _, ok := fields[name]; !ok {
				continue
			}
		}
		plane := prompttags.FrameFieldPlane(frame, name)
		whenKey := frameFieldWhenKey(frame, name)
		if whenKey == "" {
			continue
		}
		fmt.Fprintf(&b, "- [%s] %s: %s\n", plane, name, semanticText(loc, whenKey))
	}
	return strings.TrimSpace(b.String())
}

func frameFieldWhenKey(frame prompttags.FrameName, name prompttags.TagName) string {
	phase := contracts.MUPSPhaseObserve
	if frame == prompttags.FramePlanUser {
		phase = contracts.MUPSPhasePlan
	}
	for _, rule := range prompttags.SemanticsForPhase(phase).InputRules {
		if rule.Name == string(name) {
			return rule.WhenUse
		}
	}
	return ""
}
