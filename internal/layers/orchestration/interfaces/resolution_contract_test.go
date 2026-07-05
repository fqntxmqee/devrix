package interfaces

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestResolutionStrategy_ObsIDRequired(t *testing.T) {
	_, err := NewResolutionStrategy("", "read_file", "file_count > 0")
	if !errors.Is(err, ErrResolutionStrategyObsIDRequired) {
		t.Fatalf("err = %v, want ErrResolutionStrategyObsIDRequired", err)
	}
}

func TestResolutionStrategy_HappyPath(t *testing.T) {
	s, err := NewResolutionStrategy("obs-1", "read_file", "file_count > 0")
	if err != nil {
		t.Fatalf("NewResolutionStrategy failed: %v", err)
	}
	if s.ObsID != "obs-1" {
		t.Errorf("ObsID = %q, want obs-1", s.ObsID)
	}
	if s.PlannedTool != "read_file" {
		t.Errorf("PlannedTool = %q, want read_file", s.PlannedTool)
	}
	if s.HasSubWorktree() {
		t.Error("HasSubWorktree = true, want false (no SubWorktree attached)")
	}
}

func TestResolutionStrategy_WithSubWorktree(t *testing.T) {
	s, err := NewResolutionStrategy("obs-1", "read_file", "file_count > 0")
	if err != nil {
		t.Fatalf("NewResolutionStrategy: %v", err)
	}
	spec := SubWorktreeSpec{
		Title:           "investigate d7 plan dir",
		DirectiveSuffix: "list all .go files in plan/",
		ExpectedReturn:  "file list with sizes",
	}
	out, err := s.WithSubWorktree(&spec)
	if err != nil {
		t.Fatalf("WithSubWorktree: %v", err)
	}
	if !out.HasSubWorktree() {
		t.Error("HasSubWorktree = false, want true")
	}
	// Original unchanged (immutability).
	if s.HasSubWorktree() {
		t.Error("original mutated: HasSubWorktree = true")
	}
	// Clear.
	cleared, err := out.WithSubWorktree(nil)
	if err != nil {
		t.Fatalf("WithSubWorktree(nil): %v", err)
	}
	if cleared.HasSubWorktree() {
		t.Error("WithSubWorktree(nil): HasSubWorktree = true, want false")
	}
}

func TestResolutionStrategy_WithSubWorktree_TitleRequired(t *testing.T) {
	s, _ := NewResolutionStrategy("obs-1", "read_file", "")
	_, err := s.WithSubWorktree(&SubWorktreeSpec{Title: ""})
	if !errors.Is(err, ErrSubWorktreeSpecTitleRequired) {
		t.Fatalf("err = %v, want ErrSubWorktreeSpecTitleRequired", err)
	}
}

