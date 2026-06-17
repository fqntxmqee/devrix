package bootstrap

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/devrix/devrix/internal/shared/config"
)

func TestCoordinatorConfigFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "d7-test-config.yaml")
	if err := os.WriteFile(path, []byte("d7:\n  enabled: true\n  routing_mode: rule_orchestrate\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	fileCfg, err := config.LoadConfigFile(path)
	if err != nil {
		t.Fatal(err)
	}
	merged := config.BuildCoordinatorConfig(&fileCfg.Coordinator)
	if merged.RoutingMode != "rule_orchestrate" {
		t.Fatalf("routing_mode = %q, want rule_orchestrate", merged.RoutingMode)
	}
}
