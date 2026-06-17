// Package freefork — G5 自由分叉子代理,对标 clawcode src/tools/AgentTool/ForkSubagent。
//
// 关键差异(对比 run.Impl.Fork):
//  1. Free Fork 不依赖已存在的 leader agent — 接受 parent session id 作为命名空间
//  2. 默认 Worktree=true:每个分叉在 worktree 沙箱里独立 workdir
//  3. 支持批量 ForkRequest 并行 dispatch
//  4. Handle 暴露 Wait()/Terminate() 收集结果,完整 life-cycle control
//
// 设计参考:openspec/changes/devrix-diagnostic-tools-parity/design.md §2.8
package freefork

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/shared/contracts"
)

// ForkRequest 单条分叉请求。
type ForkRequest struct {
	Name     string                    // 分叉名(用于 slug/handle 标识)
	Prompt   string                    // 注入子 agent 的 initial input
	Worktree bool                      // true=分配独立 worktree 沙箱,false=共用 parent workdir
	Mode     multiagent.CollaborationMode
}

// Handle 表示一个已分叉出去的子 agent 句柄。
type Handle struct {
	Agent    multiagent.Agent
	Worktree string // worktree 路径(若 Worktree=true);否则为空
	Name     string
}

// Wait 阻塞等待 handle 关联 agent 终止。
func (h *Handle) Wait(ctx context.Context) (*multiagent.AgentResult, error) {
	return h.Agent.Wait(ctx)
}

// Forker 自由分叉器接口。
type Forker interface {
	Fork(ctx context.Context, parentSession string, reqs []ForkRequest) ([]Handle, error)
}

// ForkerDeps 依赖注入。
type ForkerDeps struct {
	Factory  multiagent.IAgentFactory
	Worktree contracts.WorktreeSandbox
	// DefaultConfig 注入默认 AgentConfig(可由调用方覆盖)
	DefaultConfig multiagent.AgentConfig
}

// DefaultForker 默认实现:批量 + 并行 + worktree 隔离。
type DefaultForker struct {
	deps ForkerDeps
}

// NewDefaultForker 构造 freefork.DefaultForker。
func NewDefaultForker(deps ForkerDeps) *DefaultForker {
	return &DefaultForker{deps: deps}
}

// Fork 批量派发 N 个 ForkRequest,并行启动子 agent,返回 handle 列表。
//
// 任一子 agent 启动失败时,已启动的会被 Terminate,整体返回 error。
// parentSession 为空时拒绝(避免污染 default namespace)。
func (f *DefaultForker) Fork(ctx context.Context, parentSession string, reqs []ForkRequest) ([]Handle, error) {
	if f == nil || f.deps.Factory == nil {
		return nil, fmt.Errorf("freefork: factory not configured")
	}
	if parentSession == "" {
		return nil, fmt.Errorf("freefork: parent session id required")
	}
	if len(reqs) == 0 {
		return nil, nil
	}

	handles := make([]Handle, 0, len(reqs))
	var mu sync.Mutex
	errs := make([]error, 0)
	var wg sync.WaitGroup

	for _, req := range reqs {
		if req.Name == "" {
			return nil, fmt.Errorf("freefork: request name is required")
		}
		req := req // capture
		wg.Add(1)
		go func() {
			defer wg.Done()
			h, err := f.spawnOne(ctx, parentSession, req)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, fmt.Errorf("fork %q: %w", req.Name, err))
				return
			}
			handles = append(handles, h)
		}()
	}
	wg.Wait()

	if len(errs) > 0 {
		// 失败回滚:终止已启动的子 agent
		for _, h := range handles {
			_ = h.Agent.Terminate(ctx)
			if h.Worktree != "" && f.deps.Worktree != nil {
				_ = f.deps.Worktree.Exit(ctx, h.Worktree, false)
			}
		}
		return nil, errs[0]
	}
	return handles, nil
}

// spawnOne 分发单个 ForkRequest。
func (f *DefaultForker) spawnOne(ctx context.Context, parentSession string, req ForkRequest) (Handle, error) {
	cfg := f.deps.DefaultConfig
	cfg.SessionID = parentSession
	if cfg.WorkDir == "" {
		cfg.WorkDir = "."
	}
	if req.Mode != "" {
		cfg.Mode = req.Mode
	}
	if req.Prompt != "" {
		cfg.InitialInput = req.Prompt
	}

	var wtPath string
	slug := slugify(req.Name)
	if req.Worktree && f.deps.Worktree != nil && f.deps.Worktree.Enabled() {
		p, err := f.deps.Worktree.Enter(ctx, parentSession, slug, cfg.WorkDir)
		if err != nil {
			return Handle{}, fmt.Errorf("worktree enter: %w", err)
		}
		wtPath = p
		cfg.WorkDir = p
	}

	agent, err := f.deps.Factory.Create(ctx, cfg, nil)
	if err != nil {
		if wtPath != "" && f.deps.Worktree != nil {
			_ = f.deps.Worktree.Exit(ctx, wtPath, false)
		}
		return Handle{}, fmt.Errorf("factory create: %w", err)
	}
	return Handle{Agent: agent, Worktree: wtPath, Name: req.Name}, nil
}

// slugify 把 Name 规整为 worktree slug。
func slugify(name string) string {
	out := make([]byte, 0, len(name))
	for i := 0; i < len(name) && i < 64; i++ {
		c := name[i]
		switch {
		case c >= 'A' && c <= 'Z':
			out = append(out, c+'a'-'A')
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9', c == '-', c == '_', c == '.':
			out = append(out, c)
		case c == ' ' || c == '/' || c == ':':
			out = append(out, '-')
		}
	}
	if len(out) == 0 {
		return fmt.Sprintf("fork-%d", time.Now().UnixNano())
	}
	return string(out)
}
