// Package retry is a backward-compatibility bridge.
// Deprecated: use github.com/devrix/devrix/internal/layers/llmgateway/protect instead.
package retry

import "github.com/devrix/devrix/internal/layers/llmgateway/protect"

// StreamFunc starts a streaming call for a model.
type StreamFunc = protect.StreamFunc

// Executor runs retry with exponential backoff.
type Executor = protect.Executor

// NewExecutor creates a retry executor.
//
// Deprecated: use protect.NewExecutor instead.
var NewExecutor = protect.NewExecutor
