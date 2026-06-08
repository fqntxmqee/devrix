package config

import (
	"testing"
	"time"
)

func TestBuildMultiAgentConfig_should_apply_defaults(t *testing.T) {
	cfg := BuildMultiAgentConfig(nil)
	if cfg.MaxChildren != 3 {
		t.Fatalf("MaxChildren = %d, want 3", cfg.MaxChildren)
	}
	if cfg.MaxTotalAgents != 5 {
		t.Fatalf("MaxTotalAgents = %d, want 5", cfg.MaxTotalAgents)
	}
	if cfg.DefaultTimeout != 5*time.Minute {
		t.Fatalf("DefaultTimeout = %v, want 5m", cfg.DefaultTimeout)
	}
}

func TestBuildMultiAgentConfig_should_merge_enabled_flag(t *testing.T) {
	cfg := BuildMultiAgentConfig(&MultiAgentFileConfig{Enabled: true})
	if !cfg.Enabled {
		t.Fatal("Enabled should be true when set in file config")
	}
}

func TestBuildMultiAgentConfig_should_merge_file_values(t *testing.T) {
	cfg := BuildMultiAgentConfig(&MultiAgentFileConfig{
		MaxChildren:    5,
		DefaultTimeout: 2 * time.Minute,
		DefaultMaxIter: 10,
		DefaultMode:    "chain-of-thought",
	})
	if cfg.MaxChildren != 5 {
		t.Fatalf("MaxChildren = %d, want 5", cfg.MaxChildren)
	}
	if cfg.DefaultMaxIter != 10 {
		t.Fatalf("DefaultMaxIter = %d, want 10", cfg.DefaultMaxIter)
	}
	if cfg.DefaultMode != "chain-of-thought" {
		t.Fatalf("DefaultMode = %q, want chain-of-thought", cfg.DefaultMode)
	}
}

func TestBuildMultiAgentConfig_should_reject_invalid_max_children(t *testing.T) {
	cfg := BuildMultiAgentConfig(&MultiAgentFileConfig{MaxChildren: 99})
	if cfg.MaxChildren != 3 {
		t.Fatalf("invalid max_children should keep default 3, got %d", cfg.MaxChildren)
	}
}
