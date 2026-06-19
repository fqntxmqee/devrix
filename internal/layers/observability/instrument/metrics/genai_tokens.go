package metrics

import (
	"fmt"
	"sync"
)

var genAITokenCounters sync.Map // key: meterName|model|tokenType → Counter

// GenAITokenBreakdown holds token counts for gen_ai.client.token.usage metrics.
type GenAITokenBreakdown struct {
	Input       int
	Output      int
	CacheRead   int
	Reasoning   int
}

// RecordGenAITokenUsage increments gen_ai.client.token.usage by token_type.
func RecordGenAITokenUsage(meter *Meter, model string, usage GenAITokenBreakdown) {
	if meter == nil {
		return
	}
	if usage.Input > 0 {
		addGenAITokenUsage(meter, model, "input", usage.Input)
	}
	if usage.Output > 0 {
		addGenAITokenUsage(meter, model, "output", usage.Output)
	}
	if usage.CacheRead > 0 {
		addGenAITokenUsage(meter, model, "cache_read", usage.CacheRead)
	}
	if usage.Reasoning > 0 {
		addGenAITokenUsage(meter, model, "reasoning", usage.Reasoning)
	}
}

func addGenAITokenUsage(meter *Meter, model, tokenType string, n int) {
	if n <= 0 {
		return
	}
	key := fmt.Sprintf("%p|%s|%s", meter, model, tokenType)
	if raw, ok := genAITokenCounters.Load(key); ok {
		if counter, ok := raw.(Counter); ok && counter != nil {
			counter.Add(int64(n))
		}
		return
	}
	c, err := meter.Int64Counter("gen_ai.client.token.usage",
		WithLabels(LabelMap{
			"token_type": tokenType,
			"model":      model,
		}))
	if err != nil || c == nil {
		return
	}
	if existing, loaded := genAITokenCounters.LoadOrStore(key, c); loaded {
		if counter, ok := existing.(Counter); ok && counter != nil {
			counter.Add(int64(n))
		}
		return
	}
	c.Add(int64(n))
}
