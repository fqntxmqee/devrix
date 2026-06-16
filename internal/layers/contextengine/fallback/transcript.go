package fallback

import (
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

// TranscriptManager manages in-memory transcript with optional compaction.
type TranscriptManager struct {
	cfg config.TranscriptConfig
}

// NewTranscriptManager creates a transcript manager.
func NewTranscriptManager(cfg config.TranscriptConfig) *TranscriptManager {
	return &TranscriptManager{cfg: cfg}
}

// Ensure returns or creates a transcript store on session context.
func (m *TranscriptManager) Ensure(sc *types.SessionContext) *types.TranscriptStore {
	if sc == nil || !m.cfg.Enabled {
		return nil
	}
	if sc.Transcript == nil {
		sc.Transcript = &types.TranscriptStore{}
	}
	return sc.Transcript
}

// AppendTurn records user and assistant messages in the transcript.
func (m *TranscriptManager) AppendTurn(sc *types.SessionContext, userMessage, assistantSummary string) {
	store := m.Ensure(sc)
	if store == nil {
		return
	}
	store.Append(types.MessageRoleUser, userMessage)
	if assistantSummary != "" {
		store.Append(types.MessageRoleAssistant, assistantSummary)
	}
	if m.cfg.CompactAfterTurns > 0 {
		maxEntries := m.cfg.CompactAfterTurns * 2
		store.Compact(maxEntries)
	}
}
