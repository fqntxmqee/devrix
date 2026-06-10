package memory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/devrix/devrix/internal/layers/contextengine/snapshot"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

// Manager manages in-memory session contexts and persistence.
type Manager struct {
	mu       sync.RWMutex
	contexts map[string]*types.SessionContext
	store    *snapshot.Store
	cfg      *config.ContextEngineConfig
	longTerm ILongTermMemory
}

// NewManager creates a memory manager.
func NewManager(cfg *config.ContextEngineConfig, store *snapshot.Store, longTerm ILongTermMemory) *Manager {
	return &Manager{
		contexts: make(map[string]*types.SessionContext),
		store:    store,
		cfg:      cfg,
		longTerm: longTerm,
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
	if m.longTerm == nil || !m.cfg.LongTerm.Enabled {
		return nil, nil
	}
	limit := m.cfg.LongTerm.RecallMaxEntries
	if limit <= 0 {
		limit = 5
	}
	return m.longTerm.Recall(ctx, query, limit)
}

// AutoStoreLongTerm persists a summary when auto_store is enabled.
func (m *Manager) AutoStoreLongTerm(ctx context.Context, sc *types.SessionContext, userMessage, summary string) error {
	if m.longTerm == nil || !m.cfg.LongTerm.Enabled || !m.cfg.LongTerm.AutoStore {
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
	return m.longTerm.Store(ctx, MemoryEntry{
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
		return sc, nil
	}

	var sc *types.SessionContext
	var err error
	if len(session.ContextSnapshot) > 0 {
		sc, err = m.store.Deserialize(session.ContextSnapshot)
		if err != nil {
			return nil, err
		}
	} else {
		max, reserved, toolResult, target := m.cfg.ToTokenBudget()
		sc = &types.SessionContext{
			SessionID:    session.SessionID,
			WorkDir:      session.WorkDir,
			Model:        session.Model,
			Messages:     []types.Message{},
			TokenBudget: types.TokenBudget{
				MaxContextTokens:  max,
				ReservedOutput:    reserved,
				ToolResultBudget:  toolResult,
				CompressionTarget: target,
			},
			PEVState:     types.DefaultPEVState(m.cfg.PEV.MaxIterations),
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
	msg := types.NewMessage(fmt.Sprintf("msg_%d", time.Now().UnixNano()), sc.SessionID, types.MessageRoleUser, content)
	sc.Messages = append(sc.Messages, *msg)
	sc.LastRequestID = requestID
	sc.UpdatedAt = time.Now()
	return true
}

// AppendMessage appends a plain text message (no metadata).
func (m *Manager) AppendMessage(sc *types.SessionContext, role types.MessageRole, content string) {
	msg := types.NewMessage(fmt.Sprintf("msg_%d", time.Now().UnixNano()), sc.SessionID, role, content)
	sc.Messages = append(sc.Messages, *msg)
	sc.UpdatedAt = time.Now()
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
	sc.Messages = append(sc.Messages, msg)
	sc.UpdatedAt = time.Now()
}

// SetCompressedView updates the LLM-facing view.
func (m *Manager) SetCompressedView(sc *types.SessionContext, view []types.Message) {
	sc.CompressedView = view
	sc.UpdatedAt = time.Now()
}

// PersistSnapshot serializes and returns snapshot bytes.
func (m *Manager) PersistSnapshot(sc *types.SessionContext) ([]byte, error) {
	data, err := m.store.Serialize(sc)
	if err != nil {
		return nil, err
	}
	if err := m.store.WriteBackup(sc.SessionID, data); err != nil {
		return data, fmt.Errorf("backup write: %w", err)
	}
	return data, nil
}

// Get returns cached context.
func (m *Manager) Get(sessionID string) (*types.SessionContext, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	sc, ok := m.contexts[sessionID]
	return sc, ok
}
