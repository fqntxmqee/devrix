package prompttags

import (
	"testing"
)

// benchFrame mirrors a typical 9-field ObserveSignalInput.
type benchFrame struct {
	WorkItemID          string
	Directive           string
	PriorParseReject    string
	PriorMean           float64
	ScopeGoal           string
	ScopeOpenQuestions  []string
	Signal              []string
	PriorObservationIDs []string
	IncrementalOnly     bool
}

var benchIn = benchFrame{
	WorkItemID:          "wi_bench",
	Directive:           "ship login v2",
	PriorParseReject:    `{"code":"parse_fail"}`,
	PriorMean:           0.5,
	ScopeGoal:           "ship login v2",
	ScopeOpenQuestions:  []string{"q1", "q2"},
	Signal:              []string{"s1", "s2"},
	PriorObservationIDs: []string{"obs_1", "obs_2"},
	IncrementalOnly:     true,
}

// BenchmarkBuildLineFrameFromStruct measures the per-round reflection cost.
func BenchmarkBuildLineFrameFromStruct(b *testing.B) {
	LineFrameRegistry["bench_frame"] = FrameSpec{
		Fields: []TagName{
			TagWorkItemID, TagDirective, TagPriorParseReject, TagPriorMean,
			TagScopeGoal, TagScopeOpenQuestion, TagSignal,
			TagPriorObservationIDs, TagIncrementalOnly,
		},
	}
	defer delete(LineFrameRegistry, "bench_frame")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = BuildLineFrameFromStruct("bench_frame", &benchIn)
	}
}
