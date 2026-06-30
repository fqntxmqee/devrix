package sessionorchestrator

import (
	"strings"
	"testing"
)

// TestLastTextQualityGate_4Kinds — DM-20260630-011 AC1 regression.
//
// Verifies the 4-way classification: valid / thin / too_short /
// inconclusive. Each case corresponds to a sess_1782814140202_7000
// regression vector:
//
//   - valid:        real review findings, ≥ 400 runes
//   - thin:         borderline (200..400 runes), no marker
//   - too_short:    pipeline artifact or empty summary (< 100 runes)
//   - inconclusive: short text + template marker (planning recap leak)
func TestLastTextQualityGate_4Kinds(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantKind SummaryQualityKind
	}{
		{
			name:    "valid_long_review_no_marker",
			input:   strings.Repeat("这是审查发现的详细说明。", 50), // ~600 runes
			wantKind: SummaryQualityValid,
		},
		{
			name:    "thin_borderline_no_marker",
			input:   strings.Repeat("结论基本合理，", 30), // ~180 runes < 400
			wantKind: SummaryQualityThin,
		},
		{
			name:    "too_short_pipeline_artifact",
			input:   "ok",
			wantKind: SummaryQualityTooShort,
		},
		{
			name:    "too_short_empty_summary",
			input:   "",
			wantKind: SummaryQualityTooShort,
		},
		{
			name:    "inconclusive_short_with_scope_contract_marker",
			input:   "<scope_contract> 任务范围限定。Need a real review here for 100+ runes. " +
				strings.Repeat("...", 30),
			wantKind: SummaryQualityInconclusive,
		},
		{
			name: "inconclusive_short_with_planning_marker",
			input: "<planning> 分配计划步骤. " +
				strings.Repeat("a", 100),
			wantKind: SummaryQualityInconclusive,
		},
		{
			name:    "valid_long_with_marker_still_classified_as_inconclusive",
			input:   strings.Repeat("审查结论。", 100) + "<planning>", // long but marker present
			wantKind: SummaryQualityInconclusive,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifyLastTextQuality(tc.input)
			if got.Kind != tc.wantKind {
				t.Fatalf("ClassifyLastTextQuality(%d runes, marker=%v) = %v, want %v",
					got.RuneLen,
					containsLastTextMarker(strings.TrimSpace(tc.input)),
					got.Kind, tc.wantKind)
			}
		})
	}
}

// TestLastTextQualityGate_MarkerCaseInsensitive verifies the marker
// detection is case-insensitive (LLM may emit <Planning> or <PLANNING>).
func TestLastTextQualityGate_MarkerCaseInsensitive(t *testing.T) {
	cases := []string{
		"<PLANNING>",
		"<Planning>",
		"<planning>",
		"<SCOPE_CONTRACT>",
		"<Scope_Contract>",
	}
	for _, marker := range cases {
		if !containsLastTextMarker(marker) {
			t.Errorf("containsLastTextMarker(%q) = false, want true (case-insensitive)", marker)
		}
	}
}

// TestLastTextQualityGate_NoFalsePositiveForCode confirms that the
// classifier doesn't false-positive on incidental occurrences of
// "planning" in legitimate text. The classifier only flags the
// exact <marker> form, not the bare word.
func TestLastTextQualityGate_NoFalsePositiveForCode(t *testing.T) {
	input := "I am planning the deployment plan for production rollout. Critical changes reviewed."
	res := ClassifyLastTextQuality(input)
	if res.Kind == SummaryQualityInconclusive && len([]rune(input)) < lastTextQualityThinMaxRunes {
		t.Errorf("incidental 'planning' word shouldn't trigger inconclusive: got %v", res.Kind)
	}
}
