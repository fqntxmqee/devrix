package config_test

import (
	"testing"
	"time"

	"github.com/devrix/devrix/internal/shared/config"
)

func TestValidateContextEngineConfig_should_reject_autocompact_timeout_over_limit(t *testing.T) {
	cfg := config.DefaultContextEngineConfig()
	cfg.Compression.Autocompact.Enabled = true
	cfg.Compression.Autocompact.Timeout = 30 * time.Second
	if err := config.ValidateContextEngineConfig(cfg); err == nil {
		t.Fatal("expected validation error")
	}
}
