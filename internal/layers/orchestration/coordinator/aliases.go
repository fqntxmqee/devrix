// Package coordinator is a legacy shim re-exporting D7-S2/S5 APIs.
// Deprecated: use sessionorchestrator and decisionplanning directly.
package coordinator

import (
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/orchestration/decisionplanning"
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"github.com/devrix/devrix/internal/layers/orchestration/sessionorchestrator"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
)

type (
	Config               = orchtypes.Config
	FileConfig           = orchtypes.FileConfig
	RoutingMode          = orchtypes.RoutingMode
	IntentKind           = orchtypes.IntentKind
	IntentClassification = orchtypes.IntentClassification
	ProcessRequest       = orchtypes.ProcessRequest
	ProcessResult        = orchtypes.ProcessResult
	TaskType             = orchtypes.TaskType
	TaskSpec             = orchtypes.TaskSpec
	Plan                 = orchtypes.Plan
	TaskStatus           = orchtypes.TaskStatus
	QueryRequest         = sessionorchestrator.QueryRequest
	TurnExecutor         = sessionorchestrator.TurnExecutor
	EventPublisher       = sessionorchestrator.EventPublisher
	SessionOrchestrator  = sessionorchestrator.SessionOrchestrator
	Entry                = sessionorchestrator.Entry
	FastPath             = sessionorchestrator.FastPath
	OrchestratePath      = sessionorchestrator.OrchestratePath
	WaveSchedulerRunner  = sessionorchestrator.WaveSchedulerRunner
	CommandHandler       = sessionorchestrator.CommandHandler
	WorkModel            = sessionorchestrator.WorkModel
	LocalWorkModel       = sessionorchestrator.LocalWorkModel
	BackgroundLite       = sessionorchestrator.BackgroundLite
	IntentClassifier     = decisionplanning.IntentClassifier
	TaskDecomposer       = decisionplanning.TaskDecomposer
	LLMTaskDecomposer    = decisionplanning.LLMTaskDecomposer
	RuleClassifier       = decisionplanning.RuleClassifier
	ShadowClassifier     = decisionplanning.ShadowClassifier
	LLMDecomposer        = decisionplanning.LLMDecomposer
	LLMDecomposerDeps    = decisionplanning.LLMDecomposerDeps
	TurnToolExecutor     = sessionorchestrator.TurnToolExecutor
	TurnToolMetrics      = sessionorchestrator.TurnToolMetrics
	TurnPrepareWrapper   = sessionorchestrator.TurnPrepareWrapper
	Task                 = workmodel.Task
	TaskManager          = workmodel.TaskManager
	TaskStore            = workmodel.TaskStore
	DiskTaskStore        = workmodel.DiskTaskStore
)

const (
	RoutingModeLoopFirst       = orchtypes.RoutingModeLoopFirst
	RoutingModeRuleOrchestrate = orchtypes.RoutingModeRuleOrchestrate
	IntentFast                 = orchtypes.IntentFast
	IntentCommand              = orchtypes.IntentCommand
	IntentOrchestrate          = orchtypes.IntentOrchestrate
	IntentSkip                 = orchtypes.IntentSkip
	TaskTypeExplore            = orchtypes.TaskTypeExplore
	TaskTypePlan               = orchtypes.TaskTypePlan
	TaskTypeExecute            = orchtypes.TaskTypeExecute
	TaskTypeBackground         = orchtypes.TaskTypeBackground
	TaskStatusPending          = orchtypes.TaskStatusPending
	TaskStatusInProgress       = orchtypes.TaskStatusInProgress
	TaskStatusCompleted        = orchtypes.TaskStatusCompleted
	TaskStatusFailed           = orchtypes.TaskStatusFailed
	TaskStatusCancelled        = orchtypes.TaskStatusCancelled
)

var (
	DefaultConfig          = orchtypes.DefaultConfig
	RuleOrchestrateConfig  = orchtypes.RuleOrchestrateConfig
	BuildConfig            = orchtypes.BuildConfig
	NewRuleClassifier      = decisionplanning.NewRuleClassifier
	NewTaskDecomposer      = decisionplanning.NewTaskDecomposer
	NewSessionOrchestrator = sessionorchestrator.NewSessionOrchestrator
	NewEntry               = sessionorchestrator.NewEntry
	NewFastPath            = sessionorchestrator.NewFastPath
	NewOrchestratePath     = sessionorchestrator.NewOrchestratePath
	NewCommandHandler      = sessionorchestrator.NewCommandHandler
	NewLocalWorkModel      = sessionorchestrator.NewLocalWorkModel
	NewTask                = workmodel.NewTask
	NewTaskManager         = workmodel.NewTaskManager
	NewDiskTaskStore       = workmodel.NewDiskTaskStore
	NewLLMDecomposer       = decisionplanning.NewLLMDecomposer
	NewTurnToolExecutor    = sessionorchestrator.NewTurnToolExecutor
	NewTurnToolMetrics     = sessionorchestrator.NewTurnToolMetrics
)

func WithSink(s EventPublisher) sessionorchestrator.OrchestratorOption {
	return sessionorchestrator.WithSink(s)
}

func WithValidator(v sessionorchestrator.AdvisoryValidator) sessionorchestrator.OrchestratorOption {
	return sessionorchestrator.WithValidator(v)
}

func WithTaskManager(tm *workmodel.TaskManager) sessionorchestrator.OrchestratorOption {
	return sessionorchestrator.WithTaskManager(tm)
}

func WithCommandHandler(h *CommandHandler) sessionorchestrator.OrchestratorOption {
	return sessionorchestrator.WithCommandHandler(h)
}

func WithOrchestratePath(op *OrchestratePath) sessionorchestrator.OrchestratorOption {
	return sessionorchestrator.WithOrchestratePath(op)
}

func WithLLMDecomposer(d LLMTaskDecomposer) sessionorchestrator.OrchestratorOption {
	return sessionorchestrator.WithLLMDecomposer(d)
}

func WithShadowClassifier(sc *ShadowClassifier) sessionorchestrator.OrchestratorOption {
	return sessionorchestrator.WithShadowClassifier(sc)
}

func WithTurnToolExecutor(e *sessionorchestrator.TurnToolExecutor) sessionorchestrator.OrchestratorOption {
	return sessionorchestrator.WithTurnToolExecutor(e)
}

func WithObservability(bridge *observability.Bridge) sessionorchestrator.OrchestratorOption {
	return sessionorchestrator.WithObservability(bridge)
}

func WithWorkModel(w WorkModel) sessionorchestrator.OrchestratorOption {
	return sessionorchestrator.WithWorkModel(w)
}
