package materialize

import (
	"context"
	"fmt"
	"strings"

	"github.com/devrix/devrix/internal/layers/contextengine/prepare/token"
	"github.com/devrix/devrix/internal/shared/types"
)

const deliveryHintBlock = `
## Work item delivery (system)
- If you form a verifiable conclusion, end with <conclusion>...</conclusion>.
- If still uncertain, list <open_questions>...</open_questions> (one question per line).
- Do not label observations as ObsFact/ObsSignal/ObsDeviation/ObsUncertainty; the Observe phase classifies signals.
`

// DefaultMaterializer implements the light Materialize path (OQ-LC-2).
type DefaultMaterializer struct {
	Store *PartitionStore
}

// NewDefaultMaterializer constructs a materializer with partition store.
func NewDefaultMaterializer(store *PartitionStore) *DefaultMaterializer {
	return &DefaultMaterializer{Store: store}
}

// Materialize composes system prompt, messages, and readonly tool profile.
func (m *DefaultMaterializer) Materialize(_ context.Context, req Request) (Result, error) {
	if req.Partition.SessionID == "" {
		return Result{}, fmt.Errorf("materialize: session id required")
	}
	if req.Partition.Kind == PartitionAgent {
		return m.materializeSubTurn(req)
	}
	if req.Partition.Kind == PartitionWave {
		return m.materializeWave(req)
	}
	sys := buildSystemPrompt(req)
	msgs := buildInitialMessages(req)
	if m != nil && m.Store != nil && req.Partition.WorkItemID != "" {
		priv, err := m.Store.Load(req.Partition.SessionID, req.Partition.WorkItemID)
		if err != nil {
			return Result{}, err
		}
		msgs = append(msgs, priv...)
	}
	msgs = compressMessages(msgs, req.Policy.TokenBudget)
	counter := token.NewCounter()
	tokEst := counter.CountMessages(msgs) + counter.CountText(sys)
	return Result{
		SystemPrompt: sys,
		Messages:     msgs,
		Tools:        toolsForProfile(req.Policy.ToolProfile),
		MessageCount: len(msgs),
		TokenEst:     tokEst,
	}, nil
}

// Append writes turn messages to the work item partition.
func (m *DefaultMaterializer) Append(_ context.Context, partition Partition, msgs []types.Message) error {
	if m == nil || m.Store == nil {
		return nil
	}
	return m.Store.Append(partition.SessionID, partition.WorkItemID, msgs)
}

func (m *DefaultMaterializer) materializeSubTurn(req Request) (Result, error) {
	mode := SubTurnBrief
	switch req.Policy.Mode {
	case ModeFork:
		mode = SubTurnFork
	case ModeResume:
		mode = SubTurnFull
	}
	preloaded, lastUser := ComposeSubTurnMessages(mode, req.SubTurnParent)
	msgs := append([]types.Message(nil), preloaded...)
	if strings.TrimSpace(lastUser.Content) != "" {
		msgs = append(msgs, lastUser)
	}
	if m != nil && m.Store != nil && req.Partition.AgentID != "" && req.Policy.Mode == ModeResume {
		priv, err := m.Store.LoadAgent(req.Partition.SessionID, req.Partition.AgentID)
		if err != nil {
			return Result{}, err
		}
		if len(priv) > 0 {
			msgs = append(priv, msgs...)
		}
	}
	msgs = compressMessages(msgs, req.Policy.TokenBudget)
	sys := strings.TrimSpace(req.SystemPrompt)
	counter := token.NewCounter()
	tokEst := counter.CountMessages(msgs) + counter.CountText(sys)
	return Result{
		SystemPrompt: sys,
		Messages:     msgs,
		Tools:        toolsForProfile(req.Policy.ToolProfile),
		MessageCount: len(msgs),
		TokenEst:     tokEst,
	}, nil
}

func (m *DefaultMaterializer) materializeWave(req Request) (Result, error) {
	sys := buildWaveSystemPrompt(req)
	msgs := []types.Message{{
		SessionID: req.Partition.SessionID,
		Role:      types.MessageRoleUser,
		Content:   req.Signals.Directive,
	}}
	if req.Policy.Mode == ModeResume {
		sid := req.Partition.ParentSessionID
		if sid == "" {
			sid = req.Partition.SessionID
		}
		if m != nil && m.Store != nil && req.Partition.AgentID != "" {
			loaded, err := m.Store.LoadAgent(sid, req.Partition.AgentID)
			if err != nil {
				return Result{}, err
			}
			if len(loaded) > 0 {
				msgs = append(append([]types.Message(nil), loaded...), msgs...)
			}
		}
	}
	msgs = compressMessages(msgs, req.Policy.TokenBudget)
	counter := token.NewCounter()
	tokEst := counter.CountMessages(msgs) + counter.CountText(sys)
	return Result{
		SystemPrompt: sys,
		Messages:     msgs,
		MessageCount: len(msgs),
		TokenEst:     tokEst,
	}, nil
}

