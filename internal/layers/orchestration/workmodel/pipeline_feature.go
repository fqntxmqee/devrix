package workmodel

// FeatureWorkItemPipelineEnabled reports whether WorkItem-per-pipeline ingress
// is active (default on since PR #244).
func FeatureWorkItemPipelineEnabled() bool {
	return true
}

// FeatureWorkItemContextGraphEnabled reports whether ContextGraph runtime is active
// (default on since PR #244).
func FeatureWorkItemContextGraphEnabled() bool {
	return true
}
