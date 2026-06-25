package memory

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/mups/learn/asset"
	"github.com/devrix/devrix/internal/shared/types"
)

// ─────────────────────────────────────────────────────────────────────────
// Test helpers
// ─────────────────────────────────────────────────────────────────────────

// newTestAsset creates a minimal valid asset.LearningAsset for the given class.
// Returns the asset; callers should ignore the error for valid classes.
func newTestAsset(t *testing.T, class asset.LearningClass, key string) *asset.LearningAsset {
	t.Helper()
	var content asset.AssetContent
	switch class {
	case asset.LearningClass(types.LearningSOP):
		content = &asset.SOPAssetContent{Name: "test-sop", Steps: []string{"step1"}}
	case asset.LearningClass(types.LearningProtocol):
		content = &asset.ProtocolAssetContent{Trigger: "trigger.test"}
	case asset.LearningClass(types.LearningKnowledge):
		content = &asset.KnowledgeAssetContent{Topic: "t", Hypothesis: "h", Confidence: 0.5}
	case asset.LearningClass(types.LearningConclusion):
		content = &asset.ConclusionAssetContent{Statement: "s"}
	case asset.LearningClass(types.LearningPending):
		content = &asset.PendingAssetContent{
			IndeterminateReason: "env_limited",
			OriginalArtifactID:  "art_1",
		}
	default:
		t.Fatalf("unsupported class: %v", class)
	}
	asset, err := asset.NewLearningAsset("asset_test", "sess_test", class, content, key)
	if err != nil {
		t.Fatalf("asset.NewLearningAsset: %v", err)
	}
	return asset
}

// ─────────────────────────────────────────────────────────────────────────
// E4.1 MemoryChannel + MemoryFilter
// ─────────────────────────────────────────────────────────────────────────

func TestMemoryChannel_String_3Channels(t *testing.T) {
	cases := []struct {
		c    MemoryChannel
		want string
	}{
		{MemorySkill, "skill"},
		{MemoryFeedback, "feedback"},
		{MemoryScheduled, "scheduled"},
	}
	for _, tc := range cases {
		if got := tc.c.String(); got != tc.want {
			t.Errorf("MemoryChannel(%d).String() = %q, want %q", tc.c, got, tc.want)
		}
	}
}

func TestMemoryFilter_ZeroValue(t *testing.T) {
	var f MemoryFilter
	if f.Class != asset.LearningClass(types.LearningUnknown) {
		t.Errorf("zero Class = %v, want LearningUnknown", f.Class)
	}
	if f.SessionID != "" {
		t.Errorf("zero SessionID = %q, want empty", f.SessionID)
	}
	if f.MinStrength != 0 {
		t.Errorf("zero MinStrength = %d, want 0", f.MinStrength)
	}
	if f.Expired {
		t.Error("zero Expired = true, want false")
	}
}

func TestMemoryFilter_4Fields(t *testing.T) {
	f := MemoryFilter{
		Class:       asset.LearningClass(types.LearningSOP),
		SessionID:   "sess_1",
		MinStrength: asset.StrengthProtocol,
		Expired:     true,
	}
	if f.Class != asset.LearningClass(types.LearningSOP) {
		t.Errorf("Class = %v, want LearningSOP", f.Class)
	}
	if f.SessionID != "sess_1" {
		t.Errorf("SessionID = %q, want sess_1", f.SessionID)
	}
	if f.MinStrength != asset.StrengthProtocol {
		t.Errorf("MinStrength = %d, want asset.StrengthProtocol", f.MinStrength)
	}
	if !f.Expired {
		t.Error("Expired = false, want true")
	}
}

// ─────────────────────────────────────────────────────────────────────────
// E4.2 SkillMemory — SOP + Protocol channel (LP-2 隔离)
// ─────────────────────────────────────────────────────────────────────────

