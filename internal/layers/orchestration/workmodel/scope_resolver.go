package workmodel

import (
	"path/filepath"
	"regexp"
	"strings"
)

// scenarioScopeEntry maps a registered L2 scenario slug to a repo path prefix
// (mirrors openspec/specs/architecture/code-layout.md scenario registry).
type scenarioScopeEntry struct {
	DomainTokens []string
	ScenarioSlug string
	PathPrefix   string
}

var registeredScenarioScopes = []scenarioScopeEntry{
	{
		DomainTokens: []string{"d7", "orchestration"},
		ScenarioSlug: "plan",
		PathPrefix:   "internal/layers/orchestration/plan/",
	},
}

var explicitInternalPathRE = regexp.MustCompile(`internal/layers/[\w./-]+`)

// DirectiveScopeCandidates returns repo path prefixes implied by the directive
// via registry lookup and explicit internal/ path literals.
func DirectiveScopeCandidates(directive string) []string {
	directive = strings.TrimSpace(directive)
	if directive == "" {
		return nil
	}
	lower := strings.ToLower(directive)
	seen := map[string]struct{}{}
	var out []string
	add := func(p string) {
		p = normalizeScopePrefix(p)
		if p == "" {
			return
		}
		if _, ok := seen[p]; ok {
			return
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	for _, e := range registeredScenarioScopes {
		if scenarioMentionedInDirective(lower, e) {
			add(e.PathPrefix)
		}
	}
	for _, m := range explicitInternalPathRE.FindAllString(directive, -1) {
		add(m)
	}
	return out
}

func scenarioMentionedInDirective(lower string, e scenarioScopeEntry) bool {
	domainOK := len(e.DomainTokens) == 0
	for _, d := range e.DomainTokens {
		if strings.Contains(lower, strings.ToLower(d)) {
			domainOK = true
			break
		}
	}
	if !domainOK {
		return false
	}
	slug := strings.ToLower(e.ScenarioSlug)
	if strings.Contains(lower, "/"+slug+"/") || strings.Contains(lower, slug+"/") {
		return true
	}
	if slug == "plan" && strings.Contains(lower, "plan") && strings.Contains(lower, "目录") {
		return true
	}
	return strings.Contains(lower, "/"+slug)
}

func normalizeScopePrefix(p string) string {
	p = filepath.ToSlash(strings.TrimSpace(p))
	if p == "" {
		return ""
	}
	if !strings.HasSuffix(p, "/") {
		p += "/"
	}
	return p
}

func scopeIntersectsCandidates(scopePaths, candidates []string) bool {
	if len(candidates) == 0 {
		return true
	}
	for _, raw := range scopePaths {
		p := normalizeScopePrefix(raw)
		if p == "" {
			continue
		}
		for _, c := range candidates {
			c = normalizeScopePrefix(c)
			if strings.HasPrefix(p, c) || strings.HasPrefix(c, p) {
				return true
			}
		}
	}
	return false
}

func filterExistingScopePaths(scopeIn []string, workDir string) ([]string, []string) {
	var valid, invalid []string
	for _, raw := range scopeIn {
		p := strings.TrimSpace(raw)
		if p == "" {
			continue
		}
		if blockedScopePath(p) {
			invalid = append(invalid, p)
			continue
		}
		if workDir != "" && !scopePathExists(workDir, p) {
			invalid = append(invalid, p)
			continue
		}
		valid = append(valid, p)
	}
	return valid, invalid
}
