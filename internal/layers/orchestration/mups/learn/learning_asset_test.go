package learn

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/shared/types"
)

func TestClassToStrength_5Classes(t *testing.T) {
	cases := []struct {
		class LearningClass
		want  CertaintyStrength
	}{
		{types.LearningSOP, StrengthSOP},
		{types.LearningProtocol, StrengthProtocol},
		{types.LearningKnowledge, StrengthKnowledge},
		{types.LearningConclusion, StrengthConclusion},
		{types.LearningPending, StrengthPending},
	}
	for _, tc := range cases {
		if got := ClassToStrength(tc.class); got != tc.want {
			t.Errorf("ClassToStrength(%v) = %v, want %v", tc.class, got, tc.want)
		}
	}
}

func TestClassToStrength_UnknownReturnsZero(t *testing.T) {
	if got := ClassToStrength(types.LearningUnknown); got != StrengthUnknown {
		t.Errorf("ClassToStrength(LearningUnknown) = %v, want StrengthUnknown (0)", got)
	}
}

func TestCertaintyStrength_String_5Levels(t *testing.T) {
	cases := []struct {
		s    CertaintyStrength
		want string
	}{
		{StrengthSOP, "sop"},
		{StrengthProtocol, "protocol"},
		{StrengthKnowledge, "knowledge"},
		{StrengthConclusion, "conclusion"},
		{StrengthPending, "pending"},
	}
	for _, tc := range cases {
		if got := tc.s.String(); got != tc.want {
			t.Errorf("CertaintyStrength(%d).String() = %q, want %q", uint8(tc.s), got, tc.want)
		}
	}
}

func TestCertaintyStrength_String_UnknownValue(t *testing.T) {
	unknown := CertaintyStrength(99)
	got := unknown.String()
	want := "CertaintyStrength(99)"
	if got != want {
		t.Errorf("unknown CertaintyStrength.String() = %q, want %q", got, want)
	}
}

func TestNewLearningAsset_RequiredFieldsFailFast(t *testing.T) {
	content := &SOPAssetContent{Name: "test_sop", Steps: []string{"step1"}}

	cases := []struct {
		name      string
		id        string
		sessionID string
		class     LearningClass
		content   AssetContent
		assetKey  string
		wantErr   bool
	}{
		{"empty_id", "", "sess_1", types.LearningSOP, content, "key_1", true},
		{"empty_session", "asset_1", "", types.LearningSOP, content, "key_1", true},
		{"unknown_class", "asset_1", "sess_1", types.LearningUnknown, content, "key_1", true},
		{"nil_content", "asset_1", "sess_1", types.LearningSOP, nil, "key_1", true},
		{"empty_asset_key", "asset_1", "sess_1", types.LearningSOP, content, "", true},
		{"valid", "asset_1", "sess_1", types.LearningSOP, content, "key_1", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			asset, err := NewLearningAsset(tc.id, tc.sessionID, tc.class, tc.content, tc.assetKey)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !errors.Is(err, ErrAssetIncomplete) {
					t.Errorf("expected ErrAssetIncomplete, got %v", err)
				}
				if asset != nil {
					t.Error("expected nil asset on error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if asset == nil {
				t.Fatal("expected asset, got nil")
			}
		})
	}
}

func TestNewLearningAsset_AutoTimestamps(t *testing.T) {
	content := &SOPAssetContent{Name: "test_sop", Steps: []string{"step1"}}
	before := time.Now()
	asset, err := NewLearningAsset("asset_1", "sess_1", types.LearningSOP, content, "key_1")
	after := time.Now()
	if err != nil {
		t.Fatalf("NewLearningAsset: %v", err)
	}

	if asset.CreatedAt.Before(before) || asset.CreatedAt.After(after) {
		t.Errorf("CreatedAt = %v, want between %v and %v", asset.CreatedAt, before, after)
	}

	expectedExpiry := asset.CreatedAt.Add(DefaultAssetTTL)
	if !asset.ExpiryAt.Equal(expectedExpiry) {
		t.Errorf("ExpiryAt = %v, want %v (CreatedAt + DefaultAssetTTL)", asset.ExpiryAt, expectedExpiry)
	}

	if asset.ContentHash == "" {
		t.Error("ContentHash should be auto-derived")
	}
	if len(asset.ContentHash) != 16 {
		t.Errorf("ContentHash len = %d, want 16 hex chars", len(asset.ContentHash))
	}
	if !strings.HasPrefix(asset.AssetKey, "") {
		// AssetKey passed through unchanged
		if asset.AssetKey != "key_1" {
			t.Errorf("AssetKey = %q, want %q", asset.AssetKey, "key_1")
		}
	}
}

func TestNewLearningAsset_DefaultFailureCriterion(t *testing.T) {
	content := &SOPAssetContent{Name: "test_sop", Steps: []string{"step1"}}
	asset, err := NewLearningAsset("asset_1", "sess_1", types.LearningSOP, content, "key_1")
	if err != nil {
		t.Fatalf("NewLearningAsset: %v", err)
	}
	if asset.FailureCriterion != "ExpiryAt < now() OR UseCount > MaxUseCount" {
		t.Errorf("FailureCriterion = %q, want default", asset.FailureCriterion)
	}
}

