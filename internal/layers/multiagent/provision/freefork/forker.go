package freefork

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/shared/contracts"
)

type ForkRequest struct {
	Name     string
	Prompt   string
	Sandbox  bool
	Worktree bool // deprecated alias for Sandbox
	Mode     multiagent.CollaborationMode
}

func (r ForkRequest) WantsSandbox() bool { return r.Sandbox || r.Worktree }

type Handle struct {
	Agent       multiagent.Agent
	SandboxPath string
	Name        string
}

func (h *Handle) Wait(ctx context.Context) (*multiagent.AgentResult, error) {
	return h.Agent.Wait(ctx)
}

type Forker interface {
	Fork(ctx context.Context, parentSession string, reqs []ForkRequest) ([]Handle, error)
}

type ForkerDeps struct {
	Factory       multiagent.IAgentFactory
	Sandbox       contracts.WorkerDirSandbox
	DefaultConfig multiagent.AgentConfig
}

type DefaultForker struct{ deps ForkerDeps }

func NewDefaultForker(deps ForkerDeps) *DefaultForker { return &DefaultForker{deps: deps} }

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
	var errs []error
	var wg sync.WaitGroup
	for _, req := range reqs {
		if req.Name == "" {
			return nil, fmt.Errorf("freefork: request name is required")
		}
		req := req
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
		for _, h := range handles {
			_ = h.Agent.Terminate(ctx)
			if h.SandboxPath != "" && f.deps.Sandbox != nil {
				_ = f.deps.Sandbox.Exit(ctx, h.SandboxPath, false)
			}
		}
		return nil, errs[0]
	}
	return handles, nil
}

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
	var sbPath string
	if req.WantsSandbox() && f.deps.Sandbox != nil && f.deps.Sandbox.Enabled() {
		p, err := f.deps.Sandbox.Enter(ctx, parentSession, slugify(req.Name), cfg.WorkDir)
		if err != nil {
			return Handle{}, fmt.Errorf("sandbox enter: %w", err)
		}
		sbPath = p
		cfg.WorkDir = p
	}
	agent, err := f.deps.Factory.Create(ctx, cfg, nil)
	if err != nil {
		if sbPath != "" && f.deps.Sandbox != nil {
			_ = f.deps.Sandbox.Exit(ctx, sbPath, false)
		}
		return Handle{}, fmt.Errorf("factory create: %w", err)
	}
	return Handle{Agent: agent, SandboxPath: sbPath, Name: req.Name}, nil
}

func slugify(name string) string {
	out := make([]byte, 0, len(name))
	for i := 0; i < len(name) && i < 64; i++ {
		c := name[i]
		switch {
		case c >= 'A' && c <= 'Z':
			out = append(out, c+'a'-'A')
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9', c == '-', c == '_', c == '.':
			out = append(out, c)
		case c == ' ', c == '/', c == ':':
			out = append(out, '-')
		}
	}
	if len(out) == 0 {
		return fmt.Sprintf("fork-%d", time.Now().UnixNano())
	}
	return string(out)
}
