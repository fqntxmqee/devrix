package adapters

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/devrix/devrix/internal/layers/communication/core"
	"github.com/devrix/devrix/internal/shared/textutil"
	"github.com/devrix/devrix/internal/shared/types"
)

const progressStyleStructured = "structured"

type feishuSessionStream struct {
	mu             sync.Mutex
	progressMsgID  string
	responseMsgID  string
	thinkingMsgID  string
	lastToolMsgID  string
	lastToolName   string
	lastToolInput  string
	textBuffer     strings.Builder
	thinkingBuffer strings.Builder
	toolCount      int
	summaries      []string
	progressPct    int
	taskName       string
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
		return a.sendStructuredToolResultCard(ctx, msg)
	case "milestone_progress", "info":
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
		Title("思考", "blue").
		Markdown(text).
		Build()
	cardJSON := BuildCardJSON(card)

	if thinkingMsgID == "" {
		msgID, err := a.sendCardReplyAndGetID(ctx, msg.SessionID, msg.ChatID, cardJSON)
		if err != nil {
			return err
		}
		stream.mu.Lock()
		stream.thinkingMsgID = msgID
		stream.mu.Unlock()
		return nil
	}
	return a.patchMessage(ctx, thinkingMsgID, cardJSON)
}

func (a *FeishuAdapter) sendStructuredToolCard(ctx context.Context, msg *types.OutboundMessage) error {
	stream := a.sessionStream(msg.SessionID)
	stream.mu.Lock()
	stream.toolCount++
	idx := stream.toolCount
	stream.mu.Unlock()

	toolName := strings.TrimSpace(msg.Metadata["tool_name"])
	if toolName == "" {
		toolName = strings.TrimSpace(msg.Content)
	}
	input := strings.TrimSpace(msg.Metadata["input"])
	if input == "" {
		input = strings.TrimSpace(msg.Content)
	}

	var body strings.Builder
	fmt.Fprintf(&body, "**工具 #%d:** `%s`", idx, toolName)
	if input != "" && input != toolName {
		body.WriteString("\n```\n")
		body.WriteString(input)
		body.WriteString("\n```")
	}

	card := NewCard().
		Title("工具", "orange").
		Markdown(body.String()).
		Build()
	cardJSON := BuildCardJSON(card)

	msgID, err := a.sendCardReplyAndGetID(ctx, msg.SessionID, msg.ChatID, cardJSON)
	if err != nil {
		return err
	}
	stream.mu.Lock()
	stream.lastToolMsgID = msgID
	stream.lastToolName = toolName
	stream.lastToolInput = input
	stream.mu.Unlock()
	return nil
}

func (a *FeishuAdapter) sendStructuredToolResultCard(ctx context.Context, msg *types.OutboundMessage) error {
	toolName := strings.TrimSpace(msg.Metadata["tool_name"])
	if toolName == "" {
		toolName = strings.TrimSpace(msg.Content)
	}
	resultBody := formatToolResultMarkdown(msg.Content)

	stream := a.sessionStream(msg.SessionID)
	stream.mu.Lock()
	toolMsgID := stream.lastToolMsgID
	pendingName := stream.lastToolName
	pendingInput := stream.lastToolInput
	stream.lastToolMsgID = ""
	stream.lastToolName = ""
	stream.lastToolInput = ""
	stream.mu.Unlock()

	if toolMsgID != "" {
		displayName := pendingName
		if toolName != "" {
			displayName = toolName
		}
		var body strings.Builder
		fmt.Fprintf(&body, "**工具:** `%s`", displayName)
		if pendingInput != "" && pendingInput != displayName {
			body.WriteString("\n```\n")
			body.WriteString(pendingInput)
			body.WriteString("\n```")
		}
		body.WriteString("\n\n**结果**\n")
		body.WriteString(resultBody)
		card := NewCard().Title("工具", "orange").Markdown(body.String()).Build()
		return a.patchMessage(ctx, toolMsgID, BuildCardJSON(card))
	}

	title := "工具结果"
	if toolName != "" {
		title = fmt.Sprintf("工具结果 · %s", toolName)
	}
	card := NewCard().Title(title, "green").Markdown(resultBody).Build()
	return a.sendCardToSession(ctx, msg.SessionID, msg.ChatID, card)
}

func formatToolResultMarkdown(content string) string {
	body := strings.TrimSpace(stripOuterCodeFence(content))
	if body == "" {
		return "_无输出_"
	}
	if strings.Contains(body, "```") {
		return PreprocessMarkdown(body)
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
	}
	stream.mu.Unlock()
	return a.upsertTaskProgressCard(ctx, msg.SessionID, msg.ChatID, false)
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
		return a.upsertTaskProgressCard(ctx, sessionID, chatID, true)
	}
	if responseMsgID != "" && strings.TrimSpace(summary) != "" {
		footer := responseText + "\n\n---\n_" + strings.TrimSpace(summary) + "_"
		card := NewCard().
			Title("回复", "green").
			Markdown(PreprocessMarkdown(footer)).
			Build()
		return a.patchMessage(ctx, responseMsgID, BuildCardJSON(card))
	}
	return nil
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

	card := NewCard().
		Title("回复", "green").
		Markdown(PreprocessMarkdown(content)).
		Build()
	cardJSON := BuildCardJSON(card)

	stream.mu.Lock()
	defer stream.mu.Unlock()

	if stream.responseMsgID == "" {
		msgID, err := a.sendCardReplyAndGetID(ctx, sessionID, chatID, cardJSON)
		if err != nil {
			return err
		}
		stream.responseMsgID = msgID
		return nil
	}
	return a.patchMessage(ctx, stream.responseMsgID, cardJSON)
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
