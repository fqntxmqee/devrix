package workmodel

import "os"

// FeatureWorkItemPipelineEnv is the env var that enables RunSessionTurnLoop
// ingress (Phase C). Default off for safe rollout (OQ-1).
const FeatureWorkItemPipelineEnv = "D7_WORKITEM_PIPELINE"

// FeatureWorkItemPipelineEnabled reports whether WorkItem-per-pipeline ingress
// is active.
func FeatureWorkItemPipelineEnabled() bool {
	return os.Getenv(FeatureWorkItemPipelineEnv) == "1"
}

// FeatureWorkItemContextGraphEnv enables ContextGraph materialization (Phase F3+).
const FeatureWorkItemContextGraphEnv = "D7_WORKITEM_CONTEXT_GRAPH"

// FeatureWorkItemContextGraphEnabled reports whether ContextGraph runtime is active.
func FeatureWorkItemContextGraphEnabled() bool {
	return os.Getenv(FeatureWorkItemContextGraphEnv) == "1"
}