func TestSkillMemory_Store_AcceptsSOPAndProtocol(t *testing.T) {
	m := NewSkillMemory()
	ctx := context.Background()
	for _, class := range []asset.LearningClass{
		asset.LearningClass(types.LearningSOP),
		asset.LearningClass(types.LearningProtocol),
	} {
		asset := newTestAsset(t, class, "key_"+class.String())
		if err := m.Store(ctx, asset); err != nil {
			t.Errorf("Store(%s): %v", class, err)
		}
	}
}

func TestSkillMemory_Store_RejectsKnowledgeAndConclusionAndPending(t *testing.T) {
	m := NewSkillMemory()
	ctx := context.Background()
	for _, class := range []asset.LearningClass{
		asset.LearningClass(types.LearningKnowledge),
		asset.LearningClass(types.LearningConclusion),
		asset.LearningClass(types.LearningPending),
	} {
		a := newTestAsset(t, class, "key_"+class.String())
		err := m.Store(ctx, a)
		if !errors.Is(err, asset.ErrAssetClassMismatch) {
			t.Errorf("Store(%s) err = %v, want asset.ErrAssetClassMismatch", class, err)
		}
	}
}

func TestSkillMemory_Store_NilAsset(t *testing.T) {
	m := NewSkillMemory()
	if err := m.Store(context.Background(), nil); !errors.Is(err, asset.ErrAssetIncomplete) {
		t.Errorf("nil asset err = %v, want asset.ErrAssetIncomplete", err)
	}
}

