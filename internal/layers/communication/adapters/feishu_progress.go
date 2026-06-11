package adapters

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/devrix/devrix/internal/layers/communication/core"
	"github.com/devrix/devrix/internal/shared/textutil"
	"github.com/devrix/devrix/internal/shared/types"
)

const progressStyleStructured = "structured"

type toolCallEntry struct {
	name   string
	input  string
	result string // populated when show_tool_results is enabled
}

type feishuSessionStream struct {
	mu             sync.Mutex
	progressMsgID  string
	responseMsgID  string
	thinkingMsgID  string
	agentOutputMsgID string
	toolsMsgID     string
	toolCalls      []toolCallEntry
	textBuffer     strings.Builder
	thinkingBuffer strings.Builder
	agentOutputBuffer strings.Builder
	summaries      []string
	progressPct    int
	taskName       string

	replyCardID        string
	cardkitEnabled     bool
	cardkitSequence    int
	lastStreamPutAt    time.Time
	lastStreamPutRunes int
}

// normalizeProgressStyle always returns structured; legacy card/compact modes were removed.
func normalizeProgressStyle(string) string {
	return progressStyleStructured
}

func (a *FeishuAdapter) sessionStream(sessionID string) *feishuSessionStream {
	if value, ok := a.sessionStreams.Load(sessionID); ok {
		if stream, ok := value.(*feishuSessionStream); ok {
			return stream
		}
	}
	stream := &feishuSessionStream{}
	actual, _ := a.sessionStreams.LoadOrStore(sessionID, stream)
	return actual.(*feishuSessionStream)
}

func (a *FeishuAdapter) clearSessionStream(sessionID string) {
	a.sessionStreams.Delete(sessionID)
}

func (a *FeishuAdapter) handleProgressEvent(ctx context.Context, msg *types.OutboundMessage) error {
	return a.handleStructuredProgressEvent(ctx, msg)
}

func (a *FeishuAdapter) handleStructuredProgressEvent(ctx context.Context, msg *types.OutboundMessage) error {
	switch msg.Metadata["event_type"] {
	case "thinking":
		return a.sendStructuredThinkingCard(ctx, msg)
	case "tool_call":
		return a.sendStructuredToolCard(ctx, msg)
	case "tool_result":
		return a.handleStructuredToolResult(ctx, msg)
	case "milestone_progress", "info", "worker_progress":
		return a.appendTaskProgress(ctx, msg)
	default:
		return nil
	}
}

func (a *FeishuAdapter) sendStructuredThinkingCard(ctx context.Context, msg *types.OutboundMessage) error {
	stream := a.sessionStream(msg.SessionID)
	stream.mu.Lock()
	if msg.Content != "" {
		stream.thinkingBuffer.WriteString(msg.Content)
	}
	text := strings.TrimSpace(stream.thinkingBuffer.String())
	thinkingMsgID := stream.thinkingMsgID
	stream.mu.Unlock()

	if text == "" {
		text = "思考中..."
	}
	card := NewCard().
		Markdown(text).
		Build()
	cardJSON := BuildCardJSON(card)

	if thinkingMsgID == "" {
		msgID, err := a.sendCardReplyAndGetID(ctx, msg.SessionID, msg.ChatID, cardJSON)
		if err != nil {
			return err
		}
		stream.mu.Lock()
		if stream.thinkingMsgID != "" {
			stream.mu.Unlock()
			return a.patchMessage(ctx, stream.thinkingMsgID, cardJSON)
		}
		stream.thinkingMsgID = msgID
		stream.mu.Unlock()
		return nil
	}
	return a.patchMessage(ctx, thinkingMsgID, cardJSON)
}

