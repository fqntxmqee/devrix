//go:build integration && d2

package integration

import (
	"context"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine"
	"github.com/devrix/devrix/internal/layers/contextengine/compression"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

type recordingCompressionObserver struct {
	steps      []string
	autocompact int
}

func (o *recordingCompressionObserver) EmitCompressionStep(_ string, step string, _, _ int) {
	o.steps = append(o.steps, step)
}

func (o *recordingCompressionObserver) EmitAutocompact(_ string, _ contextengine.AutocompactMeta) {
	o.autocompact++
}

func (o *recordingCompressionObserver) EmitAutocompactComplete(_ string, _ types.Message, _ string) {}

// T: D2-S0-A01-T01
func TestIntegration_CompressionObserverReceivesSteps(t *testing.T) {
	obs := &recordingCompressionObserver{}
	cfg := config.DefaultContextEngineConfig()
	cfg.Compression.Autocompact.Enabled = false

	p := compression.NewPipeline(
		compression.WithEnabled(true),
		compression.WithAutocompactConfig(cfg.Compression.Autocompact),
		compression.WithStepObserver(&pipelineObsAdapter{obs: obs}),
	)

	var msgs []types.Message
	for i := 0; i < 8; i++ {
		msgs = append(msgs, *types.NewMessage("m", "s", types.MessageRoleUser, strings.Repeat("a ", 200)))
	}
	budget := types.DefaultTokenBudget()
	budget.CompressionTarget = 20
	_, _, err := p.Run(context.Background(), msgs, "sys", budget)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(obs.steps) == 0 {
		t.Fatal("expected compression step events")
	}
}

type pipelineObsAdapter struct {
	obs *recordingCompressionObserver
}

func (a *pipelineObsAdapter) OnStep(_ context.Context, step string, before, after int) {
	a.obs.EmitCompressionStep("", step, before, after)
}

func (a *pipelineObsAdapter) OnAutocompact(meta compression.AutocompactMeta) {
	a.obs.EmitAutocompact("", contextengine.AutocompactMeta{
		Degraded: meta.Degraded, SummaryTokens: meta.SummaryTokens, Model: meta.Model,
	})
}

func (a *pipelineObsAdapter) OnAutocompactComplete(_ types.Message, _, _ string) {}
