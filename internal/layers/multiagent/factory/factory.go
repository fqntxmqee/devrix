package factory

import (
	"context"
	"fmt"
	"time"

	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/layers/multiagent/agent"
	"github.com/devrix/devrix/internal/layers/multiagent/collaboration"
	sharedconfig "github.com/devrix/devrix/internal/shared/config"
	sharederrors "github.com/devrix/devrix/internal/shared/errors"
	"github.com/devrix/devrix/internal/shared/types"
)

// AgentFactory creates multi-agent instances.
type AgentFactory struct {
	deps multiagent.AgentDeps
	cfg  *sharedconfig.MultiAgentConfig
}

var _ multiagent.IAgentFactory = (*AgentFactory)(nil)

// NewAgentFactory constructs a factory with layer defaults.
func NewAgentFactory(deps multiagent.AgentDeps, cfg *sharedconfig.MultiAgentConfig) *AgentFactory {
	if cfg == nil {
		cfg = sharedconfig.DefaultMultiAgentConfig()
	}
	return &AgentFactory{deps: deps, cfg: cfg}
}

// Create implements multiagent.IAgentFactory.
func (f *AgentFactory) Create(
	ctx context.Context,
	cfg multiagent.AgentConfig,
	session *types.Session,
) (multiagent.Agent, error) {
	if err := ctx.Err(); err != nil {
		return nil, sharederrors.NewAgentContextCancelledError(cfg.SessionID)
	}
	if err := validateConfig(cfg, f.cfg); err != nil {
		return nil, err
	}
	if session == nil {
		return nil, sharederrors.NewAgentInvalidConfigError("session is nil")
	}
	resolved := resolveConfig(cfg, f.cfg)
	return agent.New(resolved, session, f.deps, f), nil
}

func validateConfig(cfg multiagent.AgentConfig, defaults *sharedconfig.MultiAgentConfig) error {
	if cfg.SessionID == "" {
		return sharederrors.NewAgentInvalidConfigError("session_id is required")
	}
	if cfg.WorkDir == "" {
		return sharederrors.NewAgentInvalidConfigError("work_dir is required")
	}
	if cfg.Mode != "" {
		if err := collaboration.ValidateMode(cfg.Mode); err != nil {
			return sharederrors.NewAgentInvalidConfigError(err.Error())
		}
	}
	maxChildren := cfg.MaxChildren
	if maxChildren <= 0 {
		maxChildren = defaults.MaxChildren
	}
	if maxChildren <= 0 || maxChildren > 10 {
		return sharederrors.NewAgentInvalidConfigError(
			fmt.Sprintf("max_children must be 1..10, got %d", maxChildren),
		)
	}
	if cfg.MaxIter < 0 {
		return sharederrors.NewAgentInvalidConfigError("max_iter must be >= 0")
	}
	if cfg.Timeout < 0 {
		return sharederrors.NewAgentInvalidConfigError("timeout must be >= 0")
	}
	return nil
}

func resolveConfig(cfg multiagent.AgentConfig, defaults *sharedconfig.MultiAgentConfig) multiagent.AgentConfig {
	out := cfg
	if out.Mode == "" {
		out.Mode = multiagent.CollaborationMode(defaults.DefaultMode)
		if out.Mode == "" {
			out.Mode = multiagent.ModeDefault
		}
	}
	if out.MaxChildren <= 0 {
		out.MaxChildren = defaults.MaxChildren
	}
	if out.MaxIter <= 0 {
		out.MaxIter = defaults.DefaultMaxIter
	}
	if out.Timeout <= 0 {
		out.Timeout = defaults.DefaultTimeout
	}
	if out.SystemPrompt != "" {
		out.SystemPrompt = collaboration.BuildPromptForMode(out.Mode, out.SystemPrompt)
	}
	return out
}

// LayerConfig exposes factory defaults for agents.
func (f *AgentFactory) LayerConfig() *sharedconfig.MultiAgentConfig {
	return f.cfg
}

// DefaultTimeout returns the configured default agent timeout.
func (f *AgentFactory) DefaultTimeout() time.Duration {
	if f.cfg == nil {
		return 5 * time.Minute
	}
	return f.cfg.DefaultTimeout
}
