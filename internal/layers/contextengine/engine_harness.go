package contextengine

import (
	"context"
	"fmt"
	"strings"

	"github.com/devrix/devrix/internal/layers/contextengine/fallback"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/memory"
	"github.com/devrix/devrix/internal/layers/observability/instrument/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/errors"
	"github.com/devrix/devrix/internal/shared/types"
)

// harnessPreflightResult holds the output of the legacy harness preflight+routing phase.
type harnessPreflightResult struct {
	visibleTools    []fallback.ToolDesc
	routingHint     *types.RoutingHint
	preflightResult *types.PreflightResult
}

// #deprecated: legacy harness bootstrap (only when query_loop.enabled=false).
func (e *ContextEngine) bootstrapHarness(ctx context.Context, session *types.Session,
	sc *types.SessionContext, emit func(*contracts.EngineEvent)) error {
	if !e.cfg.Harness.Enabled || session.HarnessInitialized {
		return nil
	}
	harnessState, err := e.harnessBoot.Run(ctx, session)
	if err != nil {
		emit(errorEvent(session.SessionID, errors.WithCode("CTX_HARNESS_BOOTSTRAP", err.Error(), err), false))
		return err
	}
	sc.Harness = harnessState
	session.HarnessInitialized = true
	emit(infoEvent(session.SessionID, fmt.Sprintf("Harness bootstrap 完成 (%d→%d 工具)",
		harnessState.Report.ToolCount, harnessState.Report.VisibleTools)))
	return nil
}

// #deprecated: legacy harness preflight + routing (only when harness on and not worker).
func (e *ContextEngine) runHarnessPreflight(ctx context.Context, sc *types.SessionContext,
	agentsRaw string, memoryEntries []memory.MemoryEntry, message string,
	visibleTools []fallback.ToolDesc, workerLocal bool) harnessPreflightResult {

	result := harnessPreflightResult{visibleTools: visibleTools}
	harnessEnabled := e.cfg.Harness.Enabled
	if !harnessEnabled || workerLocal {
		return result
	}

	provisionalContext := agentsRaw
	if len(memoryEntries) > 0 {
		memCtx, _ := memory.FormatMemoryContext(memoryEntries, e.cfg.LongTerm.RecallMaxTokens)
		provisionalContext += "\n" + memCtx
	}
	if e.cfg.Preflight.Enabled {
		_, pfSpan := e.startSpan(ctx, telemetry.OpD2_S5_Context_Harness_Preflight, tracer.SpanKindInternal)
		pfResult := e.preflight.Evaluate(sc, message, visibleTools, provisionalContext)
		result.preflightResult = &pfResult
		filtered, _ := e.preflight.FilterVisibleTools(message, visibleTools)
		result.visibleTools = filtered
		if sc.Harness != nil {
			sc.Harness.Report.VisibleToolList = toolDescsToVisibleTools(filtered)
			sc.Harness.Report.VisibleTools = len(filtered)
		}
		if pfSpan != nil {
			pfSpan.SetAttributes(tracer.Attribute{Key: "preflight.warning_count", Value: fmt.Sprintf("%d", len(pfResult.Warnings))})
			pfSpan.End()
		}
	}
	if e.cfg.Harness.Routing.Enabled {
		_, routeSpan := e.startSpan(ctx, telemetry.OpD2_S5_Context_Harness_Route, tracer.SpanKindInternal)
		hint := e.router.Route(message, visibleTools, e.cfg.Harness.Routing.MaxMatches)
		if len(hint.Tools) > 0 {
			result.routingHint = &hint
		}
		if routeSpan != nil {
			routeSpan.SetAttributes(tracer.Attribute{Key: "matched_tools", Value: strings.Join(hint.Tools, ",")})
			routeSpan.End()
		}
	}
	return result
}
