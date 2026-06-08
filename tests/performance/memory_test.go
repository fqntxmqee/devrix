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
	"github.com/devrix/devrix/internal/layers/contextengine/registry"
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
	cfg.Plan.Enabled = false
	engine := contextengine.NewPEVEngine(
		&mockctx.LLMGateway{Response: "ok"},
		&mockctx.ToolRunner{Output: "ok"},
		registry.NewBuiltinRegistry(),
		mockctx.AllowAllPermission{},
		contextengine.NoOpObserver{},
		&cfg.PEV,
		nil,
		contextengine.NewBuiltinVerifyRunner(t.TempDir()),
		contextengine.NoOpPEVObserver{},
		nil,
		cfg.Plan,
	)

	var g errgroup.Group
	for i := 0; i < 10; i++ {
		i := i
		g.Go(func() error {
			sc := &types.SessionContext{
				SessionID: fmt.Sprintf("mem-%d", i),
				WorkDir:   t.TempDir(),
				Model:     "test",
				PEVState:  types.DefaultPEVState(3),
			}
			_, err := engine.Run(context.Background(), sc, nil, "hello", func(*gateway.EngineEvent) {})
			return err
		})
	}
	if err := g.Wait(); err != nil {
		t.Fatalf("concurrent runs: %v", err)
	}

	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	growth := int64(after.Alloc) - int64(before.Alloc)
	const maxGrowth = 50 * 1024 * 1024
	if growth > maxGrowth {
		t.Fatalf("memory growth %d bytes exceeds %d", growth, maxGrowth)
	}
}
