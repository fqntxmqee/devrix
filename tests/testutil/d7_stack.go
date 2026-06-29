package testutil

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/devrix/devrix/internal/bootstrap"
	llmbridge "github.com/devrix/devrix/internal/bridges/llm"
	"github.com/devrix/devrix/internal/bootstrap/sessionagents"
	"github.com/devrix/devrix/internal/layers/communication/capture"
	capturetranscript "github.com/devrix/devrix/internal/layers/communication/capture/transcript"
	"github.com/devrix/devrix/internal/layers/contextengine"
	"github.com/devrix/devrix/internal/layers/contextengine/kernel"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce"
	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/llmgateway/budget"
	"github.com/devrix/devrix/internal/layers/llmgateway/configure"
	"github.com/devrix/devrix/internal/layers/llmgateway/protect"
	llmgw "github.com/devrix/devrix/internal/layers/llmgateway/stream"
	"github.com/devrix/devrix/internal/layers/llmgateway/stream/adapter"
	"github.com/devrix/devrix/internal/layers/multiagent"
	multiagentprovision "github.com/devrix/devrix/internal/layers/multiagent/provision"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/orchestration/executionflow"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
)

// D7StackOptions configures InitOrchestration integration test wiring.
type D7StackOptions struct {
	// LLMStub is the IAdapter used for D3 calls. When nil, defaults to
	// &D7LLMStub{Response: "D7 integration OK"}. Tests can inject
	// *D7LLMStub, *SequenceLLMStub, or any custom llmgateway.IAdapter.
	LLMStub       llmgateway.IAdapter
	ExecutionFlow bool
	Delegate      bool
	MultiAgent    bool

	// RoutingMode sets coordinator.routing_mode (default loop_first).
	RoutingMode string

	// WorkItemPipeline wires ItemPipelineRunner in the stack (always on by default).
	WorkItemPipeline bool

	// TranscriptDir (DM-20260628-003, D7-S15) enables transcript writing
	// for the gateway; when non-empty, a transcript.Writer is constructed
	// and passed to NewCommunicationGateway. Default "" = nil writer =
	// no transcript jsonl. Pair with PriorContextRounds to verify the
	// turn-N+1 directive is enriched with turn-N finalText.
	TranscriptDir string

	// PriorContextRounds (DM-20260628-003, D7-S15) is written into the
	// d7-test-config.yaml as d7.prior_context_rounds. >0 also forces
	// the bootstrap to wire TurnState + TranscriptReader. Default 0
	// (disabled, pre-D7-S15 behavior).
	PriorContextRounds int
}

// D7TestStack holds a production-like D1+D2+D3+D7 wiring for integration tests.
type D7TestStack struct {
	Obs          *observability.Observability
	ObsBridge    *observability.Bridge
	Gateway       *capture.CommunicationGateway
	SessionAgents *sessionagents.Manager
	Handler       *MockEventHandler
	Engine       *kernel.ContextEngine
	LLMStub      llmgateway.IAdapter
	WorkDir      string
	TaskManager  *workmodel.TaskManager
	FlowHub      contracts.ExecutionFlowHub
	SessionQueue *executionflow.SessionQueue
}

