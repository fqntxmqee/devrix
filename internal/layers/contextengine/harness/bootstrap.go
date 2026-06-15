package harness

import (
	"context"
	"fmt"
	"time"

	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/observability/instrument/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

// Bootstrap orchestrates harness bootstrap stages.
type Bootstrap struct {
	cfg       config.HarnessConfig
	toolsReg  ToolLister
	deferred  IDeferredInit
	toolPool  IToolPoolFilter
	emitStage StageEmitter
	obsBridge *observability.Bridge
}

// BootstrapDeps holds dependencies for harness bootstrap.
type BootstrapDeps struct {
	Config    config.HarnessConfig
	ToolsReg  ToolLister
	Deferred  IDeferredInit
	ToolPool  IToolPoolFilter
	EmitStage StageEmitter
	ObsBridge *observability.Bridge
}

// NewBootstrap creates a harness bootstrap orchestrator.
func NewBootstrap(deps BootstrapDeps) *Bootstrap {
	deferred := deps.Deferred
	if deferred == nil {
		deferred = NoOpDeferredInit{}
	}
	toolPool := deps.ToolPool
	if toolPool == nil {
		toolPool = NewToolPoolFilter(deps.Config.ToolPool)
	}
	return &Bootstrap{
		cfg:       deps.Config,
		toolsReg:  deps.ToolsReg,
		deferred:  deferred,
		toolPool:  toolPool,
		emitStage: deps.EmitStage,
		obsBridge: deps.ObsBridge,
	}
}

// Run executes bootstrap stages and returns harness session state.
func (b *Bootstrap) Run(ctx context.Context, session *types.Session) (state *types.HarnessSessionState, err error) {
	if session == nil {
		return nil, fmt.Errorf("session is nil")
	}
	start := time.Now()
	report := types.BootstrapReport{
		Trusted: b.cfg.Trusted,
	}

	ctx, runSpan := startHarnessSpan(ctx, b.obsBridge, telemetry.OpD2_S5_Context_Harness_Bootstrap_Run, tracer.SpanKindInternal,
		tracer.Attribute{Key: "harness.trusted", Value: fmt.Sprintf("%t", b.cfg.Trusted)},
	)
	if runSpan != nil {
		defer func() {
			if err != nil {
				runSpan.RecordError(err)
			}
			runSpan.End()
		}()
	}

	if b.cfg.Prefetch.Enabled {
		stageCtx, stageSpan := b.stageSpan(ctx, types.BootstrapStagePrefetch)
		b.emit(types.BootstrapStagePrefetch, map[string]string{"harness.stage": string(types.BootstrapStagePrefetch)})
		ws, err := ScanWorkspace(session.WorkDir, b.cfg.Prefetch)
		b.endStageSpan(stageSpan, err)
		if err != nil {
			return nil, fmt.Errorf("prefetch: %w", err)
		}
		report.Workspace = ws
		report.StagesApplied = append(report.StagesApplied, types.BootstrapStagePrefetch)
		_ = stageCtx
	}

	{
		stageCtx, stageSpan := b.stageSpan(ctx, types.BootstrapStageGuards)
		b.emit(types.BootstrapStageGuards, map[string]string{"harness.stage": string(types.BootstrapStageGuards)})
		err := CheckGuards(session.WorkDir)
		b.endStageSpan(stageSpan, err)
		if err != nil {
			return nil, fmt.Errorf("guards: %w", err)
		}
		report.StagesApplied = append(report.StagesApplied, types.BootstrapStageGuards)
		_ = stageCtx
	}

	var allTools []ToolDesc
	{
		stageCtx, stageSpan := b.stageSpan(ctx, types.BootstrapStageSetup)
		b.emit(types.BootstrapStageSetup, map[string]string{"harness.stage": string(types.BootstrapStageSetup)})
		var err error
		allTools, err = b.toolsReg.ListTools(ctx, session.WorkDir)
		b.endStageSpan(stageSpan, err)
		if err != nil {
			return nil, fmt.Errorf("setup: %w", err)
		}
		report.ToolCount = len(allTools)
		report.StagesApplied = append(report.StagesApplied, types.BootstrapStageSetup)
		_ = stageCtx
	}

	if b.cfg.DeferredInit.Enabled {
		stageCtx, stageSpan := b.stageSpan(ctx, types.BootstrapStageDeferredInit)
		b.emit(types.BootstrapStageDeferredInit, map[string]string{"harness.stage": string(types.BootstrapStageDeferredInit)})
		deferredResult, err := b.deferred.Run(ctx, b.cfg.Trusted, session)
		b.endStageSpan(stageSpan, err)
		if err != nil {
			return nil, fmt.Errorf("deferred_init: %w", err)
		}
		report.DeferredInit = deferredResult
		report.StagesApplied = append(report.StagesApplied, types.BootstrapStageDeferredInit)
		_ = stageCtx
	}

	{
		poolCtx, poolSpan := startHarnessSpan(ctx, b.obsBridge, telemetry.OpD2_S5_Context_Harness_ToolPool, tracer.SpanKindInternal,
			tracer.Attribute{Key: "tools.before", Value: fmt.Sprintf("%d", len(allTools))},
		)
		stageCtx, stageSpan := b.stageSpan(poolCtx, types.BootstrapStageToolPool)
		b.emit(types.BootstrapStageToolPool, map[string]string{"harness.stage": string(types.BootstrapStageToolPool)})
		visible := b.toolPool.Filter(allTools)
		report.VisibleTools = len(visible)
		report.VisibleToolList = toSharedVisibleTools(visible)
		report.StagesApplied = append(report.StagesApplied, types.BootstrapStageToolPool)
		if poolSpan != nil {
			poolSpan.SetAttributes(tracer.Attribute{Key: "tools.after", Value: fmt.Sprintf("%d", len(visible))})
			poolSpan.End()
		}
		b.endStageSpan(stageSpan, nil)
		_ = stageCtx
	}
	report.Duration = time.Since(start)

	return &types.HarnessSessionState{
		Initialized: true,
		Report:      report,
	}, nil
}

func (b *Bootstrap) emit(stage types.BootstrapStage, metadata map[string]string) {
	if b.emitStage != nil {
		b.emitStage(stage, metadata)
	}
}

func (b *Bootstrap) stageSpan(ctx context.Context, stage types.BootstrapStage) (context.Context, tracer.Span) {
	return startHarnessSpan(ctx, b.obsBridge, telemetry.OpD2_S5_Context_Harness_Bootstrap_Stage, tracer.SpanKindInternal,
		tracer.Attribute{Key: "harness.stage", Value: string(stage)},
	)
}

func (b *Bootstrap) endStageSpan(span tracer.Span, err error) {
	if span == nil {
		return
	}
	if err != nil {
		span.RecordError(err)
	}
	span.End()
}

func toSharedVisibleTools(tools []ToolDesc) []types.VisibleTool {
	out := make([]types.VisibleTool, 0, len(tools))
	for _, t := range tools {
		out = append(out, types.VisibleTool{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  t.Parameters,
		})
	}
	return out
}

// VisibleToolsFromState returns tool descriptions from harness session state.
func VisibleToolsFromState(state *types.HarnessSessionState) []ToolDesc {
	if state == nil {
		return nil
	}
	out := make([]ToolDesc, 0, len(state.Report.VisibleToolList))
	for _, t := range state.Report.VisibleToolList {
		out = append(out, ToolDesc{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  t.Parameters,
		})
	}
	return out
}
