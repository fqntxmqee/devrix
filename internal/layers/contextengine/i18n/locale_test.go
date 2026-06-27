package i18n_test

import (
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/i18n"
)

func TestParseLanguage_DefaultChinese(t *testing.T) {
	if got := i18n.ParseLanguage(""); got != i18n.LocaleZH {
		t.Fatalf("empty language = %q, want zh-CN", got)
	}
	if got := i18n.ParseLanguage("zh-CN"); got != i18n.LocaleZH {
		t.Fatalf("zh-CN = %q", got)
	}
}

func TestParseLanguage_English(t *testing.T) {
	for _, in := range []string{"en-US", "en", "EN_us"} {
		if got := i18n.ParseLanguage(in); got != i18n.LocaleEN {
			t.Fatalf("%q = %q, want en-US", in, got)
		}
	}
}

func TestLocalizeTool_ChineseFreeFork(t *testing.T) {
	_, params := i18n.LocalizeTool(
		"free_fork",
		"Batch fork N child agents",
		`{"type":"object","required":["parent_session","requests"]}`,
		i18n.LocaleZH,
	)
	if !strings.Contains(params, "parent_session") {
		t.Fatalf("free_fork zh params missing parent_session: %q", params)
	}
	if strings.Contains(params, "count") && strings.Contains(params, "directive") {
		t.Fatalf("free_fork still uses stale count/directive schema: %q", params)
	}
}

func TestLocalizeTool_ChineseBuiltin(t *testing.T) {
	desc, params := i18n.LocalizeTool(
		"bash",
		"Execute a shell command",
		`{"type":"object"}`,
		i18n.LocaleZH,
	)
	if !strings.Contains(desc, "shell") && !strings.Contains(desc, "命令") {
		t.Fatalf("unexpected zh bash description: %q", desc)
	}
	if params == `{"type":"object"}` {
		t.Fatal("expected localized bash parameters schema")
	}
}

func TestLocalizeTool_EnglishPassthrough(t *testing.T) {
	const enDesc = "Execute a shell command"
	const enParams = `{"type":"object"}`
	desc, params := i18n.LocalizeTool("bash", enDesc, enParams, i18n.LocaleEN)
	if desc != enDesc || params != enParams {
		t.Fatalf("en passthrough failed: desc=%q params=%q", desc, params)
	}
}

func TestPromptSections_ChineseIntro(t *testing.T) {
	sections := i18n.PromptSections(i18n.LocaleZH)
	if !strings.Contains(sections["intro"], "软件工程") {
		t.Fatalf("zh intro missing expected content: %q", sections["intro"])
	}
}

func TestPromptSections_EnglishIntro(t *testing.T) {
	sections := i18n.PromptSections(i18n.LocaleEN)
	if !strings.Contains(sections["intro"], "software engineering") {
		t.Fatalf("en intro missing expected content: %q", sections["intro"])
	}
}
