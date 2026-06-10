package tracer

import (
	"context"
	"sort"
	"strings"
)

const BaggageHeader = "baggage"

// BaggageContextKey is the key for baggage in context.
type BaggageContextKey struct{}

// BaggageItem represents a single baggage item.
type BaggageItem struct {
	Key   string
	Value string
}

// BaggageManager manages trace baggage per W3C Baggage spec.
type BaggageManager struct {
	maxItems int
}

// DefaultBaggageManager is the process-wide baggage manager.
var DefaultBaggageManager = NewBaggageManager(32)

// NewBaggageManager creates a new baggage manager.
func NewBaggageManager(maxItems int) *BaggageManager {
	if maxItems <= 0 {
		maxItems = 32
	}
	return &BaggageManager{maxItems: maxItems}
}

// Set sets a baggage item.
func (m *BaggageManager) Set(ctx context.Context, key, value string) context.Context {
	if key == "" || value == "" {
		return ctx
	}

	baggage := m.getAll(ctx)
	baggage[key] = value

	if len(baggage) > m.maxItems {
		keys := make([]string, 0, len(baggage))
		for k := range baggage {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		delete(baggage, keys[0])
	}

	return context.WithValue(ctx, BaggageContextKey{}, baggage)
}

// Get retrieves a baggage item.
func (m *BaggageManager) Get(ctx context.Context, key string) (string, bool) {
	baggage := m.getAll(ctx)
	val, ok := baggage[key]
	return val, ok
}

// List returns all baggage items in sorted order.
func (m *BaggageManager) List(ctx context.Context) []BaggageItem {
	baggage := m.getAll(ctx)
	items := make([]BaggageItem, 0, len(baggage))
	for k, v := range baggage {
		items = append(items, BaggageItem{Key: k, Value: v})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Key < items[j].Key
	})
	return items
}

// Clear removes all baggage.
func (m *BaggageManager) Clear(ctx context.Context) context.Context {
	return context.WithValue(ctx, BaggageContextKey{}, make(map[string]string))
}

func (m *BaggageManager) getAll(ctx context.Context) map[string]string {
	val := ctx.Value(BaggageContextKey{})
	if val == nil {
		return make(map[string]string)
	}
	baggage, ok := val.(map[string]string)
	if !ok {
		return make(map[string]string)
	}
	result := make(map[string]string, len(baggage))
	for k, v := range baggage {
		result[k] = v
	}
	return result
}

// FormatHeader serializes baggage into the W3C baggage header value.
func (m *BaggageManager) FormatHeader(ctx context.Context) string {
	items := m.List(ctx)
	if len(items) == 0 {
		return ""
	}

	parts := make([]string, 0, len(items))
	for _, item := range items {
		parts = append(parts, item.Key+"="+item.Value)
	}
	return strings.Join(parts, ",")
}

// ApplyHeader merges baggage from a W3C baggage header into context.
func (m *BaggageManager) ApplyHeader(ctx context.Context, header string) context.Context {
	if header == "" {
		return ctx
	}

	baggage := m.getAll(ctx)
	for _, pair := range strings.Split(header, ",") {
		kv := strings.SplitN(strings.TrimSpace(pair), "=", 2)
		if len(kv) == 2 && kv[0] != "" && kv[1] != "" {
			baggage[kv[0]] = kv[1]
		}
	}

	if len(baggage) == 0 {
		return ctx
	}

	return context.WithValue(ctx, BaggageContextKey{}, baggage)
}
