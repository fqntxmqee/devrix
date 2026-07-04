package i18n

import (
	"strings"
	"testing"
)

// DM-20260704-003 / D2-S15-A82-T02 (negative): EN intro must not contain the
// Chinese hard rule. Symmetry guard prevents Chinese pollution of English
// locale prompts.
func TestPromptSectionsEN_IntroHasNoChineseHardRule(t *testing.T) {
	intro, ok := promptSectionsEN["intro"]
	if !ok {
		t.Fatalf("promptSectionsEN missing 'intro' key")
	}
	forbidden := []string{
		"请始终用中文回复",
		"请始终用中文",
	}
	for _, f := range forbidden {
		if strings.Contains(intro, f) {
			t.Errorf("promptSectionsEN[intro] must not contain %q\nactual:\n%s", f, intro)
		}
	}
}

// Symmetry: EN tone_and_style must not contain Chinese output mandates.
func TestPromptSectionsEN_ToneHasNoChineseMandate(t *testing.T) {
	tone, ok := promptSectionsEN["tone_and_style"]
	if !ok {
		t.Fatalf("promptSectionsEN missing 'tone_and_style' key")
	}
	forbidden := []string{
		"必须用中文",
		"可见输出",
	}
	for _, f := range forbidden {
		if strings.Contains(tone, f) {
			t.Errorf("promptSectionsEN[tone_and_style] must not contain %q\nactual:\n%s", f, tone)
		}
	}
}

// Regression: every section in the EN map must be non-empty.
func TestPromptSectionsEN_AllSectionsNonEmpty(t *testing.T) {
	if len(promptSectionsEN) == 0 {
		t.Fatalf("promptSectionsEN is empty")
	}
	for key, val := range promptSectionsEN {
		if strings.TrimSpace(val) == "" {
			t.Errorf("promptSectionsEN[%q] is empty/whitespace", key)
		}
	}
}

// Cross-locale invariant: ZH and EN must NOT be byte-identical (else
// localization regressed silently).
func TestPromptSectionsZHEN_DifferByIntro(t *testing.T) {
	zh := promptSectionsZH["intro"]
	en, ok := promptSectionsEN["intro"]
	if !ok {
		t.Fatalf("promptSectionsEN missing 'intro' key")
	}
	if zh == en {
		t.Errorf("ZH and EN intro are byte-identical; localization regressed")
	}
}
