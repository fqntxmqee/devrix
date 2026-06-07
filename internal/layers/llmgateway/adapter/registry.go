package adapter

import (
	"fmt"
	"sync"

	"github.com/devrix/devrix/internal/layers/llmgateway"
)

// Registry holds provider adapters by name.
type Registry struct {
	mu       sync.RWMutex
	adapters map[string]llmgateway.IAdapter
}

// NewRegistry creates an empty adapter registry.
func NewRegistry() *Registry {
	return &Registry{adapters: make(map[string]llmgateway.IAdapter)}
}

// Register adds an adapter for its Provider() name.
func (r *Registry) Register(a llmgateway.IAdapter) error {
	if a == nil {
		return fmt.Errorf("adapter is nil")
	}
	name := a.Provider()
	if name == "" {
		return fmt.Errorf("adapter provider name is empty")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.adapters[name] = a
	return nil
}

// Get returns the adapter for a provider.
func (r *Registry) Get(provider string) (llmgateway.IAdapter, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.adapters[provider]
	if !ok {
		return nil, fmt.Errorf("adapter not found for provider %q", provider)
	}
	return a, nil
}

// List returns registered provider names.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.adapters))
	for name := range r.adapters {
		names = append(names, name)
	}
	return names
}
