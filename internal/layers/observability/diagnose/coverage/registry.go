package coverage

import (
	"sort"
	"sync"
)

// SpanProvider is implemented by each domain to provide its spans metadata.
// Each domain's spans.go should call RegisterProvider in its init() function.
type SpanProvider interface {
	Spans() []OperationMeta
}

var (
	providersMu   sync.RWMutex
	providers     []SpanProvider
	providersOnce sync.Once
)

// RegisterProvider registers a SpanProvider for inclusion in AllOperations.
// Called by each domain's init() function via spans.go.
func RegisterProvider(p SpanProvider) {
	providersMu.Lock()
	defer providersMu.Unlock()
	providers = append(providers, p)
}

// RegisteredSpans returns all spans from registered providers.
func RegisteredSpans() []OperationMeta {
	providersMu.RLock()
	defer providersMu.RUnlock()
	var all []OperationMeta
	for _, p := range providers {
		all = append(all, p.Spans()...)
	}
	return all
}

// OperationMeta describes a canonical operation for registry and reconciliation.
type OperationMeta struct {
	Name          string
	Layer         string
	Component     string
	SinceVersion  string
	Instrumented  bool
}

// AllOperations returns the canonical operation registry sorted by name.
// It merges static definitions with dynamically registered spans from providers.
func AllOperations() []OperationMeta {
	// Static operations defined in this file (legacy, migrated domains fill their own)
	ops := []OperationMeta{
		// Empty: all spans are now defined in domain spans.go files
		// Kept for documentation and potential future shared spans
	}
	// Append spans registered by domain providers
	ops = append(ops, RegisteredSpans()...)
	sort.Slice(ops, func(i, j int) bool { return ops[i].Name < ops[j].Name })
	return ops
}

// KnownOperations returns a set of registered operation names.
func KnownOperations() map[string]struct{} {
	ops := AllOperations()
	set := make(map[string]struct{}, len(ops))
	for _, op := range ops {
		set[op.Name] = struct{}{}
	}
	return set
}

// IsKnown reports whether operation is in the canonical registry.
func IsKnown(operation string) bool {
	_, ok := KnownOperations()[operation]
	return ok
}
