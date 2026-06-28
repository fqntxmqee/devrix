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
	if parent != nil && parent.ScopeContract != nil {
		if len(dl.ScopeIn) == 0 {
			dl.ScopeIn = append([]string(nil), parent.ScopeContract.InScope...)
		}
		if len(dl.ScopeOut) == 0 {
			dl.ScopeOut = append([]string(nil), parent.ScopeContract.OutOfScope...)
		}
	}
	if dl.ExpectedReturn == "" && spec.Directive != "" {
		dl.ExpectedReturn = "Deliverable aligned with directive: " + spec.Directive
	}
	return dl
}
