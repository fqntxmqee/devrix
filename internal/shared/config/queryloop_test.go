package config

import "testing"

// D2-S11-A01-T01: query_loop.enabled 默认值为 true（DM-20260611-004）。
// 验证 DefaultQueryLoopConfig().Enabled == true。
func TestDefaultQueryLoopConfig_EnabledByDefault(t *testing.T) {
	got := DefaultQueryLoopConfig()
	if !got.Enabled {
		t.Fatalf("DefaultQueryLoopConfig().Enabled = false, want true (D2-S11-A01-T01: query_loop.enabled must default to true)")
	}
}

// D2-S11-A01-T01 延伸: DefaultContextEngineConfig().QueryLoop.Enabled 也必须为 true。
func TestDefaultContextEngineConfig_QueryLoopEnabled(t *testing.T) {
	cfg := DefaultContextEngineConfig()
	if !cfg.QueryLoop.Enabled {
		t.Fatalf("DefaultContextEngineConfig().QueryLoop.Enabled = false, want true (D2-S11-A01-T01)")
	}
}
