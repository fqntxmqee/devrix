package contextengine_test

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/contextengine"
	mockctx "github.com/devrix/devrix/internal/layers/contextengine/mock"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
	sharederrors "github.com/devrix/devrix/internal/shared/errors"
	"github.com/devrix/devrix/internal/shared/types"
	"golang.org/x/sync/errgroup"
)

type infiniteToolLLM struct {
	calls atomic.Int32
}

func (m *infiniteToolLLM) ChatStream(ctx context.Context, req *contextengine.LLMRequest) (<-chan contextengine.LLMChunk, error) {
	m.calls.Add(1)
	ch := make(chan contextengine.LLMChunk, 1)
	go func() {
		defer close(ch)
		select {
		case <-ctx.Done():
			return
		case ch <- contextengine.LLMChunk{
			ToolCalls: []contextengine.ToolCall{{ID: "tc1", Name: "bash", Input: "ls"}},
			Done:      true,
		}:
		}
	}()
	return ch, nil
}

type ctxErrLLM struct{}

func (ctxErrLLM) ChatStream(ctx context.Context, _ *contextengine.LLMRequest) (<-chan contextengine.LLMChunk, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

type failingToolRunner struct{}

func (failingToolRunner) Execute(context.Context, contextengine.ToolCall) (*contextengine.ToolResult, error) {
	return &contextengine.ToolResult{Error: "tool failed"}, nil
}

func collectEvents(run func(emit func(*contracts.EngineEvent))) []*contracts.EngineEvent {
	var events []*contracts.EngineEvent
	run(func(e *contracts.EngineEvent) {
		events = append(events, e)
	})
	return events
}

func newTestPEVEngine(t *testing.T, llm contextengine.ILLMGateway, tools contextengine.IToolRunner, maxIter int) *contextengine.PEVEngine {
	t.Helper()
	cfg := config.DefaultContextEngineConfig()
	cfg.PEV.MaxIterations = maxIter
	cfg.PEV.VerifyMode = config.VerifyModeBasic
	cfg.Plan.Enabled = false
	return contextengine.NewPEVEngine(
		llm,
		tools,
		mustBuiltinRegistry(t),
		mockctx.AllowAllPermission{},
		contextengine.NoOpObserver{},
		&cfg.PEV,
		nil,
		contextengine.NewBuiltinVerifyRunner(t.TempDir()),
		contextengine.NoOpPEVObserver{},
		nil,
		cfg.Plan,
	)
}

// Covers: L5-CTX-28
func TestPEV_ConcurrentSessionIsolation(t *testing.T) {
	engine := newTestPEVEngine(t, &mockctx.LLMGateway{Response: "session ok"}, &mockctx.ToolRunner{Output: "ok"}, 3)

	var g errgroup.Group
	for i := 0; i < 10; i++ {
		i := i
		g.Go(func() error {
			dir := t.TempDir()
			sc := &types.SessionContext{
				SessionID: fmt.Sprintf("session-%d", i),
				WorkDir:   dir,
				Model:     "test",
				PEVState:  types.DefaultPEVState(3),
			}
			events := collectEvents(func(emit func(*contracts.EngineEvent)) {
				_, _ = engine.Run(context.Background(), sc, nil, "hello", emit)
			})
			for _, ev := range events {
				if ev.SessionID != sc.SessionID {
					return fmt.Errorf("event session leak: want %s got %s", sc.SessionID, ev.SessionID)
				}
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		t.Fatalf("concurrent sessions: %v", err)
	}
}

// Covers: L5-CTX-29
func TestPEV_ContextCancellation_Cleanup(t *testing.T) {
	engine := newTestPEVEngine(t, ctxErrLLM{}, &mockctx.ToolRunner{Output: "ok"}, 3)
	before := runtime.NumGoroutine()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	sc := &types.SessionContext{
		SessionID: "cancel-session",
		WorkDir:   t.TempDir(),
		Model:     "test",
		PEVState:  types.DefaultPEVState(3),
	}
	_, err := engine.Run(ctx, sc, nil, "hello", func(*contracts.EngineEvent) {})
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		var se *sharederrors.SentinelError
		if !errors.As(err, &se) || se.Code != sharederrors.CodeLLMUnavailable {
			t.Fatalf("expected DeadlineExceeded or LLM unavailable, got %v", err)
		}
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if runtime.NumGoroutine() <= before+5 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("goroutine leak suspected: before=%d after=%d", before, runtime.NumGoroutine())
}

// Covers: L5-CTX-29
func TestPEV_MaxIterations_Exhausted(t *testing.T) {
	llm := &infiniteToolLLM{}
	cfg := config.DefaultContextEngineConfig()
	cfg.PEV.MaxIterations = 3
	cfg.PEV.VerifyMode = config.VerifyModeCommands
	cfg.Plan.Enabled = false
	engine := contextengine.NewPEVEngine(
		llm,
		failingToolRunner{},
		mustBuiltinRegistry(t),
		mockctx.AllowAllPermission{},
		contextengine.NoOpObserver{},
		&cfg.PEV,
		nil,
		contextengine.NewBuiltinVerifyRunner(t.TempDir()),
		contextengine.NoOpPEVObserver{},
		nil,
		cfg.Plan,
	)
	sc := &types.SessionContext{
		SessionID: "max-iter",
		WorkDir:   t.TempDir(),
		Model:     "test",
		PEVState:  types.DefaultPEVState(3),
	}

	_, err := engine.Run(context.Background(), sc, nil, "hello", func(*contracts.EngineEvent) {})
	if err == nil {
		t.Fatal("expected max iterations error")
	}
	var se *sharederrors.SentinelError
	if !errors.As(err, &se) {
		t.Fatalf("expected SentinelError, got %T", err)
	}
	if se.Code != sharederrors.CodePEVMaxIterations {
		t.Fatalf("code: got %s want %s", se.Code, sharederrors.CodePEVMaxIterations)
	}
	if llm.calls.Load() != 3 {
		t.Fatalf("expected 3 LLM calls, got %d", llm.calls.Load())
	}
}
