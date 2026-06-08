package compression

import (
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/layers/contextengine/token"
)

// Option configures a compression Pipeline.
type Option func(*Pipeline)

// WithEnabled sets whether compression is enabled.
func WithEnabled(enabled bool) Option {
	return func(p *Pipeline) {
		p.enabled = enabled
	}
}

// WithCounter sets the token counter implementation.
func WithCounter(counter contracts.ITokenCounter) Option {
	return func(p *Pipeline) {
		if counter != nil {
			p.counter = counter
		}
	}
}

// WithAutocompactConfig sets autocompact configuration.
func WithAutocompactConfig(cfg config.AutocompactConfig) Option {
	return func(p *Pipeline) {
		p.autocompactCfg = cfg
	}
}

// WithSummarizer sets the LLM summarizer for autocompact.
func WithSummarizer(s Summarizer) Option {
	return func(p *Pipeline) {
		p.summarizer = s
	}
}

// WithStepObserver sets the compression step observer.
func WithStepObserver(obs StepObserver) Option {
	return func(p *Pipeline) {
		p.stepObserver = obs
	}
}

// WithAsyncAutocompacter enables background autocompact summarization.
func WithAsyncAutocompacter(a *AsyncAutocompacter) Option {
	return func(p *Pipeline) {
		p.asyncCompact = a
	}
}

// WithSessionID sets the session ID for async autocompact deduplication.
func WithSessionID(sessionID string) Option {
	return func(p *Pipeline) {
		p.sessionID = sessionID
	}
}

func defaultPipeline() *Pipeline {
	return &Pipeline{
		counter:        token.NewCounter(),
		enabled:        true,
		autocompactCfg: config.DefaultAutocompactConfig(),
	}
}
