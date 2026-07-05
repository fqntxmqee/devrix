package sessionorchestrator

import (
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/interfaces"
)

func TestAppendResolutionClaimHint_should_skip_when_no_strategies(t *testing.T) {
	got := AppendResolutionClaimHint("base", nil)
	if got != "base" {
		t.Errorf("AppendResolutionClaimHint(nil) = %q, want base", got)
	}
	got = AppendResolutionClaimHint("base", []interfaces.ResolutionStrategy{})
	if got != "base" {
		t.Errorf("AppendResolutionClaimHint([]) = %q, want base", got)
	}
}

func TestAppendResolutionClaimHint_should_list_obs_ids(t *testing.T) {
	strategies := []interfaces.ResolutionStrategy{
		{ObsID: "obs-1"},
		{ObsID: "obs-2", PlannedTool: "read_file"},
	}
	got := AppendResolutionClaimHint("base", strategies)
	if !strings.Contains(got, "obs-1") || !strings.Contains(got, "obs-2") {
		t.Errorf("missing obs_ids in hint: %q", got)
	}
	if !strings.Contains(got, "resolution_claims") {
		t.Errorf("hint missing wire-format token: %q", got)
	}
	if !strings.HasPrefix(got, "base\n\n") {
		t.Errorf("hint should be appended, not prepended: %q", got)
	}
}

func TestAppendResolutionClaimHint_should_dedupe_obs_ids(t *testing.T) {
	strategies := []interfaces.ResolutionStrategy{
		{ObsID: "obs-1"},
		{ObsID: "obs-1"},
		{ObsID: "obs-1"},
	}
	got := AppendResolutionClaimHint("base", strategies)
	if !strings.Contains(got, "ObsIDs: obs-1") {
		t.Errorf("deduped ObsIDs list missing: %q", got)
	}
}

func TestParseResolutionClaims_should_parse_balanced_block(t *testing.T) {
	raw := `Here is my answer.

<resolution_claims>
[{"obs_id":"obs-1","answer":"42","confidence":0.9,"supporting_evidence":"file a/b/c"},{"obs_id":"obs-2","answer":"maybe","confidence":0.4,"supporting_evidence":"lacking"}]
</resolution_claims>
`
	claims, cleaned := ParseResolutionClaims(raw)
	if len(claims) != 2 {
		t.Fatalf("claims len = %d, want 2", len(claims))
	}
	if claims[0].ObsID != "obs-1" || claims[1].ObsID != "obs-2" {
		t.Errorf("claims ObsID mismatched: %+v", claims)
	}
	if strings.Contains(cleaned, "<resolution_claims>") {
		t.Errorf("cleaned still has marker: %q", cleaned)
	}
}

func TestParseResolutionClaims_should_skip_when_no_block(t *testing.T) {
	raw := `Prose without any claims block.`
	claims, cleaned := ParseResolutionClaims(raw)
	if claims != nil {
		t.Errorf("claims = %+v, want nil", claims)
	}
	if cleaned != "Prose without any claims block." {
		t.Errorf("cleaned = %q, want prose untouched", cleaned)
	}
}

func TestParseResolutionClaims_should_drop_malformed_payload_silently(t *testing.T) {
	raw := `body <resolution_claims>{not valid json}</resolution_claims> tail`
	claims, _ := ParseResolutionClaims(raw)
	if claims != nil {
		t.Errorf("malformed claims should drop, got %+v", claims)
	}
}

func TestParseResolutionClaims_should_drop_entries_with_empty_obs_id(t *testing.T) {
	raw := `<resolution_claims>[{"obs_id":"obs-1","answer":"x","confidence":0.9},{"obs_id":"","answer":"y","confidence":0.9}]</resolution_claims>`
	claims, _ := ParseResolutionClaims(raw)
	if len(claims) != 1 || claims[0].ObsID != "obs-1" {
		t.Errorf("expected only obs-1, got %+v", claims)
	}
}

func TestResolutionClaimsFingerprint_should_be_stable(t *testing.T) {
	if ResolutionClaimsFingerprint(nil) != "" {
		t.Errorf("fingerprint(nil) should be empty")
	}
	a := []interfaces.ResolutionClaim{{ObsID: "obs-1"}, {ObsID: "obs-2"}}
	if got := ResolutionClaimsFingerprint(a); got != "obs=2[obs-1,obs-2]" {
		t.Errorf("fingerprint = %q, want obs=2[obs-1,obs-2]", got)
	}
}
