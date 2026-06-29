package bootstrap

import (
	"os"
	"testing"
)

func withFeatureFlagEnv(t *testing.T, key, value string) {
	t.Helper()
	old, had := os.LookupEnv(key)
	if value != "" {
		_ = os.Setenv(key, value)
	} else {
		_ = os.Unsetenv(key)
	}
	t.Cleanup(func() {
		if had {
			_ = os.Setenv(key, old)
		} else {
			_ = os.Unsetenv(key)
		}
	})
}

func TestHardEvidenceEnabled_DefaultFalse(t *testing.T) {
	withFeatureFlagEnv(t, "D7_HARD_EVIDENCE_ENABLED", "")
	if HardEvidenceEnabled() {
		t.Fatalf("default (env unset) should be false")
	}
}

func TestHardEvidenceEnabled_TruthyValues(t *testing.T) {
	for _, v := range []string{"1", "true", "yes", "on", "TRUE", "Yes"} {
		withFeatureFlagEnv(t, "D7_HARD_EVIDENCE_ENABLED", v)
		if !HardEvidenceEnabled() {
			t.Fatalf("value=%q should be enabled", v)
		}
	}
}

func TestHardEvidenceEnabled_FalsyValues(t *testing.T) {
	for _, v := range []string{"0", "false", "no", "off", "garbage", ""} {
		withFeatureFlagEnv(t, "D7_HARD_EVIDENCE_ENABLED", v)
		// "garbage" and "" → unset effectively → false.
		// Other explicit falsy → false.
		if HardEvidenceEnabled() {
			t.Fatalf("value=%q should be disabled", v)
		}
	}
}

func TestSimilarityCheckEnabled_DefaultFalse(t *testing.T) {
	withFeatureFlagEnv(t, "D7_SIMILARITY_CHECK_ENABLED", "")
	if SimilarityCheckEnabled() {
		t.Fatalf("default should be false")
	}
}

func TestSimilarityCheckEnabled_TruthyValues(t *testing.T) {
	for _, v := range []string{"1", "true", "yes", "on"} {
		withFeatureFlagEnv(t, "D7_SIMILARITY_CHECK_ENABLED", v)
		if !SimilarityCheckEnabled() {
			t.Fatalf("value=%q should be enabled", v)
		}
	}
}

func TestSimilarityCheckEnabled_DoesNotPolluteHardEvidence(t *testing.T) {
	// Setting SimilarityCheckEnabled must not flip HardEvidenceEnabled.
	withFeatureFlagEnv(t, "D7_HARD_EVIDENCE_ENABLED", "")
	withFeatureFlagEnv(t, "D7_SIMILARITY_CHECK_ENABLED", "true")
	if HardEvidenceEnabled() {
		t.Fatalf("HardEvidenceEnabled should remain false")
	}
	if !SimilarityCheckEnabled() {
		t.Fatalf("SimilarityCheckEnabled should be true")
	}
}
