package tools_test

// W11 phase 2c migration: free_fork tool 单元测试 now exercises the
// surface path (FreeForkSurface) instead of the deleted legacy
// tools.freeforkRunner + tools.SetFreeForker path.
//
// The semantics are identical: pass a FreeForkerFunc into
// surface.NewFreeForkSurface, then call surface.Execute. The previous
// "global not initialized" case is reproduced by passing nil.

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce/tools"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce/tools/surface"
)

// stubFreeforkFunc 是直接构造给 surface.NewFreeForkSurface 的 stub 函数。
type stubFreeforkFunc struct {
	failOn int32
	calls  int32
}

func (s *stubFreeforkFunc) Fork(ctx context.Context, parentSession string, reqs []tools.FreeForkRequestDTO) ([]tools.FreeForkHandleDTO, error) {
	n := atomic.AddInt32(&s.calls, 1)
	if s.failOn > 0 && n == s.failOn {
		return nil, errors.New("stub: factory failed on request")
	}
	handles := make([]tools.FreeForkHandleDTO, 0, len(reqs))
	for i, r := range reqs {
		handles = append(handles, tools.FreeForkHandleDTO{
			AgentID:  "agent-" + r.Name,
			SandboxPath: "/wt/" + r.Name,
			Name:     r.Name,
		})
		_ = i
	}
	return handles, nil
}

func executeFreeFork(t *testing.T, forker tools.FreeForkerFunc, input string) (string, string) {
	t.Helper()
	s := surface.NewFreeForkSurface(forker)
	res, err := s.Execute(context.Background(), "free_fork", input, "")
	if err != nil {
		t.Fatalf("surface.Execute: %v", err)
	}
	return res.Output, res.Error
}

// T: D4-S11-A02-T01 / D4-S13-A02-T01 — 批量分叉 3 个。
func TestFreeForkTool_BatchForkThree(t *testing.T) {
	stub := &stubFreeforkFunc{}
	input, _ := json.Marshal(map[string]any{
		"parent_session": "sess-1",
		"requests": []map[string]any{
			{"name": "r1", "prompt": "p1", "worktree": true},
			{"name": "r2", "prompt": "p2", "worktree": true},
			{"name": "r3", "prompt": "p3", "worktree": true},
		},
	})
	out, errStr := executeFreeFork(t, stub.Fork, string(input))
	if errStr != "" {
		t.Fatalf("tool error: %s", errStr)
	}
	var o struct {
		SpawnedCount int      `json:"spawned_count"`
		AgentIDs     []string `json:"agent_ids"`
	}
	if err := json.Unmarshal([]byte(out), &o); err != nil {
		t.Fatalf("unmarshal: %v, output=%s", err, out)
	}
	if o.SpawnedCount != 3 {
		t.Errorf("spawned_count = %d, want 3", o.SpawnedCount)
	}
	if len(o.AgentIDs) != 3 {
		t.Errorf("agent_ids len = %d, want 3", len(o.AgentIDs))
	}
	for _, id := range o.AgentIDs {
		if !strings.HasPrefix(id, "agent-") {
			t.Errorf("agent_id %q missing agent- prefix", id)
		}
	}
}

// T: D4-S11-A02-T02 — factory 失败 → 整体回滚 + tool 返回 error。
func TestFreeForkTool_FactoryFailureRollback(t *testing.T) {
	stub := &stubFreeforkFunc{failOn: 1}
	input, _ := json.Marshal(map[string]any{
		"parent_session": "sess-2",
		"requests":       []map[string]any{{"name": "r1", "prompt": "p1"}},
	})
	_, errStr := executeFreeFork(t, stub.Fork, string(input))
	if !strings.Contains(errStr, "factory failed") {
		t.Errorf("expected factory failure, got %q", errStr)
	}
}

// T: D4-S11-A02-T03 — requests count > 5 → 拒绝。
func TestFreeForkTool_MaxRequestsLimit(t *testing.T) {
	stub := &stubFreeforkFunc{}
	reqs := make([]map[string]any, 6)
	for i := range reqs {
		reqs[i] = map[string]any{"name": "r" + string(rune('1'+i)), "prompt": "p"}
	}
	input, _ := json.Marshal(map[string]any{
		"parent_session": "sess-3",
		"requests":       reqs,
	})
	_, errStr := executeFreeFork(t, stub.Fork, string(input))
	if !strings.Contains(errStr, "requests count must be in [1,5]") {
		t.Errorf("expected limit error, got %q", errStr)
	}
}

// T: forker 未注入 → surface 拒绝。
func TestFreeForkTool_ForkerNil(t *testing.T) {
	input, _ := json.Marshal(map[string]any{
		"parent_session": "sess-4",
		"requests":       []map[string]any{{"name": "r1", "prompt": "p1"}},
	})
	_, errStr := executeFreeFork(t, nil, string(input))
	if !strings.Contains(errStr, "forker not initialized") {
		t.Errorf("expected not initialized error, got %q", errStr)
	}
}
