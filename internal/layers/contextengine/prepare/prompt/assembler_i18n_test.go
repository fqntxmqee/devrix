package prompt_test

import (
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/i18n"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/prompt"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

func TestSystemPromptAssembler_DefaultChineseCore(t *testing.T) {
	a := prompt.NewSystemPromptAssembler(config.WorkspacePromptConfig{})
	out, _ := a.Build(prompt.SystemPromptBuildInput{
		WorkDir: "/tmp/ws",
		Session: types.NewSession("sess-1", "d7", ""),
	})
	if !strings.Contains(out, "你是 Devrix") {
		t.Fatalf("expected Chinese core template, got prefix: %.120q", out)
	}
	if strings.Contains(out, "## Session Context") {
		t.Fatal("expected Chinese session context header")
	}
	if !strings.Contains(out, "工作区指引") {
		t.Fatal("missing Chinese workspace guidance title")
	}
	if !strings.Contains(out, "日期:") {
		t.Fatal("missing Chinese session date label")
	}
}

func TestSystemPromptAssembler_EnglishCore(t *testing.T) {
	a := prompt.NewSystemPromptAssembler(config.WorkspacePromptConfig{
		Language: string(i18n.LocaleEN),
	})
	out, _ := a.Build(prompt.SystemPromptBuildInput{
		WorkDir: "/tmp/ws",
		Session: types.NewSession("sess-1", "d7", ""),
	})
	if !strings.Contains(out, "You are Devrix") {
		t.Fatalf("expected English core template, got prefix: %.120q", out)
	}
	if !strings.Contains(out, "## Session Context") {
		t.Fatal("missing English session context header")
	}
}
