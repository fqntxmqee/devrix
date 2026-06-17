package toolrunner

// W7 — D4-S11-A02 + D4-S13-A02 (alias G5) free_fork LLM tool wiring。
//
// AC5:
//   - 批量分叉 3 个 ForkRequest → 返回 {spawned_count, agent_ids:[...]}
//   - factory 失败 → 整体回滚 + 错误返回
//   - requests count > 5 → 拒绝
//
// S4-Gate H-3 fix: D2 Thin 架构 — D2 (contextengine) 不应 import D4 (multiagent).
// 这里用 function-based DI 隔离边界: toolrunner 定义 DTO + SetFreeForker 注入点,
// 真实 freefork 包装放在 bootstrap 层. DTO 是纯数据, 不依赖 D4 类型.

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/devrix/devrix/internal/shared/types"
)

// maxFreeForkRequests 限制单次调用分叉数,避免 LLM 误用导致 agent 爆炸。
const maxFreeForkRequests = 5

// FreeForkRequestDTO toolrunner 层的请求 DTO, 隔离 multiagent 依赖。
// Mode 字段以字符串形式承载, 注入方负责转 multiagent.CollaborationMode。
type FreeForkRequestDTO struct {
	Name     string
	Prompt   string
	Worktree bool
	Mode     string
}

// FreeForkHandleDTO toolrunner 层的句柄 DTO, 隔离 multiagent.Agent 依赖。
// 注入方负责在包装时把 *multiagent.Agent 转成 AgentID 字符串。
type FreeForkHandleDTO struct {
	AgentID  string
	Worktree string
	Name     string
}

// FreeForkerFunc 是 free_fork 工具的注入签名。
// bootstrap 阶段通过 SetFreeForker 把 freefork.GlobalForker().Fork 包成这个签名;
// toolrunner 不需要 import multiagent / freefork。
type FreeForkerFunc func(ctx context.Context, parentSession string, reqs []FreeForkRequestDTO) ([]FreeForkHandleDTO, error)

// globalFreeForker 默认实现 = 返回 "not initialized" 错误。
// bootstrap 阶段用 SetFreeForker 替换成真实包装。
var globalFreeForker FreeForkerFunc = func(_ context.Context, _ string, _ []FreeForkRequestDTO) ([]FreeForkHandleDTO, error) {
	return nil, fmt.Errorf("freefork: global forker not initialized")
}

// SetFreeForker 注入 free_fork 工具的真实实现. 通常由 bootstrap 调用.
func SetFreeForker(f FreeForkerFunc) {
	if f != nil {
		globalFreeForker = f
	}
}

// SetFreeForkerForTest 同 SetFreeForker, 但同时返回之前的函数, 便于测试
// 通过 t.Cleanup 恢复. nil 参数会保留现有值, 同 SetFreeForker 语义.
func SetFreeForkerForTest(f FreeForkerFunc) FreeForkerFunc {
	prev := globalFreeForker
	if f != nil {
		globalFreeForker = f
	}
	return prev
}

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

type freeforkInput struct {
	ParentSession string             `json:"parent_session"`
	Requests      []freeforkRequest  `json:"requests"`
}

type freeforkRequest struct {
	Name     string `json:"name"`
	Prompt   string `json:"prompt"`
	Worktree bool   `json:"worktree"`
	Mode     string `json:"mode,omitempty"`
}

type freeforkOutput struct {
	SpawnedCount int      `json:"spawned_count"`
	AgentIDs     []string `json:"agent_ids"`
}

func (f *freeforkRunner) Execute(ctx context.Context, _ /*workDir*/, input string) (*ToolResult, error) {
	if globalFreeForker == nil {
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
	dtos := make([]FreeForkRequestDTO, 0, len(in.Requests))
	for _, r := range in.Requests {
		if r.Name == "" {
			return &ToolResult{Error: "free_fork: each request must have a non-empty name"}, nil
		}
		dtos = append(dtos, FreeForkRequestDTO{
			Name:     r.Name,
			Prompt:   r.Prompt,
			Worktree: r.Worktree,
			Mode:     r.Mode,
		})
	}
	handles, err := globalFreeForker(ctx, in.ParentSession, dtos)
	if err != nil {
		return &ToolResult{Error: fmt.Sprintf("free_fork: %s", err.Error())}, nil
	}
	out := freeforkOutput{
		SpawnedCount: len(handles),
		AgentIDs:     make([]string, 0, len(handles)),
	}
	for _, h := range handles {
		out.AgentIDs = append(out.AgentIDs, h.AgentID)
	}
	bz, _ := json.Marshal(out)
	return &ToolResult{Output: string(bz)}, nil
}
