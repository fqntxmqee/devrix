package compression_test

import (
	"context"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/compression"
	"github.com/devrix/devrix/internal/layers/contextengine/token"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

type highTokenCounter struct {
	inner *token.Counter
}

func (h *highTokenCounter) CountText(text string) int {
	return h.inner.CountText(text) + 500
}

func (h *highTokenCounter) CountMessages(msgs []types.Message) int {
	return h.inner.CountMessages(msgs) + 500
}

func (h *highTokenCounter) CountWithSystemPrompt(systemPrompt string, messages []types.Message) int {
	return h.inner.CountWithSystemPrompt(systemPrompt, messages) + 500
}

func (h *highTokenCounter) TruncateToTokens(text string, maxTokens int) string {
	return h.inner.TruncateToTokens(text, maxTokens)
}

func (h *highTokenCounter) EncodingForModel(model string) string {
	return h.inner.EncodingForModel(model)
}

type mockSummarizer struct {
	response string
	err      error
}

func (m *mockSummarizer) Summarize(ctx context.Context, model, prompt string, maxTokens int) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	if m.response != "" {
		return m.response, nil
	}
	return `{"topics":["auth"],"decisions":["use jwt"],"open_items":[]}`, nil
}

// Covers: L5-CTX-12
func TestPipeline_should_autocompact_when_enabled_and_over_budget(t *testing.T) {
	counter := &highTokenCounter{inner: token.NewCounter()}
	cfg := config.DefaultAutocompactConfig()
	cfg.Enabled = true
	cfg.MinMessagesForSummary = 4
	cfg.PreserveHeadTurns = 1
	cfg.PreserveTailTurns = 1

	p := compression.NewPipeline(
		compression.WithEnabled(true),
		compression.WithCounter(counter),
		compression.WithAutocompactConfig(cfg),
		compression.WithSummarizer(&mockSummarizer{}),
	)

	var msgs []types.Message
	for i := 0; i < 12; i++ {
		role := types.MessageRoleUser
		if i%2 == 1 {
			role = types.MessageRoleAssistant
		}
		msgs = append(msgs, *types.NewMessage("m", "s", role, strings.Repeat("word ", 80)))
	}
	budget := types.DefaultTokenBudget()
	budget.CompressionTarget = 100

	out, report, err := p.Run(context.Background(), msgs, "system", budget)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	found := false
	for _, step := range report.StepsApplied {
		if step == "autocompact" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected autocompact step, got %v", report.StepsApplied)
	}
	if len(out) == 0 {
		t.Fatal("expected output messages")
	}
}

// Covers: L5-CTX-13, L5-CTX-30
func TestPipeline_should_degrade_autocompact_on_llm_failure(t *testing.T) {
	counter := &highTokenCounter{inner: token.NewCounter()}
	cfg := config.DefaultAutocompactConfig()
	cfg.Enabled = true
	cfg.MinMessagesForSummary = 4
	cfg.PreserveHeadTurns = 1
	cfg.PreserveTailTurns = 1

	p := compression.NewPipeline(
		compression.WithEnabled(true),
		compression.WithCounter(counter),
		compression.WithAutocompactConfig(cfg),
		compression.WithSummarizer(&mockSummarizer{err: context.DeadlineExceeded}),
	)

	var msgs []types.Message
	for i := 0; i < 12; i++ {
		role := types.MessageRoleUser
		if i%2 == 1 {
			role = types.MessageRoleAssistant
		}
		msgs = append(msgs, *types.NewMessage("m", "s", role, strings.Repeat("x ", 100)))
	}
	budget := types.DefaultTokenBudget()
	budget.CompressionTarget = 100

	_, report, err := p.Run(context.Background(), msgs, "", budget)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	found := false
	for _, step := range report.StepsApplied {
		if step == "autocompact:degraded" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected autocompact:degraded, got %v", report.StepsApplied)
	}
}
