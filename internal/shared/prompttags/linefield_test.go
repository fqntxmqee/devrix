package prompttags

import "testing"

func TestBuildLineFrame_ObserveUserFrame_Golden(t *testing.T) {
	got := BuildLineFrame(ObserveUserFrame, map[TagName]any{
		TagWorkItemID: "wi_1",
		TagDirective:  "review auth",
		TagPriorMean:  0.75,
		TagScopeGoal:  "fix login",
		TagScopeOpenQuestion: []string{
			"OAuth or JWT?",
			"  ",
			"Cloud or on-prem?",
		},
		TagSignal: []string{"user ping", "tool result"},
	})
	want := "" +
		"work_item_id: wi_1\n" +
		"directive: review auth\n" +
		"prior_mean: 0.750\n" +
		"scope_goal: fix login\n" +
		"scope_open_question: OAuth or JWT?\n" +
		"scope_open_question: Cloud or on-prem?\n" +
		"signal: user ping\n" +
		"signal: tool result\n"
	if got != want {
		t.Fatalf("Observe frame mismatch:\ngot:\n%q\nwant:\n%q", got, want)
	}
}

func TestBuildLineFrame_ObserveUserFrame_Minimal(t *testing.T) {
	got := BuildLineFrame(ObserveUserFrame, map[TagName]any{
		TagWorkItemID: "wi_x",
		TagDirective:  "do x",
	})
	want := "work_item_id: wi_x\n" + "directive: do x\n"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

// T: D7-S16-A96-T03 (DM-20260704-005) Observe frame incremental fields when prior obs present.
func TestBuildLineFrame_ObserveUserFrame_Incremental(t *testing.T) {
	got := BuildLineFrame(ObserveUserFrame, map[TagName]any{
		TagWorkItemID:          "wi_1",
		TagDirective:           "review",
		TagPriorObservationIDs: []string{"obs_a", "obs_b"},
		TagIncrementalOnly:     "true",
	})
	want := "" +
		"work_item_id: wi_1\n" +
		"directive: review\n" +
		"prior_observation_ids: obs_a,obs_b\n" +
		"incremental_only: true\n"
	if got != want {
		t.Fatalf("incremental frame mismatch:\ngot:\n%q\nwant:\n%q", got, want)
	}
}

func TestBuildLineFrame_PlanUserFrame_Golden(t *testing.T) {
	got := BuildLineFrame(PlanUserFrame, map[TagName]any{
		TagWorkItemID:        "wi_42",
		TagDirective:         "review d2 code",
		TagDepth:             1,
		TagMaxDepth:          3,
		TagExistingChildren:  2,
		TagRemainingChildren: 5,
		TagMaxChildren:       7,
		TagDecomposeUsedToday: 1,
		TagRemainingDaily:    4,
		TagMaxDaily:          5,
		TagMaxIters:          5,
		TagParentScopeIn:     []string{"internal/layers/contextengine/", "internal/layers/orchestration/"},
	})
	want := "" +
		"work_item_id: wi_42\n" +
		"directive: review d2 code\n" +
		"depth: 1\n" +
		"max_depth: 3\n" +
		"existing_children: 2\n" +
		"remaining_children: 5\n" +
		"max_children: 7\n" +
		"decompose_used_today: 1\n" +
		"remaining_daily: 4\n" +
		"max_daily: 5\n" +
		"max_iters: 5\n" +
		"parent_scope_in: internal/layers/contextengine/,internal/layers/orchestration/\n"
	if got != want {
		t.Fatalf("Plan frame mismatch:\ngot:\n%q\nwant:\n%q", got, want)
	}
}

func TestBuildLineFrame_PlanUserFrame_WithObservations(t *testing.T) {
	got := BuildLineFrame(PlanUserFrame, map[TagName]any{
		TagWorkItemID:         "wi_1",
		TagDirective:          "plan work",
		TagObservationIDs:     []string{"obs_a", "obs_b"},
		TagObservationSummary: "two open questions",
	})
	want := "" +
		"work_item_id: wi_1\n" +
		"directive: plan work\n" +
		"observation_ids: obs_a,obs_b\n" +
		"observation_summary: two open questions\n"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestBuildLineFrame_PlanUserFrame_FieldOrder(t *testing.T) {
	got := BuildLineFrame(PlanUserFrame, map[TagName]any{
		TagWorkItemID:         "wi",
		TagDirective:          "d",
		TagObservationSummary: "summary",
		TagObservationIDs:     []string{"o1"},
		TagDepth:              1,
		TagMaxDepth:           2,
		TagExistingChildren:   0,
		TagRemainingChildren:  3,
		TagMaxChildren:        3,
		TagDecomposeUsedToday: 0,
		TagRemainingDaily:     5,
		TagMaxDaily:           5,
		TagMaxIters:           4,
		TagParentScopeIn:      []string{"a/"},
		TagUncertaintyMean:    0.42,
	})
	order := []string{
		"work_item_id:",
		"directive:",
		"observation_ids:",
		"observation_summary:",
		"depth:",
		"max_depth:",
		"existing_children:",
		"remaining_children:",
		"max_children:",
		"decompose_used_today:",
		"remaining_daily:",
		"max_daily:",
		"max_iters:",
		"parent_scope_in:",
		"uncertainty_mean:",
	}
	pos := 0
	for _, prefix := range order {
		idx := indexFrom(got, prefix, pos)
		if idx < 0 {
			t.Fatalf("missing %q in output:\n%s", prefix, got)
		}
		pos = idx
	}
}

func indexFrom(s, sub string, from int) int {
	if from >= len(s) {
		return -1
	}
	idx := -1
	for i := from; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			idx = i
			break
		}
	}
	return idx
}
