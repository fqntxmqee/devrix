// SessionLoader adapter: D2-S15-A01 LoadSession — wraps memory.Manager
// with snapshot-restore observability + event emission.
//
// The adapter matches prepare.SessionLoader:
//
//	LoadOrInit(session *types.Session, model string) (*types.SessionContext, error)
//
// Behavior:
//   - delegate to memory.Manager.LoadOrInit(session, systemPrompt)
//   - emit "snapshot-restored" / "snapshot-corrupt" events through h.Emit
//   - start a span around the load (D2-S2-Context-Snapshot-Load op name)
//   - on corrupt snapshot, reset and retry once
//
// This file replaces facade/engine_prepare.go::loadOrInitSession and will be
// wired in P1-d (facade → prepare.Orchestrator.Prepare).
package adapters

import (
	"context"
	"fmt"

	"github.com/devrix/devrix/internal/layers/contextengine/prepare/memory"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/prompt"
	"github.com/devrix/devrix/internal/layers/observability/instrument/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/errors"
	"github.com/devrix/devrix/internal/shared/types"
)

// SessionLoaderAdapter implements prepare.SessionLoader.
//
// It must NOT import orchestration or multiagent (D2 thin boundary — D2-THIN-T01).
type SessionLoaderAdapter struct {
	manager *memory.Manager
	hooks   Hooks
}

// NewSessionLoaderAdapter constructs a SessionLoader that delegates to the
// supplied memory.Manager and emits observability hooks.
func NewSessionLoaderAdapter(manager *memory.Manager, opts ...HooksOption) *SessionLoaderAdapter {
	return &SessionLoaderAdapter{manager: manager, hooks: applyHooks(opts)}
}

// LoadOrInit loads or initializes the session context. On snapshot corruption
// it resets the snapshot, clears dynamic caches, and retries once.
//
// Returns (sc, isNew, err) where isNew is true on first init (no prior
// snapshot existed), false on snapshot restore.
//
// The `model` parameter is accepted for interface compatibility (legacy facade
// signature); the underlying Manager.LoadOrInit uses the session's stored model.
func (a *SessionLoaderAdapter) LoadOrInit(ctx context.Context, session *types.Session, model string) (*types.SessionContext, bool, error) {
	ctx, span := a.hooks.startSpan(ctx, telemetry.OpD2_S2_Context_Snapshot_Load, tracer.SpanKindInternal,
		tracer.Attribute{Key: "session.id", Value: session.SessionID},
		tracer.Attribute{Key: "session.request_id", Value: session.RequestID},
		tracer.Attribute{Key: "snapshot.had_snapshot", Value: boolStr(session.ContextSnapshot != nil)},
		tracer.Attribute{Key: "snapshot.model", Value: model},
		tracer.Attribute{Key: "context.caller", Value: "d7"},
	)
	if span != nil {
		defer span.End()
	}

	hadSnapshot := session.ContextSnapshot != nil
	isNew := !hadSnapshot
	sc, err := a.manager.LoadOrInit(session, "")
	if err != nil {
		// Snapshot was corrupt → reset, clear caches, retry.
		a.hooks.emit(infoEvent(session.SessionID, "快照已重置，开始新上下文"))
		session.ContextSnapshot = nil
		prompt.ClearDynamicSectionCache(session.SessionID)
		prompt.ClearAgentsCache()
		isNew = true
		sc, err = a.manager.LoadOrInit(session, "")
		if err != nil {
			if span != nil {
				span.RecordError(err)
			}
			a.hooks.emit(errorEvent(session.SessionID, errors.NewSnapshotCorruptError(err), false))
			return nil, false, fmt.Errorf("snapshot load failed after reset: %w", err)
		}
		a.hooks.emit(snapshotRestoredEvent(session.SessionID, false))
	}

	if span != nil {
		span.SetAttributes(
			tracer.Attribute{Key: "snapshot.message_count", Value: fmt.Sprintf("%d", len(sc.Messages))},
			tracer.Attribute{Key: "snapshot.restored", Value: fmt.Sprintf("%t", hadSnapshot)},
			tracer.Attribute{Key: "snapshot.is_new", Value: boolStr(isNew)},
		)
	}
	return sc, isNew, nil
}

// --- event helpers (mirror facade/engine_events.go shape, no business logic) ---

func infoEvent(sessionID, content string) *contracts.EngineEvent {
	return &contracts.EngineEvent{
		Type:      "info",
		Content:   content,
		SessionID: sessionID,
		Metadata:  map[string]string{"category": "context"},
	}
}

func errorEvent(sessionID string, err *errors.SentinelError, recoverable bool) *contracts.EngineEvent {
	rec := "false"
	if recoverable {
		rec = "true"
	}
	return &contracts.EngineEvent{
		Type:      "error",
		Content:   err.Error(),
		SessionID: sessionID,
		Metadata: map[string]string{
			"code":        err.Code,
			"recoverable": rec,
		},
	}
}

func snapshotRestoredEvent(sessionID string, fromDisk bool) *contracts.EngineEvent {
	return &contracts.EngineEvent{
		Type:      "snapshot_restored",
		SessionID: sessionID,
		Metadata:  map[string]string{"from_disk": boolStr(fromDisk)},
	}
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}