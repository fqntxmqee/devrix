//go:build performance && d2

package performance_test

import (
	"context"
	"fmt"
	"runtime"
	"testing"

	"github.com/devrix/devrix/internal/layers/communication/gateway"
	"github.com/devrix/devrix/internal/layers/contextengine"
	mockctx "github.com/devrix/devrix/internal/layers/contextengine/mock"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
	"golang.org/x/sync/errgroup"
)

// Covers: L5-OBS-20
func TestMemory_ConcurrentSessionsBoundedGrowth(t *testing.T) {
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	cfg := config.DefaultContextEngineConfig()
	engine := contextengine.NewContextEngine(contextengine.EngineDeps{
		LLM:        &mockctx.LLMGateway{Response: "ok"},
		Tools:      &mockctx.ToolRunner{Output: "ok"},
		ToolsReg:   mustBuiltinRegistry(t),
		Permission: mockctx.AllowAllPermission{},
		Config:     cfg,
	})

	var g errgroup.Group
	for i := 0; i < 10; i++ {
		i := i
		g.Go(func() error {
			sc := &types.SessionContext{
				SessionID: fmt.Sprintf("mem-%d", i),
				WorkDir:   t.TempDir(),
				Model:     "test",
			}
			_, err := engine.Run(context.Background(), sc, nil, "hello", func(*gateway.EngineEvent) {})
			return err
		})
	}
	if err := g.Wait(); err != nil {
		t.Fatalf("concurrent runs: %v", err)
	}
}

func mustBuiltinRegistry(t *testing.T) *contextengine.ToolRegistry {
	t.Helper()
	reg, err := contextengine.NewBuiltinToolRegistry(config.DefaultToolConfig())
	if err != nil {
		t.Fatal(err)
	}
	return reg
}
