package orchtypes

import "github.com/devrix/devrix/internal/shared/types"

// ArtifactKind is a re-export of shared/types.ArtifactKind. The concrete
// enum (4 values) + String()/Parse/Marshal/Unmarshal live in
// shared/types/execute.go and are promoted there to break a cyclic import
// between orchtypes, workmodel, and wavescheduler (DM-20260625-001 PR-C1).
type ArtifactKind = types.ArtifactKind

const (
	ArtifactStateChangeCert = types.ArtifactStateChangeCert
	ArtifactResponseRecord  = types.ArtifactResponseRecord
	ArtifactProbeReport     = types.ArtifactProbeReport
	ArtifactExperimentData  = types.ArtifactExperimentData
)

// SideEffectDetail is a re-export of shared/types.SideEffectDetail.
type SideEffectDetail = types.SideEffectDetail