func TestResolutionStrategy_Validate(t *testing.T) {
	tests := []struct {
		name string
		s    ResolutionStrategy
		want error
	}{
		{
			name: "valid no sub_worktree",
			s:    ResolutionStrategy{ObsID: "obs-1"},
			want: nil,
		},
		{
			name: "valid with sub_worktree",
			s: ResolutionStrategy{
				ObsID: "obs-1",
				SubWorktree: &SubWorktreeSpec{
					Title: "investigate",
				},
			},
			want: nil,
		},
		{
			name: "empty ObsID",
			s:    ResolutionStrategy{ObsID: ""},
			want: ErrResolutionStrategyObsIDRequired,
		},
		{
			name: "sub_worktree empty title",
			s: ResolutionStrategy{
				ObsID:      "obs-1",
				SubWorktree: &SubWorktreeSpec{Title: ""},
			},
			want: ErrSubWorktreeSpecTitleRequired,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.s.Validate()
			if tt.want == nil {
				if err != nil {
					t.Fatalf("Validate: unexpected err = %v", err)
				}
				return
			}
			if !errors.Is(err, tt.want) {
				t.Errorf("Validate err = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestResolutionClaim_NewValidatesObsID(t *testing.T) {
	_, err := NewResolutionClaim("", "answer", "evidence", 0.8)
	if !errors.Is(err, ErrResolutionClaimObsIDRequired) {
		t.Fatalf("err = %v, want ErrResolutionClaimObsIDRequired", err)
	}
}

func TestResolutionClaim_ClampsConfidence(t *testing.T) {
	c, err := NewResolutionClaim("obs-1", "x", "y", 1.5)
	if err != nil {
		t.Fatalf("NewResolutionClaim: %v", err)
	}
	if c.Confidence != 1.0 {
		t.Errorf("Confidence = %.3f, want 1.0 (clamped)", c.Confidence)
	}
	c, err = NewResolutionClaim("obs-1", "x", "y", -0.5)
	if err != nil {
		t.Fatalf("NewResolutionClaim: %v", err)
	}
	if c.Confidence != 0.0 {
		t.Errorf("Confidence = %.3f, want 0.0 (clamped)", c.Confidence)
	}
}

func TestResolutionClaim_IsResolved(t *testing.T) {
	tests := []struct {
		name  string
		claim ResolutionClaim
		want  bool
	}{
		{
			name:  "fully resolved",
			claim: ResolutionClaim{ObsID: "obs-1", Answer: "x", Confidence: 0.9, SupportingEvidence: "evidence"},
			want:  true,
		},
		{
			name:  "low confidence",
			claim: ResolutionClaim{ObsID: "obs-1", Answer: "x", Confidence: 0.5, SupportingEvidence: "evidence"},
			want:  false,
		},
		{
			name:  "empty answer",
			claim: ResolutionClaim{ObsID: "obs-1", Answer: "", Confidence: 0.9, SupportingEvidence: "evidence"},
			want:  false,
		},
		{
			name:  "empty obs_id",
			claim: ResolutionClaim{ObsID: "", Answer: "x", Confidence: 0.9, SupportingEvidence: "evidence"},
			want:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.claim.IsResolved(); got != tt.want {
				t.Errorf("IsResolved = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResolutionReport_AllResolved(t *testing.T) {
	strats := []ResolutionStrategy{
		{ObsID: "obs-1", PlannedTool: "read_file"},
		{ObsID: "obs-2", PlannedTool: "bash"},
	}
	claims := []ResolutionClaim{
		{ObsID: "obs-1", Answer: "x", Confidence: 0.9, SupportingEvidence: "ev1"},
		{ObsID: "obs-2", Answer: "y", Confidence: 0.85, SupportingEvidence: "ev2"},
	}
	r, err := NewResolutionReport("s1", "wi-1", 1, strats, claims)
	if err != nil {
		t.Fatalf("NewResolutionReport: %v", err)
	}
	if r.TotalStrategies != 2 {
		t.Errorf("TotalStrategies = %d, want 2", r.TotalStrategies)
	}
	if r.TotalClaims != 2 {
		t.Errorf("TotalClaims = %d, want 2", r.TotalClaims)
	}
	if r.CoverageRatio != 1.0 {
		t.Errorf("CoverageRatio = %.3f, want 1.0", r.CoverageRatio)
	}
	if len(r.UnresolvedObs) != 0 {
		t.Errorf("UnresolvedObs = %v, want empty", r.UnresolvedObs)
	}
}

func TestResolutionReport_NoClaim(t *testing.T) {
	strats := []ResolutionStrategy{{ObsID: "obs-1"}}
	claims := []ResolutionClaim{}
	r, err := NewResolutionReport("s1", "wi-1", 1, strats, claims)
	if err != nil {
		t.Fatalf("NewResolutionReport: %v", err)
	}
	if r.CoverageRatio != 0 {
		t.Errorf("CoverageRatio = %.3f, want 0", r.CoverageRatio)
	}
	if len(r.UnresolvedObs) != 1 {
		t.Fatalf("UnresolvedObs len = %d, want 1", len(r.UnresolvedObs))
	}
	if r.UnresolvedObs[0].Reason != ResolutionReasonNoClaim {
		t.Errorf("Reason = %s, want no_resolution_claim", r.UnresolvedObs[0].Reason)
	}
}

func TestResolutionReport_LowConfidence(t *testing.T) {
	strats := []ResolutionStrategy{{ObsID: "obs-1"}}
	claims := []ResolutionClaim{{ObsID: "obs-1", Answer: "x", Confidence: 0.4, SupportingEvidence: "ev"}}
	r, err := NewResolutionReport("s1", "wi-1", 1, strats, claims)
	if err != nil {
		t.Fatalf("NewResolutionReport: %v", err)
	}
	if len(r.UnresolvedObs) != 1 {
		t.Fatalf("UnresolvedObs len = %d, want 1", len(r.UnresolvedObs))
	}
	if r.UnresolvedObs[0].Reason != ResolutionReasonLowConfidence {
		t.Errorf("Reason = %s, want low_confidence", r.UnresolvedObs[0].Reason)
	}
}

func TestResolutionReport_EvidenceMissing(t *testing.T) {
	strats := []ResolutionStrategy{{ObsID: "obs-1"}}
	claims := []ResolutionClaim{{ObsID: "obs-1", Answer: "x", Confidence: 0.9, SupportingEvidence: ""}}
	r, err := NewResolutionReport("s1", "wi-1", 1, strats, claims)
	if err != nil {
		t.Fatalf("NewResolutionReport: %v", err)
	}
	if len(r.UnresolvedObs) != 1 {
		t.Fatalf("UnresolvedObs len = %d, want 1", len(r.UnresolvedObs))
	}
	if r.UnresolvedObs[0].Reason != ResolutionReasonEvidenceMissing {
		t.Errorf("Reason = %s, want evidence_missing", r.UnresolvedObs[0].Reason)
	}
}

func TestResolutionReport_EmptyAnswer(t *testing.T) {
	// DM-20260704-006 S4 Phase 2 review fix: NewResolutionReport must
	// mirror ResolutionClaim.IsResolved() — an empty Answer downgrades
	// the claim to UnresolvedObs{Reason: no_resolution_claim} even when
	// Confidence and SupportingEvidence look fine. Without this guard
	// the report silently counts a half-formed claim as "resolved" and
	// the Decide layer routes an unreliable answer.
	strats := []ResolutionStrategy{{ObsID: "obs-1"}}
	claims := []ResolutionClaim{
		{ObsID: "obs-1", Answer: "", Confidence: 0.95, SupportingEvidence: "looks-good-evidence"},
	}
	r, err := NewResolutionReport("s1", "wi-1", 1, strats, claims)
	if err != nil {
		t.Fatalf("NewResolutionReport: %v", err)
	}
	if len(r.UnresolvedObs) != 1 {
		t.Fatalf("UnresolvedObs len = %d, want 1 (empty Answer → no_resolution_claim)", len(r.UnresolvedObs))
	}
	if r.UnresolvedObs[0].Reason != ResolutionReasonNoClaim {
		t.Errorf("Reason = %s, want no_resolution_claim", r.UnresolvedObs[0].Reason)
	}
	if r.CoverageRatio != 0 {
		t.Errorf("CoverageRatio = %.3f, want 0 (empty Answer does not count as resolved)", r.CoverageRatio)
	}
}

func TestResolutionReport_NoStrategy(t *testing.T) {
	strats := []ResolutionStrategy{{ObsID: "obs-1"}}
	claims := []ResolutionClaim{
		{ObsID: "obs-1", Answer: "x", Confidence: 0.9, SupportingEvidence: "ev"},
		{ObsID: "obs-orphan", Answer: "y", Confidence: 0.9, SupportingEvidence: "ev2"},
	}
	r, err := NewResolutionReport("s1", "wi-1", 1, strats, claims)
	if err != nil {
		t.Fatalf("NewResolutionReport: %v", err)
	}
	if len(r.UnresolvedObs) != 1 {
		t.Fatalf("UnresolvedObs len = %d, want 1 (only orphan claim)", len(r.UnresolvedObs))
	}
	if r.UnresolvedObs[0].ObsID != "obs-orphan" {
		t.Errorf("ObsID = %s, want obs-orphan", r.UnresolvedObs[0].ObsID)
	}
	if r.UnresolvedObs[0].Reason != ResolutionReasonNoStrategy {
		t.Errorf("Reason = %s, want no_resolution_strategy", r.UnresolvedObs[0].Reason)
	}
}

func TestResolutionReport_HasSubWorktreePropagates(t *testing.T) {
	strats := []ResolutionStrategy{
		{ObsID: "obs-1", SubWorktree: &SubWorktreeSpec{Title: "investigate"}},
	}
	claims := []ResolutionClaim{} // no claim → unresolved
	r, err := NewResolutionReport("s1", "wi-1", 1, strats, claims)
	if err != nil {
		t.Fatalf("NewResolutionReport: %v", err)
	}
	if len(r.UnresolvedObs) != 1 {
		t.Fatalf("UnresolvedObs len = %d, want 1", len(r.UnresolvedObs))
	}
	if !r.UnresolvedObs[0].HasSubWorktree {
		t.Error("HasSubWorktree = false, want true (mirrors ResolutionStrategy)")
	}
	if !r.AnySubWorktreePending() {
		t.Error("AnySubWorktreePending = false, want true")
	}
}

func TestResolutionReport_EmptyStrategies(t *testing.T) {
	r, err := NewResolutionReport("s1", "wi-1", 1, nil, nil)
	if err != nil {
		t.Fatalf("NewResolutionReport: %v", err)
	}
	if r.CoverageRatio != 0 {
		t.Errorf("CoverageRatio = %.3f, want 0 (no strategies)", r.CoverageRatio)
	}
	if r.TotalStrategies != 0 {
		t.Errorf("TotalStrategies = %d, want 0", r.TotalStrategies)
	}
	if r.MaxUnresolvedStrength() != 0 {
		t.Errorf("MaxUnresolvedStrength = %.3f, want 0", r.MaxUnresolvedStrength())
	}
}

func TestResolutionReport_MaxUnresolvedStrength(t *testing.T) {
	strats := []ResolutionStrategy{
		{ObsID: "obs-1"},
		{ObsID: "obs-2"},
		{ObsID: "obs-3"},
	}
	// We can't pass strengths via NewResolutionReport (extractObsStrength
	// is a Phase 1 placeholder) — so we build the report then mutate the
	// strengths post-hoc to test MaxUnresolvedStrength in isolation.
	r, err := NewResolutionReport("s1", "wi-1", 1, strats, nil)
	if err != nil {
		t.Fatalf("NewResolutionReport: %v", err)
	}
	r.UnresolvedObs[0].Strength = 0.4
	r.UnresolvedObs[1].Strength = 0.92
	r.UnresolvedObs[2].Strength = 0.7
	if got := r.MaxUnresolvedStrength(); got != 0.92 {
		t.Errorf("MaxUnresolvedStrength = %.3f, want 0.92", got)
	}
}

func TestResolutionReport_ValidatesRequiredFields(t *testing.T) {
	tests := []struct {
		name      string
		sessionID string
		wiID      string
		round     int
		want      error
	}{
		{"empty sessionID", "", "wi-1", 1, ErrResolutionReportSessionIDRequired},
		{"empty workitemID", "s1", "", 1, ErrResolutionReportWorkItemIDRequired},
		{"negative round", "s1", "wi-1", -1, ErrResolutionReportRoundNoNegative},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewResolutionReport(tt.sessionID, tt.wiID, tt.round, nil, nil)
			if !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestResolutionReason_WireFormat(t *testing.T) {
	// Stable wire format — downstream dashboards filter by these strings.
	want := map[ResolutionReason]string{
		ResolutionReasonNoClaim:         "no_resolution_claim",
		ResolutionReasonLowConfidence:   "low_confidence",
		ResolutionReasonEvidenceMissing: "evidence_missing",
		ResolutionReasonNoStrategy:      "no_resolution_strategy",
	}
	for got, exp := range want {
		if string(got) != exp {
			t.Errorf("ResolutionReason wire format = %q, want %q", got, exp)
		}
		if !strings.Contains(string(got), "_") {
			t.Errorf("ResolutionReason %q must be snake_case", got)
		}
	}
}

func TestResolutionContract_EndToEnd_C6F2D6910496E2EA63CBCF8F207B2C0A(t *testing.T) {
	// Reproduce the trace c6f2d6910496e2ea63cbcf8f207b2c0a scenario:
	//   - 3 obs_uncertainties returned by Observe with strengths 0.92/0.88/0.82.
	//   - Plan emits 3 ResolutionStrategies, 1 with sub_worktree.
	//   - Execute emits 2 claims (resolves 2/3).
	//   - Verify produces a report with 1 UnresolvedObs carrying HasSubWorktree=true.
	//   - Decide reads AnySubWorktreePending → forces SpawnDecompose.
	strats := []ResolutionStrategy{
		{ObsID: "obs-1", PlannedTool: "read_file", SuccessCriterion: "file_count > 0"},
		{
			ObsID:            "obs-2",
			PlannedTool:      "bash",
			SuccessCriterion: "exit_code == 0",
			SubWorktree:      &SubWorktreeSpec{Title: "investigate plan dir"},
		},
		{ObsID: "obs-3", PlannedTool: "grep", SuccessCriterion: "match_count > 0"},
	}
	claims := []ResolutionClaim{
		{ObsID: "obs-1", Answer: "found 3 files", Confidence: 0.95, SupportingEvidence: "glob match"},
		{ObsID: "obs-3", Answer: "found 2 matches", Confidence: 0.88, SupportingEvidence: "regex hit"},
	}
	r, err := NewResolutionReport("sess_1783239758810_0", "wi_d0_s0_goal", 1, strats, claims)
	if err != nil {
		t.Fatalf("NewResolutionReport: %v", err)
	}

	// Coverage 2/3 ≈ 0.667.
	if r.TotalStrategies != 3 {
		t.Errorf("TotalStrategies = %d, want 3", r.TotalStrategies)
	}
	if r.TotalClaims != 2 {
		t.Errorf("TotalClaims = %d, want 2", r.TotalClaims)
	}
	want := 2.0 / 3.0
	if r.CoverageRatio < want-1e-9 || r.CoverageRatio > want+1e-9 {
		t.Errorf("CoverageRatio = %.6f, want %.6f", r.CoverageRatio, want)
	}

	// 1 UnresolvedObs: obs-2 (no claim, has sub_worktree).
	if len(r.UnresolvedObs) != 1 {
		t.Fatalf("UnresolvedObs len = %d, want 1", len(r.UnresolvedObs))
	}
	uo := r.UnresolvedObs[0]
	if uo.ObsID != "obs-2" {
		t.Errorf("ObsID = %s, want obs-2", uo.ObsID)
	}
	if uo.Reason != ResolutionReasonNoClaim {
		t.Errorf("Reason = %s, want no_resolution_claim", uo.Reason)
	}
	if !uo.HasSubWorktree {
		t.Error("HasSubWorktree = false, want true (RC-4a trigger)")
	}

	// Decide hook: AnySubWorktreePending → SpawnDecompose.
	if !r.AnySubWorktreePending() {
		t.Error("AnySubWorktreePending = false, want true (Decide should SpawnDecompose)")
	}
}

func TestResolutionContract_JSON_RoundTrip(t *testing.T) {
	// Wire format must round-trip cleanly so persisted reports survive
	// the trace log write-read cycle. Strategy + SubWorktree + UnresolvedObs
	// are each exercised separately since ResolutionReport itself only
	// carries the derived counts + UnresolvedObs slice.
	t.Run("strategy with sub_worktree", func(t *testing.T) {
		s := ResolutionStrategy{
			ObsID:            "obs-1",
			PlannedTool:      "read_file",
			SuccessCriterion: "file_count > 0",
			SubWorktree: &SubWorktreeSpec{
				Title:           "investigate",
				DirectiveSuffix: "list all files",
			},
		}
		data, err := json.Marshal(s)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if !strings.Contains(string(data), `"obs_id":"obs-1"`) {
			t.Errorf("missing obs_id: %s", data)
		}
		if !strings.Contains(string(data), `"sub_worktree"`) {
			t.Errorf("missing sub_worktree: %s", data)
		}
		var got ResolutionStrategy
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if got.SubWorktree == nil || got.SubWorktree.Title != "investigate" {
			t.Errorf("round-trip lost SubWorktree: %+v", got.SubWorktree)
		}
	})

	t.Run("report with has_sub_worktree=true", func(t *testing.T) {
		// No claim → unresolved with HasSubWorktree mirrored from strategy.
		strats := []ResolutionStrategy{{
			ObsID:       "obs-1",
			SubWorktree: &SubWorktreeSpec{Title: "investigate"},
		}}
		r, err := NewResolutionReport("s1", "wi-1", 1, strats, nil)
		if err != nil {
			t.Fatalf("NewResolutionReport: %v", err)
		}
		data, err := json.Marshal(r)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if !strings.Contains(string(data), `"obs_id":"obs-1"`) {
			t.Errorf("missing obs_id in unresolved_obs: %s", data)
		}
		if !strings.Contains(string(data), `"has_sub_worktree":true`) {
			t.Errorf("missing has_sub_worktree=true: %s", data)
		}
		var got ResolutionReport
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if len(got.UnresolvedObs) != 1 || !got.UnresolvedObs[0].HasSubWorktree {
			t.Errorf("round-trip lost UnresolvedObs: %+v", got.UnresolvedObs)
		}
	})
}