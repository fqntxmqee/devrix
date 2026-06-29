package usercontext

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/devrix/devrix/internal/layers/contextengine/prepare/conversation"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/prompt"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

// Provider loads user-facing context blocks (Claude Code getUserContext aligned).
type Provider struct {
	loader *prompt.Loader
	cfg    config.UserContextConfig
}

// NewProvider creates a user context provider.
func NewProvider(loader *prompt.Loader, cfg config.UserContextConfig) *Provider {
	if cfg.Mode == "" {
		cfg.Mode = "prepend"
	}
	return &Provider{loader: loader, cfg: cfg}
}

// Get returns context map for prepend (claudeMd, currentDate, workDir).
func (p *Provider) Get(_ context.Context, sc *types.SessionContext) map[string]string {
	out := map[string]string{
		"currentDate": fmt.Sprintf("Today's date is %s.", time.Now().Format("2006-01-02")),
	}
	if sc != nil && sc.WorkDir != "" {
		out["workDir"] = sc.WorkDir
	}
	if p.cfg.Mode == "system" {
		return out
	}
	if p.loader != nil && sc != nil {
		raw := strings.TrimSpace(p.loader.Load(sc.WorkDir))
		if raw != "" {
			out["claudeMd"] = raw
		}
	}
	return out
}

// ShouldEmbedInSystem reports whether AGENTS.md stays in system prompt assembly.
func (p *Provider) ShouldEmbedInSystem() bool {
	return p.cfg.Mode == "system" || p.cfg.Mode == "both"
}

// PrependForAPI prepends meta user context to messages for a single API call.
func PrependForAPI(msgs []types.Message, ctx map[string]string) []types.Message {
	if len(ctx) == 0 || pEmpty(ctx) {
		return msgs
	}
	var b strings.Builder
	b.WriteString("<system-reminder>\nAs you answer the user's questions, you can use the following context:\n")
	for k, v := range ctx {
		if strings.TrimSpace(v) == "" {
			continue
		}
		fmt.Fprintf(&b, "# %s\n%s\n\n", k, v)
	}
	b.WriteString("IMPORTANT: this context may or may not be relevant to your tasks. ")
	b.WriteString("You should not respond to this context unless it is highly relevant to your task.\n")
	b.WriteString("</system-reminder>\n")
	return conversation.PrependMetaUser(msgs, b.String())
}

func pEmpty(ctx map[string]string) bool {
	for _, v := range ctx {
		if strings.TrimSpace(v) != "" {
			return false
		}
	}
	return true
}

// OmitClaudeMd returns context without claudeMd (Explore/Plan sub-agents).
func OmitClaudeMd(ctx map[string]string) map[string]string {
	out := make(map[string]string, len(ctx))
	for k, v := range ctx {
		if k == "claudeMd" {
			continue
		}
		out[k] = v
	}
	return out
}

// MessagesWithUserContext prepends runtime user context at the LLM API boundary.
// Per D2 spec, prepend content is not persisted in snapshot Messages.
func MessagesWithUserContext(msgs []types.Message, uc map[string]string) []types.Message {
	if len(uc) == 0 {
		return msgs
	}
	if conversation.HasMetaUserContext(msgs) {
		return msgs
	}
	return PrependForAPI(msgs, uc)
}
