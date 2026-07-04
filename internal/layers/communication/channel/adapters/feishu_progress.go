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

	"github.com/devrix/devrix/internal/layers/communication/kernel"
	"github.com/devrix/devrix/internal/layers/orchestration/hardening"
	"github.com/devrix/devrix/internal/shared/textutil"
	"github.com/devrix/devrix/internal/shared/types"
)

const progressStyleStructured = "structured"

// metadataKeyExitReason is the metadata key set by D7 orchestrator on
// the final `complete` EngineEvent. The Feishu adapter reads it to
// decide whether to render a "✅ 任务完成" green card (natural finish
// or any non-fail reason) or a "❌ 任务失败 (verifier_fail)" red card
// (verifier_fail / verifier_abstain / system_anomaly / intent_only_
// aborted). Mirrors `metadataKeyExitReason` in
// orchestration/sessionorchestrator/turn_orchestrator.go.
const metadataKeyExitReason = "exit_reason"

// failedExitReasons lists the exit reasons that should be rendered as
// a red "任务失败" card on the user's chat. Hotfix 2026-06-27
// (sess_1782541795374_7000): without this branching the adapter
// showed "✅ 任务已完成" for verifier_fail cases, hiding the actual
// failure status from the user.
var failedExitReasons = map[string]struct{}{
	"verifier_fail":    {},
	"verifier_abstain": {},
	"system_anomaly":   {},
	"intent_only_aborted": {},
}

// dedupReplayMinBufferRunes / dedupReplayMinChunkRunes / dedupReplayMinOverlapRunes
// bound the LLM-stream-replay dedup. When the LLM re-emits a previously
// streamed prefix from scratch (a minimax M2.7 streaming artifact: the
// model occasionally regenerates the same opening narration it just
// finished), the second copy must not be appended to the feishu reply
// card. The thresholds are intentionally conservative: a duplicate must
// already have a non-trivial buffer AND the new chunk must contain at
// least 30 runes of an exact substring that appears in the recent
// buffer. Short chunks and short buffers are passed through unchanged
// to avoid false positives on legitimate restarts.
const (
	dedupReplayMinBufferRunes = 100
	dedupReplayMinChunkRunes = 50
	dedupReplayMinOverlapRunes = 30
	dedupReplayMaxPrefixRunes = 200
	dedupReplayBufferTailRunes = 4000
)

// stripTrailingSummary removes a trailing summary segment from content
// when content ends with summary (with whitespace tolerance). The
// streaming reply / response card and the standalone "任务总结" card
// would otherwise duplicate the LLM's last turn text: the LLM emits the
// summary as ordinary text events (which land in textBuffer), and
// resolveFinalText also packs lastTurnText into the summary card. Strip
// the trailing summary here so the streaming card shows only the working
// report, while the summary card owns the conclusion.
//
// Returns content unchanged when:
//   - summary is empty / whitespace-only,
//   - content does not end with summary (modulo trailing whitespace).
//
// This is the D1-side half of the DM-20260621-008 fix: D7 still emits
// both lastTurnText (as text) and the summary, but D1 deduplicates at
// the IM boundary instead of showing the user the same paragraph twice.
func stripTrailingSummary(content, summary string) string {
	contentRunes := []rune(content)
	summaryRunes := []rune(summary)
	if len(summaryRunes) == 0 {
		return content
	}
	// Trim trailing whitespace from content before the suffix check, so
	// the LLM streaming a summary that ends in a newline does not
	// defeat the match.
	tail := len(contentRunes)
	for tail > 0 && (contentRunes[tail-1] == ' ' || contentRunes[tail-1] == '\t' || contentRunes[tail-1] == '\n' || contentRunes[tail-1] == '\r') {
		tail--
	}
	if tail < len(summaryRunes) {
		return content
	}
	start := tail - len(summaryRunes)
	for i := 0; i < len(summaryRunes); i++ {
		if contentRunes[start+i] != summaryRunes[i] {
			return content
		}
	}
	// Match — strip the trailing summary plus any whitespace after the
	// prefix that bounded the summary (so the streaming card does not
	// end on a stray newline before the completion marker).
	trimmed := strings.TrimRight(string(contentRunes[:start]), " \t\n\r")
	return trimmed
}

