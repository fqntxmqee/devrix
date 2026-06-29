package workmodel

import (
	"regexp"
	"strings"
)

var scopePathPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?:^|[\s"'(])([\w./-]+\.(?:go|ts|tsx|js|py|md|yaml|yml|json))(?:[\s"'),]|$)`),
	regexp.MustCompile(`(?:^|[\s"'(])(internal/[\w./-]+)(?:[\s"'),]|$)`),
}

// InferScopeInFromDirective extracts concrete file/path hints from a directive (D7-S16-A60-T04).
func InferScopeInFromDirective(directive string) []string {
	directive = strings.TrimSpace(directive)
	if directive == "" {
		return nil
	}
	seen := make(map[string]struct{})
	var out []string
	for _, re := range scopePathPatterns {
		for _, m := range re.FindAllStringSubmatch(directive, -1) {
			if len(m) < 2 {
				continue
			}
			p := strings.TrimSpace(m[1])
			if p == "" {
				continue
			}
			if _, ok := seen[p]; ok {
				continue
			}
			seen[p] = struct{}{}
			out = append(out, p)
		}
	}
	return out
}
