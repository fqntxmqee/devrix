//go:build performance

package performance_test

import (
	"context"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/contextengine/compression"
	"github.com/devrix/devrix/internal/shared/types"
)

// Covers: L5-OBS-19
func TestCompression_P99LatencyUnder500ms(t *testing.T) {
	p := compression.NewPipelineEnabled(true)
	budget := types.DefaultTokenBudget()
	budget.CompressionTarget = 200

	var msgs []types.Message
	for i := 0; i < 50; i++ {
		msgs = append(msgs, *types.NewMessage(
			"m",
			"s",
			types.MessageRoleUser,
			strings.Repeat("payload ", 30),
		))
	}

	const iterations = 100
	latencies := make([]time.Duration, 0, iterations)
	for i := 0; i < iterations; i++ {
		start := time.Now()
		_, _, err := p.Run(context.Background(), msgs, "system prompt", budget)
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		latencies = append(latencies, time.Since(start))
	}

	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	p99 := latencies[int(float64(len(latencies))*0.99)]
	if p99 > 500*time.Millisecond {
		t.Fatalf("P99 latency %v exceeds 500ms", p99)
	}
}
