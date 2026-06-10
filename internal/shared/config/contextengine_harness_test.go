package config_test

import (
	"testing"

	"github.com/devrix/devrix/internal/shared/config"
)

func TestValidateHarnessConfig_should_reject_block_mode_in_v5a(t *testing.T) {
	err := config.ValidateHarnessConfig(config.DefaultHarnessConfig(), config.PreflightConfig{
		Enabled: true,
		Mode:    config.PreflightModeBlock,
		TokenBudget: 8000,
		WarnRatio: 0.85,
	})
	if err == nil {
		t.Fatal("expected error for block mode")
	}
}