// detectDuplicateReplay returns true when chunk's prefix is already
// present in buffer (within the last dedupReplayBufferTailRunes runes),
// indicating the LLM has started re-emitting text it just streamed.
// chunk must be at least dedupReplayMinChunkRunes runes, buffer at
// least dedupReplayMinBufferRunes runes, and the matching prefix at
// least dedupReplayMinOverlapRunes runes for the signal to fire.
func detectDuplicateReplay(buffer, chunk string) bool {
	if utf8.RuneCountInString(buffer) < dedupReplayMinBufferRunes {
		return false
	}
	chunkRunes := []rune(chunk)
	if len(chunkRunes) < dedupReplayMinChunkRunes {
		return false
	}
	maxPrefix := len(chunkRunes)
	if maxPrefix > dedupReplayMaxPrefixRunes {
		maxPrefix = dedupReplayMaxPrefixRunes
	}
	// Only scan the tail of the buffer — the duplicate, if any, is the
	// LLM's most recent narration, not something from many turns ago.
	bufferRunes := []rune(buffer)
	searchStart := len(bufferRunes) - dedupReplayBufferTailRunes
	if searchStart < 0 {
		searchStart = 0
	}
	tail := string(bufferRunes[searchStart:])
	for n := maxPrefix; n >= dedupReplayMinOverlapRunes; n-- {
		if strings.Contains(tail, string(chunkRunes[:n])) {
			return true
		}
	}
	return false
}

// DM-20260626-009 follow-up: dedupRepeatedText alias removed. The
// textutil.DedupRepeatedText it forwarded to is deleted (see
// internal/shared/textutil), and detectDuplicateReplay at streaming time
// is the sole surviving dedup layer in this adapter.

type toolCallEntry struct {
	name   string
	input  string
	result string // populated when show_tool_results is enabled
}

type feishuSessionStream struct {
	mu                sync.Mutex
	progressMsgID     string
	responseMsgID     string
	thinkingMsgID     string
	agentOutputMsgID  string
	toolsMsgID        string
	toolCalls         []toolCallEntry
	textBuffer        strings.Builder
	thinkingBuffer    strings.Builder
	agentOutputBuffer strings.Builder
	summaries         []string
	progressPct       int
	taskName          string

	replyCardID        string
	cardkitEnabled     bool
	cardkitSequence    int
	lastStreamPutAt    time.Time
	lastStreamPutRunes int
	findingsJSONPlaceholderShown bool
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
		chunk := textutil.StripPriorOutputSummary(msg.Content)
		chunk = textutil.StripMiniMaxStreamMarkers(chunk)
		// minimax M2.7 streaming replay dedup (DM-20260621-006). The LLM
		// occasionally re-emits a long prefix of its own thinking, which
		// would render as the same paragraph repeated 2-3 times on the
		// thinking card. detectDuplicateReplay drops the replay at the
		// source so the thinkingBuffer never carries it.
		if detectDuplicateReplay(stream.thinkingBuffer.String(), chunk) {
			existing := stream.thinkingBuffer.String()
			stream.mu.Unlock()
			slog.Debug("feishu: dropped LLM-stream replay chunk in thinking",
				"session", slog.String("sessionID", msg.SessionID),
				"bufferRunes", utf8.RuneCountInString(existing),
				"chunkRunes", utf8.RuneCountInString(chunk),
			)
			return a.patchThinkingCard(ctx, stream, msg.SessionID, msg.ChatID)
		}
		stream.thinkingBuffer.WriteString(chunk)
	}
	stream.mu.Unlock()

	return a.patchThinkingCard(ctx, stream, msg.SessionID, msg.ChatID)
}