func (a *FeishuAdapter) sendStructuredToolCard(ctx context.Context, msg *types.OutboundMessage) error {
	toolName := strings.TrimSpace(msg.Metadata["tool_name"])
	if toolName == "" {
		toolName = strings.TrimSpace(msg.Content)
	}
	input := strings.TrimSpace(msg.Metadata["input"])
	if input == "" {
		input = strings.TrimSpace(msg.Content)
	}

	stream := a.sessionStream(msg.SessionID)
	stream.mu.Lock()
	stream.toolCalls = append(stream.toolCalls, toolCallEntry{name: toolName, input: input})
	stream.mu.Unlock()

	return a.upsertToolsCard(ctx, msg.SessionID, msg.ChatID)
}

func (a *FeishuAdapter) upsertToolsCard(ctx context.Context, sessionID, chatID string) error {
	stream := a.sessionStream(sessionID)
	stream.mu.Lock()
	card := NewCard().
		Markdown(buildToolsCardMarkdown(stream.toolCalls, a.showToolResults)).
		Build()
	cardJSON := BuildCardJSON(card)
	toolsMsgID := stream.toolsMsgID
	stream.mu.Unlock()

	if toolsMsgID == "" {
		msgID, err := a.sendCardReplyAndGetID(ctx, sessionID, chatID, cardJSON)
		if err != nil {
			return err
		}
		stream.mu.Lock()
		if stream.toolsMsgID != "" {
			stream.mu.Unlock()
			return a.patchMessage(ctx, stream.toolsMsgID, cardJSON)
		}
		stream.toolsMsgID = msgID
		stream.mu.Unlock()
		return nil
	}
	return a.patchMessage(ctx, toolsMsgID, cardJSON)
}

func buildToolsCardMarkdown(entries []toolCallEntry, showResults bool) string {
	if len(entries) == 0 {
		return "_等待工具调用…_"
	}
	var body strings.Builder
	for i, entry := range entries {
		if i > 0 {
			body.WriteString("\n\n---\n\n")
		}
		fmt.Fprintf(&body, "**工具 #%d:** `%s`", i+1, entry.name)
		if entry.input != "" && entry.input != entry.name {
			body.WriteString("\n```\n")
			body.WriteString(entry.input)
			body.WriteString("\n```")
		}
		if showResults && strings.TrimSpace(entry.result) != "" {
			body.WriteString("\n\n**结果**\n")
			body.WriteString(formatToolResultMarkdown(entry.result))
		}
	}
	return body.String()
}

func (a *FeishuAdapter) handleStructuredToolResult(ctx context.Context, msg *types.OutboundMessage) error {
	if !a.showToolResults {
		return nil
	}
	return a.sendStructuredToolResultCard(ctx, msg)
}

func (a *FeishuAdapter) sendStructuredToolResultCard(ctx context.Context, msg *types.OutboundMessage) error {
	toolName := strings.TrimSpace(msg.Metadata["tool_name"])
	if toolName == "" {
		toolName = strings.TrimSpace(msg.Content)
	}
	resultBody := strings.TrimSpace(msg.Content)

	stream := a.sessionStream(msg.SessionID)
	stream.mu.Lock()
	toolsMsgID := stream.toolsMsgID
	if toolsMsgID != "" && len(stream.toolCalls) > 0 {
		idx := findToolCallResultIndex(stream.toolCalls, toolName)
		if idx >= 0 {
			stream.toolCalls[idx].result = resultBody
		}
	}
	stream.mu.Unlock()

	if toolsMsgID != "" {
		return a.upsertToolsCard(ctx, msg.SessionID, msg.ChatID)
	}

	title := "工具结果"
	if toolName != "" {
		title = fmt.Sprintf("工具结果 · %s", toolName)
	}
	card := NewCard().Title(title, "green").Markdown(formatToolResultMarkdown(resultBody)).Build()
	return a.sendCardToSession(ctx, msg.SessionID, msg.ChatID, card)
}

// findToolCallResultIndex picks the last tool call without a result, preferring name match.
func findToolCallResultIndex(entries []toolCallEntry, toolName string) int {
	if toolName != "" {
		for i := len(entries) - 1; i >= 0; i-- {
			if entries[i].result == "" && entries[i].name == toolName {
				return i
			}
		}
	}
	for i := len(entries) - 1; i >= 0; i-- {
		if entries[i].result == "" {
			return i
		}
	}
	return -1
}

