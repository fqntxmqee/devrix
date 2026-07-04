package materialize

import (
	"github.com/devrix/devrix/internal/layers/contextengine/persist"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/compression"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/conversation"
	"github.com/devrix/devrix/internal/shared/types"
)

func (m *DefaultMaterializer) shrinkPrivateChain(sessionID string, msgs []types.Message, tokenBudget int) []types.Message {
	if len(msgs) == 0 {
		return msgs
	}
	projectDir := m.persistRoot()
	state := m.replacementState(sessionID)
	msgs = compression.ApplyPersistBudget(msgs, compression.PersistBudgetConfig{
		ProjectDir:       projectDir,
		SessionID:        sessionID,
		PerMessageBudget: m.perMessageBudget(sessionID, projectDir, state),
	})
	msgs = conversation.RepairToolMessageChain(msgs)
	msgs = compressMessages(msgs, tokenBudget)
	return conversation.RepairToolMessageChain(msgs)
}

func (m *DefaultMaterializer) persistRoot() string {
	if m != nil && m.ProjectDir != "" {
		return m.ProjectDir
	}
	if m != nil && m.Store != nil {
		return m.Store.BaseDir()
	}
	return ""
}

func (m *DefaultMaterializer) replacementState(sessionID string) *persist.ContentReplacementState {
	if m == nil || sessionID == "" {
		return persist.NewContentReplacementState()
	}
	if v, ok := m.perMsgStates.Load(sessionID); ok {
		if s, ok := v.(*persist.ContentReplacementState); ok && s != nil {
			return s
		}
	}
	s := persist.NewContentReplacementState()
	actual, _ := m.perMsgStates.LoadOrStore(sessionID, s)
	if st, ok := actual.(*persist.ContentReplacementState); ok && st != nil {
		return st
	}
	return s
}

func (m *DefaultMaterializer) perMessageBudget(sessionID, projectDir string, state *persist.ContentReplacementState) *compression.PerMessageBudget {
	if projectDir == "" || sessionID == "" {
		return nil
	}
	return &compression.PerMessageBudget{
		ProjectDir: projectDir,
		SessionID:  sessionID,
		State:      state,
	}
}
