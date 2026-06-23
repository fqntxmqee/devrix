package learn

import (
	"context"
	"errors"
	"sync"
	"testing"
)

// ─────────────────────────────────────────────────────────────────────────
// InMemoryReputationStore.Get / Update / List
// ─────────────────────────────────────────────────────────────────────────

func TestInMemoryReputationStore_Get_Empty(t *testing.T) {
	s := NewInMemoryReputationStore()
	got, err := s.Get(context.Background(), "absent")
	if err != nil {
		t.Fatalf("Get absent: %v", err)
	}
	if got != nil {
		t.Error("Get absent should return nil evidence")
	}
}

func TestInMemoryReputationStore_Update_ThenGet(t *testing.T) {
	s := NewInMemoryReputationStore()
	ctx := context.Background()
	rep, err := NewReputationEvidence("sess_1", TrackModeDeveloper)
	if err != nil {
		t.Fatalf("NewReputationEvidence: %v", err)
	}
	rep.Alpha = 5
	rep.Beta = 2

	if err := s.Update(ctx, rep); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, err := s.Get(ctx, "sess_1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil || got.Alpha != 5 || got.Beta != 2 {
		t.Errorf("Get = %+v, want Alpha=5 Beta=2", got)
	}
}

func TestInMemoryReputationStore_Update_NilEvidence_FailFast(t *testing.T) {
	s := NewInMemoryReputationStore()
	if err := s.Update(context.Background(), nil); !errors.Is(err, ErrReputationStoreUnavailable) {
		t.Errorf("Update(nil) err = %v, want ErrReputationStoreUnavailable", err)
	}
}

func TestInMemoryReputationStore_Update_EmptySessionID_FailFast(t *testing.T) {
	s := NewInMemoryReputationStore()
	rep := &ReputationEvidence{}
	if err := s.Update(context.Background(), rep); !errors.Is(err, ErrReputationStoreUnavailable) {
		t.Errorf("Update(empty sess) err = %v, want ErrReputationStoreUnavailable", err)
	}
}

func TestInMemoryReputationStore_Get_EmptySessionID_FailFast(t *testing.T) {
	s := NewInMemoryReputationStore()
	got, err := s.Get(context.Background(), "")
	if !errors.Is(err, ErrReputationStoreUnavailable) {
		t.Errorf("Get(empty sess) err = %v, want ErrReputationStoreUnavailable", err)
	}
	if got != nil {
		t.Error("Get(empty) should return nil evidence")
	}
}

func TestInMemoryReputationStore_List_FilterByTrackMode(t *testing.T) {
	s := NewInMemoryReputationStore()
	ctx := context.Background()

	dev, _ := NewReputationEvidence("sess_dev", TrackModeDeveloper)
	op, _ := NewReputationEvidence("sess_op", TrackModeOperator)
	if err := s.Update(ctx, dev); err != nil {
		t.Fatalf("Update dev: %v", err)
	}
	if err := s.Update(ctx, op); err != nil {
		t.Fatalf("Update op: %v", err)
	}

	devOnly, err := s.List(ctx, TrackModeDeveloper, 0)
	if err != nil {
		t.Fatalf("List dev: %v", err)
	}
	if len(devOnly) != 1 || devOnly[0].SessionID != "sess_dev" {
		t.Errorf("List dev = %+v, want 1 entry for sess_dev", devOnly)
	}

	opOnly, err := s.List(ctx, TrackModeOperator, 0)
	if err != nil {
		t.Fatalf("List op: %v", err)
	}
	if len(opOnly) != 1 || opOnly[0].SessionID != "sess_op" {
		t.Errorf("List op = %+v, want 1 entry for sess_op", opOnly)
	}

	all, err := s.List(ctx, "", 0)
	if err != nil {
		t.Fatalf("List all: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("List all = %d, want 2", len(all))
	}
}

func TestInMemoryReputationStore_List_LimitCap(t *testing.T) {
	s := NewInMemoryReputationStore()
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		rep, _ := NewReputationEvidence("sess_"+string(rune('a'+i)), TrackModeDeveloper)
		_ = s.Update(ctx, rep)
	}
	limited, err := s.List(ctx, TrackModeDeveloper, 2)
	if err != nil {
		t.Fatalf("List limited: %v", err)
	}
	if len(limited) != 2 {
		t.Errorf("List limit=2: got %d, want 2", len(limited))
	}
	zero, err := s.List(ctx, TrackModeDeveloper, 0)
	if err != nil {
		t.Fatalf("List zero: %v", err)
	}
	if len(zero) != 5 {
		t.Errorf("List limit=0 (default cap): got %d, want 5", len(zero))
	}
}

func TestInMemoryReputationStore_DefensiveCopy(t *testing.T) {
	s := NewInMemoryReputationStore()
	ctx := context.Background()
	rep, _ := NewReputationEvidence("sess_1", TrackModeDeveloper)
	rep.Alpha = 5
	if err := s.Update(ctx, rep); err != nil {
		t.Fatalf("Update: %v", err)
	}
	// Mutate the original after Update.
	rep.Alpha = 999
	got, _ := s.Get(ctx, "sess_1")
	if got.Alpha != 5 {
		t.Errorf("Get.Alpha = %d, want 5 (store should be defensive copy)", got.Alpha)
	}
}

func TestInMemoryReputationStore_Concurrent_GetUpdate(t *testing.T) {
	s := NewInMemoryReputationStore()
	ctx := context.Background()
	const goroutines = 10
	const opsPerG = 50

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(2)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < opsPerG; i++ {
				key := "sess_" + string(rune('A'+id)) + "_" + string(rune('a'+i%26))
				rep, _ := NewReputationEvidence(key, TrackModeDeveloper)
				rep.Alpha = i
				rep.Beta = i % 3
				_ = s.Update(ctx, rep)
			}
		}(g)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < opsPerG; i++ {
				_, _ = s.Get(ctx, "sess_A_a")
				_, _ = s.List(ctx, TrackModeDeveloper, 0)
			}
		}(g)
	}
	wg.Wait()
}