func formatToolResultMarkdown(content string) string {
	body := strings.TrimSpace(stripOuterCodeFence(content))
	if body == "" {
		return "_无输出_"
	}
	if strings.Contains(body, "```") {
		return body
	}
	return "```\n" + body + "\n```"
}

func (a *FeishuAdapter) appendTaskProgress(ctx context.Context, msg *types.OutboundMessage) error {
	stream := a.sessionStream(msg.SessionID)
	stream.mu.Lock()
	switch msg.Metadata["event_type"] {
	case "milestone_progress":
		if progress := strings.TrimSpace(msg.Metadata["progress"]); progress != "" {
			stream.progressPct = parseProgressPercent(progress)
		}
		if task := strings.TrimSpace(msg.Metadata["task"]); task != "" {
			stream.taskName = task
		}
	case "info":
		if text := strings.TrimSpace(msg.Content); text != "" {
			stream.summaries = append(stream.summaries, text)
		}
	case "worker_progress":
		if line := formatWorkerProgressSummary(msg); line != "" {
			stream.summaries = append(stream.summaries, line)
		}
	}
	stream.mu.Unlock()
	return a.upsertTaskProgressCard(ctx, msg.SessionID, msg.ChatID, false)
}

func formatWorkerProgressSummary(msg *types.OutboundMessage) string {
	kind := strings.TrimSpace(msg.Metadata["kind"])
	switch kind {
	case "tool_call", "iterating":
		return ""
	case "started", "completed", "failed", "joined", "forked", "progress", "waiting_permission":
		role := strings.TrimSpace(msg.Metadata["role"])
		worker := strings.TrimSpace(msg.Metadata["worker_id"])
		summary := strings.TrimSpace(msg.Content)
		if summary == "" {
			summary = kind
		}
		if role != "" && worker != "" {
			return fmt.Sprintf("[%s/%s] %s", role, worker, summary)
		}
		if worker != "" {
			return fmt.Sprintf("[%s] %s", worker, summary)
		}
		return summary
	default:
		return ""
	}
}

func (a *FeishuAdapter) upsertTaskProgressCard(ctx context.Context, sessionID, chatID string, completed bool) error {
	stream := a.sessionStream(sessionID)
	stream.mu.Lock()
	if completed && stream.progressPct < 100 {
		stream.progressPct = 100
	}
	card := buildTaskProgressCard(stream, completed)
	stream.mu.Unlock()

	cardJSON := BuildCardJSON(card)
	stream.mu.Lock()
	defer stream.mu.Unlock()

	if stream.progressMsgID == "" {
		msgID, err := a.sendCardReplyAndGetID(ctx, sessionID, chatID, cardJSON)
		if err != nil {
			return err
		}
		stream.progressMsgID = msgID
		return nil
	}
	return a.patchMessage(ctx, stream.progressMsgID, cardJSON)
}

func (a *FeishuAdapter) finalizeStructuredSession(ctx context.Context, sessionID, chatID, summary string) error {
	stream := a.sessionStream(sessionID)
	stream.mu.Lock()
	hasTaskCard := stream.progressMsgID != "" || stream.taskName != "" || stream.progressPct > 0
	responseMsgID := stream.responseMsgID
	responseText := stream.textBuffer.String()
	if hasTaskCard && strings.TrimSpace(summary) != "" {
		stream.summaries = append(stream.summaries, strings.TrimSpace(summary))
	}
	stream.mu.Unlock()

	if hasTaskCard {
		if err := a.upsertTaskProgressCard(ctx, sessionID, chatID, true); err != nil {
			return err
		}
	}
	stream.mu.Lock()
	cardkitActive := stream.cardkitEnabled && stream.replyCardID != ""
	stream.mu.Unlock()
	if cardkitActive {
		return a.finalizeReplyCardStreaming(ctx, stream, summary)
	}
	if responseMsgID != "" && strings.TrimSpace(summary) != "" {
		footer := responseText + "\n\n---\n_" + strings.TrimSpace(summary) + "_"
		card := NewCard().
			Markdown(footer).
			Build()
		return a.patchMessage(ctx, responseMsgID, BuildCardJSON(card))
	}
	return nil
}

