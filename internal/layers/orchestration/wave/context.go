package wave

import (
	"fmt"
	"strings"

	"github.com/devrix/devrix/internal/shared/types"
)

// SidechainLoader is the minimal interface ContextResolver needs from the
// contextengine sidechain recorder. It mirrors query.SidechainRecorder to
// avoid a circular import.
type SidechainLoader interface {
	Load(sessionID, agentID string) ([]types.Message, error)
}

// ContextResolverDeps wires ContextResolver.
type ContextResolverDeps struct {
	Artifacts        *ArtifactStore
	Sidechain        SidechainLoader
	BaseSystemPrompt string
	// LeaderMessages is the read-only Leader history passed in for fallback
	// only; fresh / upstream / resume should never include it directly.
	LeaderMessages []types.Message
}

// ContextResolver materializes a ResolvedContext per the design §4 table.
type ContextResolver struct {
	deps ContextResolverDeps
}

// NewContextResolver creates a resolver.
func NewContextResolver(deps ContextResolverDeps) *ContextResolver {
	return &ContextResolver{deps: deps}
}

// Resolve applies the policy and returns the worker's input context.
//
// Semantics (ORCH-S2-T11/12):
//   - fresh:     Messages = [user(directive)]; System = base + extra + file_scope
//   - resume:    Messages = sidechain.Load(...) + [user(directive)]
//   - upstream:  Messages = [user(directive)]; System = base + upstream summary
//
// Leader history is NEVER inherited; v1.0 keeps Workers isolated from Leader
// conversation noise.
func (r *ContextResolver) Resolve(node TaskNode) (ResolvedContext, error) {
	if r == nil {
		return ResolvedContext{}, errWave("nil resolver")
	}
	if err := node.Validate(); err != nil {
		return ResolvedContext{}, err
	}

	switch node.ContextPolicy {
	case ContextFresh:
		return r.resolveFresh(node), nil
	case ContextResume:
		return r.resolveResume(node)
	case ContextUpstream:
		return r.resolveUpstream(node)
	default:
		return ResolvedContext{}, errWave("unknown context policy: %q", node.ContextPolicy)
	}
}

func (r *ContextResolver) resolveFresh(node TaskNode) ResolvedContext {
	prompt := r.deps.BaseSystemPrompt
	if extra := strings.TrimSpace(node.SystemPromptExtra); extra != "" {
		prompt += "\n\n" + extra
	}
	if len(node.FileScope) > 0 {
		prompt += "\n\nAllowed file scope:\n- " + strings.Join(node.FileScope, "\n- ")
	}
	return ResolvedContext{
		Policy:       ContextFresh,
		SystemPrompt: prompt,
		Messages: []types.Message{
			{Role: "user", Content: node.Directive},
		},
	}
}

func (r *ContextResolver) resolveUpstream(node TaskNode) (ResolvedContext, error) {
	if r.deps.Artifacts == nil {
		return ResolvedContext{}, errWave("upstream policy requires artifact store")
	}
	art, ok := r.deps.Artifacts.Get(node.UpstreamTaskID)
	if !ok {
		return ResolvedContext{}, errWave("upstream artifact %q not found", node.UpstreamTaskID)
	}
	prompt := r.deps.BaseSystemPrompt
	if art.Summary != "" {
		prompt += "\n\nUpstream summary:\n" + art.Summary
	}
	if len(art.FilesChanged) > 0 {
		prompt += "\n\nFiles changed by upstream:\n- " + strings.Join(art.FilesChanged, "\n- ")
	}
	if art.Error != "" {
		prompt += "\n\nUpstream error (for context): " + art.Error
	}
	return ResolvedContext{
		Policy:          ContextUpstream,
		SystemPrompt:    prompt,
		UpstreamSummary: art.Summary,
		Messages: []types.Message{
			{Role: "user", Content: node.Directive},
		},
	}, nil
}

func (r *ContextResolver) resolveResume(node TaskNode) (ResolvedContext, error) {
	if r.deps.Sidechain == nil {
		return ResolvedContext{}, errWave("resume policy requires sidechain loader")
	}
	if node.ParentSessionID == "" {
		return ResolvedContext{}, errWave("resume policy requires parent_session_id")
	}
	loaded, err := r.deps.Sidechain.Load(node.ParentSessionID, node.SidechainAgentID)
	if err != nil {
		return ResolvedContext{}, fmt.Errorf("sidechain load %q/%q: %w", node.ParentSessionID, node.SidechainAgentID, err)
	}
	out := make([]types.Message, 0, len(loaded)+1)
	out = append(out, loaded...)
	out = append(out, types.Message{Role: "user", Content: node.Directive})
	return ResolvedContext{
		Policy:        ContextResume,
		SystemPrompt:  r.deps.BaseSystemPrompt,
		Messages:      out,
		ResumeAgentID: node.SidechainAgentID,
	}, nil
}
