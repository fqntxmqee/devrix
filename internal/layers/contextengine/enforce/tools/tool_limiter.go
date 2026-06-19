package tools

import "context"

// ToolLimiter bounds concurrent tool executions with a semaphore.
type ToolLimiter struct {
	semaphore chan struct{}
}

// NewToolLimiter creates a limiter. Non-positive max defaults to 10.
func NewToolLimiter(maxConcurrent int) *ToolLimiter {
	if maxConcurrent <= 0 {
		maxConcurrent = 10
	}
	return &ToolLimiter{
		semaphore: make(chan struct{}, maxConcurrent),
	}
}

// Acquire blocks until a slot is available or ctx is cancelled.
func (l *ToolLimiter) Acquire(ctx context.Context) error {
	if l == nil {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case l.semaphore <- struct{}{}:
		return nil
	}
}

// Release frees a concurrency slot.
func (l *ToolLimiter) Release() {
	if l == nil {
		return
	}
	<-l.semaphore
}

// LimitedToolRunner wraps an IToolRunner with concurrency control.
type LimitedToolRunner struct {
	inner   IToolRunner
	limiter *ToolLimiter
}

// NewLimitedToolRunner creates a concurrency-limited tool runner.
func NewLimitedToolRunner(inner IToolRunner, limiter *ToolLimiter) *LimitedToolRunner {
	return &LimitedToolRunner{inner: inner, limiter: limiter}
}

// Execute acquires a slot, delegates to inner, then releases.
func (r *LimitedToolRunner) Execute(ctx context.Context, call ToolCall) (*ToolResult, error) {
	if err := r.limiter.Acquire(ctx); err != nil {
		return &ToolResult{Error: err.Error()}, nil
	}
	defer r.limiter.Release()
	return r.inner.Execute(ctx, call)
}
