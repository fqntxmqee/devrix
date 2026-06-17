package toolrunner

// W7 — D4-S11-A02 + D4-S13-A02 (alias G5) free_fork LLM tool wiring。
//
// AC5:
//   - 批量分叉 3 个 ForkRequest → 返回 {spawned_count, agent_ids:[...]}
//   - factory 失败 → 整体回滚 + 错误返回
//   - requests count > 5 → 拒绝
//
// input 格式 (JSON):
//   {
//     "parent_session": "sess-xxx",
//     "requests": [
//       {"name": "explore-1", "prompt": "...", "worktree": true, "mode": "default"},
//       ...
//     ]
//   }

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/layers/multiagent/provision/freefork"
	"github.com/devrix/devrix/internal/shared/types"
)

// maxFreeForkRequests 限制单次调用分叉数,避免 LLM 误用导致 agent 爆炸。
const maxFreeForkRequests = 5

// freeforkRunner 实现 PluginRunner 接口。
type freeforkRunner struct{}

func (f *freeforkRunner) Name() string { return "free_fork" }

func (f *freeforkRunner) RiskLevel() types.RiskLevel { return types.RiskLevelHigh }

func (f *freeforkRunner) Schema() ToolSchema {
	return ToolSchema{
		Name: "free_fork",
		Description: "Batch fork N child agents (1..5) under a parent session. Each child inherits the parent's session id " +
			"but runs in an isolated worktree (default). Returns agent_ids list.",
		Parameters: `{"parent_session": "<id>", "requests": [{"name": "...", "prompt": "...", "worktree": true, "mode": "default"}]}`,
	}
}

type freeforkRequest struct {
	Name     string `json:"name"`
	Prompt   string `json:"prompt"`
	Worktree bool   `json:"worktree"`
	Mode     string `json:"mode,omitempty"`
}

type freeforkInput struct {
	ParentSession string             `json:"parent_session"`
	Requests      []freeforkRequest  `json:"requests"`
}

type freeforkOutput struct {
	SpawnedCount int      `json:"spawned_count"`
	AgentIDs     []string `json:"agent_ids"`
}

func (f *freeforkRunner) Execute(ctx context.Context, _ /*workDir*/, input string) (*ToolResult, error) {
	if freefork.GlobalForker() == nil {
		return &ToolResult{Error: "free_fork: global forker not initialized"}, nil
	}
	var in freeforkInput
	if err := json.Unmarshal([]byte(input), &in); err != nil {
		return &ToolResult{Error: fmt.Sprintf("free_fork: invalid input JSON: %s", err.Error())}, nil
	}
	if in.ParentSession == "" {
		return &ToolResult{Error: "free_fork: parent_session is required"}, nil
	}
	if n := len(in.Requests); n < 1 || n > maxFreeForkRequests {
		return &ToolResult{Error: fmt.Sprintf("free_fork: requests count must be in [1,%d], got %d", maxFreeForkRequests, n)}, nil
	}
	reqs := make([]freefork.ForkRequest, 0, len(in.Requests))
	for _, r := range in.Requests {
		if r.Name == "" {
			return &ToolResult{Error: "free_fork: each request must have a non-empty name"}, nil
		}
		mode := multiagent.CollaborationMode(r.Mode)
		if mode == "" {
			mode = multiagent.ModeDefault
		}
		reqs = append(reqs, freefork.ForkRequest{
			Name:     r.Name,
			Prompt:   r.Prompt,
			Worktree: r.Worktree,
			Mode:     mode,
		})
	}
	handles, err := freefork.GlobalForker().Fork(ctx, in.ParentSession, reqs)
	if err != nil {
		return &ToolResult{Error: fmt.Sprintf("free_fork: %s", err.Error())}, nil
	}
	out := freeforkOutput{
		SpawnedCount: len(handles),
		AgentIDs:     make([]string, 0, len(handles)),
	}
	for _, h := range handles {
		if h.Agent != nil {
			out.AgentIDs = append(out.AgentIDs, h.Agent.ID())
		}
	}
	bz, _ := json.Marshal(out)
	return &ToolResult{Output: string(bz)}, nil
}
