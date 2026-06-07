package adapters

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/devrix/devrix/internal/layers/communication/core"
	"github.com/devrix/devrix/internal/shared/types"
)

const (
	progressStyleLegacy     = "legacy"
	progressStyleCompact    = "compact"
	progressStyleCard       = "card"
	progressStyleStructured = "structured"
)

type progressItemKind string

const (
	progressKindThinking   progressItemKind = "thinking"
	progressKindToolCall   progressItemKind = "tool_call"
	progressKindToolResult progressItemKind = "tool_result"
	progressKindMilestone  progressItemKind = "milestone"
	progressKindInfo       progressItemKind = "info"
)

type progressItem struct {
	kind     progressItemKind
	text     string
	toolName string
	progress string
	task     string
}

type feishuSessionStream struct {
	mu            sync.Mutex
	progressMsgID string
	responseMsgID string
	items         []progressItem
	textBuffer    strings.Builder
	completed     bool
	toolCount     int
	summaries     []string
	progressPct   int
	taskName      string
}

func normalizeProgressStyle(style string) string {
	switch strings.ToLower(strings.TrimSpace(style)) {
	case progressStyleLegacy:
		return progressStyleLegacy
	case progressStyleCompact:
		return progressStyleCompact
	case progressStyleCard:
		return progressStyleCard
	default:
		return progressStyleStructured
	}
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
	switch a.progressStyle {
	case progressStyleLegacy:
		return a.sendLegacyProgressCard(ctx, msg)
	case progressStyleStructured:
		return a.handleStructuredProgressEvent(ctx, msg)
	default:
		return a.appendCoalescedProgress(ctx, msg)
	}
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
	text := strings.TrimSpace(msg.Content)
	if text == "" {
		text = "思考中..."
	}
	card := NewCard().
		Title("思考", "blue").
		Markdown(text).
		Build()
	return a.sendCardToSession(ctx, msg.SessionID, msg.ChatID, card)
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
	return a.sendCardToSession(ctx, msg.SessionID, msg.ChatID, card)
}

