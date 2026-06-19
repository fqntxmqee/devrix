package config

import "testing"

// D2-S11-A01-T01 (revised DM-20260618-010): query_loop.enabled removed;
// turn defaults remain stable for D7 wiring.
func TestDefaultQueryLoopConfig_TurnDefaults(t *testing.T) {
	got := DefaultQueryLoopConfig()
	if got.MaxTurns != 50 {
		t.Fatalf("DefaultQueryLoopConfig().MaxTurns = %d, want 50", got.MaxTurns)
	}
	if !got.CompressPerTurn {
		t.Fatalf("DefaultQueryLoopConfig().CompressPerTurn = false, want true")
	}
}

func TestDefaultContextEngineConfig_QueryLoopDefaults(t *testing.T) {
	cfg := DefaultContextEngineConfig()
	if cfg.QueryLoop.MaxTurns != 50 {
		t.Fatalf("DefaultContextEngineConfig().QueryLoop.MaxTurns = %d, want 50", cfg.QueryLoop.MaxTurns)
	}
	if !cfg.QueryLoop.CompressPerTurn {
		t.Fatalf("DefaultContextEngineConfig().QueryLoop.CompressPerTurn = false, want true")
	}
}
