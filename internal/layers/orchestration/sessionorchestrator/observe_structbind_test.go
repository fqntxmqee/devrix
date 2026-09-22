package sessionorchestrator

import (
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/i18n"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/prompttags"
)

// frameBody extracts the user-prompt frame (after the i18n guide header) for
// field-level assertions. The header + frame are joined by "\n\n" in
// buildLLMObservationUserPrompt.
func frameBody(s string) string {
	if i := strings.Index(s, "\n\n"); i >= 0 {
		return s[i+2:]
	}
	return s
}

// T: D7-S5-A99-T01 / L5-MUPS-GSD-01 — init() in this package must have registered
// ObserveSignalInput with LineFrameRegistry[FrameObserveUser] without panic.
func TestObserveSignalInput_RegisteredAtInit(t *testing.T) {
	spec, ok := prompttags.LineFrameRegistry[prompttags.FrameObserveUser]
	if !ok {
		t.Fatal("FrameObserveUser missing from LineFrameRegistry")
	}
	// DM-20260705-010 Phase 2 T8: 9 → 11 字段契约 (append-only 加 prior_artifact_summary + known_gaps).
	if len(spec.Fields) != 11 {
		t.Fatalf("FrameObserveUser has %d fields, want 11 (DM-20260705-010 v1.1): %v", len(spec.Fields), spec.Fields)
	}
	expected := []prompttags.TagName{
		prompttags.TagWorkItemID, prompttags.TagDirective, prompttags.TagPriorParseReject,
		prompttags.TagPriorMean, prompttags.TagScopeGoal, prompttags.TagScopeOpenQuestion,
		prompttags.TagSignal, prompttags.TagPriorObservationIDs, prompttags.TagIncrementalOnly,
	}
	for i, want := range expected {
		if spec.Fields[i] != want {
			t.Errorf("field[%d] = %q, want %q", i, spec.Fields[i], want)
		}
	}
}

// T: D7-S5-A99-T04 / L5-MUPS-GSD-02 — buildLLMObservationUserPrompt output
// contains classifier-visible fields only (DM-20260705-004).
func TestBuildLLMObservationUserPrompt_FullInput(t *testing.T) {
	in := ObserveSignalInput{
		SessionID:           "s1",
		WorkItemID:          "wi_1",
		Directive:           "refactor login",
		PriorParseReject:    `{"code":"parse_fail","field":"kind"}`,
		PriorMean:           0.6,
		ScopeGoal:           "ship login v2",
		ScopeOpenQuestions:  []string{"q1", "q2"},
		InboundSignalLines:  []string{"artifact_summary: prior step ok", "expected_return: token"},
		PriorObservationIDs: []string{"obs_1", "obs_2"},
		IncrementalOnly:     true,
	}
	body := frameBody(buildLLMObservationUserPrompt(in, i18n.LocaleEN))
	mustContain := []string{
		"directive: refactor login",
		"prior_parse_reject:",
		"scope_goal: ship login v2",
		"signal: artifact_summary: prior step ok",
		"prior_observation_ids: obs_1,obs_2",
	}
	for _, want := range mustContain {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q in:\n%s", want, body)
		}
	}
	mustNotContain := []string{
		"work_item_id:",
		"prior_mean:",
		"incremental_only:",
		"[control]",
		"[data]",
	}
	for _, banned := range mustNotContain {
		if strings.Contains(body, banned) {
			t.Errorf("orchestration-only field leaked %q in:\n%s", banned, body)
		}
	}
}

// T: D7-S5-A99-T04 / L5-MUPS-GSD-02 — empty optional fields are omitted from frame body.
func TestBuildLLMObservationUserPrompt_OmitEmpty(t *testing.T) {
	in := ObserveSignalInput{
		SessionID:  "s1",
		WorkItemID: "wi_1",
		Directive:  "do thing",
	}
	body := frameBody(buildLLMObservationUserPrompt(in, i18n.LocaleEN))
	mustNotContain := []string{
		"prior_parse_reject:",
		"scope_goal:",
		"scope_open_question:",
		"signal:",
		"prior_observation_ids:",
		"work_item_id:",
	}
	for _, banned := range mustNotContain {
		if strings.Contains(body, banned) {
			t.Errorf("frame should omit %q but found in:\n%s", banned, body)
		}
	}
	if !strings.Contains(body, "directive: do thing") {
		t.Errorf("missing directive in:\n%s", body)
	}
}

