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
	Close() error
}

// IAdapter streams provider-specific responses.
type IAdapter interface {
	Stream(ctx context.Context, req *Request) (<-chan *AdapterChunk, error)
	Provider() string
}

// ICircuitBreaker protects providers from cascading failures.
type ICircuitBreaker interface {
	Allow(circuitKey string) (bool, error)
	RecordSuccess(circuitKey string)
	RecordFailure(circuitKey string)
	State(circuitKey string) CircuitState
}
