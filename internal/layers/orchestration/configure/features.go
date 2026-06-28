package configure

import "os"

// FeatureLayerSubContextEnabled gates the D2 Materialize path for depth≥1 WorkItems (DM-20260627-003).
// Default false; enable via DEVRIX_LAYER_SUBCONTEXT=1. Migration deadline: 30 days (OQ-LC-10).
func FeatureLayerSubContextEnabled() bool {
	v := os.Getenv("DEVRIX_LAYER_SUBCONTEXT")
	return v == "1" || v == "true" || v == "TRUE"
}