func (a *FeishuAdapter) finalizeReplyCardStreaming(ctx context.Context, stream *feishuSessionStream, summary string) error {
	stream.mu.Lock()
	defer stream.mu.Unlock()

	if !stream.cardkitEnabled || stream.replyCardID == "" {
		return nil
	}

	content := stream.textBuffer.String()
	if strings.TrimSpace(summary) != "" {
		content += "\n\n---\n_" + strings.TrimSpace(summary) + "_"
	}

	stream.cardkitSequence++
	if err := a.cardkit.StreamElementContent(ctx, stream.replyCardID, replyTextElementID, content, stream.cardkitSequence); err != nil {
		if errors.Is(err, ErrFeishuCardRateLimited) {
			// rate-limited — skip final stream, try updateCard directly
		} else if errors.Is(err, ErrFeishuCardStreamClosed) {
			// card already closed, nothing to finalize
			return nil
		} else {
			return err
		}
	}

	stream.cardkitSequence++
	finalJSON := BuildStreamingReplyCardJSON(content, false)
	return a.cardkit.UpdateCard(ctx, stream.replyCardID, finalJSON, stream.cardkitSequence)
}

func buildTaskProgressCard(stream *feishuSessionStream, completed bool) *core.Card {
	color := "purple"
	title := "任务进度"
	if completed {
		color = "green"
		title = "任务完成"
	}

	pct := stream.progressPct
	if completed {
		pct = 100
	}

	builder := NewCard().Title(title, color)
	builder = builder.Markdown(renderProgressBarMarkdown(pct))

	if stream.taskName != "" {
		builder = builder.Markdown("**任务:** " + stream.taskName)
	}
	if len(stream.summaries) > 0 {
		builder = builder.Markdown("**小结**")
		for _, summary := range stream.summaries {
			builder = builder.Markdown("- " + summary)
		}
	}
	if completed {
		builder = builder.Markdown("✅ **已完成**")
	} else if pct == 0 && stream.taskName == "" && len(stream.summaries) == 0 {
		builder = builder.Markdown("_等待任务更新…_")
	}
	return builder.Build()
}

