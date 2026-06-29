package workmodel

import "strings"

// DefaultDecomposeProposer fills ChildSpecs when SpawnDecompose fires and no
// LLM proposer ran. Splits by ScopeContract paths or concrete review slices —
// never abstract "hypothesis A/B" labels that confuse downstream executors.
func DefaultDecomposeProposer(item *WorkItem, round *WorkItemPipelineRound) []ChildSpec {
	if item == nil || round == nil {
		return nil
	}
	exploratory := IsExploratoryPlanKind(round.PlanKind)
	kind := ChildKindForHypothesis(exploratory)
	base := itemDirectiveForProposer(item)

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
			scopeSliceChildSpec(kind, base, "in-scope slice A", filePaths[:mid]),
			scopeSliceChildSpec(kind, base, "in-scope slice B", filePaths[mid:]),
		}
	}

	return []ChildSpec{
		{
			Kind:  kind,
			Title: "contracts and API surface",
			Directive: scopedDecomposeDirective(base,
				"聚焦契约、类型定义与跨包接口。只 read_file/grep in-scope 路径，禁止探索无关目录。输出 P0/P1 清单，每条含 file:line。"),
			ExpectedReturn: "P0/P1 findings for contracts and types with file:line citations",
		},
		{
			Kind:  kind,
			Title: "implementation and observability",
			Directive: scopedDecomposeDirective(base,
				"聚焦实现逻辑、span/observability 与测试覆盖。只 read_file/grep in-scope 路径，禁止探索无关目录。输出 P0/P1 清单，每条含 file:line。"),
			ExpectedReturn: "P0/P1 findings for implementation and tests with file:line citations",
		},
	}
}

func scopeSliceChildSpec(kind WorkKind, base, title string, paths []string) ChildSpec {
	return ChildSpec{
		Kind:           kind,
		Title:          title,
		Directive:      scopedDecomposeDirective(base, "只审查以下路径："+strings.Join(paths, ", ")+"。禁止探索 scope 外目录。输出 P0/P1 清单，每条含 file:line。"),
		ScopeIn:        append([]string(nil), paths...),
		ExpectedReturn: "P0/P1 findings for assigned paths with file:line citations",
	}
}

func scopedDecomposeDirective(base, focus string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		return focus
	}
	return base + "\n\n" + focus
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
