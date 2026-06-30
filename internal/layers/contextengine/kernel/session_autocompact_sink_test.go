package kernel

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/persist/snapshot"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/compression"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/memory"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

// T: D2-S15-A80-T04 (DM-20260630-013 RH-D2-03)
//
// sessionAutocompactSink must not panic on unknown session IDs. Async
// summarization can outlive the session (process restart between summary
// started and complete).
func TestSessionAutocompactSink_unknownSession(t *testing.T) {
	cfg := config.DefaultContextEngineConfig()
	store := snapshot.NewStore(&cfg.Snapshot)
	mgr := memory.NewManager(cfg, store, nil, nil)
	sink := NewSessionAutocompactSink(mgr)
	// Must not panic.
	sink.EmitAutocompactComplete("nonexistent-session", types.Message{
		Role:    types.MessageRoleAssistant,
		Content: "summary",
	}, "token-unknown")
}

// T: D2-S15-A80-T05 (DM-20260630-013 RH-D2-03)
//
// nil memory manager must produce a defensive no-op sink rather than
// panic. Defensive design — async autocompact is opt-in.
func TestSessionAutocompactSink_nilMemoryNoop(t *testing.T) {
	sink := NewSessionAutocompactSink(nil)
	// Must not panic.
	sink.EmitCompressionStep("s1", "step", 0, 1)
	sink.EmitAutocompact("s1", compression.AutocompactMeta{})
	sink.EmitAutocompactComplete("s1", types.Message{}, "t")
}
