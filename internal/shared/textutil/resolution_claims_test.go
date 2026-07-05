package textutil

import (
	"strings"
	"testing"
)

func TestExtractResolutionClaimsJSON_should_extract_balanced_block(t *testing.T) {
	raw := `Here is the plan you asked for.

<resolution_claims>
[{"obs_id":"obs-1","answer":"42","confidence":0.9,"supporting_evidence":"file a/b/c"}]
</resolution_claims>

Hope that helps.`
	payload, cleaned := ExtractResolutionClaimsJSON(raw)
	if payload == "" {
		t.Fatalf("ExtractResolutionClaimsJSON payload empty, want JSON array")
	}
	if !strings.Contains(payload, `"obs_id":"obs-1"`) {
		t.Errorf("payload missing obs_id field: %q", payload)
	}
	if strings.Contains(cleaned, "<resolution_claims>") || strings.Contains(cleaned, "</resolution_claims>") {
		t.Errorf("cleaned should not contain markers, got %q", cleaned)
	}
	if !strings.HasPrefix(cleaned, "Here is the plan") {
		t.Errorf("cleaned lost the leading prose: %q", cleaned)
	}
	if !strings.HasSuffix(cleaned, "Hope that helps.") {
		t.Errorf("cleaned lost the trailing prose: %q", cleaned)
	}
}

func TestExtractResolutionClaimsJSON_should_be_case_insensitive(t *testing.T) {
	raw := `Prologue <Resolution_Claims> [{"obs_id":"o"}] </RESOLUTION_CLAIMS> Epilogue`
	payload, cleaned := ExtractResolutionClaimsJSON(raw)
	if payload == "" {
		t.Fatalf("payload empty on case-insensitive match")
	}
	if strings.Contains(cleaned, "<Resolution_Claims>") {
		t.Errorf("cleaned still has open tag: %q", cleaned)
	}
	if !strings.HasPrefix(cleaned, "Prologue") {
		t.Errorf("cleaned lost prologue: %q", cleaned)
	}
}

func TestExtractResolutionClaimsJSON_should_preserve_plain_text(t *testing.T) {
	raw := `No claims block here, only prose with no markers.`
	payload, cleaned := ExtractResolutionClaimsJSON(raw)
	if payload != "" {
		t.Errorf("payload = %q, want empty", payload)
	}
	if cleaned != strings.TrimSpace(raw) {
		t.Errorf("cleaned = %q, want %q", cleaned, strings.TrimSpace(raw))
	}
}

func TestExtractResolutionClaimsJSON_should_drop_unbalanced_open_tag(t *testing.T) {
	raw := `Intro <resolution_claims> {"unterminated":"oops`
	payload, cleaned := ExtractResolutionClaimsJSON(raw)
	if payload != "" {
		t.Errorf("payload = %q, want empty on unbalanced", payload)
	}
	if strings.Contains(cleaned, "<resolution_claims>") {
		t.Errorf("cleaned leaked open tag: %q", cleaned)
	}
	if !strings.HasPrefix(cleaned, "Intro") {
		t.Errorf("cleaned lost intro: %q", cleaned)
	}
}

func TestExtractResolutionClaimsJSON_should_handle_multi_claim_payload(t *testing.T) {
	raw := `body <resolution_claims>[{"obs_id":"a"},{"obs_id":"b"}]</resolution_claims> tail`
	payload, cleaned := ExtractResolutionClaimsJSON(raw)
	if !strings.Contains(payload, `"obs_id":"a"`) || !strings.Contains(payload, `"obs_id":"b"`) {
		t.Errorf("payload missing one or more claims: %q", payload)
	}
	if strings.Contains(cleaned, "<resolution_claims>") {
		t.Errorf("cleaned leaked marker: %q", cleaned)
	}
}

func TestStripResolutionClaims_is_pure_strip(t *testing.T) {
	raw := `head <resolution_claims>[]</resolution_claims> tail`
	cleaned := StripResolutionClaims(raw)
	if strings.Contains(cleaned, "resolution_claims") {
		t.Errorf("StripResolutionClaims still has marker: %q", cleaned)
	}
	if !strings.Contains(cleaned, "head") || !strings.Contains(cleaned, "tail") {
		t.Errorf("StripResolutionClaims dropped body text: %q", cleaned)
	}
}

func TestStripResolutionClaims_preserves_input_without_marker(t *testing.T) {
	raw := `no marker here`
	got := StripResolutionClaims(raw)
	if got != strings.TrimSpace(raw) {
		t.Errorf("StripResolutionClaims(plain) = %q, want %q", got, strings.TrimSpace(raw))
	}
}
