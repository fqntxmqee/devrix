package i18n

import (
	"strings"
	"testing"
)

// DM-20260704-003 / D2-S15-A82-T01: ZH intro must contain the Chinese hard rule
// to keep LLM outputs Chinese even when tool errors or <system-reminder> tags
// inject English inline content.
func TestPromptSectionsZH_IntroHasChineseHardRule(t *testing.T) {
	intro, ok := promptSectionsZH["intro"]
	if !ok {
		t.Fatalf("promptSectionsZH missing 'intro' key")
	}
	want := "请始终用中文回复用户"
	if !strings.Contains(intro, want) {
		t.Errorf("promptSectionsZH[intro] missing Chinese hard rule %q\nactual:\n%s", want, intro)
	}
}

// DM-20260704-003 / D2-S15-A82-T03: ZH tone_and_style must mandate Chinese output
// for user-visible text (excluding code/path/technical terms).
func TestPromptSectionsZH_ToneMandatesChineseOutput(t *testing.T) {
	tone, ok := promptSectionsZH["tone_and_style"]
	if !ok {
		t.Fatalf("promptSectionsZH missing 'tone_and_style' key")
	}
	if !strings.Contains(tone, "可见输出") {
		t.Errorf("promptSectionsZH[tone_and_style] missing Chinese output mandate\nactual:\n%s", tone)
	}
	if !strings.Contains(tone, "必须用中文") {
		t.Errorf("promptSectionsZH[tone_and_style] missing 'must use Chinese' phrasing\nactual:\n%s", tone)
	}
}

// Regression: every section in the ZH map must be non-empty and trimmed of
// trailing whitespace; this guards against accidental deletions during the
// hard-rule edit.
func TestPromptSectionsZH_AllSectionsNonEmpty(t *testing.T) {
	if len(promptSectionsZH) == 0 {
		t.Fatalf("promptSectionsZH is empty")
	}
	for key, val := range promptSectionsZH {
		if strings.TrimSpace(val) == "" {
			t.Errorf("promptSectionsZH[%q] is empty/whitespace", key)
		}
	}
}
