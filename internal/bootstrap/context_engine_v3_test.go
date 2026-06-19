package bootstrap

import (
	"testing"

	persistmemory "github.com/devrix/devrix/internal/layers/contextengine/persist/memory"
	"github.com/devrix/devrix/internal/shared/config"
)

func TestWireContextV3_should_return_longterm_memory(t *testing.T) {
	ctxCfg := config.DefaultContextEngineConfig()
	ctxCfg.LongTerm.Enabled = true

	recaller, store := WireContextV3(ctxCfg)
	if recaller == nil {
		t.Fatal("expected long-term recaller instance")
	}
	if store == nil {
		t.Fatal("expected long-term store instance")
	}
}

func TestWireContextV3_should_return_disabled_memory_when_longterm_disabled(t *testing.T) {
	ctxCfg := config.DefaultContextEngineConfig()
	ctxCfg.LongTerm.Enabled = false

	recaller, _ := WireContextV3(ctxCfg)
	if _, ok := recaller.(*persistmemory.DisabledLongTerm); !ok {
		t.Fatalf("expected disabled long-term memory, got %T", recaller)
	}
}
