package workmodel

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const imFindingFieldMaxRunes = 320

// FormatDeliverablePayloadForIM renders structured review findings as a
// compact markdown report for IM adapters (Feishu 任务总结 card).
func FormatDeliverablePayloadForIM(p *DeliverablePayload) string {
	if p == nil {
		return ""
	}
	findings := normalizeDeliverableFindings(p.Findings)
	if len(findings) == 0 {
		return ""
	}
	var p0, p1 int
	for _, f := range findings {
		switch strings.ToUpper(strings.TrimSpace(f.Severity)) {
		case "P0":
			p0++
		case "P1":
			p1++
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "## Code Review 结论\n\n**P0:** %d | **P1:** %d\n\n", p0, p1)
	for i, f := range findings {
		sev := strings.ToUpper(strings.TrimSpace(f.Severity))
		if sev == "" {
			sev = "P1"
		}
		title := strings.TrimSpace(f.Title)
		if title == "" {
			title = strings.TrimSpace(f.Message)
		}
		if title == "" {
			title = "finding"
		}
		fmt.Fprintf(&b, "### [%s] %s\n", sev, title)
		file := strings.TrimSpace(f.File)
		if file != "" {
			if f.Line > 0 {
				fmt.Fprintf(&b, "- 位置: `%s:%d`\n", file, f.Line)
			} else {
				fmt.Fprintf(&b, "- 位置: `%s`\n", file)
			}
		}
		if s := truncateIMField(f.Evidence); s != "" {
			fmt.Fprintf(&b, "- 证据: %s\n", s)
		}
		if s := truncateIMField(f.Impact); s != "" {
			fmt.Fprintf(&b, "- 影响: %s\n", s)
		}
		if s := truncateIMField(f.Recommendation); s != "" {
			fmt.Fprintf(&b, "- 建议: %s\n", s)
		} else if s := truncateIMField(f.Message); s != "" && s != title {
			fmt.Fprintf(&b, "- 说明: %s\n", s)
		}
		if i+1 < len(findings) {
			b.WriteString("\n")
		}
	}
	return strings.TrimSpace(b.String())
}

func truncateIMField(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if utf8.RuneCountInString(s) <= imFindingFieldMaxRunes {
		return s
	}
	runes := []rune(s)
	return string(runes[:imFindingFieldMaxRunes]) + "…"
}
