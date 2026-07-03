package workmodel

import (
	"os"
	"path/filepath"
	"strings"
)

var defaultScopeBlocklist = []string{
	"../",
	"/proc",
	"/sys",
	"node_modules/",
	".git/",
}

// FilterValidatedChildSpecs drops invalid child scopes; returns possibly empty slice.
func FilterValidatedChildSpecs(parent *WorkItem, specs []ChildSpec, workDir string) []ChildSpec {
	if len(specs) == 0 {
		return specs
	}
	out := make([]ChildSpec, 0, len(specs))
	for _, spec := range specs {
		if ok, _ := ValidateChildSpecScope(parent, spec, workDir); ok {
			out = append(out, spec)
		}
	}
	return out
}

// ValidateChildSpecScope checks blocklist, parent subset, and optional repo existence.
func ValidateChildSpecScope(parent *WorkItem, spec ChildSpec, workDir string) (bool, string) {
	for _, p := range spec.ScopeIn {
		path := strings.TrimSpace(p)
		if path == "" {
			continue
		}
		if blockedScopePath(path) {
			return false, "blocklisted path: " + path
		}
		if workDir != "" && !scopePathExists(workDir, path) {
			return false, "path not found under workdir: " + path
		}
	}
	if parent != nil && parent.ScopeContract != nil && len(spec.ScopeIn) > 0 {
		tmp := []*WorkItem{{
			ID:    "scope_check",
			Title: spec.Title,
			ScopeContract: &ScopeContract{
				InScope: append([]string(nil), spec.ScopeIn...),
			},
		}}
		res := ValidateChildScopes(parent, tmp)
		if !res.OK {
			msg := "scope not subset of parent"
			if len(res.Violations) > 0 {
				msg = res.Violations[0].Message
			}
			return false, msg
		}
	}
	return true, ""
}

// PrepareStrategicScopeIn validates Strategic Plan scope_in for Goal/single paths
// (CC-2 extension, L5-D7-CC-05). Returns corrected scope paths, whether the
// proposal was accepted as-is, and a machine reason when fallback applied.
func PrepareStrategicScopeIn(directive string, proposed []string, workDir string) (scopeOut []string, accepted bool, reason string) {
	valid, invalid := filterExistingScopePaths(proposed, workDir)
	candidates := DirectiveScopeCandidates(directive)

	if len(valid) > 0 && len(candidates) > 0 && !scopeIntersectsCandidates(valid, candidates) {
		if fallback := filterExistingScopeCandidates(candidates, workDir); len(fallback) > 0 {
			return fallback, false, "scope_disjoint_from_directive:" + strings.Join(valid, ",")
		}
	}

	if len(valid) > 0 {
		return valid, true, ""
	}

	if len(invalid) > 0 && len(candidates) > 0 {
		if fallback := filterExistingScopeCandidates(candidates, workDir); len(fallback) > 0 {
			return fallback, false, "scope_invalid_fallback:" + strings.Join(invalid, ",")
		}
	}

	if len(valid) > 0 {
		return valid, true, ""
	}
	if len(candidates) > 0 {
		if fallback := filterExistingScopeCandidates(candidates, workDir); len(fallback) > 0 {
			return fallback, false, "scope_empty_fallback"
		}
	}
	return nil, false, "scope_no_valid_paths"
}

func filterExistingScopeCandidates(candidates []string, workDir string) []string {
	var out []string
	for _, c := range candidates {
		c = strings.TrimSpace(c)
		if c == "" || blockedScopePath(c) {
			continue
		}
		if workDir != "" && !scopePathExists(workDir, c) {
			continue
		}
		out = append(out, normalizeScopePrefix(c))
	}
	return out
}

func blockedScopePath(path string) bool {
	norm := filepath.ToSlash(strings.TrimSpace(path))
	if strings.Contains(norm, "../") {
		return true
	}
	lower := strings.ToLower(norm)
	for _, b := range defaultScopeBlocklist {
		if strings.Contains(lower, strings.ToLower(b)) {
			return true
		}
	}
	return false
}

func scopePathExists(workDir, scopePath string) bool {
	workDir = strings.TrimSpace(workDir)
	scopePath = strings.TrimSpace(scopePath)
	if workDir == "" || scopePath == "" {
		return true
	}
	clean := filepath.Clean(scopePath)
	if filepath.IsAbs(clean) {
		rel, err := filepath.Rel(workDir, clean)
		if err != nil || strings.HasPrefix(rel, "..") {
			return false
		}
	} else {
		clean = filepath.Join(workDir, clean)
	}
	info, err := os.Stat(clean)
	if err != nil {
		return false
	}
	return info.Mode().IsRegular() || info.IsDir()
}
