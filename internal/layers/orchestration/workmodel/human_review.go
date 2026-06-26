package workmodel

// HumanReviewItemTitle marks WorkItems created by SpawnEscalateHuman (TD-WT-05).
const HumanReviewItemTitle = "Human review required"

// IsHumanReviewItem reports whether item is a human gate WorkItem.
func IsHumanReviewItem(item *WorkItem) bool {
	if item == nil {
		return false
	}
	return item.Kind == WorkKindVerify && item.Title == HumanReviewItemTitle
}

// EffectiveDecomposeThreshold returns the adaptive or default decompose threshold.
func EffectiveDecomposeThreshold(tm *TaskManager, userID string) float64 {
	if tm != nil && tm.adaptiveThreshold != nil {
		return tm.adaptiveThreshold.ThresholdFor(userID)
	}
	return DefaultUncertaintyDecomposeThreshold
}
