package query_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/contextengine/query"
	"github.com/devrix/devrix/internal/shared/types"
)

type slowToolExecutor struct {
	calls atomic.Int32
}

func (s *slowToolExecutor) Execute(_ context.Context, call query.ToolCall) (string, string, error) {
	s.calls.Add(1)
	time.Sleep(30 * time.Millisecond)
	return "ok:" + call.Name, "", nil
}

func TestStreamingToolExecutor_should_run_safe_tools_in_parallel(t *testing.T) {
	exec := &query.StreamingToolExecutor{Tools: &slowToolExecutor{}}
	start := time.Now()
	batch := exec.ExecuteBatch(context.Background(), &types.SessionContext{SessionID: "s1"}, []query.BatchToolRef{
		{ID: "1", Name: "read_file", Input: `{}`},
		{ID: "2", Name: "glob", Input: `{}`},
	})
	elapsed := time.Since(start)
	if len(batch) != 2 {
		t.Fatalf("expected 2 results, got %d", len(batch))
	}
	if elapsed >= 55*time.Millisecond {
		t.Fatalf("expected parallel execution under 55ms, took %v", elapsed)
	}
}