func buildWaveSystemPrompt(req Request) string {
	var b strings.Builder
	if base := strings.TrimSpace(req.SystemPrompt); base != "" {
		b.WriteString(base)
	}
	if extra := strings.TrimSpace(req.Signals.WaveExtraPrompt); extra != "" {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(extra)
	}
	if len(req.Signals.WaveFileScope) > 0 {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString("Allowed file scope:\n- ")
		b.WriteString(strings.Join(req.Signals.WaveFileScope, "\n- "))
	}
	if req.Policy.Mode == ModeUpstream {
		for _, line := range req.Signals.SignalLines {
			if strings.TrimSpace(line) == "" {
				continue
			}
			if b.Len() > 0 {
				b.WriteString("\n\n")
			}
			b.WriteString(line)
		}
		if len(req.Signals.WaveUpstreamFiles) > 0 {
			if b.Len() > 0 {
				b.WriteString("\n\n")
			}
			b.WriteString("Files changed by upstream:\n- ")
			b.WriteString(strings.Join(req.Signals.WaveUpstreamFiles, "\n- "))
		}
		if errMsg := strings.TrimSpace(req.Signals.WaveUpstreamError); errMsg != "" {
			if b.Len() > 0 {
				b.WriteString("\n\n")
			}
			b.WriteString("Upstream error (for context): ")
			b.WriteString(errMsg)
		}
	}
	return b.String()
}

func buildSystemPrompt(req Request) string {
	var b strings.Builder
	b.WriteString("You are executing one WorkItem in a layered work tree.\n")
	if req.Signals.Directive != "" {
		b.WriteString("\nDirective: ")
		b.WriteString(req.Signals.Directive)
		b.WriteByte('\n')
	}
	if len(req.Signals.ScopeIn) > 0 {
		b.WriteString("\nScopeIn:\n")
		for _, p := range req.Signals.ScopeIn {
			b.WriteString("- ")
			b.WriteString(p)
			b.WriteByte('\n')
		}
	}
	if len(req.Signals.ScopeOut) > 0 {
		b.WriteString("\nScopeOut:\n")
		for _, p := range req.Signals.ScopeOut {
			b.WriteString("- ")
			b.WriteString(p)
			b.WriteByte('\n')
		}
	}
	if req.Signals.ExpectedReturn != "" {
		b.WriteString("\nExpectedReturn: ")
		b.WriteString(req.Signals.ExpectedReturn)
		b.WriteByte('\n')
	}
	for _, line := range req.Signals.SignalLines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		b.WriteString("\n")
		b.WriteString(line)
	}
	b.WriteString(deliveryHintBlock)
	return b.String()
}

func buildInitialMessages(req Request) []types.Message {
	if strings.TrimSpace(req.Signals.Directive) == "" {
		return nil
	}
	return []types.Message{{
		SessionID: req.Partition.SessionID,
		Role:      types.MessageRoleUser,
		Content:   req.Signals.Directive,
	}}
}

func compressMessages(msgs []types.Message, budget int) []types.Message {
	if budget <= 0 || len(msgs) == 0 {
		return msgs
	}
	counter := token.NewCounter()
	total := counter.CountMessages(msgs)
	if total <= budget {
		return msgs
	}
	// Keep last messages until budget — simple tail preserve.
	out := make([]types.Message, 0, len(msgs))
	for i := len(msgs) - 1; i >= 0; i-- {
		trial := append([]types.Message{msgs[i]}, out...)
		if counter.CountMessages(trial) > budget && len(out) > 0 {
			break
		}
		out = trial
	}
	if len(out) == 0 && len(msgs) > 0 {
		last := msgs[len(msgs)-1]
		last.Content = counter.TruncateToTokens(last.Content, budget)
		out = []types.Message{last}
	}
	return out
}

func toolsForProfile(profile string) []ToolDescriptor {
	switch profile {
	case "implement", "":
		return nil // executor uses full tool set from ContextPreparer fallback merge
	case "readonly":
		return []ToolDescriptor{
			{Name: "read_file", Description: "Read file contents"},
			{Name: "grep", Description: "Search codebase"},
		}
	default:
		return nil
	}
}
