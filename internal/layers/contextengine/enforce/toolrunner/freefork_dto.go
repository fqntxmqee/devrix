package toolrunner

// W11 phase 2c: free_fork tool 拆面后, freeforkRequest/Output/Handle DTO 与
// FreeForkerFunc 注入签名从 freefork_tool.go 提到独立的 freefork_dto.go,
// 不再与 freeforkRunner (已被 surface.FreeForkSurface 替代) 混在一起。
//
// 真实 freefork 包装在 bootstrap 层 (freefork_injection.go 里的 freeforkGlobalFunc);
// toolrunner 层只保留 DTO + 函数类型, 不 import multiagent / freefork 内部包.

import (
	"context"
	"fmt"
)

// maxFreeForkRequests 限制单次调用分叉数,避免 LLM 误用导致 agent 爆炸。
const maxFreeForkRequests = 5

// FreeForkRequestDTO toolrunner 层的请求 DTO, 隔离 multiagent 依赖。
// Mode 字段以字符串形式承载, 注入方负责转 multiagent.CollaborationMode。
type FreeForkRequestDTO struct {
	Name     string
	Prompt   string
	Sandbox  bool
	Worktree bool // deprecated alias
	Mode     string
}

func (r FreeForkRequestDTO) WantsSandbox() bool { return r.Sandbox || r.Worktree }

type FreeForkHandleDTO struct {
	AgentID     string
	SandboxPath string
	Name        string
}

// FreeForkerFunc 是 free_fork 工具的注入签名。
// bootstrap 阶段构造 surface.FreeForkSurface 时把 freefork.GlobalForker().Fork
// 包成这个签名; toolrunner 不需要 import multiagent / freefork。
type FreeForkerFunc func(ctx context.Context, parentSession string, reqs []FreeForkRequestDTO) ([]FreeForkHandleDTO, error)

// errFreeforkNotInitialized 是 FreeForkSurface.Execute 在 forker 未注入时
// 返回的稳定错误 (供 surface 测试比对)。
var errFreeforkNotInitialized = fmt.Errorf("freefork: forker not initialized")
