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
