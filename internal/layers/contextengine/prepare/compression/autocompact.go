package compression

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/devrix/devrix/internal/layers/contextengine/i18n"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/conversation"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
	"github.com/google/uuid"
)

type summaryOutput struct {
	Topics    []string `json:"topics"`
	Decisions []string `json:"decisions"`
	OpenItems []string `json:"open_items"`
}

// runAutocompact executes step 6 on message history (no system prompt).
func runAutocompact(
	ctx context.Context,
	sessionID string,
	msgs []types.Message,
	budget types.TokenBudget,
	counter contracts.ITokenCounter,
	cfg config.AutocompactConfig,
	summarizer Summarizer,
	observer StepObserver,
	async *AsyncAutocompacter,
	loc i18n.Locale,
) ([]types.Message, string, error) {
	if !shouldAutocompact(msgs, budget, counter, cfg) {
		return msgs, stepAutocompact + ":skipped", nil
	}
	if summarizer == nil {
		return msgs, stepAutocompact + ":degraded", nil
	}

	turns := splitTurns(msgs)
	head := cfg.PreserveHeadTurns
	tail := cfg.PreserveTailTurns
	if head <= 0 {
		head = 2
	}
	if tail <= 0 {
		tail = 2
	}
	if len(turns) <= head+tail {
		return msgs, stepAutocompact + ":skipped", nil
	}

	if async != nil && sessionID != "" {
		asyncToken := uuid.NewString()
		out := buildAutocompactPlaceholder(turns, head, tail, asyncToken)
		async.StartAsync(sessionID, asyncToken, cfg, turns, head, tail, observer, loc)
		return out, stepAutocompact, nil
	}

	middle := flattenTurns(turns[head : len(turns)-tail])
	before := counter.CountMessages(msgs)

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	summary, err := summarizeWithRetry(runCtx, summarizer, cfg, middle, loc)
	if err != nil {
		if observer != nil {
			observer.OnAutocompact(AutocompactMeta{Degraded: true, Model: cfg.Model})
		}
		return msgs, stepAutocompact + ":degraded", nil
	}

	summaryMsg := types.Message{
		Role:    types.MessageRoleAssistant,
		Content: formatSummaryContent(summary, loc),
		Metadata: map[string]string{
			"compressed_by":  "autocompact",
			"original_count": fmt.Sprintf("%d", len(middle)),
		},
	}

	var out []types.Message
	for i := 0; i < head; i++ {
		out = append(out, turns[i]...)
	}
	out = append(out, conversation.NewCompactBoundaryMessage(sessionID, "auto", len(middle), loc))
	out = append(out, summaryMsg)
	for i := len(turns) - tail; i < len(turns); i++ {
		out = append(out, turns[i]...)
	}

	after := counter.CountMessages(out)
	if observer != nil {
		observer.OnAutocompact(AutocompactMeta{
			Degraded:      false,
			SummaryTokens: counter.CountText(summaryMsg.Content),
			Model:         cfg.Model,
		})
	}
	_ = before
	_ = after
	return out, stepAutocompact, nil
}

func shouldAutocompact(msgs []types.Message, budget types.TokenBudget, counter contracts.ITokenCounter, cfg config.AutocompactConfig) bool {
	if !cfg.Enabled {
		return false
	}
	minMsgs := cfg.MinMessagesForSummary
	if minMsgs <= 0 {
		minMsgs = 8
	}
	if len(msgs) < minMsgs {
		return false
	}
	return counter.CountMessages(msgs) > budget.CompressionTarget
}

func summarizeWithRetry(ctx context.Context, summarizer Summarizer, cfg config.AutocompactConfig, segment []types.Message, loc i18n.Locale) (string, error) {
	maxTok := cfg.SummaryMaxTokens
	if maxTok <= 0 {
		maxTok = 512
	}
	prompt := buildAutocompactPrompt(segment, loc)
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		raw, err := summarizer.Summarize(ctx, cfg.Model, prompt, maxTok)
		if err != nil {
			lastErr = err
			continue
		}
		if _, err := parseSummaryJSON(raw); err != nil {
			lastErr = err
			continue
		}
		return raw, nil
	}
	return "", lastErr
}

func buildAutocompactPrompt(segment []types.Message, loc i18n.Locale) string {
	var b strings.Builder
	for _, m := range segment {
		b.WriteString(string(m.Role))
		b.WriteString(": ")
		b.WriteString(m.Content)
		b.WriteString("\n")
	}
	return i18n.BuildAutocompactPrompt(loc, b.String())
}

func formatSummaryContent(raw string, loc i18n.Locale) string {
	parsed, err := parseSummaryJSON(raw)
	if err != nil {
		return i18n.FormatAutocompactSummaryContent(loc, nil, nil, nil, raw)
	}
	return i18n.FormatAutocompactSummaryContent(loc, parsed.Topics, parsed.Decisions, parsed.OpenItems, raw)
}

func parseSummaryJSON(raw string) (*summaryOutput, error) {
	trimmed := strings.TrimSpace(raw)
	if idx := strings.Index(trimmed, "{"); idx >= 0 {
		if end := strings.LastIndex(trimmed, "}"); end > idx {
			trimmed = trimmed[idx : end+1]
		}
	}
	var out summaryOutput
	if err := json.Unmarshal([]byte(trimmed), &out); err != nil {
		return nil, err
	}
	return &out, nil
}
