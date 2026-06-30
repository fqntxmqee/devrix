package workmodel

import (
	"fmt"
	"sort"
	"strings"
)

// ChildScopeViolationType categorizes one ValidateChildScopes finding.
// RH-MUPS-05/09 (DM-20260701-001 T-P2-2): the validator produces a
// structured list so the runner can format prompt guidance ("your child
// scope is not a subset of parent; please set spec.scope_in") instead
// of failing silently.
type ChildScopeViolationType string

const (
	// ScopeViolationMissing — child has no scope; parent does. The LLM
	// forgot to set spec.scope_in.
	ScopeViolationMissing ChildScopeViolationType = "missing"
	// ScopeViolationNotSubset — child scope contains a path not in the
	// parent's InScope. Either the parent is too narrow, or the child
	// is reaching outside its mandate.
	ScopeViolationNotSubset ChildScopeViolationType = "not_subset"
	// ScopeViolationEmpty — parent has InScope=[]; the validator can't
	// check subset, surfaces as a warning (not a failure).
	ScopeViolationEmpty ChildScopeViolationType = "parent_unbounded"
)

// ChildScopeViolation is one finding from ValidateChildScopes.
type ChildScopeViolation struct {
	ChildWorkItemID string
	Type            ChildScopeViolationType
	OffendingPath   string // for ScopeViolationNotSubset
	ParentScopeIn   []string
	ChildScopeIn    []string
	Message         string
}

// ChildScopeValidationResult bundles the verdict + a list of violations
// for prompt guidance. OK=true means no Missing/NotSubset violations
// (Empty is a warning, not a failure — parent-unbounded is permitted
// but the LLM should know).
type ChildScopeValidationResult struct {
	OK         bool
	Violations []ChildScopeViolation
	// ChildrenCoverage reports what fraction of the parent's InScope is
	// covered by the union of the children's InScope. < 1.0 means
	// there are parent paths no child is going to touch; the LLM should
	// either add a child or move the paths to OutOfScope.
	ChildrenCoverage float64
	UncoveredPaths   []string
}

// ValidateChildScopes enforces RH-MUPS-05/09 invariants on a
// parent→children scope relationship:
//
//  1. Each child's ScopeIn MUST be a (non-strict) subset of the
//     parent's InScope. Empty parent InScope is allowed (parent is
//     unbounded) but produces a warning.
//  2. Each child's ScopeIn MUST be non-empty. An empty child scope
//     means the LLM forgot to set spec.scope_in — produce a Missing
//     violation so the prompt can ask for it.
//  3. The union of children's ScopeIn SHOULD cover the parent's
//     InScope. Under-coverage is a soft warning (UncoveredPaths) so
//     the LLM can either add a child or move the paths to
//     OutOfScope.
//
// OK=true means no hard violations; warnings still flow through
// Violations for the prompt guidance.
func ValidateChildScopes(parent *WorkItem, children []*WorkItem) ChildScopeValidationResult {
	res := ChildScopeValidationResult{OK: true}
	if parent == nil {
		return res
	}
	parentIn := scopeInStrings(parent)

	if len(parentIn) == 0 {
		// Parent is unbounded — every child is technically a valid
		// subset (any ⊆ ∅? false; any ⊆ UniversalSet? true). We
		// don't enforce subset, but we record the warning so the
		// prompt can ask the LLM to bound the parent.
		for _, c := range children {
			if c == nil {
				continue
			}
			if len(scopeInStrings(c)) == 0 {
				res.Violations = append(res.Violations, ChildScopeViolation{
					ChildWorkItemID: c.ID,
					Type:            ScopeViolationEmpty,
					Message:         "parent has no in_scope; child also has no scope — child review is unbounded, please set spec.scope_in",
				})
				res.OK = false // missing + unbounded = same risk as before the fix
			}
		}
		return res
	}

	// Parent is bounded: check each child.
	parentSet := stringSetFromSlice(parentIn)
	covered := map[string]bool{}
	hasAnyChildScope := false
	for _, c := range children {
		if c == nil {
			continue
		}
		childIn := scopeInStrings(c)
		if len(childIn) == 0 {
			res.Violations = append(res.Violations, ChildScopeViolation{
				ChildWorkItemID: c.ID,
				Type:            ScopeViolationMissing,
				ParentScopeIn:   append([]string(nil), parentIn...),
				ChildScopeIn:    nil,
				Message:         "child has empty scope_in; please set spec.scope_in to a non-empty subset of parent.scope_in",
			})
			res.OK = false
			continue
		}
		hasAnyChildScope = true
		var badPaths []string
		for _, p := range childIn {
			if !parentSet[p] {
				badPaths = append(badPaths, p)
			}
			covered[p] = true
		}
		if len(badPaths) > 0 {
			sort.Strings(badPaths)
			res.Violations = append(res.Violations, ChildScopeViolation{
				ChildWorkItemID: c.ID,
				Type:            ScopeViolationNotSubset,
				OffendingPath:   badPaths[0],
				ParentScopeIn:   append([]string(nil), parentIn...),
				ChildScopeIn:    append([]string(nil), childIn...),
				Message:         fmt.Sprintf("child scope contains paths not in parent scope: %s", strings.Join(badPaths, ", ")),
			})
			res.OK = false
		}
	}
	if hasAnyChildScope && len(covered) < len(parentIn) {
		var uncovered []string
		for _, p := range parentIn {
			if !covered[p] {
				uncovered = append(uncovered, p)
			}
		}
		sort.Strings(uncovered)
		res.UncoveredPaths = uncovered
		res.ChildrenCoverage = float64(len(covered)) / float64(len(parentIn))
	}
	return res
}

func scopeInStrings(item *WorkItem) []string {
	if item == nil || item.ScopeContract == nil {
		return nil
	}
	out := make([]string, 0, len(item.ScopeContract.InScope))
	for _, s := range item.ScopeContract.InScope {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func stringSetFromSlice(ss []string) map[string]bool {
	out := make(map[string]bool, len(ss))
	for _, s := range ss {
		out[s] = true
	}
	return out
}