// patchThinkingCard renders the current thinkingBuffer (after the
// streaming-time StripToolCallXML only) and sends/patches the thinking
// card. Used by sendStructuredThinkingCard on every event (with the
// dedup logic upstream) so a single render path owns the create-or-patch
// branching.
//
// DM-20260626-009 follow-up: post-hoc dedup via textutil.DedupRepeatedText
// was removed. The LCP-based dedup false-positives on natural Chinese
// repetition (e.g. "先看一下代码。先看一下代码的结构。" lost its second
// occurrence), and detectDuplicateReplay at the streaming-time layer
// already drops the genuine M2.7 replay pattern at the source. Render
// the buffer verbatim so legitimate text is never silently truncated.
func (a *FeishuAdapter) patchThinkingCard(ctx context.Context, stream *feishuSessionStream, sessionID, chatID string) error {
	stream.mu.Lock()
	text := textutil.StripToolCallXML(stream.thinkingBuffer.String())
	text = strings.TrimSpace(text)
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
		msgID, err := a.sendCardReplyAndGetID(ctx, sessionID, chatID, cardJSON)
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
	return a.upsertTaskProgressCard(ctx, msg.SessionID, msg.ChatID, false, "")
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

func (a *FeishuAdapter) upsertTaskProgressCard(ctx context.Context, sessionID, chatID string, completed bool, exitReason string) error {
	stream := a.sessionStream(sessionID)
	stream.mu.Lock()
	if completed && stream.progressPct < 100 {
		stream.progressPct = 100
	}
	card := buildTaskProgressCard(stream, completed, exitReason)
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

func (a *FeishuAdapter) finalizeStructuredSession(ctx context.Context, sessionID, chatID, summary, exitReason string, taskIncomplete bool) error {
	stream := a.sessionStream(sessionID)
	stream.mu.Lock()
	hasTaskCard := stream.progressMsgID != "" || stream.taskName != "" || stream.progressPct > 0
	responseMsgID := stream.responseMsgID
	responseText := stream.textBuffer.String()
	trimmedSummary := strings.TrimSpace(summary)
	taskFailed := isFailedExitReason(exitReason)
	// Hotfix 2026-07-01 (sess_1782826968112_7000 / sess_1782885908460_4000):
	// taskIncomplete (D1 EmitComplete flagged BOTH summary AND final Content
	// as transitional / inconclusive) must override taskFailed for rendering
	// purposes — the user-visible status is "任务未完成", distinct from the
	// failed-state "任务失败" red marker.
	taskIncompleteForRender := taskIncomplete
	// DM-20260621-008: do NOT append trimmedSummary to stream.summaries.
	// stream.summaries is rendered on the "任务进度" / "任务完成" progress
	// card (buildTaskProgressCard), and that card already coexists with
	// the standalone "任务总结" card sent below. Pushing trimmedSummary in
	// here used to duplicate the LLM's final paragraph on the progress
	// card (the user sees the same "需要我针对其中某条..." line twice —
	// once on the green "任务完成" card, once on the blue "任务总结"
	// card). The progress card now lists only streaming-time worker /
	// info summaries; the conclusion lives exclusively on the "任务总结"
	// card.
	stream.mu.Unlock()

	if hasTaskCard {
		// Hotfix 2026-07-01: when taskIncomplete, the progress card must NOT
		// show "任务完成" green check — the user already saw a fragmentary
		// "任务未完成" indicator from the summary card. Pass !taskIncomplete
		// as the "completed" flag so the progress card renders the yellow
		// "任务未完成" state instead.
		progressCompleted := !taskFailed && !taskIncompleteForRender
		if err := a.upsertTaskProgressCard(ctx, sessionID, chatID, progressCompleted, exitReason); err != nil {
			return err
		}
	}
	stream.mu.Lock()
	cardkitActive := stream.cardkitEnabled && stream.replyCardID != ""
	// DM-20260626-009 follow-up: post-hoc dedup removed. detectDuplicateReplay
	// at streaming time already drops the M2.7 replay pattern at the source.
	// Stripping tool-call XML still runs so the card body never carries
	// raw <function_calls> blocks (that's an LLM protocol leak, not dedup).
	responseText = textutil.StripFindingsJSONBlocks(textutil.StripToolCallXML(responseText))
	stream.mu.Unlock()
	if cardkitActive {
		// Pass trimmedSummary so finalizeReplyCardStreaming can strip it
		// from the streaming card's tail (DM-20260621-008). The LLM emits
		// the summary as ordinary text events, so textBuffer already ends
		// with the same paragraph; without the strip, the user sees the
		// summary twice — once at the tail of the streaming reply card,
		// once in the standalone 任务总结 card.
		if err := a.finalizeReplyCardStreaming(ctx, stream, trimmedSummary, sessionID, exitReason, taskIncompleteForRender); err != nil {
			return err
		}
	} else if responseMsgID != "" {
		// Non-cardkit path: patch the response card with the LLM's report
		// (with trailing summary stripped) plus a completion footer.
		//
		// DM-20260621-008: when the LLM emits the summary as text events
		// (the common minimax M2.7 pattern), the response card already
		// carries report + summary on its tail. Strip the summary so the
		// user sees the report only once on this card; the standalone
		// 任务总结 card still carries the summary as a separate message.
		//
		// When trimmedSummary is empty (e.g. max-turns reached mid-tool-
		// call while the LLM was still looping), the response card is
		// still patched with a minimal "✅ 任务已完成" footer so the user
		// never sees a dangling partial card with no closure.
		//
		// Hotfix 2026-06-27: when exitReason is in failedExitReasons
		// (verifier_fail / verifier_abstain / system_anomaly /
		// intent_only_aborted) the footer reads "❌ 任务失败" + the
		// reason, so the user sees the actual outcome rather than a
		// misleading "✅ 任务已完成".
		stripped := stripTrailingSummary(responseText, trimmedSummary)
		footer := buildCompletionFooter(stripped, taskFailed, exitReason, taskIncompleteForRender)
		card := NewCard().
			Markdown(footer).
			Build()
		if err := a.patchMessage(ctx, responseMsgID, BuildCardJSON(card)); err != nil {
			return err
		}
	}
	if trimmedSummary != "" {
		return a.sendSummaryCard(ctx, sessionID, chatID, trimmedSummary, taskIncompleteForRender)
	}
	return nil
}

// isFailedExitReason reports whether the given exit_reason should be
// rendered as a red "❌ 任务失败" card on the user's chat. Hotfix
// 2026-06-27 (sess_1782541795374_7000).
func isFailedExitReason(reason string) bool {
	_, ok := failedExitReasons[reason]
	return ok
}

// buildCompletionFooter renders the trailing footer line on the
// non-cardkit path's reply card. Hotfix 2026-06-27: when the task
// failed (verifier_fail / verifier_abstain / system_anomaly /
// intent_only_aborted) the footer reads "❌ 任务失败" plus the exit
// reason, so the user sees the actual outcome rather than a
// misleading "✅ 任务已完成".
func buildCompletionFooter(stripped string, taskFailed bool, exitReason string, taskIncomplete bool) string {
	var marker string
	// Hotfix 2026-07-01 (sess_1782885908460_4000): taskIncomplete takes
	// priority over taskFailed because they reflect distinct outcomes —
	// taskIncomplete = LLM never produced a deliverable, taskFailed =
	// verifier rejected an existing deliverable.
	if taskIncomplete {
		marker = "_❌ 任务未完成_"
	} else if taskFailed {
		if exitReason != "" {
			marker = fmt.Sprintf("_❌ 任务失败（%s）_", exitReason)
		} else {
			marker = "_❌ 任务失败_"
		}
	} else {
		marker = "_✅ 任务已完成_"
	}
	if strings.TrimSpace(stripped) != "" {
		return stripped + "\n\n---\n" + marker
	}
	return marker
}

// sendSummaryCard delivers the D7 final summary as a standalone "任务总结"
// card so it appears as a separate message in the user's chat history instead
// of being glued onto the previous reply / progress card with a `---`
// separator. The card honors the standard precheck + plain-text fallback
// path via sendCardToSession.
//
// Hotfix 2026-07-01 (sess_1782826968112_7000): when taskIncomplete is true
// the LLM produced no valid conclusion (D1 EmitComplete detected BOTH
// summary AND final Content classified as bad — transitional text leaked to
// the user). Surface that with a red "❌ 任务未完成" title instead of the
// blue "任务总结" header, so the user does not see a misleading
// "✅ 任务已完成" green check + a fragmentary "任务总结" card at the same
// time. body remains the TaskIncompleteMessage that conclusion.EmitComplete
// already placed in the OutboundMessage.Content slot.
func (a *FeishuAdapter) sendSummaryCard(ctx context.Context, sessionID, chatID, summary string, taskIncomplete bool) error {
	// Strip D2 context-budget fold markers so the LLM's echo of its own
	// prior <prior-output-summary> blocks (which the D7 lastTurnText may
	// carry over from a long tool loop) does not leak into the 任务总结 card.
	summary = textutil.StripPriorOutputSummary(summary)
	if strings.TrimSpace(summary) == "" {
		// Nothing left after stripping — fall through; the caller will
		// patch the reply card with a "✅ 任务已完成" footer instead.
		return nil
	}
	cardBuilder := NewCard()
	if taskIncomplete {
		cardBuilder = cardBuilder.Title("❌ 任务未完成", "red").Markdown(summary)
	} else {
		cardBuilder = cardBuilder.Title("任务总结", "blue").Markdown(summary)
	}
	card := cardBuilder.Build()
	if err := a.sendCardToSession(ctx, sessionID, chatID, card); err != nil {
		return fmt.Errorf("feishu: send summary card failed: %w", err)
	}
	return nil
}

func (a *FeishuAdapter) finalizeReplyCardStreaming(ctx context.Context, stream *feishuSessionStream, summary, sessionID, exitReason string, taskIncomplete bool) error {
	stream.mu.Lock()
	defer stream.mu.Unlock()

	if !stream.cardkitEnabled || stream.replyCardID == "" {
		return nil
	}

	content := stream.textBuffer.String()
	// DM-20260626-009 follow-up: post-hoc dedup removed. detectDuplicateReplay
	// at streaming time already drops the M2.7 replay pattern at the source;
	// the LCP-based dedup that lived here was both fragile (false-positives
	// on natural Chinese repetition) and redundant with the streaming-time
	// layer. Stripping tool-call XML + prior-output-summary markers still
	// runs — those are LLM protocol leaks, not dedup candidates.
	content = textutil.StripToolCallXML(content)
	content = textutil.StripPriorOutputSummary(content)
	// DM-20260621-008: the LLM emits the D7 final summary as ordinary
	// text events, so textBuffer already ends with the same paragraph
	// that the standalone 任务总结 card will carry. Strip the trailing
	// summary here so the streaming card shows the report only once.
	// When summary is empty (e.g. max-turns reached mid-tool-call) this
	// is a no-op and the streaming card keeps the full report.
	content = stripTrailingSummary(content, summary)
	if strings.TrimSpace(content) != "" {
		// Minimal completion marker on the streaming card so the user
		// sees the task finished even when the D7 orchestrator did not
		// produce a final summary.
		//
		// Hotfix 2026-06-27: when the task failed, surface "❌ 任务
		// 失败" + the exit reason rather than the misleading green
		// "✅ 任务已完成" marker. Mirrors buildCompletionFooter on the
		// non-cardkit path.
		// Hotfix 2026-07-01 (sess_1782885908460_4000): when taskIncomplete
		// (D1 EmitComplete detected BOTH summary AND final Content as
		// transitional / inconclusive) surface "❌ 任务未完成" instead of
		// the misleading green "✅ 任务已完成" marker. taskIncomplete
		// takes priority over taskFailed because they reflect distinct
		// outcomes: taskFailed = verifier rejected the deliverable;
		// taskIncomplete = LLM never produced a deliverable.
		if taskIncomplete {
			content += "\n\n---\n_❌ 任务未完成_"
		} else if isFailedExitReason(exitReason) {
			if exitReason != "" {
				content += "\n\n---\n_❌ 任务失败（" + exitReason + "）_"
			} else {
				content += "\n\n---\n_❌ 任务失败_"
			}
		} else {
			content += "\n\n---\n_✅ 任务已完成_"
		}
	}

	stream.cardkitSequence++
	updateMethod := "stream_final+update_card"
	if err := a.cardkit.StreamElementContent(ctx, stream.replyCardID, replyTextElementID, content, stream.cardkitSequence); err != nil {
		if errors.Is(err, ErrFeishuCardRateLimited) {
			// rate-limited — skip final stream, try updateCard directly
			updateMethod = "rate_limited+update_card"
		} else if errors.Is(err, ErrFeishuCardStreamClosed) {
			// Card's streaming channel was closed by Feishu (idle timeout
			// or prior finalization) but the card itself still exists.
			// Previously this branch returned nil and left the card stale —
			// the user saw a partial / no-op reply when the deep-review
			// LLM took >30min. Fall through to UpdateCard so the full
			// textBuffer content still reaches the user.
			slog.Warn("feishu: card stream closed at finalize, falling back to UpdateCard",
				"session", slog.String("cardID", stream.replyCardID))
			updateMethod = "stream_closed+update_card"
		} else {
			return err
		}
	}

	stream.cardkitSequence++
	finalJSON := BuildStreamingReplyCardJSON(content, false)
	// DM-20260629-001 PR-6 t-span-coverage (T42): emit the
	// D7_Feishu_Card_Render span so D7→D1 cross-domain traces show the
	// final card lifecycle event. lastVerdict / lastExitReason are
	// empty at this layer (the D7 verdict pipeline is not directly
	// visible from finalizeReplyCardStreaming); a follow-up change can
	// plumb the ProcessAutoClose output through
	// feishuSessionStream.lastVerdict / lastExitReason fields.
	endCardRender := hardening.EmitFeishuCardRender(
		ctx,
		sessionID,
		"final",
		updateMethod,
		"",
		"",
	)
	if err := a.cardkit.UpdateCard(ctx, stream.replyCardID, finalJSON, stream.cardkitSequence); err != nil {
		endCardRender(err)
		return err
	}
	endCardRender(nil)
	return nil
}

func buildTaskProgressCard(stream *feishuSessionStream, completed bool, exitReason string) *kernel.Card {
	taskFailed := isFailedExitReason(exitReason)
	color := "purple"
	title := "任务进度"
	if taskFailed {
		color = "red"
		title = "任务失败"
	} else if completed {
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
	if taskFailed {
		// Hotfix 2026-06-27 (sess_1782541795374_7000): render a red
		// "❌ 失败" marker + exit reason so the user sees the actual
		// outcome rather than the misleading green "✅ 已完成".
		if exitReason != "" {
			builder = builder.Markdown(fmt.Sprintf("❌ **失败（%s）**", exitReason))
		} else {
			builder = builder.Markdown("❌ **失败**")
		}
	} else if completed {
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
	chunk = textutil.StripAssistantInternalMarkers(chunk)
	if strings.TrimSpace(chunk) == "" {
		return nil
	}
	if textutil.LooksLikeFindingsJSONStream(chunk) {
		stream := a.sessionStream(sessionID)
		stream.mu.Lock()
		defer stream.mu.Unlock()
		return a.patchFindingsJSONPlaceholderLocked(ctx, stream, sessionID, chatID)
	}

	stream := a.sessionStream(sessionID)
	stream.mu.Lock()
	if detectDuplicateReplay(stream.textBuffer.String(), chunk) {
		// The LLM (notably minimax M2.7) occasionally re-emits a long
		// prefix of text it just streamed. Appending the duplicate
		// causes the feishu reply card to show the user's report
		// twice. Drop the replay at the source so neither the live
		// stream nor the final UpdateCard carries the duplicate.
		existing := stream.textBuffer.String()
		stream.mu.Unlock()
		slog.Debug("feishu: dropped LLM-stream replay chunk",
			"session", slog.String("sessionID", sessionID),
			"bufferRunes", utf8.RuneCountInString(existing),
			"chunkRunes", utf8.RuneCountInString(chunk),
		)
		return nil
	}
	// DM-20260625-007 → DM-20260626-009 follow-up: chunk-self dedup and
	// buffer-self dedup were removed. detectDuplicateReplay at this same
	// site already drops the genuine M2.7 cross-chunk prefix replay at
	// the source, and the LCP-based dedup that lived here was false-
	// positive prone on natural Chinese repetition. The chunk now flows
	// into textBuffer verbatim so the user sees legitimate text intact.
	if strings.TrimSpace(chunk) == "" {
		stream.mu.Unlock()
		return nil
	}
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

func (a *FeishuAdapter) patchFindingsJSONPlaceholderLocked(ctx context.Context, stream *feishuSessionStream, sessionID, chatID string) error {
	if stream.findingsJSONPlaceholderShown {
		return nil
	}
	stream.findingsJSONPlaceholderShown = true
	if stream.textBuffer.Len() > 0 && !strings.HasSuffix(stream.textBuffer.String(), "\n") {
		stream.textBuffer.WriteString("\n")
	}
	stream.textBuffer.WriteString("⏳ 正在整理 review 结论…")
	content := stream.textBuffer.String()
	if !a.streamingEnabled {
		return a.patchResponseCard(ctx, sessionID, chatID, stream, content)
	}
	started := stream.responseMsgID != "" || stream.cardkitEnabled
	if !started {
		return a.startStreamingReplyCard(ctx, sessionID, chatID, stream, content)
	}
	if stream.cardkitEnabled && stream.replyCardID != "" {
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
			// Do NOT decrement cardkitSequence — another goroutine may
			// have already incremented past it, causing sequence inversion.
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
	chunk := textutil.StripAssistantInternalMarkers(msg.Content)
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
