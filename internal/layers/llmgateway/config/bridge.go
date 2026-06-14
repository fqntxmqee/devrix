// Package config is a backward-compatibility bridge.
// Deprecated: use github.com/devrix/devrix/internal/layers/llmgateway/configure instead.
package config

import "github.com/devrix/devrix/internal/layers/llmgateway/configure"

// Loader validates and normalizes LLM gateway configuration.
type Loader = configure.Loader

// NewLoader creates a config loader.
var NewLoader = configure.NewLoader

// APIKey returns the API key for a provider.
var APIKey = configure.APIKey