// NewD7TestStack wires bootstrap.InitOrchestration with mock LLM and context engine.
func NewD7TestStack(t *testing.T, opt D7StackOptions) *D7TestStack {
	t.Helper()

	workDir := t.TempDir()
	obs, err := observability.New(observability.DefaultConfig())
	if err != nil {
		t.Fatalf("observability: %v", err)
	}
	t.Cleanup(func() { _ = obs.Shutdown(context.Background()) })
	obsBridge := observability.NewBridge(obs)

	counter, err := budget.NewCounter()
	if err != nil {
		t.Skip("tiktoken not available:", err)
	}

	stub := opt.LLMStub
	if stub == nil {
		stub = &D7LLMStub{Response: "D7 integration OK"}
	}

	llmCfg := configure.DefaultLLMGatewayConfig()
	llmCfg.DefaultProvider = "deepseek"
	// DefaultModel must be non-empty so the LLMInvoker's defaultTier
	// is populated (otherwise bridge.ResolveTier("") errors out and
	// every nested turn call short-circuits). The production default
	// is intentionally empty (operator fills it via env / config),
	// but for tests we want a deterministic stub target.
	if p, ok := llmCfg.Providers["deepseek"]; ok && p.DefaultModel == "" {
		p.DefaultModel = "deepseek-v4-flash"
		llmCfg.Providers["deepseek"] = p
	}
	llmCfg.DefaultModel = llmCfg.Providers["deepseek"].DefaultModel
	// ModelRouting must be populated so the gateway's router can map
	// model names → providers. The default map is empty, which causes
	// "unsupported llm model" for any LLM call. Tests use the same
	// "deepseek-*" prefix match as production routing.
	if len(llmCfg.ModelRouting) == 0 {
		llmCfg.ModelRouting = map[string]string{"deepseek-*": "deepseek"}
	}
	reg := adapter.NewRegistry()
	if err := reg.Register(stub); err != nil {
		t.Fatalf("register llm stub: %v", err)
	}
	llmGW := llmgw.New(llmgw.Deps{
		Config:   llmCfg,
		Registry: reg,
		Breaker:  protect.New(llmCfg.CircuitBreaker),
		Retry:    protect.NewExecutor(),
		Counter:  counter,
		Obs:      obsBridge,
	})
	bridge := llmbridge.New(llmGW)
	llmStack := llmbridge.ContextLLMStack{
		Gateway:      bridge,
		RawGateway:   llmGW,
		TokenCounter: counter,
		DefaultModel: llmCfg.DefaultModel,
		TierResolver: bridge,
	}

	ctxCfg := config.DefaultContextEngineConfig()
	ctxCfg.LongTerm.Enabled = false
	if opt.ExecutionFlow {
		ctxCfg.ExecutionFlow.Enabled = true
		ctxCfg.ExecutionFlow.LinkTasks = true
		ctxCfg.ExecutionFlow.IMProgress = false
	}
	ctxCfg.Tasks.Mode = "v2"
	ctxCfg.Tasks.StoreDir = workDir
	// DM-20260617-008 W4: TaskManager constructed locally and shared with
	// InitOrchestration + WireDelegate + tests (was: workmodel.GlobalTaskManager
	// process-wide singleton).
	tm := workmodel.NewTaskManagerFromConfig(ctxCfg.Tasks, obsBridge)

	var toolsReg contextengine.IToolRegistry
	if opt.Delegate {
		toolReg, err := contextengine.NewBuiltinToolRegistry(nil)
		if err != nil {
			t.Fatalf("tool registry: %v", err)
		}
		toolsReg = toolReg
	} else {
		toolsReg = MustBuiltinRegistry(t)
	}

	engine := kernel.NewContextEngine(MergeEngineDeps(
		ContextEngineDepsFromStack(llmStack, ctxCfg),
		kernel.EngineDeps{
			Tools:      &enforce.ToolRunner{},
			ToolsReg:   toolsReg,
			Permission: enforce.AllowAllPermission{},
			ObsBridge:  obsBridge,
		},
	))

	store, err := capture.NewFileSessionStore(workDir)
	if err != nil {
		t.Fatalf("session store: %v", err)
	}
	handler := NewMockEventHandler()
	permMgr := capture.NewPermissionManager(&config.DefaultConfig().Permission)
	var transcriptWriter *capturetranscript.Writer
	if opt.TranscriptDir != "" {
		if err := os.MkdirAll(opt.TranscriptDir, 0o755); err != nil {
			t.Fatalf("transcript dir: %v", err)
		}
		tw, terr := capturetranscript.NewWriter(opt.TranscriptDir)
		if terr != nil {
			t.Fatalf("transcript writer: %v", terr)
		}
		transcriptWriter = tw
	}
	gw := capture.NewCommunicationGateway(store, handler, permMgr, config.DefaultConfig(), transcriptWriter)
	gw.SetObservability(obs)

	var sessionAgents *sessionagents.Manager
	if opt.MultiAgent {
		maCfg := config.DefaultMultiAgentConfig()
		factory := multiagentprovision.NewAgentFactory(
			multiagent.AgentDeps{Engine: engine},
			maCfg,
		)
		sessionAgents = WireGatewaySessionAgents(gw, factory)
	}

	var flowHub contracts.ExecutionFlowHub
	var sessionQ *executionflow.SessionQueue
	if opt.ExecutionFlow || opt.Delegate {
		flowHub, sessionQ = bootstrap.WireExecutionFlow(ctxCfg, gw, obsBridge, tm)
	}
	if opt.Delegate {
		maCfg := config.DefaultMultiAgentConfig()
		maCfg.Delegate.Enabled = true
		if toolReg, ok := toolsReg.(*contextengine.ToolRegistry); ok {
			bootstrap.WireDelegate(ctxCfg, maCfg, gw, sessionAgents, engine, toolReg, flowHub, tm)
		} else {
			t.Fatal("delegate wiring requires *contextengine.ToolRegistry")
		}
	}

	configFile := ""
	if opt.RoutingMode != "" || opt.PriorContextRounds > 0 {
		cfgPath := filepath.Join(workDir, "d7-test-config.yaml")
		yaml := "d7:\n  enabled: true\n"
		if opt.RoutingMode != "" {
			yaml += "  routing_mode: " + opt.RoutingMode + "\n"
		}
		if opt.PriorContextRounds > 0 {
			yaml += fmt.Sprintf("  prior_context_rounds: %d\n", opt.PriorContextRounds)
		}
		// DM-20260628-003 (D7-S15): when TranscriptDir is set, propagate
		// it into the file so bootstrap's resolveTranscriptDir reads the
		// same dir the gateway writer writes to. Without this the
		// orchestrator's TranscriptReader would fall back to the default
		// ~/.devrix/transcripts and the LP-5 enrichment would silently
		// no-op.
		if opt.TranscriptDir != "" {
			yaml += "context_engine:\n  diagnostics:\n    transcript_dir: " + opt.TranscriptDir + "\n"
		}
		if err := os.WriteFile(cfgPath, []byte(yaml), 0o600); err != nil {
			t.Fatalf("write test config: %v", err)
		}
		configFile = cfgPath
	}

	if err := bootstrap.InitOrchestration(configFile, gw, engine, obsBridge, llmStack, nil); err != nil {
		t.Fatalf("InitOrchestration: %v", err)
	}

	return &D7TestStack{
		Obs:           obs,
		ObsBridge:     obsBridge,
		Gateway:       gw,
		SessionAgents: sessionAgents,
		Handler:       handler,
		Engine:       engine,
		LLMStub:      stub,
		WorkDir:      workDir,
		TaskManager:  tm,
		FlowHub:      flowHub,
		SessionQueue: sessionQ,
	}
}
