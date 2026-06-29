package orchtypes

import "github.com/devrix/devrix/internal/shared/types"

// ArtifactKind is a re-export of shared/types.ArtifactKind. The concrete
// enum (4 values) + String()/Parse/Marshal/Unmarshal live in
// shared/types/execute.go and are promoted there to break a cyclic import
// between orchtypes, workmodel, and wavescheduler (DM-20260625-001 PR-C1).
//
// v2.6.0 (DM-20260629-001): 4 alias constants (ArtifactStateChangeCert /
// ArtifactResponseRecord / ArtifactProbeReport / ArtifactExperimentData) removed.
// Callers should use `types.ArtifactStateChangeCert` etc. directly from shared/types.
type ArtifactKind = types.ArtifactKind

// SideEffectDetail is a re-export of shared/types.SideEffectDetail.
type SideEffectDetail = types.SideEffectDetail
