package kernel

import (
	"context"
	"fmt"

	"github.com/devrix/devrix/internal/layers/contextengine/materialize"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// MaterializeForMUPS assembles MUPS node context (DM-20260704-001).
func (e *ContextEngine) MaterializeForMUPS(ctx context.Context, req contracts.MUPSContextRequest) (contracts.MUPSPreparedContext, error) {
	if e == nil {
		return contracts.MUPSPreparedContext{}, fmt.Errorf("contextengine: nil engine")
	}
	mat := e.mupsMaterializer()
	return mat.MaterializeForMUPS(ctx, req)
}

func (e *ContextEngine) mupsMaterializer() *materialize.MUPSMaterializer {
	partitionMat := e.defaultPartitionMaterializer()
	return materialize.NewMUPSMaterializer(materialize.MUPSMaterializerDeps{
		FilterDeps: materialize.FilterPipelineDeps{
			ToolsReg:    e.toolsReg,
			Surfaces:    e.surfaces,
			AgentFilter: e.agentRoleToolFilter,
			Locale:      e.PromptLocale(),
		},
		PartitionMat: partitionMat,
		ContractDoc:  "",
		PrepareBase:  e.prepareBaseForMUPS,
	})
}

func (e *ContextEngine) defaultPartitionMaterializer() *materialize.DefaultMaterializer {
	if e.partitionMaterializer != nil {
		return e.partitionMaterializer
	}
	return materialize.NewDefaultMaterializer(nil, "")
}

func (e *ContextEngine) prepareBaseForMUPS(ctx context.Context, sessionID, userMessage string) (string, map[string]string, error) {
	e.wirePrepareOrchestrator()
	session := types.NewSession(sessionID, "d7", "")
	if sc, ok := e.memory.Get(sessionID); ok && sc != nil && sc.WorkDir != "" {
		session.WorkDir = sc.WorkDir
	}
	out, err := e.prepareOrchestrator.Prepare(ctx, e.mupsPrepareInput(session, userMessage), e.startSpan)
	if err != nil {
		return "", nil, err
	}
	var prepend map[string]string
	if out != nil && out.SessionContext != nil {
		prepend = e.UserContextForPrepend(ctx, out.SessionContext)
	}
	if out == nil {
		return "", prepend, nil
	}
	return out.SystemPrompt, prepend, nil
}

func (e *ContextEngine) mupsPrepareInput(session *types.Session, message string) prepare.PrepareInput {
	workerLocal := false
	mode := e.cfg.UserContext.Mode
	if mode == "" {
		mode = "prepend"
	}
	agentsRaw := e.prompt.Load(session.WorkDir)
	return prepare.PrepareInput{
		Session:         session,
		Message:         message,
		WorkerLocal:     workerLocal,
		CompressPerTurn: e.cfg.TurnRuntime.CompressPerTurn,
		AgentsRaw:       agentsRaw,
		UserContextMode: mode,
	}
}

// SetPartitionMaterializer wires the WorkItem partition materializer for MUPS execute.
func (e *ContextEngine) SetPartitionMaterializer(m *materialize.DefaultMaterializer) {
	if e != nil {
		e.partitionMaterializer = m
	}
}
