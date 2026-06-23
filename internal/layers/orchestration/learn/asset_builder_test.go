package learn

import (
	"context"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/plan"
	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/types"
)

// ─────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────

func makeReq(t *testing.T, kind types.VerdictKind, sessionID, sourceID, reason string) LearnRequest {
	t.Helper()
	return LearnRequest{
		SessionID: sessionID,
		Verdict: workmodel.Verdict{
			Kind:     kind,
			SourceID: sourceID,
			Reason:   reason,
		},
	}
}

func makeReqWithPlan(t *testing.T, kind types.VerdictKind, sessionID, sourceID, planID string) LearnRequest {
	t.Helper()
	p := plan.NewPlan(planID, sessionID, plan.CommitmentPlan, []string{"obs_1"}, []plan.Step{{ID: "step_1"}}, 0.8)
	return LearnRequest{
		SessionID: sessionID,
		Verdict: workmodel.Verdict{
			Kind:     kind,
			SourceID: sourceID,
			Reason:   "verdict reason",
		},
		Plan: p,
	}
}

func makeReqWithArtifact(t *testing.T, kind types.VerdictKind, sessionID, sourceID, planID, summary string) LearnRequest {
	t.Helper()
	req := makeReqWithPlan(t, kind, sessionID, sourceID, planID)
	req.Artifact = &wavescheduler.Artifact{
		TaskID:    "art_1",
		SessionID: sessionID,
		Summary:   summary,
		FilesChanged: []string{"foo.go"},
	}
	return req
}

// ─────────────────────────────────────────────────────────────────────────
// AssetBuilder: 5 typed content constructors
// ─────────────────────────────────────────────────────────────────────────

func TestAssetBuilder_BuildSOPAsset(t *testing.T) {
	b := NewAssetBuilder()
	req := makeReqWithPlan(t, types.VerdictPass, "sess_1", "v_pass", "plan_1")
	req.Artifact = &wavescheduler.Artifact{
		TaskID:       "art_1",
		SessionID:    "sess_1",
		Summary:      "sop summary",
		FilesChanged: []string{"foo.go", "bar.go"},
	}

	asset, err := b.Build(context.Background(), req, LearningClass(types.LearningSOP))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if asset.Class != LearningClass(types.LearningSOP) {
		t.Errorf("Class = %s, want LearningSOP", asset.Class)
	}
	if asset.Strength != StrengthSOP {
		t.Errorf("Strength = %d, want StrengthSOP(%d)", asset.Strength, StrengthSOP)
	}
	sop, ok := asset.Content.(*SOPAssetContent)
	if !ok {
		t.Fatalf("Content type = %T, want *SOPAssetContent", asset.Content)
	}
	if !strings.HasPrefix(sop.Name, "sop:plan:") {
		t.Errorf("Name = %q, want prefix sop:plan:", sop.Name)
	}
	if len(sop.Steps) == 0 {
		t.Error("Steps empty, want ≥1")
	}
	if len(sop.ApplicableTools) != 2 {
		t.Errorf("ApplicableTools len = %d, want 2", len(sop.ApplicableTools))
	}
}

func TestAssetBuilder_BuildProtocolAsset(t *testing.T) {
	b := NewAssetBuilder()
	req := makeReqWithPlan(t, types.VerdictPartial, "sess_1", "v_partial", "plan_1")

	asset, err := b.Build(context.Background(), req, LearningClass(types.LearningProtocol))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if asset.Class != LearningClass(types.LearningProtocol) {
		t.Errorf("Class = %s, want LearningProtocol", asset.Class)
	}
	proto, ok := asset.Content.(*ProtocolAssetContent)
	if !ok {
		t.Fatalf("Content type = %T, want *ProtocolAssetContent", asset.Content)
	}
	if !strings.HasPrefix(proto.Trigger, "on:") {
		t.Errorf("Trigger = %q, want prefix on:", proto.Trigger)
	}
	if proto.SLA.MaxRetries == 0 {
		t.Error("SLA.MaxRetries = 0, want ≥1 (default)")
	}
}

