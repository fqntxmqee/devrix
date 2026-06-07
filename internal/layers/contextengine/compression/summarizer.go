package compression

import "context"

// Summarizer generates a text summary for autocompact.
type Summarizer interface {
	Summarize(ctx context.Context, model, prompt string, maxTokens int) (string, error)
}

// StepObserver receives compression step notifications.
type StepObserver interface {
	OnStep(step string, before, after int)
	OnAutocompact(meta AutocompactMeta)
}

// AutocompactMeta describes an autocompact event.
type AutocompactMeta struct {
	Degraded      bool
	SummaryTokens int
	Model         string
}
