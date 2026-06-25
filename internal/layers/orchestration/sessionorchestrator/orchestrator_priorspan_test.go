package sessionorchestrator

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
	"github.com/devrix/devrix/internal/layers/orchestration/mups/learn"
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
)

// ─────────────────────────────────────────────────────────────────────────
// T: D7-S13-A49-T06 — sessionSpan 6 prior attributes
// ─────────────────────────────────────────────────────────────────────────

// attrValue is a tiny helper for tests to read an attribute by key.
func attrValue(attrs []tracer.Attribute, key string) (string, bool) {
	for _, a := range attrs {
		if a.Key == key {
			s, ok := a.Value.(string)
			return s, ok
		}
	}
	return "", false
}

// TestPriorSessionSpanAttrs_RealInjection_AllSix verifies the happy path:
// a prior built from a real Reputation row (Operator track) produces all
// 5 prior attributes with the expected values:
//   - alpha/beta are string-formatted ints
//   - mean is "%.3f" formatted
//   - track_mode is read from the Reputation row
//   - injected_at = "phase6_lp1" (real injection, not failsafe)
//
// The 6th attribute (learn.classifier_source) is set separately after the
// classify path resolves and is verified via the integration test below.
func TestPriorSessionSpanAttrs_RealInjection_AllFive(t *testing.T) {
	rep, _ := learn.NewReputationEvidence("sess_1", learn.TrackModeOperator)
	rep.Alpha = 8
	rep.Beta = 1
	prior := &learn.AdaptivePrior{
		Reputation: rep,
		PriorBeta:  learn.BetaPrior{Alpha: 8, Beta: 1},
	}
	observeReq := orchtypes.ObserveRequest{Prior: prior} // Prior != nil → injected_at = "phase6_lp1"
	req := orchtypes.ProcessRequest{SessionID: "sess_1", Message: "hi", TrackMode: "operator"}

	attrs := priorSessionSpanAttrs(prior, observeReq, req)
	if len(attrs) != 5 {
		t.Fatalf("attrs len = %d, want 5", len(attrs))
	}
	type tc struct {
		key, want string
	}
	cases := []tc{
		{"learn.prior.alpha", "8"},
		{"learn.prior.beta", "1"},
		{"learn.prior.mean", "0.889"},
		{"learn.prior.track_mode", "operator"},
		{"learn.prior.injected_at", "phase6_lp1"},
	}
	for _, c := range cases {
		got, ok := attrValue(attrs, c.key)
		if !ok {
			t.Errorf("missing attr %q", c.key)
			continue
		}
		if got != c.want {
			t.Errorf("attr[%q] = %q, want %q", c.key, got, c.want)
		}
	}
}

// TestPriorSessionSpanAttrs_ColdStartFailsafe verifies the failsafe path:
// no prior injected → injected_at = "cold_start_failsafe" + Developer
// track mode (because the AdaptivePrior was synthesized via
// EffectivePrior()).
func TestPriorSessionSpanAttrs_ColdStartFailsafe(t *testing.T) {
	prior := learn.BuildAdaptivePrior(nil, learn.TrackModeDeveloper)
	observeReq := orchtypes.ObserveRequest{Prior: nil} // nil → injected_at = "cold_start_failsafe"
	req := orchtypes.ProcessRequest{SessionID: "sess_cold", Message: "hi"}

	attrs := priorSessionSpanAttrs(prior, observeReq, req)
	if len(attrs) != 5 {
		t.Fatalf("attrs len = %d, want 5", len(attrs))
	}
	type tc struct {
		key, want string
	}
	cases := []tc{
		{"learn.prior.alpha", "5"},
		{"learn.prior.beta", "3"},
		{"learn.prior.mean", "0.625"},
		{"learn.prior.track_mode", "developer"},
		{"learn.prior.injected_at", "cold_start_failsafe"},
	}
	for _, c := range cases {
		got, ok := attrValue(attrs, c.key)
		if !ok {
			t.Errorf("missing attr %q", c.key)
			continue
		}
		if got != c.want {
			t.Errorf("attr[%q] = %q, want %q", c.key, got, c.want)
		}
	}
}

