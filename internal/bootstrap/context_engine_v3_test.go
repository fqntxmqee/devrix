package bootstrap

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/prepare/memory"
	"github.com/devrix/devrix/internal/shared/config"
)

func TestWireContextV3_should_return_longterm_memory(t *testing.T) {
	ctxCfg := config.DefaultContextEngineConfig()
	ctxCfg.LongTerm.Enabled = true

	longTerm := WireContextV3(ctxCfg)
	if longTerm == nil {
		t.Fatal("expected long-term memory instance")
	}
}

func TestWireContextV3_should_return_disabled_memory_when_longterm_disabled(t *testing.T) {
	ctxCfg := config.DefaultContextEngineConfig()
	ctxCfg.LongTerm.Enabled = false

	longTerm := WireContextV3(ctxCfg)
	if _, ok := longTerm.(*memory.DisabledLongTermMemory); !ok {
		t.Fatalf("expected disabled long-term memory, got %T", longTerm)
	}
}
