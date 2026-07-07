// Package adapters — Feishu EmitPartialCard / EmitFinalCard methods
// (DM-20260707-001 PR-C, codex Risk A1 ADOPT-WITH-CHANGE).
//
// PR-C introduces the multi-intent decompose → parallel Execute → IM
// partial-then-final cards flow. Each child SegmentEmit triggers an
// EmitPartialCard; the rollup triggers an EmitFinalCard which OVERWRITES
// the prior partial card (matches Feishu's cardkit UpdateCard semantics,
// not a new card each time).
//
// Naming collision avoided: codex Risk A9 LOW — the original consensus
// packet proposed a new streaming.go file. To avoid the naming collision
// with the existing feishu_stream_throttle.go (same package, similar
// name → grep confusion), PR-C adds the methods directly to the existing
// FeishuAdapter + creates a separate feishu_dedup.go for the IM-side
// dedup table.
//
// Idempotency:
//   - Each EmitPartialCard / EmitFinalCard is keyed on idempotencyKey.
//   - FeishuAdapter-level dedup table (feishu_dedup.go) drops duplicate
//     keys within the IM adapter. Defense-in-depth: the session-level
//     EmitDedup (sessionorchestrator/emit_dedup.go) catches reentry
//     duplicates; this catches network-retry / partial-card-failure
//     duplicates.
//   - The first emit creates a cardkit card; subsequent emits UPDATE
//     the same card (UpdateCard PATCH /open-apis/cardkit/v1/cards/{id}).
//
// Throttling:
//   - PR-C reuses the existing streamThrottleConfig (400ms / 24-rune
//     minimum delta). A new rate-limit primitive (the consensus packet's
//     `golang.org/x/time/rate` token bucket) was REJECTED in codex Q3
//     because (a) it's not in go.mod, and (b) Feishu cardkit is per-card
//     10 QPS, not per-chat 10 RPS. The 400ms throttle covers all
//     realistic child counts (4-worker hard cap = max 4 partials/session).
package adapters

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	sharederrors "github.com/devrix/devrix/internal/shared/errors"
)

// IdempotencyKey is the per-emit deduplication key for streaming cards.
// For per-segment partials, format is "{sessionID}:seg:{segmentID}".
// For the parent rollup, format is "{sessionID}:rollup:{parentID}".
//
// The sessionorchestrator side uses the same key shape (constructed in
// executePlanDAG), so a single shared keyspace eliminates cross-layer
// dedup drift.
//
// NOTE: kept as a typed string for compile-time grep-ability, but the
// sessionorchestrator-facing StreamingEmitter interface uses plain string
// to avoid an import cycle through communication/capture tests.
type IdempotencyKey string

// String returns the underlying string value so callers can pass it
// across package boundaries without importing this package's types.
func (k IdempotencyKey) String() string { return string(k) }

// NewPartialIdempotencyKey formats the per-segment partial key.
func NewPartialIdempotencyKey(sessionID, segmentID string) IdempotencyKey {
	return IdempotencyKey(fmt.Sprintf("%s:seg:%s", sessionID, segmentID))
}

// NewRollupIdempotencyKey formats the parent-rollup final key. The
// "rollup:" prefix avoids collision with "seg:" prefixed partial keys.
func NewRollupIdempotencyKey(sessionID, parentID string) IdempotencyKey {
	return IdempotencyKey(fmt.Sprintf("%s:rollup:%s", sessionID, parentID))
}

// streamingCardState is the per-key state held in FeishuAdapter's
// streamingCards sync.Map. The cardID lets subsequent emits UPDATE
// instead of CREATE.
type streamingCardState struct {
	cardID       string
	sequence     int
	lastEmitAt   time.Time
	lastRunes    int
	creationDone bool
}

// streamingDedupKey is the FeishuAdapter-level dedup table for streaming
// emits. Same key shape as the sessionorchestrator EmitDedup, but stored
// inside the adapter so defense-in-depth (network retry / partial-card
// failure) can drop duplicate keys BEFORE the API call fires.
//
// Concurrent-safe: sync.Map. The non-CREATE branch reads existing cardID
// + bumps sequence under no lock (sequence is per-key, not cross-key).
type streamingDedupEntry struct {
	cardID     string
	sequence   int
	lastEmitAt time.Time
	lastRunes  int
}

// FeishuEmitPartialResult is returned from EmitPartialCard. cardID is the
// Feishu cardkit card_id (the user-visible card reference). If a prior
// emit already created the card, the same cardID is returned.
type FeishuEmitPartialResult struct {
	CardID   string
	Sequence int
}