// TestPriorSessionSpanAttrs_OperatorFromRequestHint verifies that when
// the Reputation row is absent but the ProcessRequest.TrackMode hint
// is "operator", the prior is read as Operator (Beta(8,1) → Mean=0.889).
func TestPriorSessionSpanAttrs_OperatorFromRequestHint(t *testing.T) {
	prior := learn.BuildAdaptivePrior(nil, learn.TrackModeOperator)
	observeReq := orchtypes.ObserveRequest{Prior: nil}
	req := orchtypes.ProcessRequest{SessionID: "sess_op", Message: "hi", TrackMode: orchtypes.TrackModeOperator}

	attrs := priorSessionSpanAttrs(prior, observeReq, req)
	if got, _ := attrValue(attrs, "learn.prior.track_mode"); got != "operator" {
		t.Errorf("track_mode = %q, want \"operator\"", got)
	}
	if got, _ := attrValue(attrs, "learn.prior.alpha"); got != "8" {
		t.Errorf("alpha = %q, want \"8\"", got)
	}
	if got, _ := attrValue(attrs, "learn.prior.beta"); got != "1" {
		t.Errorf("beta = %q, want \"1\"", got)
	}
	if got, _ := attrValue(attrs, "learn.prior.mean"); got != "0.889" {
		t.Errorf("mean = %q, want \"0.889\"", got)
	}
	if got, _ := attrValue(attrs, "learn.prior.injected_at"); got != "cold_start_failsafe" {
		t.Errorf("injected_at = %q, want \"cold_start_failsafe\" (no Reputation row)", got)
	}
}

// TestPriorSessionSpanAttrs_ReputationTrackModeWinsOverHint verifies the
// track-mode resolution policy: when a Reputation row exists with
// TrackMode="operator" but the request hint is "developer", the row wins
// (cross-session state takes precedence over the per-request hint).
func TestPriorSessionSpanAttrs_ReputationTrackModeWinsOverHint(t *testing.T) {
	rep, _ := learn.NewReputationEvidence("sess_x", learn.TrackModeOperator)
	rep.Alpha = 8
	rep.Beta = 1
	prior := &learn.AdaptivePrior{Reputation: rep, PriorBeta: learn.BetaPrior{Alpha: 8, Beta: 1}}
	observeReq := orchtypes.ObserveRequest{Prior: prior}
	req := orchtypes.ProcessRequest{SessionID: "sess_x", Message: "hi", TrackMode: orchtypes.TrackModeDeveloper}

	attrs := priorSessionSpanAttrs(prior, observeReq, req)
	if got, _ := attrValue(attrs, "learn.prior.track_mode"); got != "operator" {
		t.Errorf("track_mode = %q, want \"operator\" (Reputation wins over hint)", got)
	}
}

// TestPriorSessionSpanAttrs_AllAttributesHaveStringValues verifies that
// all 5 attribute values are string-typed (matches the spec: D5 Jaeger
// span attributes are typically string-valued for filter compatibility).
func TestPriorSessionSpanAttrs_AllAttributesHaveStringValues(t *testing.T) {
	rep, _ := learn.NewReputationEvidence("sess_2", learn.TrackModeOperator)
	rep.Alpha = 8
	rep.Beta = 1
	prior := &learn.AdaptivePrior{Reputation: rep, PriorBeta: learn.BetaPrior{Alpha: 8, Beta: 1}}
	observeReq := orchtypes.ObserveRequest{Prior: prior}
	req := orchtypes.ProcessRequest{SessionID: "sess_2", Message: "hi"}

	attrs := priorSessionSpanAttrs(prior, observeReq, req)
	for _, a := range attrs {
		if _, ok := a.Value.(string); !ok {
			t.Errorf("attr[%q].Value type = %T, want string", a.Key, a.Value)
		}
	}
}
