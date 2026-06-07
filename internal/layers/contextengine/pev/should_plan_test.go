package pev_test

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/pev"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

func TestShouldPlan(t *testing.T) {
	cfg := config.DefaultPlanConfig()
	cfg.Enabled = true

	if !pev.ShouldPlan(cfg, types.DefaultPEVState(3), "/plan big task") {
		t.Fatal("expected /plan prefix to trigger plan")
	}

	cfg.AutoDetect = true
	longMsg := string(make([]byte, 250))
	if !pev.ShouldPlan(cfg, types.DefaultPEVState(3), longMsg) {
		t.Fatal("expected auto_detect for long message")
	}

	cfg.Enabled = false
	if pev.ShouldPlan(cfg, types.DefaultPEVState(3), "/plan x") {
		t.Fatal("expected plan disabled")
	}
}
