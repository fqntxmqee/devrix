package workmodel

import (
	"fmt"
	"path"
	"strings"
)

// DefaultDecomposeProposer fills ChildSpecs when SpawnDecompose fires and no
// LLM proposer ran. Structural fallback only: split by ScopeIn paths or pass
// through the parent directive — never inject natural-language tactics (see
// workspace coding rule: orchestration tactical hardcoding).
func DefaultDecomposeProposer(item *WorkItem, round *WorkItemPipelineRound) []ChildSpec {
	if item == nil || round == nil {
		return nil
	}
	exploratory := IsExploratoryPlanKind(round.PlanKind)
	kind := ChildKindForHypothesis(exploratory)
	base := strings.TrimSpace(itemDirectiveForProposer(item))
	if base == "" {
		return nil
	}
	expected := DefaultChildExpectedReturn(item, base)

	var scopePaths []string
	if item.ScopeContract != nil {
		scopePaths = append(scopePaths, item.ScopeContract.InScope...)
	}
	if len(scopePaths) == 0 {
		scopePaths = InferScopeInFromDirective(base)
	}
	filePaths := filterDecomposeFilePaths(scopePaths)
	if len(filePaths) >= 2 {
		mid := len(filePaths) / 2
		return []ChildSpec{
			scopeSliceChildSpec(kind, base, filePaths[:mid], expected),
			scopeSliceChildSpec(kind, base, filePaths[mid:], expected),
		}
	}
	if len(filePaths) == 1 {
		return []ChildSpec{{
			Kind:           kind,
			Title:          scopeSliceTitle(filePaths[0]),
			Directive:      base,
			ScopeIn:        append([]string(nil), filePaths...),
			ExpectedReturn: expected,
		}}
	}
	return []ChildSpec{{
		Kind:           kind,
		Title:          passThroughChildTitle(base),
		Directive:      base,
		ExpectedReturn: expected,
	}}
}

func scopeSliceChildSpec(kind WorkKind, base string, paths []string, expected string) ChildSpec {
	return ChildSpec{
		Kind:           kind,
		Title:          scopeSliceTitle(paths[0]),
		Directive:      base,
		ScopeIn:        append([]string(nil), paths...),
		ExpectedReturn: expected,
	}
}

func scopeSliceTitle(firstPath string) string {
	firstPath = strings.TrimSpace(firstPath)
	if firstPath == "" {
		return "scope_slice"
	}
	return "scope:" + path.Base(firstPath)
}

func passThroughChildTitle(base string) string {
	line := strings.Split(strings.TrimSpace(base), "\n")[0]
	if len([]rune(line)) > 48 {
		line = string([]rune(line)[:48])
	}
	if line == "" {
		return "work_slice"
	}
	return fmt.Sprintf("slice:%s", line)
}

func filterDecomposeFilePaths(paths []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] {
			continue
		}
		if strings.Contains(p, "/") || strings.HasSuffix(p, ".go") {
			seen[p] = true
			out = append(out, p)
		}
	}
	return out
}

func itemDirectiveForProposer(item *WorkItem) string {
	if item == nil {
		return ""
	}
	if d := item.Directive; d != "" {
		return d
	}
	return item.Title
}
