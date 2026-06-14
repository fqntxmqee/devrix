package prompt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/shared/config"
)

func TestDiscoverAgents_should_prefer_closer_directory(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "pkg", "app")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	writeAgents(t, root, "AGENTS.md", "root-rule")
	writeAgents(t, sub, "AGENTS.md", "leaf-rule")

	loader := NewLoader(&config.SystemPromptConfig{
		Sources: []string{"AGENTS.md"},
		WalkUp:  configBool(true),
	})
	merged := loader.Load(sub)
	if !strings.Contains(merged, "root-rule") {
		t.Fatal("expected ancestor AGENTS.md")
	}
	if !strings.Contains(merged, "leaf-rule") {
		t.Fatal("expected leaf AGENTS.md")
	}
	if strings.LastIndex(merged, "leaf-rule") <= strings.LastIndex(merged, "root-rule") {
		t.Fatal("leaf AGENTS.md should appear after ancestor (higher priority)")
	}
}

func TestDiscoverAgents_should_load_devrix_before_agents_in_same_dir(t *testing.T) {
	dir := t.TempDir()
	writeAgents(t, dir, "AGENTS.md", "generic")
	writeAgents(t, dir, ".devrix/AGENTS.md", "specific")

	loader := NewLoader(&config.SystemPromptConfig{
		Sources: []string{".devrix/AGENTS.md", "AGENTS.md"},
		WalkUp:  configBool(false),
	})
	merged := loader.Load(dir)
	if !strings.Contains(merged, "generic") || !strings.Contains(merged, "specific") {
		t.Fatalf("unexpected merge: %q", merged)
	}
	if strings.LastIndex(merged, "specific") <= strings.LastIndex(merged, "generic") {
		t.Fatal(".devrix/AGENTS.md should follow AGENTS.md in same directory")
	}
}

func TestDiscoverAgents_should_load_rules_glob(t *testing.T) {
	dir := t.TempDir()
	rulesDir := filepath.Join(dir, ".devrix", "rules")
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeAgents(t, rulesDir, "01-style.md", "style-rule")
	writeAgents(t, dir, ".devrix/AGENTS.md", "agent-rule")

	loader := NewLoader(nil)
	merged := loader.Load(dir)
	if !strings.Contains(merged, "style-rule") {
		t.Fatal("expected rules file content")
	}
	if strings.Index(merged, "style-rule") > strings.Index(merged, "agent-rule") {
		t.Fatal("rules should load before main AGENTS in same directory walk")
	}
}

func configBool(v bool) *bool { return &v }

func writeAgents(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
