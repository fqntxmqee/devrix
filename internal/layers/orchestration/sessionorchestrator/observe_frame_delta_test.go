// Package sessionorchestrator — observe_frame_delta_test.go
//
// DM-20260705-010 (devrix-d7-mups-frame-delta-closure) Phase 2 T10:
// 6 子测试覆盖 Observe→Plan frame delta 闭环节流契约 (design.md §3.2)。
//
// 子测试清单 (AC1 + AC2 + AC5):
//  1. TestBuildObservePriorDelta_FirstRoundZero — prevExecCtx nil → FrameDelta{} 零值
//  2. TestBuildObservePriorDelta_NonFirstRoundHasArtifactSummary — 非首轮含上一轮收敛度量 (从 LastRound.ArtifactSummary 截断到 ≤80)
//  3. TestBuildObservePriorDelta_KnownGapsPhase2StubEmpty — Phase 2 stub known_gaps 留空数组 (Phase 3+ 接入 LastPlanScopeIn - ObservedResolved)
//  4. TestBuildObservePriorDelta_ObserveUserFrame_Contract — 封闭式 JSON 不破坏 (BuildLineFrame 11 字段契约 0 修改)
//  5. TestBuildObservePriorDelta_FrameObserveUserAppendOnly — 既存 9 字段顺序 0 修改 (append-only 新增 2 字段)
//  6. TestBuildObservePriorDelta_I18nKeysComplete — i18n en + zh CondPriorArtifactSummary + CondKnownGaps 键完整
//
// Span emit (telemetry) 不在断言中 — hardening nil-bridge + deferred end,
// 测试环境无 telemetry bridge, span 调用走 no-op 安全路径.
package sessionorchestrator

import (
	"context"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/i18n"
	"github.com/devrix/devrix/internal/layers/orchestration/interfaces"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/prompttags"
)

// TestBuildObservePriorDelta_FirstRoundZero — case 1: 首轮零值.
func TestBuildObservePriorDelta_FirstRoundZero(t *testing.T) {
	ctx := context.Background()
	// nil prevExecCtx → fresh session → no signal
	if got := BuildObservePriorDelta(ctx, "sess_test", nil); !got.IsZero() {
		t.Fatalf("nil prevExecCtx should return zero FrameDelta, got %+v", got)
	}
	// nil Item.LastRound → first round after item creation
	ec := &WorkItemExecContext{Item: nil}
	if got := BuildObservePriorDelta(ctx, "sess_test", ec); !got.IsZero() {
		t.Fatalf("nil Item should return zero FrameDelta, got %+v", got)
	}
	ec = &WorkItemExecContext{Item: &workmodel.WorkItem{}}
	if got := BuildObservePriorDelta(ctx, "sess_test", ec); !got.IsZero() {
		t.Fatalf("nil LastRound should return zero FrameDelta, got %+v", got)
	}
}

// TestBuildObservePriorDelta_NonFirstRoundHasArtifactSummary — case 2: 非首轮含上一轮收敛度量.
func TestBuildObservePriorDelta_NonFirstRoundHasArtifactSummary(t *testing.T) {
	ctx := context.Background()
	longSummary := strings.Repeat("a", 200) // > 80 char
	prev := &WorkItemExecContext{
		Item: &workmodel.WorkItem{
			LastRound: &workmodel.WorkItemPipelineRound{
				ArtifactSummary: longSummary,
			},
		},
	}
	fd := BuildObservePriorDelta(ctx, "sess_test", prev)
	if fd.IsZero() {
		t.Fatal("non-zero prevExecCtx with ArtifactSummary should produce non-zero FrameDelta")
	}
	if len(fd.PriorArtifactSummary) > interfaces.MaxPriorArtifactSummaryChars {
		t.Fatalf("PriorArtifactSummary length %d exceeds MaxPriorArtifactSummaryChars=%d",
			len(fd.PriorArtifactSummary), interfaces.MaxPriorArtifactSummaryChars)
	}
	if !strings.HasSuffix(fd.PriorArtifactSummary, "...") {
		t.Fatalf("expected truncated summary to end with '...', got %q", fd.PriorArtifactSummary)
	}
}

// TestBuildObservePriorDelta_KnownGapsPhase2StubEmpty — case 3: known_gaps Phase 2 stub 空数组.
func TestBuildObservePriorDelta_KnownGapsPhase2StubEmpty(t *testing.T) {
	ctx := context.Background()
	prev := &WorkItemExecContext{
		Item: &workmodel.WorkItem{
			LastRound: &workmodel.WorkItemPipelineRound{
				ArtifactSummary: "Round 1: committed",
			},
		},
	}
	fd := BuildObservePriorDelta(ctx, "sess_test", prev)
	if len(fd.KnownGaps) != 0 {
		t.Fatalf("Phase 2 stub KnownGaps should be empty (Phase 3+ will fill from LastPlanScopeIn - ObservedResolved), got %v", fd.KnownGaps)
	}
}