func renderProgressBarMarkdown(pct int) string {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	filled := pct / 5
	if filled > 20 {
		filled = 20
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", 20-filled)
	return fmt.Sprintf("**进度:** %d%%\n`%s`", pct, bar)
}

func parseProgressPercent(raw string) int {
	raw = strings.TrimSpace(strings.TrimSuffix(raw, "%"))
	if raw == "" {
		return 0
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0
	}
	return n
}

func (a *FeishuAdapter) appendResponseText(ctx context.Context, sessionID, chatID, chunk string) error {
	chunk = textutil.StripThinkingTags(chunk)
	if strings.TrimSpace(chunk) == "" {
		return nil
	}

	stream := a.sessionStream(sessionID)
	stream.mu.Lock()
	stream.textBuffer.WriteString(chunk)
	content := stream.textBuffer.String()
	stream.mu.Unlock()

	if !a.streamingEnabled {
		return a.patchResponseCard(ctx, sessionID, chatID, stream, content)
	}

	stream.mu.Lock()
	started := stream.responseMsgID != "" || stream.cardkitEnabled
	stream.mu.Unlock()
	if !started {
		return a.startStreamingReplyCard(ctx, sessionID, chatID, stream, content)
	}

	stream.mu.Lock()
	useCardkit := stream.cardkitEnabled && stream.replyCardID != ""
	stream.mu.Unlock()
	if useCardkit {
		return a.streamReplyElement(ctx, stream, content, false)
	}
	return a.patchResponseCard(ctx, sessionID, chatID, stream, content)
}

func (a *FeishuAdapter) startStreamingReplyCard(ctx context.Context, sessionID, chatID string, stream *feishuSessionStream, content string) error {
	cardJSON := BuildStreamingReplyCardJSON("", true)
	cardID, err := a.cardkit.CreateCard(ctx, cardJSON)
	if err != nil {
		slog.Warn("feishu: cardkit create failed, falling back to patch streaming", "error", err)
		return a.patchResponseCard(ctx, sessionID, chatID, stream, content)
	}

	sendContent, err := buildCardIDMessageContent(cardID)
	if err != nil {
		return err
	}
	msgID, err := a.sendInteractiveContentAndGetID(ctx, sessionID, chatID, sendContent)
	if err != nil {
		return err
	}

	stream.mu.Lock()
	stream.responseMsgID = msgID
	stream.replyCardID = cardID
	stream.cardkitEnabled = true
	stream.mu.Unlock()

	if strings.TrimSpace(content) == "" {
		return nil
	}
	return a.streamReplyElement(ctx, stream, content, true)
}

func (a *FeishuAdapter) streamReplyElement(ctx context.Context, stream *feishuSessionStream, content string, force bool) error {
	stream.mu.Lock()
	if !stream.cardkitEnabled || stream.replyCardID == "" {
		stream.mu.Unlock()
		return nil
	}
	newRunes := utf8.RuneCountInString(content)
	if !a.streamThrottle.shouldFlush(stream.lastStreamPutAt, stream.lastStreamPutRunes, newRunes, force) {
		stream.mu.Unlock()
		return nil
	}
	stream.cardkitSequence++
	seq := stream.cardkitSequence
	cardID := stream.replyCardID
	stream.mu.Unlock()

	err := a.cardkit.StreamElementContent(ctx, cardID, replyTextElementID, content, seq)

	stream.mu.Lock()
	defer stream.mu.Unlock()
	if err != nil {
		if errors.Is(err, ErrFeishuCardRateLimited) {
			stream.cardkitSequence--
			return nil
		}
		if errors.Is(err, ErrFeishuCardStreamClosed) {
			// Card was closed by Feishu (timeout or prior finalization).
			// Reset cardkit state so the next appendResponseText call creates a fresh card.
			stream.replyCardID = ""
			stream.cardkitEnabled = false
			stream.cardkitSequence = 0
			stream.lastStreamPutAt = time.Time{}
			stream.lastStreamPutRunes = 0
			slog.Debug("feishu: card stream closed, will create new card on next chunk",
				"session", slog.String("cardID", cardID))
			return nil
		}
		return err
	}
	stream.lastStreamPutAt = time.Now()
	stream.lastStreamPutRunes = newRunes
	return nil
}

func (a *FeishuAdapter) patchResponseCard(ctx context.Context, sessionID, chatID string, stream *feishuSessionStream, content string) error {
	card := NewCard().Markdown(content).Build()
	cardJSON := BuildCardJSON(card)

	stream.mu.Lock()
	defer stream.mu.Unlock()

	if stream.responseMsgID == "" {
		msgID, err := a.sendInteractiveContentAndGetID(ctx, sessionID, chatID, cardJSON)
		if err != nil {
			return err
		}
		stream.responseMsgID = msgID
		return nil
	}
	return a.patchMessage(ctx, stream.responseMsgID, cardJSON)
}

func (a *FeishuAdapter) sendInteractiveContentAndGetID(ctx context.Context, sessionID, chatID, content string) (string, error) {
	if replyCtx, ok := a.getSessionReplyContext(sessionID); ok && replyCtx.userMessageID != "" {
		return a.replyToUserMessage(ctx, replyCtx.userMessageID, "interactive", content)
	}
	return a.createInteractiveMessage(ctx, chatID, content)
}

func isCallAgentTool(msg *types.OutboundMessage) bool {
	if msg == nil {
		return false
	}
	toolName := strings.TrimSpace(msg.Metadata["tool_name"])
	if toolName == "" {
		toolName = strings.TrimSpace(msg.Content)
	}
	return strings.HasPrefix(toolName, "call_")
}

func agentToolCardTitle(msg *types.OutboundMessage) string {
	if agent := strings.TrimSpace(msg.Metadata["agent"]); agent != "" {
		return agent + " 输出"
	}
	toolName := strings.TrimSpace(msg.Metadata["tool_name"])
	if toolName == "" {
		toolName = strings.TrimSpace(msg.Content)
	}
	if strings.HasPrefix(toolName, "call_") {
		name := strings.TrimPrefix(toolName, "call_")
		name = strings.ReplaceAll(name, "-", " ")
		if name != "" {
			return strings.ToUpper(name[:1]) + name[1:] + " 输出"
		}
	}
	return "Agent 输出"
}

// ensureAgentStreamCard creates the purple agent output card before the first stream chunk arrives.
func (a *FeishuAdapter) ensureAgentStreamCard(ctx context.Context, msg *types.OutboundMessage, placeholder string) error {
	stream := a.sessionStream(msg.SessionID)
	stream.mu.Lock()
	if stream.agentOutputMsgID != "" {
		stream.mu.Unlock()
		return nil
	}
	stream.mu.Unlock()

	placeholder = strings.TrimSpace(placeholder)
	if placeholder == "" {
		placeholder = "⏳ 执行中，输出将实时更新…"
	}
	return a.patchAgentStreamCard(ctx, msg, placeholder)
}

func (a *FeishuAdapter) appendAgentStreamText(ctx context.Context, msg *types.OutboundMessage) error {
	chunk := textutil.StripThinkingTags(msg.Content)
	if strings.TrimSpace(chunk) == "" {
		return nil
	}
	return a.patchAgentStreamCard(ctx, msg, chunk)
}

func (a *FeishuAdapter) patchAgentStreamCard(ctx context.Context, msg *types.OutboundMessage, chunk string) error {
	stream := a.sessionStream(msg.SessionID)
	stream.mu.Lock()
	if stream.agentOutputBuffer.Len() > 0 {
		existing := stream.agentOutputBuffer.String()
		if !strings.HasSuffix(existing, "\n") {
			stream.agentOutputBuffer.WriteString("\n")
		}
	}
	stream.agentOutputBuffer.WriteString(chunk)
	content := flattenMarkdownTablesForFeishu(stream.agentOutputBuffer.String())
	agentOutputMsgID := stream.agentOutputMsgID
	stream.mu.Unlock()

	card := NewCard().
		Title(agentToolCardTitle(msg), "purple").
		Markdown("```\n" + content + "\n```").
		Build()
	cardJSON := BuildCardJSON(card)

	stream.mu.Lock()
	defer stream.mu.Unlock()

	if agentOutputMsgID == "" {
		msgID, err := a.sendCardReplyAndGetID(ctx, msg.SessionID, msg.ChatID, cardJSON)
		if err != nil {
			return err
		}
		stream.agentOutputMsgID = msgID
		slog.Info("feishu: agent stream card created",
			"sessionID", msg.SessionID,
			"title", agentToolCardTitle(msg),
		)
		return nil
	}
	return a.patchMessage(ctx, agentOutputMsgID, cardJSON)
}

func stripOuterCodeFence(content string) string {
	trimmed := strings.TrimSpace(content)
	if !strings.HasPrefix(trimmed, "```") {
		return trimmed
	}
	lines := strings.Split(trimmed, "\n")
	if len(lines) < 2 {
		return trimmed
	}
	start := 1
	end := len(lines)
	if end > start && strings.TrimSpace(lines[end-1]) == "```" {
		end--
	}
	if end <= start {
		return trimmed
	}
	return strings.TrimSpace(strings.Join(lines[start:end], "\n"))
}
