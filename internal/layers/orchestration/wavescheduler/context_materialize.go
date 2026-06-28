package wavescheduler

import (
	"context"
	"strings"

	"github.com/devrix/devrix/internal/layers/contextengine/materialize"
)

// materializeContextResolver delegates Wave context assembly to D2 Materializer (D7-S16 T34).
type materializeContextResolver struct {
	deps ContextResolverDeps
}

func (r *materializeContextResolver) Resolve(node TaskNode) (ResolvedContext, error) {
	if r == nil || r.deps.Materializer == nil {
		return ResolvedContext{}, errWave("nil materializer resolver")
	}
	if err := node.Validate(); err != nil {
		return ResolvedContext{}, err
	}

	req, upstreamSummary, err := buildWaveMaterializeRequest(node, r.deps)
	if err != nil {
		return ResolvedContext{}, err
	}
	res, err := r.deps.Materializer.Materialize(context.Background(), req)
	if err != nil {
		return ResolvedContext{}, err
	}
	return ResolvedContext{
		Policy:          node.ContextPolicy,
		SystemPrompt:    res.SystemPrompt,
		Messages:        res.Messages,
		UpstreamSummary: upstreamSummary,
		ResumeAgentID:   node.SidechainAgentID,
	}, nil
}

func buildWaveMaterializeRequest(node TaskNode, deps ContextResolverDeps) (materialize.Request, string, error) {
	signals := materialize.InboundSignals{
		Directive:       node.Directive,
		WaveExtraPrompt: node.SystemPromptExtra,
		WaveFileScope:   append([]string(nil), node.FileScope...),
	}
	var upstreamSummary string
	policy := materialize.PolicyFromWaveContext(string(node.ContextPolicy))

	switch node.ContextPolicy {
	case ContextUpstream:
		if deps.Artifacts == nil {
			return materialize.Request{}, "", errWave("upstream policy requires artifact store")
		}
		art, ok := deps.Artifacts.Get(node.UpstreamTaskID)
		if !ok {
			return materialize.Request{}, "", errWave("upstream artifact %q not found", node.UpstreamTaskID)
		}
		upstreamSummary = art.Summary
		if art.Summary != "" {
			signals.SignalLines = append(signals.SignalLines, "Upstream summary:\n"+art.Summary)
		}
		signals.WaveUpstreamFiles = append([]string(nil), art.FilesChanged...)
		signals.WaveUpstreamError = art.Error
	case ContextResume:
		if node.ParentSessionID == "" {
			return materialize.Request{}, "", errWave("resume policy requires parent_session_id")
		}
		if node.SidechainAgentID == "" {
			return materialize.Request{}, "", errWave("resume policy requires sidechain_agent_id")
		}
	}

	partition := materialize.Partition{
		Kind:            materialize.PartitionWave,
		AgentID:         node.SidechainAgentID,
		ParentSessionID: node.ParentSessionID,
		SessionID:       node.ParentSessionID,
	}
	if partition.SessionID == "" {
		partition.SessionID = "wave:" + node.ID
	}

	return materialize.Request{
		Partition:    partition,
		Policy:       policy,
		SystemPrompt: deps.BaseSystemPrompt,
		Signals:      signals,
	}, upstreamSummary, nil
}

// NewMaterializingContextResolver returns a resolver backed by D2 Materializer.
func NewMaterializingContextResolver(deps ContextResolverDeps) ContextResolverIface {
	if deps.Materializer == nil {
		return NewContextResolver(deps)
	}
	if strings.TrimSpace(deps.BaseSystemPrompt) == "" {
		deps.BaseSystemPrompt = "base"
	}
	return &materializeContextResolver{deps: deps}
}