func TestSkillMemory_Retrieve_Delete_List(t *testing.T) {
	m := NewSkillMemory()
	ctx := context.Background()

	sop := newTestAsset(t, asset.LearningClass(types.LearningSOP), "k1")
	proto := newTestAsset(t, asset.LearningClass(types.LearningProtocol), "k2")
	if err := m.Store(ctx, sop); err != nil {
		t.Fatalf("Store k1: %v", err)
	}
	if err := m.Store(ctx, proto); err != nil {
		t.Fatalf("Store k2: %v", err)
	}

	// Retrieve hit.
	got, err := m.Retrieve(ctx, "k1")
	if err != nil {
		t.Fatalf("Retrieve k1: %v", err)
	}
	if got != sop {
		t.Error("Retrieve k1 returned different pointer")
	}

	// Retrieve miss.
	got, err = m.Retrieve(ctx, "absent")
	if err != nil {
		t.Fatalf("Retrieve absent: %v", err)
	}
	if got != nil {
		t.Error("Retrieve absent should return nil asset")
	}

	// List (zero filter = all 2).
	all, _ := m.List(ctx, MemoryFilter{})
	if len(all) != 2 {
		t.Errorf("List zero filter: len = %d, want 2", len(all))
	}

	// Filter by Class.
	sopOnly, _ := m.List(ctx, MemoryFilter{Class: asset.LearningClass(types.LearningSOP)})
	if len(sopOnly) != 1 || sopOnly[0] != sop {
		t.Errorf("List Class=SOP: got %d items, want 1 (the SOP)", len(sopOnly))
	}

	// Delete.
	if err := m.Delete(ctx, "k1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	got, _ = m.Retrieve(ctx, "k1")
	if got != nil {
		t.Error("Retrieve after Delete should be nil")
	}
}

func TestSkillMemory_List_FilterBySessionID_AndStrength(t *testing.T) {
	m := NewSkillMemory()
	ctx := context.Background()

	sop := newTestAsset(t, asset.LearningClass(types.LearningSOP), "k1")
	if err := m.Store(ctx, sop); err != nil {
		t.Fatalf("Store: %v", err)
	}

	// SessionID match.
	matched, _ := m.List(ctx, MemoryFilter{SessionID: "sess_test"})
	if len(matched) != 1 {
		t.Errorf("SessionID match: len = %d, want 1", len(matched))
	}

	// SessionID miss.
	missed, _ := m.List(ctx, MemoryFilter{SessionID: "other_sess"})
	if len(missed) != 0 {
		t.Errorf("SessionID miss: len = %d, want 0", len(missed))
	}

	// MinStrength filter (SOP=asset.StrengthSOP=5, set MinStrength=4 → still match).
	matched, _ = m.List(ctx, MemoryFilter{MinStrength: asset.StrengthProtocol})
	if len(matched) != 1 {
		t.Errorf("MinStrength=4: len = %d, want 1", len(matched))
	}

	// MinStrength too high.
	missed, _ = m.List(ctx, MemoryFilter{MinStrength: asset.StrengthSOP + 1})
	if len(missed) != 0 {
		t.Errorf("MinStrength>SOP: len = %d, want 0", len(missed))
	}
}

func TestSkillMemory_List_ExpiredFilter(t *testing.T) {
	m := NewSkillMemory()
	ctx := context.Background()

	sop := newTestAsset(t, asset.LearningClass(types.LearningSOP), "k1")
	if err := m.Store(ctx, sop); err != nil {
		t.Fatalf("Store: %v", err)
	}

	// Default Expired=false → skip expired (none in fresh asset).
	all, _ := m.List(ctx, MemoryFilter{})
	if len(all) != 1 {
		t.Errorf("default filter: len = %d, want 1", len(all))
	}

	// Expired=true → no expired assets in fresh store.
	expired, _ := m.List(ctx, MemoryFilter{Expired: true})
	if len(expired) != 0 {
		t.Errorf("Expired=true (fresh): len = %d, want 0", len(expired))
	}

	// Now force an asset to be expired by mutating ExpiryAt (test only).
	m.mu.Lock()
	m.store["k1"].ExpiryAt = time.Now().Add(-1 * time.Hour)
	m.mu.Unlock()

	// Default filter → expired skipped.
	all, _ = m.List(ctx, MemoryFilter{})
	if len(all) != 0 {
		t.Errorf("default filter (after expiry): len = %d, want 0", len(all))
	}

	// Expired=true → returns it.
	expired, _ = m.List(ctx, MemoryFilter{Expired: true})
	if len(expired) != 1 {
		t.Errorf("Expired=true (after expiry): len = %d, want 1", len(expired))
	}
}

func TestSkillMemory_Concurrent_StoreRetrieve(t *testing.T) {
	m := NewSkillMemory()
	ctx := context.Background()

	const goroutines = 10
	const opsPerG = 50

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(2)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < opsPerG; i++ {
				key := "concurrent_k_" + string(rune('A'+id)) + "_" + string(rune('a'+i%26))
				asset := newTestAsset(t, asset.LearningClass(types.LearningSOP), key)
				_ = m.Store(ctx, asset)
			}
		}(g)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < opsPerG; i++ {
				_, _ = m.Retrieve(ctx, "concurrent_k_A_a")
				_, _ = m.List(ctx, MemoryFilter{})
			}
		}(g)
	}
	wg.Wait()
}

// ─────────────────────────────────────────────────────────────────────────
// E4.2 FeedbackMemory — Knowledge + Conclusion channel (LP-2 隔离)
// ─────────────────────────────────────────────────────────────────────────

func TestFeedbackMemory_Store_AcceptsKnowledgeAndConclusion(t *testing.T) {
	m := NewFeedbackMemory()
	ctx := context.Background()
	for _, class := range []asset.LearningClass{
		asset.LearningClass(types.LearningKnowledge),
		asset.LearningClass(types.LearningConclusion),
	} {
		asset := newTestAsset(t, class, "key_"+class.String())
		if err := m.Store(ctx, asset); err != nil {
			t.Errorf("Store(%s): %v", class, err)
		}
	}
}

