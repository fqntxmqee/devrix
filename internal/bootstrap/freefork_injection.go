package bootstrap

// S4-Gate H-3 fix: free_fork tool 的 function-based DI 注入实现.
//
// DM-20260617-008 W5: freeforkGlobalFunc is now a factory that closes over
// an explicit *freefork.DefaultForker. The package-level freefork.GlobalForker
// / SetGlobalForker have been removed; callers pass the forker directly when
// building the engine's surface list.
import (
	"context"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce/toolrunner"
	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/layers/multiagent/provision/freefork"
)

// freeforkGlobalFunc wraps a freefork.Forker in the toolrunner.FreeForkerFunc
// signature. The returned closure captures f by value; if f is nil the closure
// returns freeforkNotInitializedError so the surface still produces a stable
// error string for the LLM.
func freeforkGlobalFunc(f freefork.Forker) toolrunner.FreeForkerFunc {
	return func(ctx context.Context, parentSession string, reqs []toolrunner.FreeForkRequestDTO) ([]toolrunner.FreeForkHandleDTO, error) {
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
				Sandbox:  r.WantsSandbox(),
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
				SandboxPath: h.SandboxPath,
				Name:        h.Name,
			}
			if h.Agent != nil {
				dto.AgentID = h.Agent.ID()
			}
			out = append(out, dto)
		}
		return out, nil
	}
}

// freeforkNotInitializedError 单独定义以便错误文本稳定 (避免 LLM 看到
// 注入链路上不一致的 not initialized 文案).
type freeforkNotInitializedError struct{}

func (e *freeforkNotInitializedError) Error() string { return "freefork: forker not initialized" }