func TestNewLearningAsset_SourceSessionIDsDefaults(t *testing.T) {
	content := &SOPAssetContent{Name: "test_sop", Steps: []string{"step1"}}
	asset, err := NewLearningAsset("asset_1", "sess_1", types.LearningSOP, content, "key_1")
	if err != nil {
		t.Fatalf("NewLearningAsset: %v", err)
	}
	if len(asset.SourceSessionIDs) != 1 || asset.SourceSessionIDs[0] != "sess_1" {
		t.Errorf("SourceSessionIDs = %v, want [sess_1]", asset.SourceSessionIDs)
	}
	if len(asset.SourceVerdictIDs) != 0 {
		t.Errorf("SourceVerdictIDs = %v, want empty", asset.SourceVerdictIDs)
	}
}

func TestNewLearningAsset_ContentValidateError(t *testing.T) {
	// SOPAssetContent with empty Steps fails Validate().
	content := &SOPAssetContent{Name: "test_sop", Steps: []string{}}
	asset, err := NewLearningAsset("asset_1", "sess_1", types.LearningSOP, content, "key_1")
	if err == nil {
		t.Fatal("expected error from Validate(), got nil")
	}
	if !errors.Is(err, ErrAssetIncomplete) {
		t.Errorf("expected ErrAssetIncomplete, got %v", err)
	}
	if asset != nil {
		t.Error("expected nil asset on validation failure")
	}
}

func TestLearningAsset_ImmutableSetters(t *testing.T) {
	content := &SOPAssetContent{Name: "test_sop", Steps: []string{"step1"}}
	asset, err := NewLearningAsset("asset_1", "sess_1", types.LearningSOP, content, "key_1")
	if err != nil {
		t.Fatalf("NewLearningAsset: %v", err)
	}

	// WithTraceID
	withTrace := asset.WithTraceID("trace_xyz")
	if asset.TraceID != "" {
		t.Error("original asset TraceID should remain empty after WithTraceID")
	}
	if withTrace.TraceID != "trace_xyz" {
		t.Errorf("withTrace.TraceID = %q, want %q", withTrace.TraceID, "trace_xyz")
	}

	// WithUseCount
	withUse := asset.WithUseCount()
	if asset.UseCount != 0 {
		t.Error("original asset UseCount should remain 0 after WithUseCount")
	}
	if withUse.UseCount != 1 {
		t.Errorf("withUse.UseCount = %d, want 1", withUse.UseCount)
	}

	// WithSourceVerdictIDs
	withVerdicts := asset.WithSourceVerdictIDs([]string{"v_1", "v_2"})
	if len(asset.SourceVerdictIDs) != 0 {
		t.Error("original asset SourceVerdictIDs should remain empty")
	}
	if len(withVerdicts.SourceVerdictIDs) != 2 {
		t.Errorf("withVerdicts.SourceVerdictIDs len = %d, want 2", len(withVerdicts.SourceVerdictIDs))
	}
}

func TestLearningAsset_IsExpired(t *testing.T) {
	content := &SOPAssetContent{Name: "test_sop", Steps: []string{"step1"}}
	asset, err := NewLearningAsset("asset_1", "sess_1", types.LearningSOP, content, "key_1")
	if err != nil {
		t.Fatalf("NewLearningAsset: %v", err)
	}

	if asset.IsExpired() {
		t.Error("fresh asset should not be expired")
	}

	// Force expire
	past := time.Now().Add(-time.Hour)
	asset.ExpiryAt = past
	if !asset.IsExpired() {
		t.Error("asset with past ExpiryAt should be expired")
	}
}

func TestNewAssetID_UniquePrefix(t *testing.T) {
	id1 := NewAssetID()
	id2 := NewAssetID()
	if id1 == id2 {
		t.Error("two consecutive NewAssetID() should not collide")
	}
	if !strings.HasPrefix(id1, "asset_") {
		t.Errorf("id = %q, want asset_ prefix", id1)
	}
}

func TestLearningAsset_ContentHashStable(t *testing.T) {
	content1 := &SOPAssetContent{Name: "test_sop", Steps: []string{"step1", "step2"}}
	content2 := &SOPAssetContent{Name: "test_sop", Steps: []string{"step1", "step2"}}

	h1 := hashContentBytes(content1)
	h2 := hashContentBytes(content2)
	if h1 != h2 {
		t.Errorf("identical content should produce identical hash; got %q vs %q", h1, h2)
	}

	content3 := &SOPAssetContent{Name: "test_sop", Steps: []string{"step1", "step3"}}
	h3 := hashContentBytes(content3)
	if h1 == h3 {
		t.Errorf("different content should produce different hash; both %q", h1)
	}
}