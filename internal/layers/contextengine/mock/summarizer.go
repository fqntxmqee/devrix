package mockctx

import "context"

// StaticSummarizer is a test double for contracts.Summarizer.
//
// Cross-domain fixture: D7-S2-A07 CompressionSummarizer is the production
// implementation; D2 compression.Pipeline consumes via EngineDeps.Summarizer.
// Lives in D2/mock/ because D2 tests need a fakes and D2 cannot import D7.
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
