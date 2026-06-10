package observability

import (
	"fmt"
	"sync"

	"github.com/devrix/devrix/internal/layers/observability/metrics"
)

var genAITokenCounters sync.Map // key: meterName|model|tokenType → metrics.Counter

// RecordGenAITokenUsage increments gen_ai.client.token.usage for input/output tokens.
func RecordGenAITokenUsage(meter *metrics.Meter, model string, inputTokens, outputTokens int) {
	if meter == nil {
		return
	}
	if inputTokens > 0 {
		addGenAITokenUsage(meter, model, "input", inputTokens)
	}
	if outputTokens > 0 {
		addGenAITokenUsage(meter, model, "output", outputTokens)
	}
}

func addGenAITokenUsage(meter *metrics.Meter, model, tokenType string, n int) {
	if n <= 0 {
		return
	}
	key := fmt.Sprintf("%p|%s|%s", meter, model, tokenType)
	if raw, ok := genAITokenCounters.Load(key); ok {
		if counter, ok := raw.(metrics.Counter); ok && counter != nil {
			counter.Add(int64(n))
		}
		return
	}
	c, err := meter.Int64Counter("gen_ai.client.token.usage",
		metrics.WithLabels(metrics.LabelMap{
			"token_type": tokenType,
			"model":      model,
		}))
	if err != nil || c == nil {
		return
	}
	if existing, loaded := genAITokenCounters.LoadOrStore(key, c); loaded {
		if counter, ok := existing.(metrics.Counter); ok && counter != nil {
			counter.Add(int64(n))
		}
		return
	}
	c.Add(int64(n))
}