func (a *FeishuAdapter) sendStructuredToolResultCard(ctx context.Context, msg *types.OutboundMessage) error {
	toolName := strings.TrimSpace(msg.Metadata["tool_name"])
	body := stripOuterCodeFence(msg.Content)
	if !strings.Contains(body, "```") && body != "" {
		body = "```\n" + body + "\n```"
	}
	title := "工具结果"
	if toolName != "" {
		title = fmt.Sprintf("工具结果 · %s", toolName)
	}
	card := NewCard().Title(title, "green").Markdown(body).Build()
	return a.sendCardToSession(ctx, msg.SessionID, msg.ChatID, card)
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
	if completed {
		stream.completed = true
		if stream.progressPct < 100 {
			stream.progressPct = 100
		}
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

func (a *FeishuAdapter) appendCoalescedProgress(ctx context.Context, msg *types.OutboundMessage) error {
	stream := a.sessionStream(msg.SessionID)
	item := progressItemFromOutbound(msg)
	if item.kind == "" {
		return nil
	}

	stream.mu.Lock()
	stream.items = append(stream.items, item)
	stream.mu.Unlock()

	return a.upsertProgressCard(ctx, msg.SessionID, msg.ChatID, false)
}

func progressItemFromOutbound(msg *types.OutboundMessage) progressItem {
	eventType := msg.Metadata["event_type"]
	switch eventType {
	case "thinking":
		return progressItem{kind: progressKindThinking, text: strings.TrimSpace(msg.Content)}
	case "tool_call":
		tool := strings.TrimSpace(msg.Metadata["tool_name"])
		text := strings.TrimSpace(msg.Content)
		if text == "" && tool != "" {
			text = tool
		}
		return progressItem{kind: progressKindToolCall, text: text, toolName: tool}
	case "tool_result":
		return progressItem{
			kind:     progressKindToolResult,
			text:     stripOuterCodeFence(msg.Content),
			toolName: strings.TrimSpace(msg.Metadata["tool_name"]),
		}
	case "milestone_progress":
		return progressItem{
			kind:     progressKindMilestone,
			progress: strings.TrimSpace(msg.Metadata["progress"]),
			task:     strings.TrimSpace(msg.Metadata["task"]),
		}
	case "info":
		return progressItem{kind: progressKindInfo, text: strings.TrimSpace(msg.Content)}
	default:
		return progressItem{}
	}
}

func (a *FeishuAdapter) upsertProgressCard(ctx context.Context, sessionID, chatID string, completed bool) error {
	stream := a.sessionStream(sessionID)
	stream.mu.Lock()
	items := append([]progressItem(nil), stream.items...)
	stream.completed = completed || stream.completed
	stream.mu.Unlock()

	card := buildCoalescedProgressCard(items, a.progressStyle, completed)
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

func (a *FeishuAdapter) appendResponseText(ctx context.Context, sessionID, chatID, chunk string) error {
	if strings.TrimSpace(chunk) == "" {
		return nil
	}

	stream := a.sessionStream(sessionID)
	stream.mu.Lock()
	stream.textBuffer.WriteString(chunk)
	content := stream.textBuffer.String()
	stream.mu.Unlock()

	card := NewCard().Markdown(content).Build()
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

func buildCoalescedProgressCard(items []progressItem, style string, completed bool) *core.Card {
	builder := NewCard()
	color := "blue"
	title := "Devrix 处理中"
	if completed {
		color = "green"
		title = "Devrix 完成"
	}
	builder = builder.Title(title, color)

	if style == progressStyleCompact {
		var sections []string
		for _, item := range items {
			if line := renderProgressItemCompact(item); line != "" {
				sections = append(sections, line)
			}
		}
		if len(sections) == 0 {
			builder = builder.Markdown("_处理中…_")
		} else {
			builder = builder.Markdown(strings.Join(sections, "\n\n"))
		}
		return builder.Build()
	}

	for _, item := range items {
		if block := renderProgressItemCard(item); block != "" {
			builder = builder.Markdown(block)
		}
	}
	if len(items) == 0 {
		builder = builder.Markdown("_处理中…_")
	}
	return builder.Build()
}

func renderProgressItemCompact(item progressItem) string {
	switch item.kind {
	case progressKindThinking:
		return "💭 " + item.text
	case progressKindToolCall:
		if item.toolName != "" {
			return fmt.Sprintf("🔧 `%s`", item.toolName)
		}
		return "🔧 " + item.text
	case progressKindToolResult:
		if item.toolName != "" {
			return fmt.Sprintf("✅ `%s`\n%s", item.toolName, item.text)
		}
		return "✅ " + item.text
	case progressKindMilestone:
		if item.task != "" {
			return fmt.Sprintf("📊 %s — %s", item.progress, item.task)
		}
		return "📊 " + item.progress
	case progressKindInfo:
		return item.text
	default:
		return ""
	}
}

func renderProgressItemCard(item progressItem) string {
	switch item.kind {
	case progressKindThinking:
		return "**思考**\n" + item.text
	case progressKindToolCall:
		if item.toolName != "" {
			return fmt.Sprintf("**工具调用**\n`%s`", item.toolName)
		}
		return "**工具调用**\n" + item.text
	case progressKindToolResult:
		body := item.text
		if !strings.Contains(body, "```") && body != "" {
			body = "```\n" + body + "\n```"
		}
		if item.toolName != "" {
			return fmt.Sprintf("**工具结果** · `%s`\n%s", item.toolName, body)
		}
		return "**工具结果**\n" + body
	case progressKindMilestone:
		lines := make([]string, 0, 2)
		if item.progress != "" {
			lines = append(lines, "**进度:** "+item.progress)
		}
		if item.task != "" {
			lines = append(lines, "**任务:** "+item.task)
		}
		if len(lines) == 0 {
			return ""
		}
		return "**任务进度**\n" + strings.Join(lines, "\n")
	case progressKindInfo:
		return item.text
	default:
		return ""
	}
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
	if strings.TrimSpace(lines[0]) == "```" {
		start = 1
	} else if strings.HasPrefix(lines[0], "```") {
		start = 1
	}
	end := len(lines)
	if end > start && strings.TrimSpace(lines[end-1]) == "```" {
		end--
	}
	if end <= start {
		return trimmed
	}
	return strings.TrimSpace(strings.Join(lines[start:end], "\n"))
}

func (a *FeishuAdapter) sendLegacyProgressCard(ctx context.Context, msg *types.OutboundMessage) error {
	eventType := msg.Metadata["event_type"]
	content := msg.Content

	switch eventType {
	case "thinking":
		card := NewCard().Title(content, "blue").Build()
		return a.sendCardToSession(ctx, msg.SessionID, msg.ChatID, card)
	case "tool_call":
		tool := msg.Metadata["tool_name"]
		card := NewCard().
			Title("工具调用", "orange").
			Markdown(fmt.Sprintf("**工具:** `%s`", tool)).
			Build()
		return a.sendCardToSession(ctx, msg.SessionID, msg.ChatID, card)
	case "tool_result":
		body := stripOuterCodeFence(content)
		if !strings.Contains(body, "```") && body != "" {
			body = "```\n" + body + "\n```"
		}
		card := NewCard().Title("工具执行结果", "green").Markdown(body).Build()
		return a.sendCardToSession(ctx, msg.SessionID, msg.ChatID, card)
	case "milestone_progress":
		card := NewCard().
			Title("任务进度", "purple").
			Markdown("**进度:** " + msg.Metadata["progress"]).
			Markdown("**任务:** " + msg.Metadata["task"]).
			Build()
		return a.sendCardToSession(ctx, msg.SessionID, msg.ChatID, card)
	default:
		return nil
	}
}
