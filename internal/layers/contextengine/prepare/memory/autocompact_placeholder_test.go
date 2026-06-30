package memory

import (
	"testing"

	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

// T: D2-S15-A80-T01 (DM-20260630-013 RH-D2-03/04)
//
// Manager.ReplaceAutocompactPlaceholder must swap the first pending
// autocompact placeholder in sc.Messages with summary, flipping
// metadata["status"] from "pending" to "complete". Without this closure
// hook, async autocompact summaries never land in the session history.
func TestReplaceAutocompactPlaceholder_replacesFirstPending(t *testing.T) {
	cfg := config.DefaultContextEngineConfig()
	mgr := NewManager(cfg, nil, nil, nil)

	sc := &types.SessionContext{
		SessionID: "sess_ac",
		Messages: []types.Message{
			{Role: types.MessageRoleUser, Content: "hi"},
			{Role: types.MessageRoleAssistant, Content: "head"},
			{
				Role:    types.MessageRoleAssistant,
				Content: "[compressing conversation... keeping 4 most recent exchanges]",
				Metadata: map[string]string{
					"compressed_by": "autocompact",
					"status":        "pending",
				},
			},
			{Role: types.MessageRoleUser, Content: "tail-1"},
			{Role: types.MessageRoleAssistant, Content: "tail-2"},
		},
	}

	summary := types.Message{
		Role:    types.MessageRoleAssistant,
		Content: "summary of middle",
		Metadata: map[string]string{
			"compressed_by": "autocompact",
			"status":        "complete",
		},
	}

	if replaced := mgr.ReplaceAutocompactPlaceholder(sc, summary); !replaced {
		t.Fatal("expected placeholder to be replaced")
	}

	var found bool
	for _, m := range sc.Messages {
		if md := m.Metadata; md != nil && md["compressed_by"] == "autocompact" {
			if m.Content != "summary of middle" {
				t.Fatalf("placeholder replaced with wrong content: %q", m.Content)
			}
			if md["status"] != "complete" {
				t.Fatalf("placeholder status not flipped to complete: %q", md["status"])
			}
			found = true
		}
	}
	if !found {
		t.Fatal("placeholder missing from messages after replacement")
	}
}

// T: D2-S15-A80-T02 (DM-20260630-013 RH-D2-03)
//
// When the session has no pending placeholder (e.g. sync fallback already
// replaced it), ReplaceAutocompactPlaceholder must return false and leave
// messages untouched.
func TestReplaceAutocompactPlaceholder_noPlaceholder(t *testing.T) {
	cfg := config.DefaultContextEngineConfig()
	mgr := NewManager(cfg, nil, nil, nil)

	sc := &types.SessionContext{
		SessionID: "sess_no_placeholder",
		Messages: []types.Message{
			{Role: types.MessageRoleUser, Content: "hi"},
		},
	}

	replaced := mgr.ReplaceAutocompactPlaceholder(sc, types.Message{
		Role:    types.MessageRoleAssistant,
		Content: "summary",
	})
	if replaced {
		t.Fatal("expected no replacement when no pending placeholder exists")
	}
	if len(sc.Messages) != 1 {
		t.Fatalf("messages were unexpectedly modified: %+v", sc.Messages)
	}
}

// T: D2-S15-A80-T03 (DM-20260630-013 RH-D2-03)
//
// nil SessionContext must not panic — callers like the kernel sink may
// receive a missing session race.
func TestReplaceAutocompactPlaceholder_nilSession(t *testing.T) {
	cfg := config.DefaultContextEngineConfig()
	mgr := NewManager(cfg, nil, nil, nil)
	if replaced := mgr.ReplaceAutocompactPlaceholder(nil, types.Message{}); replaced {
		t.Fatal("expected false for nil session")
	}
}
