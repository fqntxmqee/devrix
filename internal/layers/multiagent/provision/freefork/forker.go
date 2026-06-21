package freefork

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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

type DefaultForker struct {
	deps    ForkerDeps
	metrics *ForkerMetrics
}

func NewDefaultForker(deps ForkerDeps) *DefaultForker { return &DefaultForker{deps: deps} }

// WithMetrics attaches a metrics sink. Backward-compatible setter for
// callers that constructed DefaultForker via NewDefaultForker; safe to
// call before Fork. nil disables metric recording.
func (f *DefaultForker) WithMetrics(m *ForkerMetrics) *DefaultForker {
	f.metrics = m
	return f
}

func (f *DefaultForker) recordSpawned() {
	if f.metrics != nil {
		f.metrics.Spawned.Add(1)
	}
}

func (f *DefaultForker) recordSpawnFailed() {
	if f.metrics != nil {
		f.metrics.SpawnFailed.Add(1)
	}
}

func (f *DefaultForker) recordSandboxEnterFailed() {
	if f.metrics != nil {
		f.metrics.SandboxEnterFailed.Add(1)
	}
}

func (f *DefaultForker) recordSandboxExitFailed(name, path string, err error) {
	if f.metrics != nil {
		f.metrics.SandboxExitFailed.Add(1)
	}
	slog.Warn("freefork: sandbox exit failed",
		"forkName", name, "sandboxPath", path, "err", err)
}

func (f *DefaultForker) recordFactoryCreateFailed() {
	if f.metrics != nil {
		f.metrics.FactoryCreateFailed.Add(1)
	}
}

func (f *DefaultForker) recordRollbackTriggered() {
	if f.metrics != nil {
		f.metrics.RollbackTriggered.Add(1)
	}
}

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
				f.recordSpawnFailed()
				errs = append(errs, fmt.Errorf("fork %q: %w", req.Name, err))
				return
			}
			f.recordSpawned()
			handles = append(handles, h)
		}()
	}
	wg.Wait()
	if len(errs) > 0 {
		f.recordRollbackTriggered()
		for _, h := range handles {
			if err := h.Agent.Terminate(ctx); err != nil {
				slog.Warn("freefork: rollback Terminate failed",
					"forkName", h.Name, "err", err)
			}
			if h.SandboxPath != "" && f.deps.Sandbox != nil {
				if err := f.deps.Sandbox.Exit(ctx, h.SandboxPath, false); err != nil {
					f.recordSandboxExitFailed(h.Name, h.SandboxPath, err)
				}
			}
		}
		return nil, errors.Join(errs...)
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
			f.recordSandboxEnterFailed()
			return Handle{}, fmt.Errorf("sandbox enter: %w", err)
		}
		sbPath = p
		cfg.WorkDir = p
	}
	agent, err := f.deps.Factory.Create(ctx, cfg, nil)
	if err != nil {
		f.recordFactoryCreateFailed()
		if sbPath != "" && f.deps.Sandbox != nil {
			if exitErr := f.deps.Sandbox.Exit(ctx, sbPath, false); exitErr != nil {
				f.recordSandboxExitFailed(req.Name, sbPath, exitErr)
			}
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
