package materialize

import (
	"context"
	"fmt"
	"strings"

	"github.com/devrix/devrix/internal/layers/contextengine/i18n"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/token"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// MUPSMaterializer assembles MUPS node context (DM-20260704-001).
type MUPSMaterializer struct {
	Deps MUPSMaterializerDeps
}

// MUPSMaterializerDeps holds D2 internals required for MaterializeForMUPS.
type MUPSMaterializerDeps struct {
	FilterDeps   FilterPipelineDeps
	PartitionMat *DefaultMaterializer
	ContractDoc  string
	// PrepareBase runs D2 PrepareForTurn for observe/plan base system prompts.
	PrepareBase func(ctx context.Context, sessionID, userMessage string) (systemPrompt string, userContextPrepend map[string]string, err error)
}

// NewMUPSMaterializer constructs a MUPS materializer.
func NewMUPSMaterializer(deps MUPSMaterializerDeps) *MUPSMaterializer {
	return &MUPSMaterializer{Deps: deps}
}

// MaterializeForMUPS implements contracts.IMUPSContextMaterializer logic in D2.
func (m *MUPSMaterializer) MaterializeForMUPS(ctx context.Context, req contracts.MUPSContextRequest) (contracts.MUPSPreparedContext, error) {
	if err := validateMUPSPhase(req.Phase); err != nil {
		return contracts.MUPSPreparedContext{}, err
	}
	if req.Phase == contracts.MUPSPhaseExecute && req.WorkItem == nil {
		return contracts.MUPSPreparedContext{}, contracts.ErrWorkItemRequired
	}

	loc := i18n.ParseLanguage(req.Policy.Locale)
	if loc == "" {
		loc = i18n.DefaultLocale
	}

	toolProfile := req.ToolProfile
	if toolProfile == "" && req.WorkItem != nil {
		toolProfile = inferToolProfile(req)
	}
	agentProfile := req.AgentProfile
	if agentProfile == "" {
		agentProfile = req.Policy.AgentProfile
	}

	phaseAppendix := BuildPhaseAppendix(req.Phase, loc, req.WorkItem, toolProfile, contractDocForRequest(req, m.Deps.ContractDoc))
	outputHints := ""
	if req.Phase == contracts.MUPSPhaseExecute {
		outputHints = BuildExecuteOutputHints(loc, req.WorkItem)
	}

	var baseSystem string
	var wiBody string
	var messages []types.Message
	var tokenBudget int
	var userContextPrepend map[string]string

	switch req.Phase {
	case contracts.MUPSPhaseObserve, contracts.MUPSPhasePlan:
		userContent := strings.TrimSpace(req.UserMessage)
		if userContent == "" && req.WorkItem != nil {
			userContent = strings.TrimSpace(req.WorkItem.Directive)
		}
		sessionID := ""
		if req.Turn != nil {
			sessionID = req.Turn.SessionID
		}
		if m.Deps.PrepareBase != nil && sessionID != "" && userContent != "" {
			prompt, prepend, err := m.prepareBaseForMUPS(ctx, sessionID, userContent, req.Phase)
			if err != nil {
				return contracts.MUPSPreparedContext{}, fmt.Errorf("mups prepare base: %w", err)
			}
			baseSystem = prompt
			userContextPrepend = prepend
		}
		if userContent != "" && req.Turn != nil {
			messages = []types.Message{{
				SessionID: req.Turn.SessionID,
				Role:      types.MessageRoleUser,
				Content:   userContent,
			}}
		}
		tokenBudget = req.Policy.TokenBudget
	default:
		matReq, err := m.buildExecuteMaterializeRequest(req)
		if err != nil {
			return contracts.MUPSPreparedContext{}, err
		}
		if m.Deps.PartitionMat == nil {
			return contracts.MUPSPreparedContext{}, fmt.Errorf("mups materialize: partition materializer required")
		}
		userContent := strings.TrimSpace(matReq.Signals.Directive)
		sessionID := matReq.Partition.SessionID
		if req.Turn != nil && sessionID == "" {
			sessionID = req.Turn.SessionID
		}
		var coreBase string
		if m.Deps.PrepareBase != nil && sessionID != "" && userContent != "" {
			prompt, prepend, err := m.prepareBaseForMUPS(ctx, sessionID, userContent, req.Phase)
			if err != nil {
				return contracts.MUPSPreparedContext{}, fmt.Errorf("mups prepare base: %w", err)
			}
			coreBase = prompt
			userContextPrepend = prepend
		}
		mat, err := m.Deps.PartitionMat.Materialize(ctx, matReq)
		if err != nil {
			return contracts.MUPSPreparedContext{}, err
		}
		wiBody = buildWorkItemSystemBody(matReq)
		baseSystem = coreBase
		messages = mat.Messages
		tokenBudget = matReq.Policy.TokenBudget
	}

	systemPrompt := AssembleMUPSSystemPrompt(baseSystem, outputHints, wiBody, phaseAppendix)
	counter := token.NewCounter()
	tokEst := counter.CountMessages(messages) + counter.CountText(systemPrompt)

	var tools []ToolDescriptor
	if req.Phase == contracts.MUPSPhaseExecute {
		sc := sessionCtxFromTurn(req.Turn)
		tools = RunFilterPipeline(ctx, m.Deps.FilterDeps, FilterPipelineInput{
			Phase:        req.Phase,
			TaskKind:     req.TaskKind,
			ToolProfile:  toolProfile,
			AgentProfile: agentProfile,
			WorkDir:      workDirFromTurn(req.Turn),
			SessionCtx:   sc,
		})
	}

	if tokenBudget > 0 && tokEst > tokenBudget {
		return contracts.MUPSPreparedContext{}, contracts.ErrTokenBudgetExceeded
	}

	return contracts.MUPSPreparedContext{
		SystemPrompt:       systemPrompt,
		Messages:           messages,
		Tools:              toContractTools(tools),
		TokenBudget:        mupsTokenBudget(tokenBudget),
		OutputHints:        outputHints,
		PhaseAppendix:      phaseAppendix,
		UserContextPrepend: userContextPrepend,
		TokenEst:           tokEst,
		MessageCount:       len(messages),
	}, nil
}

func validateMUPSPhase(phase contracts.MUPSPhase) error {
	switch phase {
	case contracts.MUPSPhaseObserve, contracts.MUPSPhasePlan, contracts.MUPSPhaseExecute:
		return nil
	case contracts.MUPSPhaseVerify, contracts.MUPSPhaseLearn, contracts.MUPSPhaseDecide:
		return contracts.ErrPhaseNotMaterializable
	default:
		return fmt.Errorf("mups: unknown phase %q", phase)
	}
}

func (m *MUPSMaterializer) buildExecuteMaterializeRequest(req contracts.MUPSContextRequest) (Request, error) {
	wi := req.WorkItem
	partition := Partition{
		SessionID:  wi.Partition.SessionID,
		Kind:       PartitionWorkItem,
		WorkItemID: wi.Partition.WorkItemID,
	}
	if wi.Partition.ParentWorkItemID != "" {
		partition.ParentWorkItemID = wi.Partition.ParentWorkItemID
	}
	toolProfile := req.ToolProfile
	if toolProfile == "" {
		toolProfile = inferToolProfile(req)
	}
	mode := ModeFresh
	if toolProfile == "rollup_synth" {
		mode = ModeRollupSynth
	}
	budget := req.Policy.TokenBudget
	if budget <= 0 {
		budget = 32000
	}
	return Request{
		Partition: partition,
		Policy: Policy{
			Mode:         mode,
			TokenBudget:  budget,
			ToolProfile:  toolProfile,
			Locale:       req.Policy.Locale,
			AgentProfile: req.Policy.AgentProfile,
			Depth:        req.Policy.Depth,
		},
		Signals: InboundSignals{
			Directive:      wi.Directive,
			ScopeIn:        append([]string(nil), wi.ScopeIn...),
			ScopeOut:       append([]string(nil), wi.ScopeOut...),
			ExpectedReturn: wi.ExpectedReturn,
		},
	}, nil
}

func contractDocForRequest(req contracts.MUPSContextRequest, fallback string) string {
	if s := strings.TrimSpace(req.ContractDimensionDoc); s != "" {
		return s
	}
	return fallback
}

func inferToolProfile(req contracts.MUPSContextRequest) string {
	if req.ToolProfile != "" {
		return req.ToolProfile
	}
	if req.WorkItem != nil && strings.TrimSpace(req.WorkItem.ExpectedReturn) == "rollup_synth" {
		return "rollup_synth"
	}
	return "implement"
}

func sessionCtxFromTurn(turn *contracts.MUPSTurnContext) *types.SessionContext {
	if turn == nil {
		return nil
	}
	return &types.SessionContext{
		SessionID:      turn.SessionID,
		WorkDir:        turn.WorkDir,
		PermissionMode: turn.PermissionMode,
		PlanFilePath:   turn.PlanFilePath,
		AgentID:        turn.AgentID,
	}
}

func workDirFromTurn(turn *contracts.MUPSTurnContext) string {
	if turn == nil {
		return ""
	}
	return turn.WorkDir
}

func mupsTokenBudget(budget int) contracts.MUPSTokenBudget {
	if budget <= 0 {
		budget = 128000
	}
	return contracts.MUPSTokenBudget{
		MaxContextTokens: budget,
		ReservedOutput:   8192,
		Target:           budget - 8192,
	}
}

func toContractTools(tools []ToolDescriptor) []contracts.MUPSToolDescriptor {
	out := make([]contracts.MUPSToolDescriptor, len(tools))
	for i, t := range tools {
		out[i] = contracts.MUPSToolDescriptor{
			Name:        t.Name,
			Description: t.Description,
			Schema:      t.Schema,
		}
	}
	return out
}

func (m *MUPSMaterializer) prepareBaseForMUPS(ctx context.Context, sessionID, userContent string, phase contracts.MUPSPhase) (string, map[string]string, error) {
	switch phase {
	case contracts.MUPSPhaseObserve, contracts.MUPSPhasePlan, contracts.MUPSPhaseExecute:
		if prompt, prepend, ok := contracts.TryMUPSPrepareBase(ctx, sessionID, userContent); ok {
			return prompt, prepend, nil
		}
	}
	if m.Deps.PrepareBase == nil {
		return "", nil, nil
	}
	prompt, prepend, err := m.Deps.PrepareBase(ctx, sessionID, userContent)
	if err != nil {
		return "", nil, err
	}
	if phase == contracts.MUPSPhaseObserve || phase == contracts.MUPSPhasePlan {
		contracts.StoreMUPSPrepareBase(ctx, sessionID, userContent, prompt, prepend)
	}
	return prompt, prepend, nil
}

// ToSessionToolSchemas converts MUPS tool descriptors to D7 ToolSchema slice.
func ToSessionToolSchemas(tools []contracts.MUPSToolDescriptor) []struct {
	Name, Description string
	Schema            map[string]any
} {
	out := make([]struct {
		Name, Description string
		Schema            map[string]any
	}, len(tools))
	for i, t := range tools {
		out[i].Name = t.Name
		out[i].Description = t.Description
		out[i].Schema = t.Schema
	}
	return out
}
