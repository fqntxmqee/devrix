// Package adapter is a backward-compatibility bridge.
// Deprecated: use github.com/devrix/devrix/internal/layers/llmgateway/stream/adapter instead.
package adapter

import streamadapter "github.com/devrix/devrix/internal/layers/llmgateway/stream/adapter"

// Registry manages adapter registrations.
type Registry = streamadapter.Registry

// NewRegistry creates an adapter registry.
//
// Deprecated: use stream/adapter.NewRegistry instead.
var NewRegistry = streamadapter.NewRegistry

// OpenAIStreamClient performs OpenAI-compatible streaming calls.
type OpenAIStreamClient = streamadapter.OpenAIStreamClient

// NewOpenAIStreamClient creates a streaming client.
var NewOpenAIStreamClient = streamadapter.NewOpenAIStreamClient

// DeepSeekAdapter adapts DeepSeek's API.
type DeepSeekAdapter = streamadapter.DeepSeekAdapter

// NewDeepSeekAdapter creates a DeepSeek adapter.
var NewDeepSeekAdapter = streamadapter.NewDeepSeekAdapter

// MiniMaxAdapter adapts MiniMax's API.
type MiniMaxAdapter = streamadapter.MiniMaxAdapter

// NewMiniMaxAdapter creates a MiniMax adapter.
var NewMiniMaxAdapter = streamadapter.NewMiniMaxAdapter
