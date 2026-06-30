package memory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/devrix/devrix/internal/layers/contextengine/persist/snapshot"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/conversation"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// Manager manages in-memory session contexts and persistence.
//
// P4 split (AC-P4-3): long-term memory is now injected as two
// independent ports — a LongTermRecaller (S15-A02, read-side) and a
// LongTermStore (S17-A03, write-side). Both typically resolve to the
// same *SQLiteLongTerm value, but the consumer code only sees the
// narrow role it needs. Port interfaces live in shared/contracts so
// this package and persist/memory can share them without an import
// cycle (D2-STRUCT-T04).
type Manager struct {
	mu         sync.RWMutex
	messagesMu sync.RWMutex // protects sc.Messages concurrent reads/writes
	contexts   map[string]*types.SessionContext
	store      *snapshot.Store
	cfg        *config.ContextEngineConfig
	recaller   contracts.LongTermRecaller
	writer     contracts.LongTermStore
}

// NewManager creates a memory manager.
//
// recaller / writer may be nil; in that case the corresponding
// read/write calls become no-ops (EnrichWithLongTermRecall, AutoStoreLongTerm).
func NewManager(
	cfg *config.ContextEngineConfig,
	store *snapshot.Store,
	recaller contracts.LongTermRecaller,
	writer contracts.LongTermStore,
) *Manager {
	return &Manager{
		contexts: make(map[string]*types.SessionContext),
		store:    store,
		cfg:      cfg,
		recaller: recaller,
		writer:   writer,
	}
}

// EnrichWithLongTermRecall appends recalled entries to the session system prompt.
func (m *Manager) EnrichWithLongTermRecall(ctx context.Context, sc *types.SessionContext, query string) error {
	entries, err := m.RecallLongTermEntries(ctx, query)
	if err != nil {
		return err
	}
	appendix := FormatLongTermAppendix(entries, m.cfg.LongTerm.RecallMaxTokens)
	if appendix != "" {
		sc.SystemPrompt += appendix
	}
	return nil
}

// RecallLongTermEntries returns recalled memory entries without mutating session context.
func (m *Manager) RecallLongTermEntries(ctx context.Context, query string) ([]MemoryEntry, error) {
	if m.recaller == nil || !m.cfg.LongTerm.Enabled {
		return nil, nil
	}
	limit := m.cfg.LongTerm.RecallMaxEntries
	if limit <= 0 {
		limit = 5
	}
	return m.recaller.Recall(ctx, query, limit)
}

// AutoStoreLongTerm persists a summary when auto_store is enabled.
func (m *Manager) AutoStoreLongTerm(ctx context.Context, sc *types.SessionContext, userMessage, summary string) error {
	if m.writer == nil || !m.cfg.LongTerm.Enabled || !m.cfg.LongTerm.AutoStore {
		return nil
	}
	topic := ResolveStoreTopic(userMessage, m.cfg.LongTerm.Topics)
	if topic == "" {
		return nil
	}
	content := summary
	if len(content) > 2000 {
		content = content[:2000]
	}
	if content == "" {
		return nil
	}
	return m.writer.Store(ctx, MemoryEntry{
		SessionID: sc.SessionID,
		Topic:     topic,
		Content:   content,
	})
}

// LoadOrInit loads session context from snapshot or initializes fresh.
func (m *Manager) LoadOrInit(session *types.Session, systemPrompt string) (*types.SessionContext, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if sc, ok := m.contexts[session.SessionID]; ok {
		// Always repair on load — even cached in-memory data may have
		// orphaned tool results from process interruptions or old
		// snapshot corruption. Without this, MiniMax rejects the chain
		// with error 2013.
		sc.Messages = conversation.RepairToolMessageChain(sc.Messages)
		return sc, nil
	}

	var sc *types.SessionContext
	var err error
	snapshotData := session.ContextSnapshot
	if len(snapshotData) == 0 {
		snapshotData, _ = m.store.ReadBackup(session.SessionID)
	}
	if len(snapshotData) > 0 {
		sc, err = m.store.Deserialize(snapshotData)
		if err != nil {
			return nil, err
		}
		sc.Messages = conversation.RepairToolMessageChain(sc.Messages)
	} else {
		max, reserved, toolResult, target, snipTarget := m.cfg.ToTokenBudget()
		sc = &types.SessionContext{
			SessionID: session.SessionID,
			WorkDir:   session.WorkDir,
			Model:     session.Model,
			Messages:  []types.Message{},
			TokenBudget: types.TokenBudget{
				MaxContextTokens:  max,
				ReservedOutput:    reserved,
				ToolResultBudget:  toolResult,
				CompressionTarget: target,
				SnipTarget:        snipTarget,
			},
			SystemPrompt: systemPrompt,
			UpdatedAt:    time.Now(),
		}
	}
	m.contexts[session.SessionID] = sc
	return sc, nil
}

// AppendUserMessage appends a user message with idempotency on RequestID.
func (m *Manager) AppendUserMessage(sc *types.SessionContext, requestID, content string) bool {
	if requestID != "" && requestID == sc.LastRequestID {
		return false
	}
	m.messagesMu.Lock()
	msg := types.NewMessage(fmt.Sprintf("msg_%d", time.Now().UnixNano()), sc.SessionID, types.MessageRoleUser, content)
	sc.Messages = append(sc.Messages, *msg)
	sc.LastRequestID = requestID
	sc.UpdatedAt = time.Now()
	m.messagesMu.Unlock()
	return true
}

