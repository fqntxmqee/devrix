//go:build smoke

package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/devrix/devrix/internal/shared/config"
)

func TestSmoke_DefaultConfigLoads(t *testing.T) {
	cfg := config.DefaultConfig()
	if cfg == nil {
		t.Fatal("DefaultConfig() returned nil")
	}
	if cfg.Session.MaxSessions <= 0 {
		t.Errorf("expected positive MaxSessions, got %d", cfg.Session.MaxSessions)
	}
	if cfg.Permission.DefaultTimeout <= 0 {
		t.Errorf("expected positive permission timeout, got %v", cfg.Permission.DefaultTimeout)
	}
}

func TestSmoke_ProjectConfigFileExists(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}

	root := filepath.Join(wd, "..", "..")
	configPath := filepath.Join(root, "devrix.yaml")
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("expected devrix.yaml at repo root: %v", err)
	}

	commCfg, authCfg, _, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if commCfg == nil {
		t.Fatal("communication config is nil")
	}
	if authCfg == nil {
		t.Fatal("auth config is nil")
	}
}

func TestSmoke_CommunicationPackageBuilds(t *testing.T) {
	cfg := config.DefaultConfig()
	if cfg.Commands.Prefix != "/" {
		t.Errorf("expected command prefix /, got %q", cfg.Commands.Prefix)
	}
	if len(cfg.Commands.List) == 0 {
		t.Error("expected non-empty command list")
	}
}
