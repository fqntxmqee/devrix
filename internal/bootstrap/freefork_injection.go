package bootstrap

// S4-Gate H-3 fix: free_fork tool 的 function-based DI 注入实现.
import (
	"context"
	"sync"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce/toolrunner"
	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/layers/multiagent/provision/freefork"
)

// freeforkInjectionOnce is retained as a sentinel for tests that want to
// assert "the freefork DI closure has been initialised" without actually
// touching a global. The wireFreeForkerInjection hook is now a no-op kept
// only for source-level compatibility with previous W11 callers; the real
// forker is held by surface.FreeForkSurface (TOOL-SURFACE-1 SoT).
var freeforkInjectionOnce sync.Once

// wireFreeForkerInjection is preserved as a no-op for back-compat with
// earlier OpenSpec W11 callers. W11 phase 2c removes the package-level
// toolrunner.globalFreeForker; the FreeForkerFunc is now plumbed via
// BuildSurfaces(SurfaceBuildOpts.Forker) instead.
func wireFreeForkerInjection() {
	freeforkInjectionOnce.Do(func() {})
}

// freeforkGlobalFunc 是 BuildSurfaces 阶段注入到 surface.FreeForkSurface 的真实实现.
//
// W11 phase 2c: this is no longer a global — it's a regular package function
// called by context_engine.go / context_engine_builder.go at engine build
// time. The closure is held in the surface for the lifetime of the engine.
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
