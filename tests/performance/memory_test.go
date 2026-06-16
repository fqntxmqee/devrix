//go:build performance && d2

package performance_test

import (
	"context"
	"fmt"
	"runtime"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine"
	mockctx "github.com/devrix/devrix/internal/layers/contextengine/mock"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
	"golang.org/x/sync/errgroup"
)

// T: D5-S2-A01-T07
func TestMemory_ConcurrentSessionsBoundedGrowth(t *testing.T) {
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	cfg := config.DefaultContextEngineConfig()
	engine := contextengine.NewContextEngine(contextengine.EngineDeps{
		QueryLLMCaller: &mockctx.StaticLLMCaller{Response: "ok"},
		Summarizer:     &mockctx.StaticSummarizer{},
		Tools:      &mockctx.ToolRunner{Output: "ok"},
		ToolsReg:   mustBuiltinRegistry(t),
		Permission: mockctx.AllowAllPermission{},
		Config:     cfg,
	})

	var g errgroup.Group
	for i := 0; i < 10; i++ {
		i := i
		g.Go(func() error {
			session := types.NewSession(fmt.Sprintf("mem-%d", i), "cli", t.TempDir())
			ch := engine.Process(context.Background(), session, "hello")
			for ev := range ch {
				if ev.Type == "error" {
					return fmt.Errorf("engine error: %s", ev.Content)
				}
			}
			return nil
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
