package config

import (
	"os"
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

func TestBuildAgentToolsConfig_should_return_defaults_when_nil(t *testing.T) {
	cfg := BuildAgentToolsConfig(nil)
	if cfg.Enabled {
		t.Fatal("Enabled should be false by default")
	}
	if len(cfg.Tools) != 0 {
		t.Fatalf("Tools len = %d, want 0", len(cfg.Tools))
	}
}

func TestBuildAgentToolsConfig_should_apply_defaults(t *testing.T) {
	file := &AgentToolsFileConfig{
		Enabled: true,
		Tools: []AgentToolFileConfig{
			{
				Name:        "test-tool",
				DisplayName: "Test Tool",
				Description: "A test tool",
				Command:     "bash",
				Args:        []string{"-c", "echo hello"},
			},
		},
	}
	cfg := BuildAgentToolsConfig(file)
	if !cfg.Enabled {
		t.Fatal("Enabled should be true")
	}
	if len(cfg.Tools) != 1 {
		t.Fatalf("Tools len = %d, want 1", len(cfg.Tools))
	}
	if cfg.Tools[0].Timeout != 5*time.Minute {
		t.Fatalf("default timeout = %v, want 5m", cfg.Tools[0].Timeout)
	}
	if cfg.Tools[0].IdleTimeout != 5*time.Minute {
		t.Fatalf("default idle_timeout = %v, want 5m", cfg.Tools[0].IdleTimeout)
	}
}

func TestBuildAgentToolsConfig_should_parse_duration_overrides(t *testing.T) {
	file := &AgentToolsFileConfig{
		Enabled: true,
		Tools: []AgentToolFileConfig{
			{
				Name:        "test-tool",
				Command:     "bash",
				Timeout:     "10s",
				IdleTimeout: "30s",
			},
		},
	}
	cfg := BuildAgentToolsConfig(file)
	if cfg.Tools[0].Timeout != 10*time.Second {
		t.Fatalf("Timeout = %v, want 10s", cfg.Tools[0].Timeout)
	}
	if cfg.Tools[0].IdleTimeout != 30*time.Second {
		t.Fatalf("IdleTimeout = %v, want 30s", cfg.Tools[0].IdleTimeout)
	}
}

func TestBuildAgentToolsConfig_should_ignore_invalid_duration(t *testing.T) {
	file := &AgentToolsFileConfig{
		Enabled: true,
		Tools: []AgentToolFileConfig{
			{
				Name:        "test-tool",
				Command:     "bash",
				Timeout:     "not-a-duration",
				IdleTimeout: "-5s",
			},
		},
	}
	cfg := BuildAgentToolsConfig(file)
	if cfg.Tools[0].Timeout != 5*time.Minute {
		t.Fatalf("invalid timeout should fall back to default 5m, got %v", cfg.Tools[0].Timeout)
	}
	if cfg.Tools[0].IdleTimeout != 5*time.Minute {
		t.Fatalf("negative idle_timeout should fall back to default 5m, got %v", cfg.Tools[0].IdleTimeout)
	}
}

func TestBuildAgentToolsConfig_should_support_multiple_tools(t *testing.T) {
	file := &AgentToolsFileConfig{
		Enabled: true,
		Tools: []AgentToolFileConfig{
			{Name: "tool1", Command: "cmd1"},
			{Name: "tool2", Command: "cmd2"},
			{Name: "tool3", Command: "cmd3"},
		},
	}
	cfg := BuildAgentToolsConfig(file)
	if len(cfg.Tools) != 3 {
		t.Fatalf("Tools len = %d, want 3", len(cfg.Tools))
	}
}

func TestLoadAgentToolsConfig_should_return_defaults_when_empty_path(t *testing.T) {
	cfg, err := LoadAgentToolsConfig("")
	if err != nil {
		t.Fatalf("LoadAgentToolsConfig('') err = %v", err)
	}
	if cfg.Enabled {
		t.Fatal("Enabled should be false for empty path")
	}
}

func TestLoadAgentToolsConfig_should_load_from_file(t *testing.T) {
	yaml := `
agent_tools:
  enabled: true
  tools:
    - name: test-agent
      display_name: "Test Agent"
      description: "A test agent tool"
      capabilities: ["coding", "review"]
      command: "bash"
      args: ["-c", "echo hello"]
      timeout: "30s"
      idle_timeout: "1m"
`
	tmpDir := t.TempDir()
	path := tmpDir + "/devrix.yaml"
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatalf("write temp config: %v", err)
	}

	cfg, err := LoadAgentToolsConfig(path)
	if err != nil {
		t.Fatalf("LoadAgentToolsConfig err = %v", err)
	}
	if !cfg.Enabled {
		t.Fatal("Enabled should be true")
	}
	if len(cfg.Tools) != 1 {
		t.Fatalf("Tools len = %d, want 1", len(cfg.Tools))
	}
	if cfg.Tools[0].Name != "test-agent" {
		t.Fatalf("Name = %q, want 'test-agent'", cfg.Tools[0].Name)
	}
	if len(cfg.Tools[0].Capabilities) != 2 {
		t.Fatalf("Capabilities len = %d, want 2", len(cfg.Tools[0].Capabilities))
	}
	if cfg.Tools[0].Timeout != 30*time.Second {
		t.Fatalf("Timeout = %v, want 30s", cfg.Tools[0].Timeout)
	}
	if cfg.Tools[0].IdleTimeout != 1*time.Minute {
		t.Fatalf("IdleTimeout = %v, want 1m", cfg.Tools[0].IdleTimeout)
	}
}
