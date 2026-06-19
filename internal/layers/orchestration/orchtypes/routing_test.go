package orchtypes

import "testing"

func TestConfig_IsLoopFirst_Default(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = true
	if !cfg.IsLoopFirst() {
		t.Fatal("default routing mode should be loop_first")
	}
}
