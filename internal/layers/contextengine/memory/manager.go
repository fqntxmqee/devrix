package memory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/devrix/devrix/internal/layers/contextengine/conversation"
	"github.com/devrix/devrix/internal/layers/contextengine/snapshot"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

// Manager manages in-memory session contexts and persistence.
type Manager struct {
	mu          sync.RWMutex
	messagesMu  sync.RWMutex // protects sc.Messages concurrent reads/writes
	contexts    map[string]*types.SessionContext
	store       *snapshot.Store
	cfg         *config.ContextEngineConfig
	longTerm    ILongTermMemory
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
			SessionID:    session.SessionID,
			WorkDir:      session.WorkDir,
			Model:        session.Model,
			Messages:     []types.Message{},
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

// TrimMessages trims sc.Messages to keep only the last N messages,
// then repairs the chain to remove orphaned tool results.
// Prevents unbounded growth of the persistent message history across interactions.
func (m *Manager) TrimMessages(sc *types.SessionContext, keep int) {
	m.messagesMu.Lock()
	defer m.messagesMu.Unlock()
	if len(sc.Messages) > keep {
		sc.Messages = sc.Messages[len(sc.Messages)-keep:]
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

// Get returns cached context.
func (m *Manager) Get(sessionID string) (*types.SessionContext, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	sc, ok := m.contexts[sessionID]
	return sc, ok
}
