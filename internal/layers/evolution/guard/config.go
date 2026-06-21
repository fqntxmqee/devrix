package guard

import "github.com/devrix/devrix/internal/shared/config"

// GuardConfig is the runtime configuration for the guard validator.
//
// PR-B (DM-20260621-011): renamed from OrchestrationConfig to align with
// guard/ domain naming. The underlying shared/config.OrchestrationConfig
// keeps its name to avoid cross-cutting breakage of all consumers; this
// alias will be re-pointed in a dedicated follow-up Change.
type GuardConfig = config.OrchestrationConfig

// OrchestrationConfig is an alias for GuardConfig kept for backward compatibility.
//
// Deprecated: use GuardConfig. This alias will be removed in v2.5.0 (DM-20260621-011).
//go:deprecated
type OrchestrationConfig = config.OrchestrationConfig