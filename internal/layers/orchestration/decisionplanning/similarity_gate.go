// Similarity Check Gate (PR-C, DM-20260629-009).
//
// Provides decomposer-time similarity intercept — when a new taskDirective
// is too similar (Jaccard > 0.85) to a recent entry in the same session's
// VersionChain, the gate can return an Intercept result and short-circuit
// the decompose step. Closes AC14 (相似子任务无防御，烧 token).
//
// Gated behind the D7_SIMILARITY_CHECK_ENABLED feature flag. Default off →
// 0 behavior change. When on, callers can optionally override via
// SetSimilarityCheckEnabledForTest.
package decisionplanning

import (
	"os"
	"strings"

	"github.com/devrix/devrix/internal/layers/orchestration/interfaces"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
)

// similarityCheckEnabled reads D7_SIMILARITY_CHECK_ENABLED directly.
// Inlined here to avoid an import cycle (bootstrap → sessionorchestrator →
// decisionplanning).
func similarityCheckEnabled() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("D7_SIMILARITY_CHECK_ENABLED")))
	switch v {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// similarityCheckEnabledFn is the function-var indirection used by the gate.
// Tests can swap it via SetSimilarityCheckEnabledForTest.
var similarityCheckEnabledFn = similarityCheckEnabled

// SetSimilarityCheckEnabledForTest overrides the gate's enabled flag. Returns
// the restore function for use with defer.
func SetSimilarityCheckEnabledForTest(enabled bool) func() {
	old := similarityCheckEnabledFn
	similarityCheckEnabledFn = func() bool { return enabled }
	return func() { similarityCheckEnabledFn = old }
}

// InterceptResult conveys the outcome of a similarity check back to the
// decomposer. Zero value means "no intercept".
type InterceptResult struct {
	Intercepted  bool
	Similar      bool
	Warn         bool
	Score        float64
	MatchedHash  interfaces.Hash
	MatchedChain string // session ID whose chain held the match (empty if n/a)
}

// CheckDecomposeSimilarity runs the similarity check against the registry.
// When the gate is disabled (default), returns zero-value (no intercept).
func CheckDecomposeSimilarity(reg *workmodel.VersionChainRegistry, sessionID, directive string) InterceptResult {
	if !similarityCheckEnabledFn() {
		return InterceptResult{}
	}
	if reg == nil {
		return InterceptResult{}
	}
	cfg := interfaces.NewDefaultSimilarityConfig()
	res, err := workmodel.CheckSimilarityForSession(reg, sessionID, directive, cfg)
	if err != nil {
		// Invalid config or other error — treat as no-match.
		return InterceptResult{}
	}
	return InterceptResult{
		Intercepted: res.Similar,
		Similar:     res.Similar,
		Warn:        res.Warn,
		Score:       res.Score,
		MatchedHash: res.MatchedHash,
		MatchedChain: sessionID,
	}
}
