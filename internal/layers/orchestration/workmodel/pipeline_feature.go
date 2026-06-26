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
