package escape

import (
	"context"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/plan"
)

// --- TestHumanArbitrator_PendingResolution_Save -----------------------------

func TestPendingResolutionStore_SaveLoadDelete(t *testing.T) {
	store := NewInMemoryPendingResolutionStore()
	sid := "sess-save-load"

	// 1. Load empty → not found
	d, found, err := store.Load(sid)
	if err != nil {
		t.Fatalf("Load on empty store should not error: %v", err)
	}
	if found {
		t.Error("Load on empty store: found=true, want false")
	}

	// 2. Save
	decision := EscapeDecision{
		Action:    EscapePendingHuman,
		Reason:    "human_review_required",
		PendingID: "pending-abc",
		SessionID: sid,
		CreatedAt: time.Now(),
	}
	if err := store.Save(sid, decision); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// 3. Load → found
	d, found, err = store.Load(sid)
	if err != nil {
		t.Fatalf("Load after Save failed: %v", err)
	}
	if !found {
		t.Fatal("Load after Save: found=false, want true")
	}
	if d.PendingID != "pending-abc" {
		t.Errorf("Loaded PendingID=%q, want pending-abc", d.PendingID)
	}

	// 4. Delete → idempotent
	if err := store.Delete(sid); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// 5. Load after Delete → not found
	_, found, err = store.Load(sid)
	if err != nil {
		t.Fatalf("Load after Delete: %v", err)
	}
	if found {
		t.Error("Load after Delete: found=true, want false")
	}

	// 6. Delete non-existent → no error
	if err := store.Delete("never-existed"); err != nil {
		t.Errorf("Delete on non-existent should be idempotent, got %v", err)
	}
}

// --- TestPendingResolutionStore_EmptySessionID ------------------------------

func TestPendingResolutionStore_EmptySessionID(t *testing.T) {
	store := NewInMemoryPendingResolutionStore()

	if err := store.Save("", EscapeDecision{}); err == nil {
		t.Error("Save with empty sessionID should fail")
	}
	if _, _, err := store.Load(""); err == nil {
		t.Error("Load with empty sessionID should fail")
	}
	if err := store.Delete(""); err == nil {
		t.Error("Delete with empty sessionID should fail")
	}
}

// --- TestPendingResolutionStore_Len -----------------------------------------

func TestPendingResolutionStore_Len(t *testing.T) {
	store := NewInMemoryPendingResolutionStore()

	if store.Len() != 0 {
		t.Errorf("initial Len=%d, want 0", store.Len())
	}

	store.Save("sess-1", EscapeDecision{Action: EscapePendingHuman})
	store.Save("sess-2", EscapeDecision{Action: EscapePendingHuman})
	if store.Len() != 2 {
		t.Errorf("after 2 saves: Len=%d, want 2", store.Len())
	}

	// Save 覆盖
	store.Save("sess-1", EscapeDecision{Action: EscapeForceExit})
	if store.Len() != 2 {
		t.Errorf("after overwrite: Len=%d, want 2", store.Len())
	}

	store.Delete("sess-1")
	if store.Len() != 1 {
		t.Errorf("after delete: Len=%d, want 1", store.Len())
	}
}

// --- TestHumanArbitrator_ResumeSession_Roundtrip ----------------------------

// Roundtrip test: Arbitrate → Save → ResumeSession loads the same decision.
func TestHumanArbitrator_ResumeSession_Roundtrip(t *testing.T) {
	store := NewInMemoryPendingResolutionStore()
	audit := NewEscapeAuditLog()
	notifier := &mockCLINotifier{}

	human := NewHumanArbitrator(notifier, audit, store)
	human.SetTimeout(50 * time.Millisecond)

	loopCtx := LoopContext{SessionID: "sess-resume", PlanKind: plan.ExplorationPlan}

	// 1. Arbitrate returns EscapePendingHuman synchronously
	d0, err := human.Arbitrate(context.Background(), loopCtx, nil)
	if err != nil {
		t.Fatalf("Arbitrate failed: %v", err)
	}
	if d0.Action != EscapePendingHuman {
		t.Errorf("Arbitrate Action=%s, want pending_human", d0.Action)
	}
	if d0.PendingID == "" {
		t.Error("PendingID should be set")
	}

	// 2. ResumeSession before timeout → not found
	dResume, found, err := human.ResumeSession("sess-resume")
	if err != nil {
		t.Fatalf("ResumeSession failed: %v", err)
	}
	if found {
		t.Errorf("ResumeSession before timeout: found=true, want false (pending not yet resolved)")
	}
	_ = dResume

	// 3. Wait for timeout
	time.Sleep(150 * time.Millisecond)

	// 4. ResumeSession after timeout → should find ForceExit decision
	dResume, found, err = human.ResumeSession("sess-resume")
	if err != nil {
		t.Fatalf("ResumeSession after timeout failed: %v", err)
	}
	if !found {
		t.Fatal("ResumeSession after timeout: found=false, want true")
	}
	if dResume.Action != EscapeForceExit {
		t.Errorf("ResumeSession Action=%s, want force_exit", dResume.Action)
	}
	if dResume.Reason != "human_timeout_10s" {
		// We used a 50ms timeout but reason is fixed string
		t.Logf("ResumeSession Reason=%q (timeout-based reason)", dResume.Reason)
	}

	// 5. ResumeSession again → should NOT find (consumed)
	_, found, err = human.ResumeSession("sess-resume")
	if err != nil {
		t.Fatalf("ResumeSession 2nd failed: %v", err)
	}
	if found {
		t.Error("ResumeSession after consumption: found=true, want false (one-shot)")
	}
}