// AppendMessage appends a plain text message (no metadata).
func (m *Manager) AppendMessage(sc *types.SessionContext, role types.MessageRole, content string) {
	m.messagesMu.Lock()
	msg := types.NewMessage(fmt.Sprintf("msg_%d", time.Now().UnixNano()), sc.SessionID, role, content)
	sc.Messages = append(sc.Messages, *msg)
	sc.UpdatedAt = time.Now()
	m.messagesMu.Unlock()
}

// AppendFullMessage appends a message preserving metadata (tool_calls, tool_call_id).
func (m *Manager) AppendFullMessage(sc *types.SessionContext, msg types.Message) {
	if msg.ID == "" {
		msg.ID = fmt.Sprintf("msg_%d", time.Now().UnixNano())
	}
	if msg.SessionID == "" {
		msg.SessionID = sc.SessionID
	}
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}
	m.messagesMu.Lock()
	sc.Messages = append(sc.Messages, msg)
	sc.UpdatedAt = time.Now()
	m.messagesMu.Unlock()
}

// SetCompressedView updates the LLM-facing view.
func (m *Manager) SetCompressedView(sc *types.SessionContext, view []types.Message) {
	sc.CompressedView = view
	sc.UpdatedAt = time.Now()
}

// ReplaceAutocompactPlaceholder swaps the first pending autocompact
// placeholder in sc.Messages with summary, returning true when a
// placeholder was found and replaced. A pending placeholder is a message
// with metadata["compressed_by"]=="autocompact" && metadata["status"]=="pending".
//
// This is the RH-D2-03 / RH-D2-04 (DM-20260630-013) closure hook: async
// autocompact summarization produces a real summary; this method makes
// it visible to the next Prepare call by replacing the placeholder rather
// than leaving "[compressing ...]" stuck in history.
//
// Concurrency: holds messagesMu so concurrent Prepare / AppendAndTrimMessages
// callers either see the placeholder or the summary, never a torn state.
func (m *Manager) ReplaceAutocompactPlaceholder(sc *types.SessionContext, summary types.Message) bool {
	if sc == nil {
		return false
	}
	m.messagesMu.Lock()
	defer m.messagesMu.Unlock()
	for i := range sc.Messages {
		md := sc.Messages[i].Metadata
		if md == nil {
			continue
		}
		if md["compressed_by"] == "autocompact" && md["status"] == "pending" {
			merged := sc.Messages[i].Metadata
			if summary.Metadata == nil {
				summary.Metadata = make(map[string]string, len(merged)+1)
			}
			for k, v := range merged {
				if _, set := summary.Metadata[k]; !set {
					summary.Metadata[k] = v
				}
			}
			summary.Metadata["status"] = "complete"
			if summary.ID == "" {
				summary.ID = sc.Messages[i].ID
			}
			if summary.SessionID == "" {
				summary.SessionID = sc.Messages[i].SessionID
			}
			if summary.Timestamp.IsZero() {
				summary.Timestamp = sc.Messages[i].Timestamp
			}
			sc.Messages[i] = summary
			sc.UpdatedAt = time.Now()
			return true
		}
	}
	return false
}

// RemoveLastUserMessage removes the last user message from sc.Messages.
// Used on stop to discard the unanswered user message.
func (m *Manager) RemoveLastUserMessage(sc *types.SessionContext) {
	m.messagesMu.Lock()
	defer m.messagesMu.Unlock()
	for i := len(sc.Messages) - 1; i >= 0; i-- {
		if sc.Messages[i].Role == types.MessageRoleUser {
			sc.Messages = append(sc.Messages[:i], sc.Messages[i+1:]...)
			sc.UpdatedAt = time.Now()
			return
		}
	}
}

// SetActiveMessages replaces the in-memory active message window.
func (m *Manager) SetActiveMessages(sc *types.SessionContext, msgs []types.Message) {
	m.messagesMu.Lock()
	defer m.messagesMu.Unlock()
	sc.Messages = append([]types.Message(nil), msgs...)
	sc.UpdatedAt = time.Now()
}

// TrimMessages trims sc.Messages using head+tail retention, then repairs tool chains.
func (m *Manager) TrimMessages(sc *types.SessionContext) {
	m.messagesMu.Lock()
	defer m.messagesMu.Unlock()

	max := m.cfg.Compression.MaxMessages
	if max <= 0 {
		max = 50
	}
	tail := m.cfg.Compression.KeepTailMessages
	headTurns := m.cfg.Compression.Autocompact.PreserveHeadTurns
	if headTurns <= 0 {
		headTurns = 1
	}

	if len(sc.Messages) > max {
		sc.Messages = conversation.HeadTailTrim(sc.Messages, max, headTurns, tail)
	}
	sc.Messages = conversation.RepairToolMessageChain(sc.Messages)
	sc.UpdatedAt = time.Now()
}

// PersistSnapshot serializes and returns snapshot bytes.
func (m *Manager) PersistSnapshot(sc *types.SessionContext) ([]byte, error) {
	m.messagesMu.RLock()
	defer m.messagesMu.RUnlock()
	data, err := m.store.Serialize(sc)
	if err != nil {
		return nil, err
	}
	if err := m.store.WriteBackup(sc.SessionID, data); err != nil {
		// Backup is best-effort; session store remains the source of truth.
		return data, nil
	}
	return data, nil
}

// WriteSnapshotBytes writes already-serialized snapshot bytes to disk.
// Used by persist.Orchestrator when the snapshot was serialized upstream
// (e.g. by facade memory.PersistSnapshot).
func (m *Manager) WriteSnapshotBytes(sessionID string, data []byte) error {
	return m.store.WriteBackup(sessionID, data)
}

// Get returns cached context.
func (m *Manager) Get(sessionID string) (*types.SessionContext, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	sc, ok := m.contexts[sessionID]
	return sc, ok
}
