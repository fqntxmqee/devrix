package config

import "testing"

// D2-S11-A01-T01 (revised DM-20260618-010): turn_runtime defaults for D7.
func TestDefaultTurnRuntimeConfig_TurnDefaults(t *testing.T) {
	got := DefaultTurnRuntimeConfig()
	if got.MaxTurns != 50 {
		t.Fatalf("DefaultTurnRuntimeConfig().MaxTurns = %d, want 50", got.MaxTurns)
	}
	if !got.CompressPerTurn {
		t.Fatalf("DefaultTurnRuntimeConfig().CompressPerTurn = false, want true")
	}
}

func TestDefaultContextEngineConfig_TurnRuntimeDefaults(t *testing.T) {
	cfg := DefaultContextEngineConfig()
	if cfg.TurnRuntime.MaxTurns != 50 {
		t.Fatalf("DefaultContextEngineConfig().TurnRuntime.MaxTurns = %d, want 50", cfg.TurnRuntime.MaxTurns)
	}
	if !cfg.TurnRuntime.CompressPerTurn {
		t.Fatalf("DefaultContextEngineConfig().TurnRuntime.CompressPerTurn = false, want true")
	}
}

func TestMergeTurnRuntimeConfig_LegacyQueryLoopAlias(t *testing.T) {
	base := DefaultTurnRuntimeConfig()
	override := TurnRuntimeConfig{MaxTurns: 12, StreamingTools: true}
	got := mergeTurnRuntimeConfig(base, override)
	if got.MaxTurns != 12 {
		t.Fatalf("MaxTurns = %d, want 12", got.MaxTurns)
	}
	if !got.StreamingTools {
		t.Fatal("StreamingTools = false, want true")
	}
}