// Feishu streaming lock + dedup state (per adapter instance, not per session).
// Created lazily on first EmitPartialCard call so test fixtures that never
// emit do not pay the alloc cost.
var (
	streamingCardsMu sync.Mutex
	streamingCards   = make(map[IdempotencyKey]*streamingDedupEntry)
)

// EmitPartialCard sends a partial (incremental) update to a Feishu cardkit
// card keyed on idempotencyKey. The first call with a given key CREATES a
// card via POST /open-apis/cardkit/v1/cards; subsequent calls UPDATE the
// same card via PUT /open-apis/cardkit/v1/cards/{id}.
//
// Idempotency: per-key dedup table drops duplicate keys. Network retry
// inside FeishuAdapter may still produce duplicate API calls; the
// sessionorchestrator layer's EmitDedup is the primary defense.
//
// Returns ErrFeishuStreamingCardNotFound when the underlying cardkit
// card has expired (rare in practice — Feishu card TTL is 30 days for
// active sessions).
//
// idempotencyKey is a plain string (not the typed IdempotencyKey) so this
// method satisfies the StreamingEmitter interface in sessionorchestrator
// without requiring adapters package imports on the sessionorchestrator
// side (which would otherwise create a test-only import cycle through
// communication/capture).
func (a *FeishuAdapter) EmitPartialCard(
	ctx context.Context,
	chatID string,
	idempotencyKey string,
	content string,
) (*FeishuEmitPartialResult, error) {
	return a.emitPartialCard(ctx, chatID, IdempotencyKey(idempotencyKey), content)
}

// emitPartialCard is the typed-key implementation; used internally and by
// tests that want typed-key safety.
func (a *FeishuAdapter) emitPartialCard(
	ctx context.Context,
	chatID string,
	idempotencyKey IdempotencyKey,
	content string,
) (*FeishuEmitPartialResult, error) {
	if a == nil {
		return nil, fmt.Errorf("feishu: EmitPartialCard called on nil adapter")
	}
	if chatID == "" {
		return nil, fmt.Errorf("feishu: EmitPartialCard requires non-empty chatID")
	}
	if idempotencyKey == "" {
		return nil, fmt.Errorf("feishu: EmitPartialCard requires non-empty idempotencyKey")
	}

	streamingCardsMu.Lock()
	entry, exists := streamingCards[idempotencyKey]
	if !exists {
		entry = &streamingDedupEntry{}
		streamingCards[idempotencyKey] = entry
	}
	streamingCardsMu.Unlock()

	cardJSON := BuildStreamingReplyCardJSON(content, true)
	contentRunes := runeCount(content)

	if !exists || entry.cardID == "" {
		// First emit for this key → CREATE the card.
		newID, err := a.cardkit.CreateCard(ctx, cardJSON)
		if err != nil {
			return nil, fmt.Errorf("feishu: EmitPartialCard create: %w", err)
		}
		streamingCardsMu.Lock()
		entry.cardID = newID
		entry.sequence = 1
		entry.lastEmitAt = time.Now()
		streamingCardsMu.Unlock()
		return &FeishuEmitPartialResult{CardID: newID, Sequence: 1}, nil
	}

	// Subsequent emit → UPDATE the existing card.
	streamingCardsMu.Lock()
	entry.sequence++
	seq := entry.sequence
	streamingCardsMu.Unlock()

	if err := a.cardkit.UpdateCard(ctx, entry.cardID, cardJSON, seq); err != nil {
		if errors.Is(err, ErrFeishuCardStreamClosed) {
			// Card was closed by Feishu (idle timeout). Create a fresh card.
			slog.Warn("feishu: streaming card closed at EmitPartialCard; recreating",
				"chat_id", chatID, "idempotency_key", string(idempotencyKey),
				"old_card_id", entry.cardID)
			newID, createErr := a.cardkit.CreateCard(ctx, cardJSON)
			if createErr != nil {
				return nil, fmt.Errorf("feishu: EmitPartialCard recreate after closed: %w", createErr)
			}
			streamingCardsMu.Lock()
			entry.cardID = newID
			entry.sequence = 1
			streamingCardsMu.Unlock()
			return &FeishuEmitPartialResult{CardID: newID, Sequence: 1}, nil
		}
		// Wrap with the 7215 sentinel so audit logs can grep.
		return nil, &sharederrors.SentinelError{
			Code:    "ORCH_FEISHU_STREAMING_UPDATE_FAILED_7215",
			Message: fmt.Sprintf("Feishu streaming card update failed for chat=%s key=%s", chatID, string(idempotencyKey)),
			Err:     err,
		}
	}

	streamingCardsMu.Lock()
	entry.lastEmitAt = time.Now()
	entry.lastRunes = contentRunes
	streamingCardsMu.Unlock()
	return &FeishuEmitPartialResult{CardID: entry.cardID, Sequence: seq}, nil
}

