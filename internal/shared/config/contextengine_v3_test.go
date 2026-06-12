package config_test

import (
	"testing"

	"github.com/devrix/devrix/internal/shared/config"
)

func TestValidateContextEngineV3Config_should_reject_invalid_longterm_topic(t *testing.T) {
	cfg := config.DefaultContextEngineConfig()
	cfg.LongTerm.Topics = []string{"Bad Topic"}
	if err := config.ValidateContextEngineConfig(cfg); err == nil {
		t.Fatal("expected validation error for topic name")
	}
}

func TestValidateContextEngineV3Config_should_accept_v3_defaults(t *testing.T) {
	cfg := config.DefaultContextEngineConfig()
	if err := config.ValidateContextEngineConfig(cfg); err != nil {
		t.Fatalf("expected valid defaults, got %v", err)
	}
}

func TestResolvedLongTermDBPath_should_expand_tilde(t *testing.T) {
	path, err := config.ResolvedLongTermDBPath(config.LongTermConfig{DBPath: "~/.devrix/memory.db"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path == "" || path[0] == '~' {
		t.Fatalf("expected expanded path, got %q", path)
	}
}
