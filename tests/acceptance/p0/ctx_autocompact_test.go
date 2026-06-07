//go:build acceptance

package p0_test

import (
	"context"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/compression"
	"github.com/devrix/devrix/internal/layers/contextengine/token"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

type accSummarizer struct{}

func (accSummarizer) Summarize(ctx context.Context, model, prompt string, maxTokens int) (string, error) {
	_ = ctx
	_ = model
	_ = prompt
	_ = maxTokens
	return `{"topics":["compression"],"decisions":[],"open_items":[]}`, nil
}

type accHighCounter struct {
	inner *token.Counter
}

func (h *accHighCounter) CountText(text string) int       { return h.inner.CountText(text) + 500 }
func (h *accHighCounter) CountMessages(msgs []types.Message) int { return h.inner.CountMessages(msgs) + 500 }
func (h *accHighCounter) CountWithSystemPrompt(s string, m []types.Message) int {
	return h.inner.CountWithSystemPrompt(s, m) + 500
}
func (h *accHighCounter) TruncateToTokens(text string, max int) string { return h.inner.TruncateToTokens(text, max) }
func (h *accHighCounter) EncodingForModel(model string) string         { return h.inner.EncodingForModel(model) }

// Covers: L5-CTX-12
func TestAcceptance_AutocompactP0(t *testing.T) {
	cfg := config.DefaultAutocompactConfig()
	cfg.Enabled = true
	cfg.MinMessagesForSummary = 4
	cfg.PreserveHeadTurns = 1
	cfg.PreserveTailTurns = 1

	p := compression.NewPipeline(
		compression.WithEnabled(true),
		compression.WithCounter(&accHighCounter{inner: token.NewCounter()}),
		compression.WithAutocompactConfig(cfg),
		compression.WithSummarizer(accSummarizer{}),
	)

	var msgs []types.Message
	for i := 0; i < 12; i++ {
		role := types.MessageRoleUser
		if i%2 == 1 {
			role = types.MessageRoleAssistant
		}
		msgs = append(msgs, *types.NewMessage("m", "s", role, strings.Repeat("ctx ", 80)))
	}
	budget := types.DefaultTokenBudget()
	budget.CompressionTarget = 100

	before := len(msgs)
	out, report, err := p.Run(context.Background(), msgs, "system", budget)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(out) >= before {
		t.Fatalf("expected fewer messages after autocompact, before=%d after=%d", before, len(out))
	}
	found := false
	for _, s := range report.StepsApplied {
		if s == "autocompact" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected autocompact step, got %v", report.StepsApplied)
	}
}