func TestFeedbackMemory_Store_RejectsSOPAndProtocolAndPending(t *testing.T) {
	m := NewFeedbackMemory()
	ctx := context.Background()
	for _, class := range []asset.LearningClass{
		asset.LearningClass(types.LearningSOP),
		asset.LearningClass(types.LearningProtocol),
		asset.LearningClass(types.LearningPending),
	} {
		a := newTestAsset(t, class, "key_"+class.String())
		err := m.Store(ctx, a)
		if !errors.Is(err, asset.ErrAssetClassMismatch) {
			t.Errorf("Store(%s) err = %v, want asset.ErrAssetClassMismatch", class, err)
		}
	}
}

func TestFeedbackMemory_Retrieve_Delete_List(t *testing.T) {
	m := NewFeedbackMemory()
	ctx := context.Background()
	know := newTestAsset(t, asset.LearningClass(types.LearningKnowledge), "k1")
	conc := newTestAsset(t, asset.LearningClass(types.LearningConclusion), "k2")
	if err := m.Store(ctx, know); err != nil {
		t.Fatalf("Store k1: %v", err)
	}
	if err := m.Store(ctx, conc); err != nil {
		t.Fatalf("Store k2: %v", err)
	}

	all, _ := m.List(ctx, MemoryFilter{})
	if len(all) != 2 {
		t.Errorf("List zero filter: len = %d, want 2", len(all))
	}

	knowOnly, _ := m.List(ctx, MemoryFilter{Class: asset.LearningClass(types.LearningKnowledge)})
	if len(knowOnly) != 1 || knowOnly[0] != know {
		t.Errorf("List Class=Knowledge: got %d items", len(knowOnly))
	}

	if err := m.Delete(ctx, "k2"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	got, _ := m.Retrieve(ctx, "k2")
	if got != nil {
		t.Error("Retrieve after Delete should be nil")
	}
}

func TestFeedbackMemory_Concurrent_StoreRetrieve(t *testing.T) {
	m := NewFeedbackMemory()
	ctx := context.Background()

	const goroutines = 10
	const opsPerG = 50

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(2)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < opsPerG; i++ {
				key := "fb_k_" + string(rune('A'+id)) + "_" + string(rune('a'+i%26))
				asset := newTestAsset(t, asset.LearningClass(types.LearningKnowledge), key)
				_ = m.Store(ctx, asset)
			}
		}(g)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < opsPerG; i++ {
				_, _ = m.Retrieve(ctx, "fb_k_A_a")
				_, _ = m.List(ctx, MemoryFilter{})
			}
		}(g)
	}
	wg.Wait()
}

// ─────────────────────────────────────────────────────────────────────────
// E4.3 ScheduledMemory — Pending channel + ScheduledRetry
// ─────────────────────────────────────────────────────────────────────────

func TestScheduledMemory_Store_AcceptsOnlyPending(t *testing.T) {
	m := NewScheduledMemory()
	ctx := context.Background()

	pending := newTestAsset(t, asset.LearningClass(types.LearningPending), "k1")
	if err := m.Store(ctx, pending); err != nil {
		t.Errorf("Store(LearningPending): %v", err)
	}

	for _, class := range []asset.LearningClass{
		asset.LearningClass(types.LearningSOP),
		asset.LearningClass(types.LearningProtocol),
		asset.LearningClass(types.LearningKnowledge),
		asset.LearningClass(types.LearningConclusion),
	} {
		a := newTestAsset(t, class, "key_"+class.String())
		err := m.Store(ctx, a)
		if !errors.Is(err, asset.ErrAssetClassMismatch) {
			t.Errorf("Store(%s) err = %v, want asset.ErrAssetClassMismatch", class, err)
		}
	}
}

