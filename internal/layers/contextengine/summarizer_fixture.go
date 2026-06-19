// Cross-domain D7 contract fixtures for D2 tests and the cmd/obs-verify smoke binary.
//
// D2 cannot import D7 (D2 is Follower; D7 is Leader), so static fakes for
// D7-owned contracts (Summarizer, PreparedTurnRunner) must live somewhere
// D2 can reach. Per the user's "mock 语义和内容不符" cleanup (2026-06-19):
//
//   - D2-owned test doubles (ToolRunner, AllowAllPermission, DenyAllPermission)
//     live in enforce/doubles.go next to the ports they implement.
//
//   - Cross-domain D7 contract fixtures (StaticSummarizer, StaticPreparedTurnRunner)
//     live at D2 root alongside tool_context.go — these are D2-level glue,
//     not D2-owned mocks.
//
// Migrated from internal/layers/contextengine/mock/ during the 2026-06-19
// mock/ semantic-cleanup. See devrix-d2-mock-semantic-split change.
package contextengine

import "context"

// StaticSummarizer is a test double for contracts.Summarizer.
//
// Cross-domain fixture: D7-S2-A07 (turn.CompressionSummarizer) is the production
// implementation; D2 (compression.Pipeline) consumes via EngineDeps.Summarizer.
// Lives at D2 root because D2 cannot import D7.
type StaticSummarizer struct {
	Summary string
	Err     error
}

// Summarize implements contracts.Summarizer.
func (s *StaticSummarizer) Summarize(_ context.Context, _, _ string, _ int) (string, error) {
	if s.Err != nil {
		return "", s.Err
	}
	if s.Summary != "" {
		return s.Summary, nil
	}
	return "summary", nil
}
