package decisionplanning

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
)

func TestCheckDecomposeSimilarity_FlagOffNoIntercept(t *testing.T) {
	defer SetSimilarityCheckEnabledForTest(false)()
	r := workmodel.NewVersionChainRegistry()
	_, _, _ = r.Append("sess_1", []byte("the quick brown fox jumps over the lazy dog now and forever"), "commit")
	res := CheckDecomposeSimilarity(r, "sess_1", "the quick brown fox jumps over the lazy dog now and forever new")
	if res.Intercepted || res.Similar || res.Warn {
		t.Fatalf("flag-off should never intercept; got %+v", res)
	}
}

func TestCheckDecomposeSimilarity_FlagOnInterceptOnHighOverlap(t *testing.T) {
	defer SetSimilarityCheckEnabledForTest(true)()
	r := workmodel.NewVersionChainRegistry()
	_, _, _ = r.Append("sess_1", []byte("the quick brown fox jumps over the lazy dog now and forever and ever"), "commit")
	// Build a near-identical directive.
	dup := "the quick brown fox jumps over the lazy dog now and forever and ever again"
	res := CheckDecomposeSimilarity(r, "sess_1", dup)
	// Jaccard > 0.85 → Intercepted=true.
	if !res.Intercepted {
		t.Fatalf("near-identical directive should be intercepted; score=%v", res.Score)
	}
}

func TestCheckDecomposeSimilarity_FlagOnNoInterceptWhenDistinct(t *testing.T) {
	defer SetSimilarityCheckEnabledForTest(true)()
	r := workmodel.NewVersionChainRegistry()
	_, _, _ = r.Append("sess_1", []byte("totally distinct original work item text"), "commit")
	res := CheckDecomposeSimilarity(r, "sess_1", "completely fresh short directive")
	if res.Intercepted {
		t.Fatalf("distinct directive should not be intercepted; score=%v", res.Score)
	}
}

func TestCheckDecomposeSimilarity_NilRegistryIsSafe(t *testing.T) {
	defer SetSimilarityCheckEnabledForTest(true)()
	res := CheckDecomposeSimilarity(nil, "sess_1", "anything")
	if res.Intercepted || res.Similar || res.Warn {
		t.Fatalf("nil registry should yield zero result; got %+v", res)
	}
}
