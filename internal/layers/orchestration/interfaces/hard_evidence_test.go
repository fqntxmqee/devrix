package interfaces

import (
	"testing"
)

func TestNewHardEvidence_DefaultKind(t *testing.T) {
	h := NewHardEvidence("")
	if h.Kind != HardEvidenceKindUnknown {
		t.Fatalf("expected unknown kind, got %q", h.Kind)
	}
}

func TestNewHardEvidence_Code(t *testing.T) {
	h := NewHardEvidence("code")
	if h.Kind != HardEvidenceKindCode {
		t.Fatalf("expected code kind, got %q", h.Kind)
	}
}

func TestNewHardEvidence_Chat(t *testing.T) {
	h := NewHardEvidence("chat")
	if h.Kind != HardEvidenceKindChat {
		t.Fatalf("expected chat kind, got %q", h.Kind)
	}
}

func TestNewHardEvidence_Normalizes(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"CODE", HardEvidenceKindCode},
		{" Chat ", HardEvidenceKindChat},
		{"  ", HardEvidenceKindUnknown},
		{"unknown", HardEvidenceKindUnknown},
		{"made-up", HardEvidenceKindUnknown},
	}
	for _, tc := range cases {
		got := NewHardEvidence(tc.in).Kind
		if got != tc.want {
			t.Fatalf("input %q → got %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestWith_BuildersAreImmutable(t *testing.T) {
	base := NewHardEvidence("code")
	t1 := base.WithTestResult(&TestResult{CoveragePct: 50, Passed: true})
	if base.TestResult != nil {
		t.Fatalf("WithTestResult must not mutate base")
	}
	if t1.TestResult == nil || t1.TestResult.CoveragePct != 50 {
		t.Fatalf("new copy should retain the value")
	}
}

func TestWith_AllBuilders(t *testing.T) {
	base := NewHardEvidence("chat")
	tr := &TestResult{CoveragePct: 10, Passed: false}
	h := base.
		WithKind("code").
		WithTestResult(tr).
		WithLogExcerpt("log").
		WithArtifactHash("ah").
		WithEntityHash("eh").
		WithCoherenceScore(0.7)
	if h.Kind != "code" {
		t.Fatalf("kind: got %q", h.Kind)
	}
	if h.TestResult != tr {
		t.Fatalf("test result: got %v, want %v", h.TestResult, tr)
	}
	if h.LogExcerpt != "log" || h.ArtifactHash != "ah" || h.EntityHash != "eh" {
		t.Fatalf("excerpt/artifact/entity not set")
	}
	if h.CoherenceScore != 0.7 {
		t.Fatalf("coherence not set")
	}
}

func TestVerified_Code_WithCoverage(t *testing.T) {
	h := NewHardEvidence("code").WithTestResult(&TestResult{CoveragePct: 50, Passed: true})
	if !h.Verified() {
		t.Fatalf("code with 50%% coverage should verify")
	}
}

func TestVerified_Code_CoverageBelowMin(t *testing.T) {
	h := NewHardEvidence("code").WithTestResult(&TestResult{CoveragePct: 0, Passed: true})
	if h.Verified() {
		t.Fatalf("code with 0%% coverage + nothing else should NOT verify")
	}
}

func TestVerified_Code_WithLog(t *testing.T) {
	h := NewHardEvidence("code").WithLogExcerpt("some log line")
	if !h.Verified() {
		t.Fatalf("code with LogExcerpt should verify")
	}
}

func TestVerified_Code_WithArtifactHash(t *testing.T) {
	h := NewHardEvidence("code").WithArtifactHash("abc123")
	if !h.Verified() {
		t.Fatalf("code with ArtifactHash should verify")
	}
}

func TestVerified_Code_Empty(t *testing.T) {
	h := NewHardEvidence("code")
	if h.Verified() {
		t.Fatalf("empty code evidence should NOT verify")
	}
}

func TestVerified_Code_TrimmedLog(t *testing.T) {
	h := NewHardEvidence("code").WithLogExcerpt("   ")
	if h.Verified() {
		t.Fatalf("whitespace-only LogExcerpt should NOT verify")
	}
}

func TestVerified_Chat_WithCoherence(t *testing.T) {
	h := NewHardEvidence("chat").WithCoherenceScore(0.5)
	if !h.Verified() {
		t.Fatalf("chat with CoherenceScore=0.5 should verify")
	}
}

func TestVerified_Chat_WithHighCoherence(t *testing.T) {
	h := NewHardEvidence("chat").WithCoherenceScore(0.95)
	if !h.Verified() {
		t.Fatalf("chat with CoherenceScore=0.95 should verify")
	}
}

func TestVerified_Chat_WithEntity(t *testing.T) {
	h := NewHardEvidence("chat").WithEntityHash("entity-1")
	if !h.Verified() {
		t.Fatalf("chat with EntityHash should verify")
	}
}

func TestVerified_Chat_BelowCoherence(t *testing.T) {
	h := NewHardEvidence("chat").WithCoherenceScore(0.4)
	if h.Verified() {
		t.Fatalf("chat with CoherenceScore=0.4 should NOT verify")
	}
}

func TestVerified_Chat_Empty(t *testing.T) {
	h := NewHardEvidence("chat")
	if h.Verified() {
		t.Fatalf("empty chat evidence should NOT verify")
	}
}

func TestVerified_Chat_DoesNotUseTestArtifacts(t *testing.T) {
	// IV-5: chat is NOT subjected to test/log/artifact checks.
	h := NewHardEvidence("chat").
		WithLogExcerpt("log").
		WithArtifactHash("ah").
		WithTestResult(&TestResult{CoveragePct: 100, Passed: true})
	if h.Verified() {
		t.Fatalf("chat verified via code-only fields violates IV-5")
	}
}

func TestVerified_UnknownKindFallsToCode(t *testing.T) {
	h := NewHardEvidence("unknown")
	// Without any code-relevant field, unknown should NOT verify.
	if h.Verified() {
		t.Fatalf("unknown without code fields should NOT verify")
	}
	h2 := h.WithLogExcerpt("a log")
	if !h2.Verified() {
		t.Fatalf("unknown with LogExcerpt should verify (conservative)")
	}
}

func TestExtractHardEvidenceFromEvidence_Nil(t *testing.T) {
	h := ExtractHardEvidenceFromEvidence(nil)
	if h.Kind != HardEvidenceKindCode {
		t.Fatalf("nil input → default kind code, got %q", h.Kind)
	}
	if h.Verified() {
		t.Fatalf("nil input should not produce verified HardEvidence")
	}
}

func TestExtractHardEvidenceFromEvidence_CoverageFromString(t *testing.T) {
	ev := &Evidence{
		TestResult:   "5/5 pass, coverage 87%",
		LogExcerpt:   "info: ok",
		ArtifactHash: "h1",
	}
	h := ExtractHardEvidenceFromEvidence(ev)
	if h.TestResult == nil || h.TestResult.CoveragePct != 87 {
		t.Fatalf("expected coverage 87, got %+v", h.TestResult)
	}
	if !h.TestResult.Passed {
		t.Fatalf("expected Passed=true (contains 'pass')")
	}
	if h.LogExcerpt != "info: ok" || h.ArtifactHash != "h1" {
		t.Fatalf("excerpt/artifact not propagated")
	}
	if !h.Verified() {
		t.Fatalf("code with coverage 87 should verify")
	}
}

func TestExtractHardEvidenceFromEvidence_NoCoveragePhrase(t *testing.T) {
	ev := &Evidence{TestResult: "build succeeded"}
	h := ExtractHardEvidenceFromEvidence(ev)
	// Without a "coverage" keyword, TestResult stays nil and no other fields
	// are populated → Verified() should return false.
	if h.TestResult != nil {
		t.Fatalf("expected nil TestResult when no 'coverage' phrase")
	}
	if h.Verified() {
		t.Fatalf("empty code without coverage should not verify")
	}
}

func TestExtractHardEvidenceFromEvidence_LogOnly(t *testing.T) {
	ev := &Evidence{LogExcerpt: "step 1 done"}
	h := ExtractHardEvidenceFromEvidence(ev)
	if h.LogExcerpt != "step 1 done" {
		t.Fatalf("log not propagated")
	}
	if !h.Verified() {
		t.Fatalf("log-only should verify")
	}
}

func TestExtractHardEvidenceFromEvidence_ArtifactOnly(t *testing.T) {
	ev := &Evidence{ArtifactHash: "deadbeef"}
	h := ExtractHardEvidenceFromEvidence(ev)
	if h.ArtifactHash != "deadbeef" {
		t.Fatalf("artifact not propagated")
	}
	if !h.Verified() {
		t.Fatalf("artifact-only should verify")
	}
}

func TestExtractHardEvidenceFromEvidence_CoverageColonFormat(t *testing.T) {
	ev := &Evidence{TestResult: "ran 3 tests, coverage: 42%, ok"}
	h := ExtractHardEvidenceFromEvidence(ev)
	if h.TestResult == nil || h.TestResult.CoveragePct != 42 {
		t.Fatalf("expected coverage 42 (colon variant), got %+v", h.TestResult)
	}
}

func TestExtractHardEvidenceFromEvidence_FailedPassAbsent(t *testing.T) {
	ev := &Evidence{TestResult: "tests failed; coverage 12%"}
	h := ExtractHardEvidenceFromEvidence(ev)
	if h.TestResult == nil || h.TestResult.Passed {
		t.Fatalf("expected Passed=false (no 'pass' keyword)")
	}
}

func TestParseCoveragePercent_TableDriven(t *testing.T) {
	cases := []struct {
		in   string
		want int
		ok   bool
	}{
		{"coverage 87%", 87, true},
		{"coverage: 42%", 42, true},
		{"Coverage 100%", 100, true},
		{"5/5 pass, coverage 87%", 87, true},
		{"coverage 0%", 0, true},
		{"no stats here", 0, false},
		{"coverage", 0, false},
		{"coverage no number", 0, false},
		{"", 0, false},
	}
	for _, tc := range cases {
		got, ok := parseCoveragePercent(tc.in)
		if ok != tc.ok || (ok && got != tc.want) {
			t.Fatalf("parseCoveragePercent(%q) = (%d, %v), want (%d, %v)",
				tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestIsPassedTestResult_TableDriven(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"tests passed", true},
		{"all pass", true},
		{"5/5 pass", true},
		{"build ok", true},
		{"tests failed", false},
		{"error building", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := isPassedTestResult(tc.in); got != tc.want {
			t.Fatalf("isPassedTestResult(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestNewHardEvidenceMissingError_Code(t *testing.T) {
	e := NewHardEvidenceMissingError()
	if e == nil || e.Code != "ORCH_HARD_EVIDENCE_MISSING_7122" {
		t.Fatalf("unexpected error: %+v", e)
	}
}

func TestNormalizeKind_ClosedSet(t *testing.T) {
	// Sanity: every input should map to one of the three ClosedSet values.
	inputs := []string{"", "CODE", "chat", "Chat", " unknown ", "video", "image"}
	for _, in := range inputs {
		got := normalizeKind(in)
		switch got {
		case HardEvidenceKindCode, HardEvidenceKindChat, HardEvidenceKindUnknown:
			// ok
		default:
			t.Fatalf("normalizeKind(%q) = %q, not in closed set", in, got)
		}
	}
}

func TestVerified_MinConstants(t *testing.T) {
	if HardEvidenceMinCoverage != 1 {
		t.Fatalf("HardEvidenceMinCoverage should be 1, got %d", HardEvidenceMinCoverage)
	}
	if HardEvidenceMinCoherence != 0.5 {
		t.Fatalf("HardEvidenceMinCoherence should be 0.5, got %v", HardEvidenceMinCoherence)
	}
}
