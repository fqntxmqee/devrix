// Package contextengine — legacy re-export shim for kernel types.
// New code should import kernel/ directly.
package contextengine

import "github.com/devrix/devrix/internal/layers/contextengine/kernel"

// AutocompactMeta describes autocompact observability metadata.
type AutocompactMeta = kernel.AutocompactMeta

// ICompressionObserver emits compression pipeline events.
type ICompressionObserver = kernel.ICompressionObserver

// NoOpCompressionObserver discards compression observer events.
type NoOpCompressionObserver = kernel.NoOpCompressionObserver

// IObserver emits context engine observability events.
type IObserver = kernel.IObserver

// NoOpObserver discards observer events.
type NoOpObserver = kernel.NoOpObserver

// TokenUsage reports token consumption for observability helpers.
type TokenUsage = kernel.TokenUsage
