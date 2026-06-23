package turn

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
)

func TestVerdictToExitReason_4Kinds(t *testing.T) {
	cases := []struct {
		name string
		kind orchtypes.VerdictKind
		want ExitReason
	}{
		{"Pass → Natural", orchtypes.VerdictPass, ExitReasonNatural},
		{"Partial → PartialVerified", orchtypes.VerdictPartial, ExitReasonPartialVerified},
		{"Indeterminate → VerifierAbstain", orchtypes.VerdictIndeterminate, ExitReasonVerifierAbstain},
		{"Fail → VerifierFail", orchtypes.VerdictFail, ExitReasonVerifierFail},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := workmodel.Verdict{Kind: tc.kind, Confidence: 0.8, SourceID: "verifier_1"}
			got := VerdictToExitReason(v, "session_1")
			if got != tc.want {
				t.Errorf("VerdictToExitReason(%v) = %q, want %q", tc.kind, got, tc.want)
			}
		})
	}
}

func TestVerdictToExitReason_SystemAnomalyOverrides(t *testing.T) {
	// SystemAnomaly=true overrides ALL VerdictKind → ExitReasonSystemAnomaly
	cases := []struct {
		name string
		kind orchtypes.VerdictKind
	}{
		{"Pass with anomaly", orchtypes.VerdictPass},
		{"Partial with anomaly", orchtypes.VerdictPartial},
		{"Indeterminate with anomaly", orchtypes.VerdictIndeterminate},
		{"Fail with anomaly", orchtypes.VerdictFail},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := workmodel.Verdict{
				Kind:          tc.kind,
				Confidence:    0.9,
				SourceID:      "verifier_anomaly",
				SystemAnomaly: true,
			}
			got := VerdictToExitReason(v, "session_1")
			if got != ExitReasonSystemAnomaly {
				t.Errorf("VerdictToExitReason(%v, SystemAnomaly=true) = %q, want ExitReasonSystemAnomaly", tc.kind, got)
			}
		})
	}
}

func TestVerdictToExitReason_EmptyVerdictKind_DefaultsToAbstain(t *testing.T) {
	// Zero-value VerdictKind (VerdictPass by default) → ExitReasonNatural.
	// Unknown / out-of-range kinds → safe default ExitReasonVerifierAbstain.
	v := workmodel.Verdict{} // zero value
	got := VerdictToExitReason(v, "")
	if got != ExitReasonNatural {
		t.Errorf("VerdictToExitReason(zero) = %q, want ExitReasonNatural (zero value VerdictPass)", got)
	}
}

func TestVerdictToExitReason_UnknownKind_DefaultsToAbstain(t *testing.T) {
	// Out-of-range VerdictKind (e.g. 99) → safe default ExitReasonVerifierAbstain.
	v := workmodel.Verdict{Kind: orchtypes.VerdictKind(99), Confidence: 0.5}
	got := VerdictToExitReason(v, "")
	if got != ExitReasonVerifierAbstain {
		t.Errorf("VerdictToExitReason(Kind=99) = %q, want ExitReasonVerifierAbstain (safe default)", got)
	}
}

func TestVerdictToExitReason_NilConfidence_HandledGracefully(t *testing.T) {
	// Confidence=0 is a valid value; not "nil" but tests that 0 doesn't break mapping.
	v := workmodel.Verdict{Kind: orchtypes.VerdictFail, Confidence: 0}
	got := VerdictToExitReason(v, "session_zero_conf")
	if got != ExitReasonVerifierFail {
		t.Errorf("VerdictToExitReason(Fail, Conf=0) = %q, want ExitReasonVerifierFail", got)
	}
}

func TestVerdictToExitReason_SessionIDAccepted(t *testing.T) {
	// sessionID parameter is currently reserved for Phase 5 attribution —
	// verify the function accepts any string without panicking.
	ids := []string{"", "session_1", "session-with-dashes", "session.with.dots"}
	for _, id := range ids {
		v := workmodel.Verdict{Kind: orchtypes.VerdictPass}
		got := VerdictToExitReason(v, id)
		if got != ExitReasonNatural {
			t.Errorf("VerdictToExitReason(Pass, sessionID=%q) = %q, want ExitReasonNatural", id, got)
		}
	}
}

// --- ExitReason 14 enumeration tests ---

func TestExitReason_14Values_AllParseable(t *testing.T) {
	all := AllExitReasons()
	if len(all) != 14 {
		t.Errorf("AllExitReasons() returned %d values, want 14", len(all))
	}
	for _, r := range all {
		t.Run(string(r), func(t *testing.T) {
			parsed, err := ParseExitReason(string(r))
			if err != nil {
				t.Errorf("ParseExitReason(%q) returned error: %v", string(r), err)
				return
			}
			if parsed != r {
				t.Errorf("ParseExitReason(%q) = %q, want %q", string(r), parsed, r)
			}
		})
	}
}

func TestExitReason_8LegacyValues_StringUnchanged(t *testing.T) {
	// Backward compat: the 8 pre-Phase-4 ExitReason enum values must
	// keep their original wire format strings unchanged.
	legacyValues := []struct {
		r    ExitReason
		want string
	}{
		{ExitReasonNatural, "natural"},
		{ExitReasonMaxTurns, "max_turns"},
		{ExitReasonAbortedUser, "aborted_user"},
		{ExitReasonAbortedLLM, "aborted_llm"},
		{ExitReasonAbortedTool, "aborted_tool"},
		{ExitReasonRepeatedTool, "repeated_tool"},
		{ExitReasonToolFailure, "tool_failure"},
		{ExitReasonTokenDiminishing, "token_diminishing"},
	}
	for _, tc := range legacyValues {
		if string(tc.r) != tc.want {
			t.Errorf("Legacy ExitReason string drift: got %q, want %q", string(tc.r), tc.want)
		}
		// Round-trip parse.
		parsed, err := ParseExitReason(tc.want)
		if err != nil {
			t.Errorf("ParseExitReason(%q) returned error: %v", tc.want, err)
			continue
		}
		if parsed != tc.r {
			t.Errorf("ParseExitReason(%q) = %q, want %q", tc.want, parsed, tc.r)
		}
	}
}

func TestParseExitReason_UnknownValue_Error(t *testing.T) {
	_, err := ParseExitReason("not_a_real_exit_reason")
	if err == nil {
		t.Fatal("ParseExitReason(\"not_a_real_exit_reason\") should return error")
	}
}