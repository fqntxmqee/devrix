package workmodel

// SiblingRelation classifies same-parent WorkItem pairs (design §5 R1–R6).
type SiblingRelation string

const (
	SiblingNotSibling    SiblingRelation = "not_sibling"
	SiblingIndependent   SiblingRelation = "independent"    // R1
	SiblingDependent     SiblingRelation = "dependent"      // R2
	SiblingParallelBatch SiblingRelation = "parallel_batch" // R3
	SiblingProjection    SiblingRelation = "projection"     // R4/R6 ephemeral or checklist
	SiblingHumanReview   SiblingRelation = "human_review"   // R5
)

// SiblingRelationResult holds classification plus R2 dependency direction.
type SiblingRelationResult struct {
	Relation     SiblingRelation
	UpstreamID   string // blocker / upstream work item (R2)
	DependentID  string // blocked dependent (R2)
}

// AreSiblings reports whether a and b share the same non-empty parent.
func AreSiblings(a, b *WorkItem) bool {
	if a == nil || b == nil {
		return false
	}
	if a.ID == b.ID {
		return false
	}
	if a.ParentID == "" || b.ParentID == "" {
		return false
	}
	return a.ParentID == b.ParentID
}

func containsID(ids []string, id string) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

// ClassifySiblingRelation classifies the relationship between siblings a and b.
// Order follows design §5.1 with R5/R6 guards before generic independent.
func ClassifySiblingRelation(a, b *WorkItem) SiblingRelationResult {
	if !AreSiblings(a, b) {
		return SiblingRelationResult{Relation: SiblingNotSibling}
	}
	if containsID(a.BlockedBy, b.ID) {
		return SiblingRelationResult{
			Relation:    SiblingDependent,
			UpstreamID:  b.ID,
			DependentID: a.ID,
		}
	}
	if containsID(b.BlockedBy, a.ID) {
		return SiblingRelationResult{
			Relation:    SiblingDependent,
			UpstreamID:  a.ID,
			DependentID: b.ID,
		}
	}
	if IsHumanReviewItem(a) || IsHumanReviewItem(b) {
		return SiblingRelationResult{Relation: SiblingHumanReview}
	}
	if isContextProjection(a) || isContextProjection(b) {
		return SiblingRelationResult{Relation: SiblingProjection}
	}
	if a.Policy == ExecPolicyParallelOK && b.Policy == ExecPolicyParallelOK {
		return SiblingRelationResult{Relation: SiblingParallelBatch}
	}
	return SiblingRelationResult{Relation: SiblingIndependent}
}

func isContextProjection(item *WorkItem) bool {
	if item == nil {
		return true
	}
	if item.Ephemeral {
		return true
	}
	if item.Kind == WorkKindChecklist {
		return true
	}
	return false
}

// DefaultLinkKindForSibling maps taxonomy to default ContextLinkKind (no LLM proposal).
func DefaultLinkKindForSibling(rel SiblingRelationResult) ContextLinkKind {
	if rel.Relation == SiblingDependent {
		return LinkUpstream
	}
	return LinkFresh
}

// AllowsLLMShareSummary reports whether share_summary proposals may be evaluated (R1 only).
func AllowsLLMShareSummary(rel SiblingRelationResult) bool {
	return rel.Relation == SiblingIndependent
}
