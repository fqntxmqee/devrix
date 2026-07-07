// Package sessionorchestrator — PR-C error sentinels (DM-20260707-001).
//
// These sentinels live on the 72xx audit-code series that
// wavescheduler/dag_executor_errors.go established (7210-7213). PR-C adds
// three new sentinels for the streaming-emit + Learn per-segment errors,
// all using sharederrors.SentinelError for cross-package error reporting
// consistency (audit + IM error contract).
//
// Code allocation (reviews/pr-c-codex-consensus-2026-07-07.md Q9
// ADOPT-WITH-CHANGE — skip 7214 because dedup hit is a debug log, not
// an error; reserve 7214 for a future real error sentinel):
//
//	ORCH_FEISHU_STREAMING_UPDATE_FAILED_7215
//	ORCH_FEISHU_STREAMING_CARD_NOT_FOUND_7216
//	ORCH_LEARN_PER_SEGMENT_FAILED_7217
//
// 7214 reserved (future). PR-C ships 3 sentinels (7215-7217).
package sessionorchestrator

import (
	"errors"
	"fmt"

	sharederrors "github.com/devrix/devrix/internal/shared/errors"
)

// Sentinel errors (DM-20260707-001 PR-C Q9 — codex + cursor ACCEPT-WITH-CHANGE).
var (
	ErrFeishuStreamingUpdateFailed = errors.New("sessionorchestrator: Feishu streaming card update failed")
	ErrFeishuStreamingCardNotFound = errors.New("sessionorchestrator: Feishu streaming partial card not found or expired")
	ErrLearnPerSegmentFailed       = errors.New("sessionorchestrator: per-segment Learn call failed")
)

// NewFeishuStreamingUpdateFailedError wraps ORCH_FEISHU_STREAMING_UPDATE_FAILED_7215.
// Surfaced when the Feishu cardkit UpdateCard call returns a transient
// error after retries — caller may opt to drop the partial or fall back
// to a fresh CreateCard on the next emit.
func NewFeishuStreamingUpdateFailedError(chatID, idempotencyKey string, cause error) *sharederrors.SentinelError {
	return sharederrors.WithCode(
		"ORCH_FEISHU_STREAMING_UPDATE_FAILED_7215",
		fmt.Sprintf("Feishu streaming card update failed for chat=%s key=%s", chatID, idempotencyKey),
		fmt.Errorf("%w: chatID=%q key=%q: %v", ErrFeishuStreamingUpdateFailed, chatID, idempotencyKey, cause),
	)
}

// NewFeishuStreamingCardNotFoundError wraps ORCH_FEISHU_STREAMING_CARD_NOT_FOUND_7216.
// Surfaced when the IM-side dedup table still holds the idempotency key
// but the underlying cardkit message has expired or been deleted (e.g.
// chat history TTL exceeded). Caller may drop the partial and switch to
// a fresh CreateCard on EmitFinal.
func NewFeishuStreamingCardNotFoundError(chatID, idempotencyKey string) *sharederrors.SentinelError {
	return sharederrors.WithCode(
		"ORCH_FEISHU_STREAMING_CARD_NOT_FOUND_7216",
		fmt.Sprintf("Feishu streaming partial card not found for chat=%s key=%s (expired or deleted)", chatID, idempotencyKey),
		fmt.Errorf("%w: chatID=%q key=%q", ErrFeishuStreamingCardNotFound, chatID, idempotencyKey),
	)
}

// NewLearnPerSegmentFailedError wraps ORCH_LEARN_PER_SEGMENT_FAILED_7217.
// Surfaced when a per-segment Learn call returns an error that should be
// surfaced to the audit log. Per PR-C Q6, per-child Learn failures are
// non-blocking — this sentinel marks the error path so dashboards can
// graph the failure rate without aborting the rollup.
func NewLearnPerSegmentFailedError(sessionID, segmentID string, cause error) *sharederrors.SentinelError {
	return sharederrors.WithCode(
		"ORCH_LEARN_PER_SEGMENT_FAILED_7217",
		fmt.Sprintf("per-segment Learn failed for session=%s segment=%s", sessionID, segmentID),
		fmt.Errorf("%w: sessionID=%q segmentID=%q: %v", ErrLearnPerSegmentFailed, sessionID, segmentID, cause),
	)
}

// IsFeishuStreamingUpdateFailed reports whether err is (or wraps) the 7215
// streaming-update-failed sentinel.
func IsFeishuStreamingUpdateFailed(err error) bool {
	return err != nil && errors.Is(err, ErrFeishuStreamingUpdateFailed)
}

// IsFeishuStreamingCardNotFound reports whether err is (or wraps) the 7216
// streaming-card-not-found sentinel.
func IsFeishuStreamingCardNotFound(err error) bool {
	return err != nil && errors.Is(err, ErrFeishuStreamingCardNotFound)
}

// IsLearnPerSegmentFailed reports whether err is (or wraps) the 7217
// learn-per-segment-failed sentinel.
func IsLearnPerSegmentFailed(err error) bool {
	return err != nil && errors.Is(err, ErrLearnPerSegmentFailed)
}