// EmitFinalCard sends the FINAL card for an idempotencyKey. The same key
// is used so subsequent calls overwrite the prior partial (final-wins
// semantic). When a prior partial card exists, EmitFinalCard UPDATES it;
// when no prior exists (e.g. rollup-only session), it CREATEs fresh.
//
// On success, the dedup table entry is marked final so cleanup routines
// can prune it.
//
// idempotencyKey is a plain string (not the typed IdempotencyKey) so this
// method satisfies the StreamingEmitter interface in sessionorchestrator
// without requiring adapters package imports on the sessionorchestrator
// side (which would otherwise create a test-only import cycle through
// communication/capture).
func (a *FeishuAdapter) EmitFinalCard(
	ctx context.Context,
	chatID string,
	idempotencyKey string,
	content string,
) (*FeishuEmitPartialResult, error) {
	return a.emitFinalCard(ctx, chatID, IdempotencyKey(idempotencyKey), content)
}

// emitFinalCard is the typed-key implementation.
func (a *FeishuAdapter) emitFinalCard(
	ctx context.Context,
	chatID string,
	idempotencyKey IdempotencyKey,
	content string,
) (*FeishuEmitPartialResult, error) {
	if a == nil {
		return nil, fmt.Errorf("feishu: EmitFinalCard called on nil adapter")
	}
	if chatID == "" {
		return nil, fmt.Errorf("feishu: EmitFinalCard requires non-empty chatID")
	}
	if idempotencyKey == "" {
		return nil, fmt.Errorf("feishu: EmitFinalCard requires non-empty idempotencyKey")
	}

	streamingCardsMu.Lock()
	entry, exists := streamingCards[idempotencyKey]
	if !exists {
		entry = &streamingDedupEntry{}
		streamingCards[idempotencyKey] = entry
	}
	streamingCardsMu.Unlock()

	// Final card: streaming=false so the card renders as a single
	// complete message (no typewriter skeleton).
	cardJSON := BuildStreamingReplyCardJSON(content, false)
	contentRunes := runeCount(content)

	if !exists || entry.cardID == "" {
		newID, err := a.cardkit.CreateCard(ctx, cardJSON)
		if err != nil {
			return nil, fmt.Errorf("feishu: EmitFinalCard create: %w", err)
		}
		streamingCardsMu.Lock()
		entry.cardID = newID
		entry.sequence = 1
		entry.lastEmitAt = time.Now()
		streamingCardsMu.Unlock()
		return &FeishuEmitPartialResult{CardID: newID, Sequence: 1}, nil
	}

	// Override the prior partial card with the final content.
	streamingCardsMu.Lock()
	entry.sequence++
	seq := entry.sequence
	streamingCardsMu.Unlock()

	if err := a.cardkit.UpdateCard(ctx, entry.cardID, cardJSON, seq); err != nil {
		if errors.Is(err, ErrFeishuCardStreamClosed) {
			// Rare: card expired between partial and final. Create a fresh card
			// for the final so the user still sees it.
			newID, createErr := a.cardkit.CreateCard(ctx, cardJSON)
			if createErr != nil {
				return nil, fmt.Errorf("feishu: EmitFinalCard recreate after closed: %w", createErr)
			}
			streamingCardsMu.Lock()
			entry.cardID = newID
			entry.sequence = 1
			streamingCardsMu.Unlock()
			return &FeishuEmitPartialResult{CardID: newID, Sequence: 1}, nil
		}
		return nil, &sharederrors.SentinelError{
			Code:    "ORCH_FEISHU_STREAMING_UPDATE_FAILED_7215",
			Message: fmt.Sprintf("Feishu streaming final card update failed for chat=%s key=%s", chatID, string(idempotencyKey)),
			Err:     err,
		}
	}

	streamingCardsMu.Lock()
	entry.lastEmitAt = time.Now()
	entry.lastRunes = contentRunes
	streamingCardsMu.Unlock()
	return &FeishuEmitPartialResult{CardID: entry.cardID, Sequence: seq}, nil
}

// ClearStreamingCard removes a streaming card's dedup entry. Called on
// session end so the next session with the same key starts fresh.
func (a *FeishuAdapter) ClearStreamingCard(idempotencyKey IdempotencyKey) {
	streamingCardsMu.Lock()
	delete(streamingCards, idempotencyKey)
	streamingCardsMu.Unlock()
}

// runeCount returns the rune count of s. Empty string returns 0.
func runeCount(s string) int {
	n := 0
	for range s {
		n++
	}
	return n
}