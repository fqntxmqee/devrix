package llmgateway

import (
	"context"

	"github.com/devrix/devrix/internal/shared/types"
)

// Request is the L3 internal chat completion input.
type Request struct {
	Provider     string
	Model        string
	SystemPrompt string
	Messages     []types.Message
	Tools        []ToolSchema
	MaxTokens    int
	Temperature  float64
	Stream       bool
}

// Chunk is a streaming LLM response fragment.
type Chunk struct {
	Content   string
	Thinking  string
	ToolCalls []ToolCall
	Done      bool
	Usage     TokenUsage
}

// TokenUsage reports token consumption from the provider.
type TokenUsage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	CacheReadTokens  int // prompt_tokens_details.cached_tokens
	ReasoningTokens  int // completion_tokens_details.reasoning_tokens
}

// ToolSchema describes a tool for the LLM.
type ToolSchema struct {
	Name        string
	Description string
	Parameters  string
}

// ToolCall is an LLM-requested tool invocation (no RiskLevel; L2 fills it).
type ToolCall struct {
	ID    string
	Name  string
	Input string
}

// CircuitState is the circuit breaker state for a provider.
type CircuitState string

const (
	CircuitClosed   CircuitState = "closed"
	CircuitOpen     CircuitState = "open"
	CircuitHalfOpen CircuitState = "half-open"
)

// CircuitBreakerConfig holds circuit breaker thresholds.
type CircuitBreakerConfig struct {
	FailureThreshold int
	SuccessThreshold int
	OpenDuration     int // seconds; resolved at config load
	Scope            string
}

// RetryConfig holds retry/backoff settings.
type RetryConfig struct {
	MaxAttempts  int
	InitialDelay int // milliseconds
	MaxDelay     int
	Backoff      float64
}

// AdapterChunk is a raw or parsed adapter stream item.
type AdapterChunk struct {
	Raw    []byte
	Parsed *Chunk
	Error  error
}

// IGateway streams chat completions (L3 internal API).
type IGateway interface {
	Stream(ctx context.Context, req *Request) (<-chan Chunk, error)
	// ResolveTier resolves a tier alias to a concrete model name.
	// Returns the input unchanged if not a known tier.
	ResolveTier(tier string) string
	Close() error
}

// ILLMGateway is the D2 Context Engine consumer contract for streaming chat.
//
// DSAFT: D3-S2-A01-F01 (AdaptToContextEngine)
type ILLMGateway interface {
	ChatStream(ctx context.Context, req *Request) (<-chan Chunk, error)
}

// ITierResolver resolves tier aliases to concrete model names.
//
// DSAFT: D3-S2-A01-F02 (ResolveTier)
type ITierResolver interface {
	ResolveTier(tier string) (string, error)
}

// IAdapter streams provider-specific responses.
//
// DSAFT: D3-S2-A01-F01 (StreamChatCompletion) + D3-S2-A01-F04 (AdapterProtocolMethod, v1.1)
type IAdapter interface {
	Stream(ctx context.Context, req *Request) (<-chan *AdapterChunk, error)
	Provider() string
	// Protocol returns the adapter's wire protocol identifier (v1.1 BREAKING).
	// Current implementations return "openai-compatible".
	// Reserved for V3: "anthropic-native" for Anthropic adapter.
	Protocol() string
}

// ICircuitBreaker protects providers from cascading failures.
type ICircuitBreaker interface {
	Allow(circuitKey string) (bool, error)
	RecordSuccess(circuitKey string)
	RecordFailure(circuitKey string)
	State(circuitKey string) CircuitState
}

// ICircuitBreakerWithObserver is the optional observer-attachment interface.
//
// DSAFT: D3-S3-A01-F02 (OnStateTransitionEmit, v1.1).
// Implementations may satisfy this to receive state-transition callbacks.
// The gateway uses a runtime type-assertion; non-observing breakers simply
// skip the callback path.
type ICircuitBreakerWithObserver interface {
	ICircuitBreaker
	WithObserver(observer BreakerStateObserver) ICircuitBreaker
}

// BreakerStateObserver receives notifications when a circuit transitions state.
//
// DSAFT: D3-S3-A01-F02.
// Implementations may emit metrics, counters, span events, or engine events.
type BreakerStateObserver interface {
	OnBreakerStateChange(provider string, from, to CircuitState)
}