func TestScheduledMemory_DefaultMaxRetries_3(t *testing.T) {
	m := NewScheduledMemory()
	ctx := context.Background()

	pending := newTestAsset(t, asset.LearningClass(types.LearningPending), "k1")
	if err := m.Store(ctx, pending); err != nil {
		t.Fatalf("Store: %v", err)
	}

	retry, err := m.Retrieve(ctx, "k1")
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if retry == nil {
		t.Fatal("Retrieve returned nil envelope")
	}
	if retry.MaxRetries != asset.DefaultPendingMaxRetries {
		t.Errorf("MaxRetries = %d, want %d", retry.MaxRetries, asset.DefaultPendingMaxRetries)
	}
	if retry.MaxRetries != 3 {
		t.Errorf("asset.DefaultPendingMaxRetries = %d, want 3", retry.MaxRetries)
	}
}

func TestScheduledMemory_MaxRetriesFromPendingContent(t *testing.T) {
	m := NewScheduledMemory()
	ctx := context.Background()

	content := &asset.PendingAssetContent{
		IndeterminateReason: "env_limited",
		OriginalArtifactID:  "art_1",
		MaxRetries:          2, // override default
		NextRetryAt:         time.Now().Add(30 * time.Minute),
	}
	asset, err := asset.NewLearningAsset("asset_x", "sess_x", asset.LearningClass(types.LearningPending), content, "kx")
	if err != nil {
		t.Fatalf("asset.NewLearningAsset: %v", err)
	}
	if err := m.Store(ctx, asset); err != nil {
		t.Fatalf("Store: %v", err)
	}

	retry, _ := m.Retrieve(ctx, "kx")
	if retry.MaxRetries != 2 {
		t.Errorf("MaxRetries = %d, want 2 (from content)", retry.MaxRetries)
	}
	if retry.TriggerAt.IsZero() {
		t.Error("TriggerAt should be set from content.NextRetryAt, not zero")
	}
}

func TestScheduledMemory_TriggerAt_DefaultsToExpiryAt(t *testing.T) {
	m := NewScheduledMemory()
	ctx := context.Background()

	// asset.PendingAssetContent without NextRetryAt → TriggerAt = asset.ExpiryAt.
	pending := newTestAsset(t, asset.LearningClass(types.LearningPending), "k1")
	if err := m.Store(ctx, pending); err != nil {
		t.Fatalf("Store: %v", err)
	}
	retry, _ := m.Retrieve(ctx, "k1")
	if !retry.TriggerAt.Equal(pending.ExpiryAt) {
		t.Errorf("TriggerAt = %v, want %v (ExpiryAt)", retry.TriggerAt, pending.ExpiryAt)
	}
}

