// Package token is a backward-compatibility bridge.
// Deprecated: use github.com/devrix/devrix/internal/layers/llmgateway/budget instead.
package token

import "github.com/devrix/devrix/internal/layers/llmgateway/budget"

// Counter counts tokens in text.
//
// Deprecated: use budget.Counter instead.
type Counter = budget.Counter

// NewCounter creates a cl100k_base token counter.
//
// Deprecated: use budget.NewCounter instead.
var NewCounter = budget.NewCounter
