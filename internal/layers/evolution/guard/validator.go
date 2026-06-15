package guard

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/observability/instrument/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/shared/types"
)

// DecisionHook is called before each validation.
type DecisionHook func(DecisionRecord)

// ValidationHook is called after each judge call.
type ValidationHook func(ValidationResult)

// InterventionHook is called when an intervention is triggered.
type InterventionHook func(Intervention)

// RuntimeOrchestrationValidator is the top-level orchestrator for runtime decision validation.
type RuntimeOrchestrationValidator struct {
	config   OrchestrationConfig
	judge    *RuntimeJudge
	executor *InterventionExecutor

	mu         sync.Mutex
	lastJudge  time.Time
	judgeCalls int

	onDecision  DecisionHook
	onValidate  ValidationHook
	onIntervene InterventionHook

	metrics *orchMetrics
	obs     *observability.Observability
}

// NewRuntimeOrchestrationValidator creates a validator.
func NewRuntimeOrchestrationValidator(
	config OrchestrationConfig,
	judge *RuntimeJudge,
	executor *InterventionExecutor,
) *RuntimeOrchestrationValidator {
	return &RuntimeOrchestrationValidator{
		config:   config,
		judge:    judge,
		executor: executor,
	}
}

// OnDecision is the main entry point for runtime decision validation.
func (v *RuntimeOrchestrationValidator) OnDecision(ctx context.Context, rec DecisionRecord, session *types.Session) {
	if !v.config.Enabled {
		return
	}

	// Start a tracing span for the full validation pipeline.
	var span tracer.Span
	if v.obs != nil && v.obs.Tracer() != nil {
		ctx, span = v.obs.Tracer().Start(ctx, telemetry.OpD6_S4_Validation_Decision,
			tracer.WithSpanKind(tracer.SpanKindInternal),
			tracer.WithSpanAttributes(telemetry.SpanAttrs(telemetry.OpD6_S4_Validation_Decision,
				tracer.Attribute{Key: "decision_id", Value: rec.ID},
				tracer.Attribute{Key: "category", Value: string(rec.Category)},
				tracer.Attribute{Key: "risk_class", Value: int(rec.RiskClass)},
				tracer.Attribute{Key: "session_id", Value: rec.SessionID},
				tracer.Attribute{Key: "agent_id", Value: rec.AgentID},
			)...),
		)
		defer span.End()
	}

	v.metrics.recordDecision(string(rec.Category), int(rec.RiskClass))
	v.metrics.recordStage("incoming")
	if v.onDecision != nil {
		v.onDecision(rec)
	}

	if v.preFilter(rec) {
		v.metrics.recordStage("prefilter_skip")
		if span != nil {
			span.AddEvent("prefilter_skip",
				tracer.WithEventAttributes(tracer.Attribute{Key: "category", Value: string(rec.Category)}),
			)
		}
		return
	}
	if span != nil {
		span.AddEvent("judge_start")
	}

	result, err := v.judge.ValidateDecision(ctx, rec)
	if err != nil {
		v.metrics.recordStage("judge_error")
		slog.Error("orchestration: judge validation failed",
			"decision_id", rec.ID,
			"error", err,
		)
		if span != nil {
			span.RecordError(err)
			span.SetStatus(tracer.StatusCodeError, "judge validation failed")
		}
		return
	}
	v.metrics.recordValidation(result.Valid, true)
	v.metrics.recordJudgeLatency(result.Duration.Seconds())
	if v.onValidate != nil {
		v.onValidate(*result)
	}
	if span != nil {
		span.AddEvent("judge_complete",
			tracer.WithEventAttributes(
				tracer.Attribute{Key: "valid", Value: result.Valid},
				tracer.Attribute{Key: "confidence", Value: result.Confidence},
				tracer.Attribute{Key: "suggested_action", Value: result.SuggestedAction},
			),
		)
	}

	if result.Valid || result.Confidence >= v.config.InterventionThreshold {
		v.metrics.recordStage("judge_pass")
		if span != nil {
			span.SetStatus(tracer.StatusCodeOk, "decision accepted")
		}
		return
	}

	iv := Intervention{
		DecisionID: rec.ID,
		Action:     result.SuggestedAction,
		Reason:     result.Reasoning,
	}
	if result.SuggestedAction == "reroute" {
		iv.TargetAgentID = result.SuggestedAgentID
		iv.AgentConfig = &multiagent.AgentConfig{
			SessionID: rec.SessionID,
			WorkDir:   session.WorkDir,
		}
	}

	v.metrics.recordStage("intervention")
	v.metrics.recordIntervention(iv.Action)
	if v.onIntervene != nil {
		v.onIntervene(iv)
	}
	if span != nil {
		span.AddEvent("intervention_triggered",
			tracer.WithEventAttributes(
				tracer.Attribute{Key: "action", Value: iv.Action},
				tracer.Attribute{Key: "reason", Value: iv.Reason},
			),
		)
	}

	if v.config.AutoIntervene {
		slog.Warn("orchestration: auto-intervening",
			"action", iv.Action,
			"reason", iv.Reason,
			"decision_id", rec.ID,
		)
		if span != nil {
			span.AddEvent("intervention_exec_start")
		}
		if err := v.executor.Execute(ctx, iv, session); err != nil {
			v.metrics.recordStage("intervention_error")
			slog.Error("orchestration: intervention failed",
				"error", err,
				"action", iv.Action,
			)
			if span != nil {
				span.RecordError(err)
				span.SetStatus(tracer.StatusCodeError, "intervention execution failed")
			}
			return
		}
		if span != nil {
			span.AddEvent("intervention_exec_complete")
			span.SetStatus(tracer.StatusCodeOk, "intervention applied")
		}
	} else if span != nil {
		span.SetStatus(tracer.StatusCodeOk, "intervention recorded (auto=false)")
	}
}

func (v *RuntimeOrchestrationValidator) preFilter(rec DecisionRecord) bool {
	if !v.config.PreFilterEnabled {
		return false
	}

	if rec.Category == DecisionToolCall && rec.ToolName != "" {
		for _, trusted := range v.config.TrustedToolAllowlist {
			if strings.EqualFold(rec.ToolName, trusted) ||
				strings.HasPrefix(rec.ToolName, trusted) {
				return true
			}
		}
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	if time.Since(v.lastJudge) < v.config.MinIntervalBetweenJudges {
		return true
	}
	if v.judgeCalls >= v.config.MaxJudgeCallsPerMinute {
		return true
	}
	v.judgeCalls++
	go func() {
		time.Sleep(1 * time.Minute)
		v.mu.Lock()
		if v.judgeCalls > 0 {
			v.judgeCalls--
		}
		v.mu.Unlock()
	}()
	v.lastJudge = time.Now()
	return false
}

func (v *RuntimeOrchestrationValidator) OnDecisionHook(fn DecisionHook)     { v.onDecision = fn }
func (v *RuntimeOrchestrationValidator) OnValidateHook(fn ValidationHook)    { v.onValidate = fn }
func (v *RuntimeOrchestrationValidator) OnInterveneHook(fn InterventionHook) { v.onIntervene = fn }

// SetObservability configures OpenTelemetry metrics and tracing for the validator.
func (v *RuntimeOrchestrationValidator) SetObservability(obs *observability.Observability) {
	v.obs = obs
	v.metrics = initOrchMetrics(obs)
	if obs != nil {
		slog.Info("orchestration: observability initialized",
			"tracing_enabled", obs.Tracer() != nil,
			"metrics_enabled", obs.Meter() != nil,
		)
	}
}
