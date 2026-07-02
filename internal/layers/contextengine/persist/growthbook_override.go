// Package persist: T05 — growthbook override (DM-20260702-008 / D2-S15-A02-T05).
//
// Per-tool persistence threshold override. Mirrors clawcode
// toolResultStorage.ts:getPersistenceThreshold:51-78 + PERSIST_THRESHOLD_OVERRIDE_FLAG.
//
// Use case: roll out the 100K per-tool thresholds progressively by
// changing the override map for the 5% canary first, then 25%, 100%.
// The hardcoded per-tool values in orthogonal_flags.go stay as the
// "consensus" baseline; GB can shift individual tools up or down.
package persist

// PersistThresholdOverrideFlag is the growthbook flag name. Mirrors
// clawcode PERSIST_THRESHOLD_OVERRIDE_FLAG ("tengu_satin_quoll"). When
// the value is `{}` (the default), the override is a no-op and the
// declared per-tool MaxResultSizeChars wins. Per-tool entries in the
// map BYPASS the default's Math.min clamp and are used verbatim — the
// GB operator is trusted to set a sane value.
const PersistThresholdOverrideFlag = "devrix_persist_threshold_override"

// ThresholdOverride is a thread-safe view over a per-tool threshold map.
// The default zero value has an empty map and returns false from Lookup —
// callers fall through to the declared MaxResultSizeChars.
//
// Production code is expected to wire this to a growthbook client via
// the WithOverrides option. The current devrix tree has no growthbook
// dependency yet, so tests and the compression pipeline use a fresh
// ThresholdOverride{} and the per-tool declared values take effect
// unmodified.
type ThresholdOverride struct {
	// values is the parsed per-tool override map. Tools absent from the
	// map have no override and the caller falls through to the declared
	// per-tool MaxResultSizeChars.
	values map[string]int
}

// NewThresholdOverride returns an override view over the given map. The
// map is copied so subsequent mutations to the caller's map don't leak
// into the override (defense-in-depth against shared-growthbook-client
// reuse).
func NewThresholdOverride(values map[string]int) *ThresholdOverride {
	if len(values) == 0 {
		return &ThresholdOverride{}
	}
	copied := make(map[string]int, len(values))
	for k, v := range values {
		copied[k] = v
	}
	return &ThresholdOverride{values: copied}
}

// OverrideGetter is the minimal interface the compression pipeline
// needs to fetch a per-tool override. Production wires this to
// growthbook's getFeatureValue_CACHED; tests pass a closure backed by
// a map or a stub.
type OverrideGetter func() map[string]int

// GetPersistenceThreshold returns the effective persistence threshold
// for a tool. Resolution order:
//
//  1. declaredMaxResultSizeChars non-finite (Inf / NaN) → return as-is.
//     This is the clawcode "hard opt-out" case for Read: persisting its
//     own output to a file the model reads back is circular.
//  2. override present and finite positive → return override verbatim,
//     bypassing the Math.min clamp. The override operator is trusted.
//  3. default → return declaredMaxResultSizeChars.
//
// Defensive (mirrors clawcode:73-78): the override map may be served as
// nil/null from a misconfigured feature flag cache. The optional
// chaining and Number.isFinite-equivalent guards mean a bad flag value
// falls through to the hardcoded default instead of throwing or
// returning 0. We mirror this with explicit type/finite/positive checks.
func GetPersistenceThreshold(
	toolName string,
	declaredMaxResultSizeChars int,
	override *ThresholdOverride,
) int {
	// Step 1: hard opt-out (declared MaxResultSizeChars is Inf / -1 / 0
	// means "never persist"). Mirrors clawcode:60-62.
	if declaredMaxResultSizeChars <= 0 {
		return declaredMaxResultSizeChars
	}
	// Step 2: GB override present and sane.
	if override != nil {
		if v, ok := override.values[toolName]; ok && v > 0 {
			return v
		}
	}
	// Step 3: declared value wins.
	return declaredMaxResultSizeChars
}

// WithOverrides is a convenience for callers that have a getter but
// want to use the simpler GetPersistenceThreshold API. It calls
// getter() once and passes the result through NewThresholdOverride.
//
// If getter is nil, returns nil — meaning "no override configured,
// use declared values". This is the production path until the GB
// client is wired in.
func WithOverrides(getter OverrideGetter) *ThresholdOverride {
	if getter == nil {
		return nil
	}
	raw := getter()
	return NewThresholdOverride(raw)
}
