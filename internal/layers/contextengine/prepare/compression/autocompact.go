package compression

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/devrix/devrix/internal/layers/contextengine/prepare/conversation"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
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
		out := buildAutocompactPlaceholder(turns, head, tail)
		async.StartAsync(sessionID, cfg, turns, head, tail, observer)
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

	summary, err := summarizeWithRetry(runCtx, summarizer, cfg, middle)
	if err != nil {
		if observer != nil {
			observer.OnAutocompact(AutocompactMeta{Degraded: true, Model: cfg.Model})
		}
		return msgs, stepAutocompact + ":degraded", nil
	}

	summaryMsg := types.Message{
		Role:    types.MessageRoleAssistant,
		Content: formatSummaryContent(summary),
		Metadata: map[string]string{
			"compressed_by":  "autocompact",
			"original_count": fmt.Sprintf("%d", len(middle)),
		},
	}

	var out []types.Message
	for i := 0; i < head; i++ {
		out = append(out, turns[i]...)
	}
	out = append(out, conversation.NewCompactBoundaryMessage(sessionID, "auto", len(middle)))
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

func summarizeWithRetry(ctx context.Context, summarizer Summarizer, cfg config.AutocompactConfig, segment []types.Message) (string, error) {
	maxTok := cfg.SummaryMaxTokens
	if maxTok <= 0 {
		maxTok = 512
	}
	prompt := buildAutocompactPrompt(segment)
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

func buildAutocompactPrompt(segment []types.Message) string {
	var b strings.Builder
	b.WriteString(`You are a conversation summarizer. Below is the middle segment of a developer-AI conversation.
Summarize ONLY what was explicitly discussed. Do NOT invent any details not present in the input.

Output strict JSON:
{
  "topics": ["list of technical topics discussed"],
  "decisions": ["list of decisions made or actions agreed"],
  "open_items": ["list of unresolved questions or pending tasks"]
}

Rules:
- If unsure about any detail, omit it rather than guess.
- Do NOT mention file paths, code, or tool outputs unless they appear in the input.
- Limit each array to at most 5 items.
- If a category has nothing to report, use an empty array.

Conversation segment:
`)
	for _, m := range segment {
		b.WriteString(string(m.Role))
		b.WriteString(": ")
		b.WriteString(m.Content)
		b.WriteString("\n")
	}
	return b.String()
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

func formatSummaryContent(raw string) string {
	parsed, err := parseSummaryJSON(raw)
	if err != nil {
		return "[autocompact summary]\n" + raw
	}
	var b strings.Builder
	b.WriteString("[autocompact summary]\n")
	if len(parsed.Topics) > 0 {
		b.WriteString("Topics: ")
		b.WriteString(strings.Join(parsed.Topics, "; "))
		b.WriteString("\n")
	}
	if len(parsed.Decisions) > 0 {
		b.WriteString("Decisions: ")
		b.WriteString(strings.Join(parsed.Decisions, "; "))
		b.WriteString("\n")
	}
	if len(parsed.OpenItems) > 0 {
		b.WriteString("Open items: ")
		b.WriteString(strings.Join(parsed.OpenItems, "; "))
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}
