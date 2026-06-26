package compression

import (
	"github.com/devrix/devrix/internal/layers/contextengine/i18n"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/token"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
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

// WithMicrocompactConfig sets microcompact (stale tool result clearing) settings.
func WithMicrocompactConfig(cfg config.MicrocompactConfig) Option {
	return func(p *Pipeline) {
		p.microcompactCfg = cfg
	}
}

// WithMessageBudget sets head+tail message count limits for compression.
func WithMessageBudget(maxMessages, keepTailMessages, preserveHeadTurns int) Option {
	return func(p *Pipeline) {
		if maxMessages > 0 {
			p.maxMessages = maxMessages
		}
		if keepTailMessages > 0 {
			p.keepTailMessages = keepTailMessages
		}
		if preserveHeadTurns > 0 {
			p.preserveHeadTurns = preserveHeadTurns
		}
	}
}

// WithCompressionConfig applies message budget and microcompact from compression config.
func WithCompressionConfig(cfg config.CompressionConfig) Option {
	return func(p *Pipeline) {
		if cfg.MaxMessages > 0 {
			p.maxMessages = cfg.MaxMessages
		}
		if cfg.KeepTailMessages > 0 {
			p.keepTailMessages = cfg.KeepTailMessages
		}
		if cfg.Autocompact.PreserveHeadTurns > 0 {
			p.preserveHeadTurns = cfg.Autocompact.PreserveHeadTurns
		}
		if cfg.Microcompact.KeepRecentToolResults > 0 {
			p.microcompactCfg = cfg.Microcompact
		}
	}
}

// WithSkipAssembly skips step 5 system prompt assembly (D7 passes system separately).
func WithSkipAssembly(skip bool) Option {
	return func(p *Pipeline) {
		p.skipAssembly = skip
	}
}

// WithLocale sets the language for LLM-facing compression prompts.
func WithLocale(loc i18n.Locale) Option {
	return func(p *Pipeline) {
		if loc != "" {
			p.locale = loc
		}
	}
}

func defaultPipeline() *Pipeline {
	return &Pipeline{
		counter:           token.NewCounter(),
		enabled:           true,
		autocompactCfg:    config.DefaultAutocompactConfig(),
		microcompactCfg:   config.DefaultMicrocompactConfig(),
		maxMessages:       config.DefaultCompressionConfig().MaxMessages,
		keepTailMessages:  config.DefaultCompressionConfig().KeepTailMessages,
		preserveHeadTurns: config.DefaultAutocompactConfig().PreserveHeadTurns,
		locale:            i18n.DefaultLocale,
	}
}
