package materialize

import (
	"strings"
	"testing"
)

func TestBuildSystemPrompt_ZHLocale(t *testing.T) {
	got := buildSystemPrompt(Request{
		Policy: Policy{Locale: "zh-CN"},
		Signals: InboundSignals{
			Directive: "review d2 kernel",
		},
	})
	if strings.Contains(got, "You are executing one WorkItem") {
		t.Fatalf("ZH prompt must not use English intro: %q", got)
	}
	if !strings.Contains(got, "任务指令: review d2 kernel") {
		t.Fatalf("ZH prompt must use localized field labels: %q", got)
	}
	if !strings.Contains(got, "Observe 节点") {
		t.Fatalf("ZH prompt missing localized output hints: %q", got)
	}
}

func TestBuildSystemPrompt_ENLocale(t *testing.T) {
	got := buildSystemPrompt(Request{
		Policy: Policy{Locale: "en"},
		Signals: InboundSignals{
			Directive: "review kernel",
		},
	})
	if !strings.Contains(got, "Directive: review kernel") {
		t.Fatalf("EN prompt: %q", got)
	}
	if !strings.Contains(got, "Do not label observations") {
		t.Fatalf("EN output hints: %q", got)
	}
}

func TestBuildSystemPrompt_DefaultLocaleIsZH(t *testing.T) {
	zh := buildSystemPrompt(Request{
		Signals: InboundSignals{Directive: "x"},
	})
	en := buildSystemPrompt(Request{Policy: Policy{Locale: "en"}, Signals: InboundSignals{Directive: "x"}})
	if zh == en {
		t.Fatal("empty locale should default to ZH via ParseLanguage")
	}
}
