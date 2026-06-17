package bootstrap

// S4-Gate H-3 fix: free_fork tool 的 function-based DI 注入实现.
import (
	"context"
	"sync"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce/toolrunner"
	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/layers/multiagent/provision/freefork"
)

// freeforkInjectionOnce 确保 wireFreeForkerInjection 只注入一次 (测试可重入)。
var freeforkInjectionOnce sync.Once

// wireFreeForkerInjection 把 freefork.GlobalForker().Fork 包成 toolrunner.FreeForkerFunc
// 注入到 toolrunner global. bootstrap 阶段调用, 之后 LLM 调 free_fork 时
// toolrunner 不需要 import freefork / multiagent 任何包.
func wireFreeForkerInjection() {
	freeforkInjectionOnce.Do(func() {
		toolrunner.SetFreeForker(freeforkGlobalFunc)
	})
}

// freeforkGlobalFunc 是注入到 toolrunner 的真实实现.
func freeforkGlobalFunc(ctx context.Context, parentSession string, reqs []toolrunner.FreeForkRequestDTO) ([]toolrunner.FreeForkHandleDTO, error) {
	f := freefork.GlobalForker()
	if f == nil {
		return nil, &freeforkNotInitializedError{}
	}
	conv := make([]freefork.ForkRequest, 0, len(reqs))
	for _, r := range reqs {
		mode := multiagent.CollaborationMode(r.Mode)
		if mode == "" {
			mode = multiagent.ModeDefault
		}
		conv = append(conv, freefork.ForkRequest{
			Name:     r.Name,
			Prompt:   r.Prompt,
			Worktree: r.Worktree,
			Mode:     mode,
		})
	}
	handles, err := f.Fork(ctx, parentSession, conv)
	if err != nil {
		return nil, err
	}
	out := make([]toolrunner.FreeForkHandleDTO, 0, len(handles))
	for _, h := range handles {
		dto := toolrunner.FreeForkHandleDTO{
			Worktree: h.Worktree,
			Name:     h.Name,
		}
		if h.Agent != nil {
			dto.AgentID = h.Agent.ID()
		}
		out = append(out, dto)
	}
	return out, nil
}

// freeforkNotInitializedError 单独定义以便错误文本稳定 (避免 LLM 看到
// 注入链路上不一致的 not initialized 文案).
type freeforkNotInitializedError struct{}

func (e *freeforkNotInitializedError) Error() string { return "freefork: global forker not initialized" }
