package observability

import (
	"context"
	"sort"
	"strings"
)

// BaggageContextKey is the key for baggage in context
type BaggageContextKey struct{}

// BaggageItem represents a single baggage item
type BaggageItem struct {
	Key   string
	Value string
}

// BaggageManager manages trace baggage
type BaggageManager struct {
	maxItems int
}

// NewBaggageManager creates a new baggage manager
func NewBaggageManager(maxItems int) *BaggageManager {
	if maxItems <= 0 {
		maxItems = 32
	}
	return &BaggageManager{maxItems: maxItems}
}

// Set sets a baggage item
func (m *BaggageManager) Set(ctx context.Context, key, value string) context.Context {
	if key == "" || value == "" {
		return ctx
	}

	baggage := m.getAll(ctx)
	baggage[key] = value

	// Limit baggage size
	if len(baggage) > m.maxItems {
		// Remove oldest (first) item by sorting keys
		keys := make([]string, 0, len(baggage))
		for k := range baggage {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		delete(baggage, keys[0])
	}

	return context.WithValue(ctx, BaggageContextKey{}, baggage)
}

// Get retrieves a baggage item
func (m *BaggageManager) Get(ctx context.Context, key string) (string, bool) {
	baggage := m.getAll(ctx)
	val, ok := baggage[key]
	return val, ok
}

// List returns all baggage items in sorted order
func (m *BaggageManager) List(ctx context.Context) []BaggageItem {
	baggage := m.getAll(ctx)
	items := make([]BaggageItem, 0, len(baggage))
	for k, v := range baggage {
		items = append(items, BaggageItem{Key: k, Value: v})
	}
	// Sort by key for deterministic order
	sort.Slice(items, func(i, j int) bool {
		return items[i].Key < items[j].Key
	})
	return items
}

// Clear removes all baggage
func (m *BaggageManager) Clear(ctx context.Context) context.Context {
	return context.WithValue(ctx, BaggageContextKey{}, make(map[string]string))
}

// getAll returns all baggage as a map
func (m *BaggageManager) getAll(ctx context.Context) map[string]string {
	val := ctx.Value(BaggageContextKey{})
	if val == nil {
		return make(map[string]string)
	}
	baggage, ok := val.(map[string]string)
	if !ok {
		return make(map[string]string)
	}
	// Return a copy to avoid mutation issues
	result := make(map[string]string, len(baggage))
	for k, v := range baggage {
		result[k] = v
	}
	return result
}

// InjectToHeader injects baggage into W3C tracestate header format
func (m *BaggageManager) InjectToHeader(ctx context.Context) string {
	items := m.List(ctx)
	if len(items) == 0 {
		return ""
	}

	var parts []string
	for _, item := range items {
		parts = append(parts, item.Key+"="+item.Value)
	}
	return strings.Join(parts, ",")
}

// ExtractFromHeader extracts baggage from W3C tracestate header
func (m *BaggageManager) ExtractFromHeader(ctx context.Context, header string) context.Context {
	if header == "" {
		return ctx
	}

	pairs := strings.Split(header, ",")
	baggage := make(map[string]string)

	for _, pair := range pairs {
		kv := strings.SplitN(strings.TrimSpace(pair), "=", 2)
		if len(kv) == 2 {
			baggage[kv[0]] = kv[1]
		}
	}

	if len(baggage) == 0 {
		return ctx
	}

	return context.WithValue(ctx, BaggageContextKey{}, baggage)
}