// T: D7-S5-A99-T02 — buildObserveSignalInput flattens ScopeContract and
// computes IncrementalOnly from PriorObservationIDs.
func TestBuildObserveSignalInput_FlattensScopeContract(t *testing.T) {
	tm := workmodel.NewTaskManager()
	item, err := tm.EnsureGoal("s1", "build feature")
	if err != nil {
		t.Fatal(err)
	}
	item.ScopeContract = &workmodel.ScopeContract{
		GoalStatement: "ship login v2",
		OpenQuestions: []string{"q1", "", "q3"},
	}
	item.LastRound = &workmodel.WorkItemPipelineRound{
		ObservationIDs: []string{"obs_1"},
	}
	in := buildObserveSignalInput("s1", item, tm, nil)
	if in.ScopeGoal != "ship login v2" {
		t.Errorf("ScopeGoal = %q, want %q", in.ScopeGoal, "ship login v2")
	}
	if len(in.ScopeOpenQuestions) != 2 || in.ScopeOpenQuestions[0] != "q1" || in.ScopeOpenQuestions[1] != "q3" {
		t.Errorf("ScopeOpenQuestions = %v, want [q1 q3]", in.ScopeOpenQuestions)
	}
	if !in.IncrementalOnly {
		t.Error("IncrementalOnly = false, want true (PriorObservationIDs non-empty)")
	}
	if len(in.PriorObservationIDs) != 1 || in.PriorObservationIDs[0] != "obs_1" {
		t.Errorf("PriorObservationIDs = %v, want [obs_1]", in.PriorObservationIDs)
	}
}

// T: D7-S5-A99-T02 — nil ScopeContract and nil LastRound yield zero flat fields.
func TestBuildObserveSignalInput_NilScopeContract(t *testing.T) {
	tm := workmodel.NewTaskManager()
	item, err := tm.EnsureGoal("s1", "build feature")
	if err != nil {
		t.Fatal(err)
	}
	item.ScopeContract = nil
	item.LastRound = nil
	in := buildObserveSignalInput("s1", item, tm, nil)
	if in.ScopeGoal != "" {
		t.Errorf("ScopeGoal = %q, want empty", in.ScopeGoal)
	}
	if in.ScopeOpenQuestions != nil {
		t.Errorf("ScopeOpenQuestions = %v, want nil", in.ScopeOpenQuestions)
	}
	if in.IncrementalOnly {
		t.Error("IncrementalOnly = true, want false (no prior obs)")
	}
	if in.PriorObservationIDs != nil {
		t.Errorf("PriorObservationIDs = %v, want nil", in.PriorObservationIDs)
	}
}

// T: D7-S5-A99-T04 / L5-MUPS-GSD-02 — golden snapshot for ZH locale: guide lists
// only present classifier-visible fields (DM-20260705-004).
func TestBuildLLMObservationUserPrompt_GoldenZH(t *testing.T) {
	in := ObserveSignalInput{
		SessionID:           "s1",
		WorkItemID:          "wi_1",
		Directive:           "refactor login",
		PriorParseReject:    `{"code":"parse_fail"}`,
		PriorMean:           0.5,
		ScopeGoal:           "ship login v2",
		ScopeOpenQuestions:  []string{"q1"},
		PriorObservationIDs: []string{"obs_1"},
		IncrementalOnly:     true,
	}
	got := buildLLMObservationUserPrompt(in, i18n.LocaleZH)
	if !strings.Contains(got, "User 帧字段") {
		t.Errorf("missing zh user-frame guide header: %s", got)
	}
	if strings.Contains(got, "work_item_id") || strings.Contains(got, "prior_mean") {
		t.Errorf("orchestration fields should not appear in LLM prompt: %s", got)
	}
	body := frameBody(got)
	frameLines := strings.Split(strings.TrimSpace(body), "\n")
	if len(frameLines) != 4 {
		t.Errorf("frame lines = %d, want 4 (P3: scope_open_question Go-only):\n%s", len(frameLines), body)
	}
}
