package compression

import (
	"context"

	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// Summarizer generates a text summary for autocompact.
//
// Re-exported from shared/contracts (DM-020拆面契约). Implemented by
// D7 turn.CompressionSummarizer; injected via EngineDeps.Summarizer.
type Summarizer = contracts.Summarizer

// StepObserver receives compression step notifications.
type StepObserver interface {
	OnStep(ctx context.Context, step string, before, after int)
	OnAutocompact(meta AutocompactMeta)
	OnAutocompactComplete(summaryMsg types.Message, sessionID, asyncToken string)
}

// AutocompactMeta describes an autocompact event.
type AutocompactMeta struct {
	Degraded      bool
	SummaryTokens int
	Model         string
}