func TestAssetBuilder_BuildKnowledgeAsset(t *testing.T) {
	b := NewAssetBuilder()
	req := makeReq(t, types.VerdictFail, "sess_1", "v_fail", "root_cause: deployment failed")
	req.Artifact = &wavescheduler.Artifact{TaskID: "art_1", SessionID: "sess_1", Summary: "summary"}

	asset, err := b.Build(context.Background(), req, LearningClass(types.LearningKnowledge))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	know, ok := asset.Content.(*KnowledgeAssetContent)
	if !ok {
		t.Fatalf("Content type = %T, want *KnowledgeAssetContent", asset.Content)
	}
	if know.Topic == "" || know.Hypothesis == "" {
		t.Errorf("Topic/Hypothesis = %q/%q, both required", know.Topic, know.Hypothesis)
	}
	if len(know.Evidence) == 0 {
		t.Error("Evidence empty, want ≥1")
	}
}

func TestAssetBuilder_BuildConclusionAsset(t *testing.T) {
	b := NewAssetBuilder()
	req := makeReq(t, types.VerdictFail, "sess_1", "v_fail", "statistical failure: regression")

	asset, err := b.Build(context.Background(), req, LearningClass(types.LearningConclusion))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	conc, ok := asset.Content.(*ConclusionAssetContent)
	if !ok {
		t.Fatalf("Content type = %T, want *ConclusionAssetContent", asset.Content)
	}
	if conc.Statement == "" {
		t.Error("Statement empty, want non-empty")
	}
	if conc.PValue != 0.05 {
		t.Errorf("PValue = %g, want 0.05 (default)", conc.PValue)
	}
}

func TestAssetBuilder_BuildPendingAsset(t *testing.T) {
	b := NewAssetBuilder()
	req := makeReq(t, types.VerdictIndeterminate, "sess_1", "v_indet", "")
	req.Verdict.IndeterminateReason = "env_limited"
	req.Artifact = &wavescheduler.Artifact{TaskID: "art_orig", SessionID: "sess_1"}

	asset, err := b.Build(context.Background(), req, LearningClass(types.LearningPending))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	pending, ok := asset.Content.(*PendingAssetContent)
	if !ok {
		t.Fatalf("Content type = %T, want *PendingAssetContent", asset.Content)
	}
	if pending.IndeterminateReason != "env_limited" {
		t.Errorf("IndeterminateReason = %q, want env_limited", pending.IndeterminateReason)
	}
	if pending.OriginalArtifactID != "art_orig" {
		t.Errorf("OriginalArtifactID = %q, want art_orig", pending.OriginalArtifactID)
	}
	if pending.MaxRetries != DefaultScheduledMaxRetries {
		t.Errorf("MaxRetries = %d, want %d", pending.MaxRetries, DefaultScheduledMaxRetries)
	}
	if pending.RetryAttempts != 0 {
		t.Errorf("RetryAttempts = %d, want 0", pending.RetryAttempts)
	}
}

// ─────────────────────────────────────────────────────────────────────────
// AssetKey format + ContentHash stability
// ─────────────────────────────────────────────────────────────────────────

func TestAssetBuilder_AssetKeyFormat(t *testing.T) {
	b := NewAssetBuilder()
	req := makeReqWithPlan(t, types.VerdictPass, "sess_1", "v_pass", "plan_1")
	asset, err := b.Build(context.Background(), req, LearningClass(types.LearningSOP))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	// Format: "{class}:{source}:{hash}"
	parts := strings.Split(asset.AssetKey, ":")
	if len(parts) != 3 {
		t.Errorf("AssetKey parts = %d, want 3: %q", len(parts), asset.AssetKey)
	}
	if parts[0] != "sop" {
		t.Errorf("AssetKey[0] = %q, want sop", parts[0])
	}
	if parts[1] != "v_pass" {
		t.Errorf("AssetKey[1] = %q, want v_pass", parts[1])
	}
	if len(parts[2]) != 16 {
		t.Errorf("AssetKey[2] len = %d, want 16 (hex prefix)", len(parts[2]))
	}
}

func TestAssetBuilder_ContentHash_Stable(t *testing.T) {
	b := NewAssetBuilder()
	req1 := makeReqWithPlan(t, types.VerdictPass, "sess_1", "v_pass", "plan_1")
	req2 := makeReqWithPlan(t, types.VerdictPass, "sess_1", "v_pass", "plan_1")

	a1, err := b.Build(context.Background(), req1, LearningClass(types.LearningSOP))
	if err != nil {
		t.Fatalf("Build1: %v", err)
	}
	a2, err := b.Build(context.Background(), req2, LearningClass(types.LearningSOP))
	if err != nil {
		t.Fatalf("Build2: %v", err)
	}
	if a1.ContentHash != a2.ContentHash {
		t.Errorf("Hash differs: %s vs %s", a1.ContentHash, a2.ContentHash)
	}
}

