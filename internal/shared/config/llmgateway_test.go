package config_test

import (
	"os"
	"path/filepath"
	"testing"

	sharedconfig "github.com/devrix/devrix/internal/shared/config"
)

// T: D3-S6-A01-T01
func TestLoadLLMGatewayConfig_should_parse_devrix_yaml(t *testing.T) {
	path := filepath.Join("..", "..", "..", "devrix.yaml")
	if _, err := os.Stat(path); err != nil {
		t.Skip("devrix.yaml not found from test cwd")
	}
	cfg, err := sharedconfig.LoadLLMGatewayConfig(path)
	if err != nil {
		t.Fatalf("LoadLLMGatewayConfig: %v", err)
	}
	if cfg.DefaultProvider != "minimax" {
		t.Errorf("DefaultProvider: got %s", cfg.DefaultProvider)
	}
	if cfg.Providers["deepseek"].FallbackModel != "deepseek-v4-pro" {
		t.Errorf("deepseek fallback: got %s", cfg.Providers["deepseek"].FallbackModel)
	}
	if _, ok := cfg.ModelRouting["deepseek-*"]; !ok {
		t.Error("expected deepseek-* routing")
	}
}
