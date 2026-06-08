package config

import "testing"

func TestBuildToolConfig_should_default_sandbox_enabled(t *testing.T) {
	cfg := BuildToolConfig(nil)
	if !cfg.SandboxEnabled() {
		t.Fatal("expected sandbox enabled by default")
	}
	if cfg.ConcurrentMax != 10 {
		t.Fatalf("concurrent_max = %d, want 10", cfg.ConcurrentMax)
	}
}

func TestBuildToolConfig_should_honor_yaml_sandbox_disabled(t *testing.T) {
	disabled := false
	cfg := BuildToolConfig(&ConfigFile{
		Tool: ToolConfig{
			Sandbox: ToolSandboxConfig{
				Enabled: &disabled,
			},
			ConcurrentMax: 5,
		},
	})
	if cfg.SandboxEnabled() {
		t.Fatal("expected sandbox disabled")
	}
	if cfg.ConcurrentMax != 5 {
		t.Fatalf("concurrent_max = %d, want 5", cfg.ConcurrentMax)
	}
}
