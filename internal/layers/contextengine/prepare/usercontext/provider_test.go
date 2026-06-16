package usercontext_test

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/prepare/usercontext"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

// T: D2-S10-A01-T35
func TestProvider_Get_returns_currentDate_when_empty_session(t *testing.T) {
	p := usercontext.NewProvider(nil, config.UserContextConfig{Mode: "prepend"})
	ctx := p.Get(context.Background(), nil)
	if ctx["currentDate"] == "" {
		t.Fatal("expected currentDate to be set")
	}
}

// T: D2-S10-A01-T35
func TestProvider_Get_returns_workDir_from_session(t *testing.T) {
	p := usercontext.NewProvider(nil, config.UserContextConfig{Mode: "prepend"})
	sc := &types.SessionContext{WorkDir: "/home/user/project"}
	ctx := p.Get(context.Background(), sc)
	if ctx["workDir"] != "/home/user/project" {
		t.Fatalf("expected workDir '/home/user/project', got %q", ctx["workDir"])
	}
}

// T: D2-S10-A01-T35
func TestProvider_ShouldEmbedInSystem_prepend_mode_returns_false(t *testing.T) {
	p := usercontext.NewProvider(nil, config.UserContextConfig{Mode: "prepend"})
	if p.ShouldEmbedInSystem() {
		t.Fatal("expected false for prepend mode")
	}
}

// T: D2-S10-A01-T35
func TestProvider_ShouldEmbedInSystem_system_mode_returns_true(t *testing.T) {
	p := usercontext.NewProvider(nil, config.UserContextConfig{Mode: "system"})
	if !p.ShouldEmbedInSystem() {
		t.Fatal("expected true for system mode")
	}
}

// T: D2-S10-A01-T35
func TestPrependForAPI_wraps_context_in_system_reminder(t *testing.T) {
	msgs := []types.Message{
		{Role: types.MessageRoleUser, Content: "hello"},
	}
	ctx := map[string]string{"currentDate": "2026-06-16"}
	result := usercontext.PrependForAPI(msgs, ctx)
	if len(result) != 2 {
		t.Fatalf("expected 2 messages (system-reminder + user), got %d", len(result))
	}
	if result[0].Role != types.MessageRoleUser {
		t.Fatalf("expected system-reminder as user-role message, got %s", result[0].Role)
	}
	if result[1].Content != "hello" {
		t.Fatalf("expected original message, got %q", result[1].Content)
	}
}

// T: D2-S10-A01-T35
func TestPrependForAPI_empty_context_returns_original(t *testing.T) {
	msgs := []types.Message{
		{Role: types.MessageRoleUser, Content: "hello"},
	}
	result := usercontext.PrependForAPI(msgs, nil)
	if len(result) != 1 {
		t.Fatalf("expected 1 message, got %d", len(result))
	}
}

// T: D2-S10-A01-T35
func TestOmitClaudeMd_removes_claudeMd_key(t *testing.T) {
	ctx := map[string]string{
		"claudeMd":    "# Project Info",
		"currentDate": "2026-06-16",
	}
	result := usercontext.OmitClaudeMd(ctx)
	if _, ok := result["claudeMd"]; ok {
		t.Fatal("expected claudeMd to be removed")
	}
	if result["currentDate"] != "2026-06-16" {
		t.Fatal("expected currentDate to be preserved")
	}
}
