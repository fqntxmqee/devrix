package materialize

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/devrix/devrix/internal/layers/contextengine/prepare/token"
	"github.com/devrix/devrix/internal/shared/types"
)

// DefaultMaterializer implements the light Materialize path (OQ-LC-2).
type DefaultMaterializer struct {
	Store *PartitionStore
	// ProjectDir is the persist root for oversized tool results
	// (<projectDir>/<sessionID>/tool-results/). Defaults to Store.BaseDir().
	ProjectDir string
	perMsgStates sync.Map // sessionID → *persist.ContentReplacementState
}

// NewDefaultMaterializer constructs a materializer with partition store.
func NewDefaultMaterializer(store *PartitionStore, projectDir string) *DefaultMaterializer {
	m := &DefaultMaterializer{Store: store, ProjectDir: projectDir}
	if m.ProjectDir == "" && store != nil {
		m.ProjectDir = store.BaseDir()
	}
	return m
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
		msgs = mergeInitialWithPrivateChain(msgs, priv)
	}
	msgs = m.shrinkPrivateChain(req.Partition.SessionID, msgs, req.Policy.TokenBudget)
	counter := token.NewCounter()
	tokEst := counter.CountMessages(msgs) + counter.CountText(sys)
	return Result{
		SystemPrompt: sys,
		Messages:     msgs,
		Tools:        nil,
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
	msgs = m.shrinkPrivateChain(req.Partition.SessionID, msgs, req.Policy.TokenBudget)
	sys := strings.TrimSpace(req.SystemPrompt)
	counter := token.NewCounter()
	tokEst := counter.CountMessages(msgs) + counter.CountText(sys)
	return Result{
		SystemPrompt: sys,
		Messages:     msgs,
		Tools:        nil,
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
	msgs = m.shrinkPrivateChain(req.Partition.SessionID, msgs, req.Policy.TokenBudget)
	counter := token.NewCounter()
	tokEst := counter.CountMessages(msgs) + counter.CountText(sys)
	return Result{
		SystemPrompt: sys,
		Messages:     msgs,
		MessageCount: len(msgs),
		TokenEst:     tokEst,
	}, nil
}