func TestScheduledMemory_Retrieve_Delete_List(t *testing.T) {
	m := NewScheduledMemory()
	ctx := context.Background()

	p1 := newTestAsset(t, asset.LearningClass(types.LearningPending), "k1")
	p2 := newTestAsset(t, asset.LearningClass(types.LearningPending), "k2")
	if err := m.Store(ctx, p1); err != nil {
		t.Fatalf("Store k1: %v", err)
	}
	if err := m.Store(ctx, p2); err != nil {
		t.Fatalf("Store k2: %v", err)
	}

	all, _ := m.List(ctx, MemoryFilter{})
	if len(all) != 2 {
		t.Errorf("List zero filter: len = %d, want 2", len(all))
	}

	if err := m.Delete(ctx, "k1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	r, _ := m.Retrieve(ctx, "k1")
	if r != nil {
		t.Error("Retrieve after Delete should be nil envelope")
	}
}

func TestScheduledMemory_ListDue(t *testing.T) {
	m := NewScheduledMemory()
	ctx := context.Background()

	now := time.Now()
	// due: TriggerAt < now
	due := newTestAsset(t, asset.LearningClass(types.LearningPending), "due")
	if err := m.Store(ctx, due); err != nil {
		t.Fatalf("Store due: %v", err)
	}
	m.mu.Lock()
	m.store["due"].TriggerAt = now.Add(-1 * time.Hour)
	m.mu.Unlock()

	// not due: TriggerAt > now
	future := newTestAsset(t, asset.LearningClass(types.LearningPending), "future")
	if err := m.Store(ctx, future); err != nil {
		t.Fatalf("Store future: %v", err)
	}
	m.mu.Lock()
	m.store["future"].TriggerAt = now.Add(1 * time.Hour)
	m.mu.Unlock()

	dueList := m.ListDue(now)
	if len(dueList) != 1 {
		t.Errorf("ListDue: len = %d, want 1", len(dueList))
	}
}

func TestScheduledRetry_IsExhausted(t *testing.T) {
	r := &ScheduledRetry{MaxRetries: 3}
	if r.IsExhausted() {
		t.Error("IsExhausted at 0 retries: should be false")
	}
	r.RetryCount = 2
	if r.IsExhausted() {
		t.Error("IsExhausted at 2/3: should be false")
	}
	r.RetryCount = 3
	if !r.IsExhausted() {
		t.Error("IsExhausted at 3/3: should be true")
	}
}

func TestScheduledMemory_Concurrent_StoreRetrieve(t *testing.T) {
	m := NewScheduledMemory()
	ctx := context.Background()

	const goroutines = 10
	const opsPerG = 50

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(2)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < opsPerG; i++ {
				key := "sched_k_" + string(rune('A'+id)) + "_" + string(rune('a'+i%26))
				asset := newTestAsset(t, asset.LearningClass(types.LearningPending), key)
				_ = m.Store(ctx, asset)
			}
		}(g)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < opsPerG; i++ {
				_, _ = m.Retrieve(ctx, "sched_k_A_a")
				_, _ = m.List(ctx, MemoryFilter{})
				_ = m.ListDue(time.Now())
			}
		}(g)
	}
	wg.Wait()
}

// ─────────────────────────────────────────────────────────────────────────
// LP-2 cross-channel isolation guarantee (compile-time-style check)
// ─────────────────────────────────────────────────────────────────────────

func TestLP2_ExhaustiveChannelPartition(t *testing.T) {
	// All 5 asset.LearningClass values must map to exactly one channel.
	channels := []MemoryChannel{MemorySkill, MemoryFeedback, MemoryScheduled}
	seen := make(map[asset.LearningClass]MemoryChannel)
	for _, ch := range channels {
		for class := range ch.allowedClasses() {
			if prev, ok := seen[class]; ok {
				t.Errorf("class %s appears in both %s and %s", class, prev, ch)
			}
			seen[class] = ch
		}
	}
	wantCount := 5
	if len(seen) != wantCount {
		t.Errorf("LP-2 partition covers %d classes, want %d (LearningUnknown is reserved)", len(seen), wantCount)
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Smoke: NewSkillMemory / NewFeedbackMemory / NewScheduledMemory
// ─────────────────────────────────────────────────────────────────────────

func TestNewMemories_NonNil(t *testing.T) {
	if NewSkillMemory() == nil {
		t.Error("NewSkillMemory returned nil")
	}
	if NewFeedbackMemory() == nil {
		t.Error("NewFeedbackMemory returned nil")
	}
	if NewScheduledMemory() == nil {
		t.Error("NewScheduledMemory returned nil")
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Error wrapping sanity
// ─────────────────────────────────────────────────────────────────────────

func TestErrAssetClassMismatch_NotConfused(t *testing.T) {
	// Sanity: ensure asset.ErrAssetClassMismatch is distinct from asset.ErrAssetIncomplete
	// (otherwise fail-fast at the boundary becomes ambiguous).
	if errors.Is(asset.ErrAssetClassMismatch, asset.ErrAssetIncomplete) {
		t.Error("asset.ErrAssetClassMismatch must NOT match asset.ErrAssetIncomplete")
	}
	if !strings.Contains(asset.ErrAssetClassMismatch.Error(), "class") {
		t.Errorf("asset.ErrAssetClassMismatch message should contain 'class': %q", asset.ErrAssetClassMismatch.Error())
	}
}