// TestBuildObservePriorDelta_ObserveUserFrame_Contract — case 4: BuildLineFrame 11 字段契约 0 破坏.
// 既存 9 字段顺序不变; append-only 新增 2 字段.
func TestBuildObservePriorDelta_ObserveUserFrame_Contract(t *testing.T) {
	spec, ok := prompttags.LineFrameRegistry[prompttags.FrameObserveUser]
	if !ok {
		t.Fatal("FrameObserveUser missing from LineFrameRegistry")
	}
	if len(spec.Fields) != 11 {
		t.Fatalf("FrameObserveUser has %d fields, want 11 (DM-20260705-010 v1.1): %v", len(spec.Fields), spec.Fields)
	}
	expected := []prompttags.TagName{
		prompttags.TagWorkItemID,
		prompttags.TagDirective,
		prompttags.TagPriorParseReject,
		prompttags.TagPriorMean,
		prompttags.TagScopeGoal,
		prompttags.TagScopeOpenQuestion,
		prompttags.TagSignal,
		prompttags.TagPriorObservationIDs,
		prompttags.TagIncrementalOnly,
		prompttags.TagPriorArtifactSummary,
		prompttags.TagKnownGaps,
	}
	for i, want := range expected {
		if spec.Fields[i] != want {
			t.Errorf("field[%d] = %q, want %q (既存 9 字段顺序 0 修改; append-only 2 字段)",
				i, spec.Fields[i], want)
		}
	}
	// BuildLineFrame produces 11 lines: 9 original data lines + 2 new lines.
	fields := map[prompttags.TagName]any{
		prompttags.TagWorkItemID:          "wi_d0_s0_goal",
		prompttags.TagDirective:           "ship login v2",
		prompttags.TagPriorParseReject:    "",
		prompttags.TagPriorMean:           0.0,
		prompttags.TagScopeGoal:           "ship v2",
		prompttags.TagScopeOpenQuestion:  []string{},
		prompttags.TagSignal:              []string{"artifact_summary: r1 done"},
		prompttags.TagPriorObservationIDs: []string{"obs-1"},
		prompttags.TagIncrementalOnly:     true,
		prompttags.TagPriorArtifactSummary: "Round 1: 60% converged",
		prompttags.TagKnownGaps:           []string{"missing:ux_flow", "unresolved:a1b2c3"},
	}
	out := prompttags.BuildLineFrame(spec, fields)
	if !strings.Contains(out, "prior_artifact_summary: Round 1: 60% converged") {
		t.Fatalf("output must contain prior_artifact_summary line, got:\n%s", out)
	}
	if !strings.Contains(out, "known_gaps: missing:ux_flow") || !strings.Contains(out, "known_gaps: unresolved:a1b2c3") {
		t.Fatalf("output must emit one line per known_gaps entry, got:\n%s", out)
	}
}

// TestBuildObservePriorDelta_FrameObserveUserAppendOnly — case 5: 既存 9 字段契约 0 修改.
// 字段顺序保证: 既存 9 字段在 frame 前 9 位, append 2 字段在 10/11 位.
func TestBuildObservePriorDelta_FrameObserveUserAppendOnly(t *testing.T) {
	spec := prompttags.ObserveUserFrame
	for i := 0; i < 9; i++ {
		switch spec.Fields[i] {
		case prompttags.TagWorkItemID,
			prompttags.TagDirective,
			prompttags.TagPriorParseReject,
			prompttags.TagPriorMean,
			prompttags.TagScopeGoal,
			prompttags.TagScopeOpenQuestion,
			prompttags.TagSignal,
			prompttags.TagPriorObservationIDs,
			prompttags.TagIncrementalOnly:
			// OK: 既存 9 字段在 frame 前 9 位
		default:
			t.Errorf("field[%d] = %q is not one of the original 9 fields; append-only contract violated",
				i, spec.Fields[i])
		}
	}
	// Append-only: 字段 10/11 是新增的 2 字段
	if spec.Fields[9] != prompttags.TagPriorArtifactSummary {
		t.Errorf("field[9] = %q, want TagPriorArtifactSummary (append-only at position 10)", spec.Fields[9])
	}
	if spec.Fields[10] != prompttags.TagKnownGaps {
		t.Errorf("field[10] = %q, want TagKnownGaps (append-only at position 11)", spec.Fields[10])
	}
}

// TestBuildObservePriorDelta_I18nKeysComplete — case 6: i18n en + zh 键完整.
func TestBuildObservePriorDelta_I18nKeysComplete(t *testing.T) {
	// Render FrameObserveUser guide in en + zh, verify 2 new keys emit.
	for _, loc := range []i18n.Locale{i18n.LocaleEN, i18n.LocaleZH} {
		fields := map[prompttags.TagName]any{
			prompttags.TagWorkItemID:           "wi_1",
			prompttags.TagPriorArtifactSummary: "Round 1: committed",
			prompttags.TagKnownGaps:            []string{"missing:auth"},
		}
		guide := i18n.RenderFrameFieldGuideForFields(prompttags.FrameObserveUser, loc, fields)
		if guide == "" {
			t.Fatalf("%s: guide is empty", loc)
		}
		// i18n guide contains the CondPriorArtifactSummary / CondKnownGaps labels.
		// Hard-code the assertions: en should have "Prior-round artifact summary",
		// zh should have "上一轮 artifact 摘要".
		switch loc {
		case i18n.LocaleEN:
			if !strings.Contains(guide, "Prior-round artifact summary") {
				t.Fatalf("en guide missing prior_artifact_summary label:\n%s", guide)
			}
			if !strings.Contains(guide, "Known unresolved gap IDs") {
				t.Fatalf("en guide missing known_gaps label:\n%s", guide)
			}
		case i18n.LocaleZH:
			if !strings.Contains(guide, "上一轮 artifact 摘要") {
				t.Fatalf("zh guide missing prior_artifact_summary label:\n%s", guide)
			}
			if !strings.Contains(guide, "已知未闭合 gap ID") {
				t.Fatalf("zh guide missing known_gaps label:\n%s", guide)
			}
		}
	}
}