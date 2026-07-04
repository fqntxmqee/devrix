package workmodel

import (
	"fmt"
	"strings"
)

// FormatDeliverablePayloadForIM renders structured review findings as a
// compact markdown report for IM adapters (Feishu 任务总结 card).
func FormatDeliverablePayloadForIM(p *DeliverablePayload) string {
	if p == nil || len(p.Findings) == 0 {
		return ""
	}
	var p0, p1 int
	for _, f := range p.Findings {
		switch strings.ToUpper(strings.TrimSpace(f.Severity)) {
		case "P0":
			p0++
		case "P1":
			p1++
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "## Code Review 结论\n\n**P0:** %d | **P1:** %d\n\n", p0, p1)
	for i, f := range p.Findings {
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
				fmt.Fprintf(&b, "- `%s:%d`\n", file, f.Line)
			} else {
				fmt.Fprintf(&b, "- `%s`\n", file)
			}
		}
		if i+1 < len(p.Findings) {
			b.WriteString("\n")
		}
	}
	return strings.TrimSpace(b.String())
}