func TestAssetBuilder_ClassToStrength_5Levels(t *testing.T) {
	cases := []struct {
		class LearningClass
		want  CertaintyStrength
	}{
		{LearningClass(types.LearningSOP), StrengthSOP},
		{LearningClass(types.LearningProtocol), StrengthProtocol},
		{LearningClass(types.LearningKnowledge), StrengthKnowledge},
		{LearningClass(types.LearningConclusion), StrengthConclusion},
		{LearningClass(types.LearningPending), StrengthPending},
		{LearningClass(types.LearningUnknown), StrengthUnknown},
	}
	for _, tc := range cases {
		got := ClassToStrength(tc.class)
		if got != tc.want {
			t.Errorf("ClassToStrength(%s) = %d, want %d", tc.class, got, tc.want)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────
// VerdictKind → LearningClass routing
// ─────────────────────────────────────────────────────────────────────────

func TestClassFromVerdictKind_4Kinds(t *testing.T) {
	cases := []struct {
		kind   types.VerdictKind
		reason string
		want   LearningClass
	}{
		{types.VerdictPass, "", LearningClass(types.LearningSOP)},
		{types.VerdictPartial, "", LearningClass(types.LearningProtocol)},
		{types.VerdictFail, "root cause: timeout", LearningClass(types.LearningKnowledge)},
		{types.VerdictFail, "statistical regression detected", LearningClass(types.LearningConclusion)},
		{types.VerdictIndeterminate, "", LearningClass(types.LearningPending)},
	}
	for _, tc := range cases {
		got := classFromVerdictKind(tc.kind, tc.reason)
		if got != tc.want {
			t.Errorf("classFromVerdictKind(%v, %q) = %s, want %s", tc.kind, tc.reason, got, tc.want)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Fail-fast: empty SessionID, missing required fields
// ─────────────────────────────────────────────────────────────────────────

func TestAssetBuilder_EmptySessionID_FailFast(t *testing.T) {
	b := NewAssetBuilder()
	req := LearnRequest{SessionID: "", Verdict: workmodel.Verdict{Kind: types.VerdictPass}}
	_, err := b.Build(context.Background(), req, LearningClass(types.LearningSOP))
	if err == nil {
		t.Error("empty SessionID should fail-fast")
	}
}

func TestAssetBuilder_SOPMissingSteps_ReturnsNil(t *testing.T) {
	b := NewAssetBuilder()
	// Pass verdict with no Plan, no Artifact, AND empty SourceID → no name
	// (no Plan → no plan: name, no Artifact → no summary: name, no SourceID
	// → no autoclose: name) → builder returns nil.
	// (Phase 7 PR-7.1: with a non-empty SourceID, the Auto-Close fallback
	//  kicks in and produces a storable SOP. See TestAssetBuilder_AutoCloseFallback.)
	req := makeReq(t, types.VerdictPass, "sess_1", "", "")
	asset, err := b.Build(context.Background(), req, LearningClass(types.LearningSOP))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if asset != nil {
		t.Error("SOP without Plan/Artifact/SourceID should return nil (signals ErrAssetBuildFailed)")
	}
}

// TestAssetBuilder_AutoCloseFallback verifies Phase 7 PR-7.1 (D7-S13-A47-T02):
// when a VerdictPass comes through processAutoClose (no Plan, no Artifact, but
// SourceID is set), the AssetBuilder falls back to the autoclose: name +
// synthetic step so the Learn deposit completes end-to-end.
func TestAssetBuilder_AutoCloseFallback(t *testing.T) {
	b := NewAssetBuilder()
	req := makeReq(t, types.VerdictPass, "sess_ac", "autoclose:sess_ac:12345", "process complete")
	asset, err := b.Build(context.Background(), req, LearningClass(types.LearningSOP))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if asset == nil {
		t.Fatal("Auto-Close fallback should produce a non-nil asset when SourceID is set")
	}
	sop, ok := asset.Content.(*SOPAssetContent)
	if !ok {
		t.Fatalf("Content type = %T, want *SOPAssetContent", asset.Content)
	}
	if !strings.HasPrefix(sop.Name, "sop:autoclose:") {
		t.Errorf("Name = %q, want prefix sop:autoclose:", sop.Name)
	}
	if len(sop.Steps) != 1 || sop.Steps[0] != "autoclose-completion" {
		t.Errorf("Steps = %v, want [autoclose-completion]", sop.Steps)
	}
}