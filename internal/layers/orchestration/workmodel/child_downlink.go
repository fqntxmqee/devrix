package workmodel

// ChildDownlink is the parent→child downlink contract persisted at decompose (DM-20260627-003).
type ChildDownlink struct {
	ParentWorkItemID string            `json:"parent_work_item_id"`
	ChildWorkItemID  string            `json:"child_work_item_id"`
	Directive        string            `json:"directive,omitempty"`
	ScopeIn          []string          `json:"scope_in,omitempty"`
	ScopeOut         []string          `json:"scope_out,omitempty"`
	ExpectedReturn   string            `json:"expected_return,omitempty"`
	FailureCriteria  []string          `json:"failure_criteria,omitempty"`
	ContextPolicy    ContextLinkKind   `json:"context_policy,omitempty"`
}

// DefaultChildDownlink builds a downlink from parent scope and child spec.
//
// RH-MUPS-05/09 (DM-20260701-001 T-P2-1): the prior version silently
// inherited the parent's full ScopeContract whenever spec.ScopeIn was
// empty. That made a focused child review (e.g. "review only the
// materialize/ subdir") accidentally end up doing the entire parent
// review when the LLM forgot to set the spec scope. The post-validate
// step (ValidateChildScopes, T-P2-2) now enforces proper-subset; this
// function trusts the spec as the authoritative source.
//
// New contract:
//   - If spec.ScopeIn is set → use it as-is.
//   - If spec.ScopeIn is empty AND parent has NO ScopeContract →
//     leave ScopeIn empty (parent is unbounded, child inherits via
//     Materialize ModeUpstream / ModeInheritCohort).
//   - If spec.ScopeIn is empty AND parent has a ScopeContract →
//     leave ScopeIn empty (caller / ValidateChildScopes surfaces this).
//     The previous silent inheritance was the bug.
func DefaultChildDownlink(parent *WorkItem, child *WorkItem, spec ChildSpec) ChildDownlink {
	dl := ChildDownlink{
		ParentWorkItemID: parent.ID,
		ChildWorkItemID:  child.ID,
		Directive:        spec.Directive,
		ScopeIn:          append([]string(nil), spec.ScopeIn...),
		ScopeOut:         append([]string(nil), spec.ScopeOut...),
		ExpectedReturn:   spec.ExpectedReturn,
		ContextPolicy:    LinkFresh,
	}
	if dl.ExpectedReturn == "" && spec.Directive != "" {
		// RH-D7-08 (DM-20260630-013 T-P1-4): the prior plain-string fallback
		// ("Deliverable aligned with directive: ...") carried no schema tag,
		// so the child executor / verifier saw a prose instruction and the
		// deterministic verify step would fall back to the structural default.
		// Now we route through DefaultChildExpectedReturn so the fallback
		// emits a <deliverable_schema> tag (or same_as_parent_directive) that
		// ParseDeliverableSchemaTag can recognize, giving the child a
		// verifiable acceptance bar instead of a wish.
		dl.ExpectedReturn = DefaultChildExpectedReturn(parent, spec.Directive)
	}
	return dl
}
