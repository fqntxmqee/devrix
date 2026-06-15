package testutil

import (
	"context"
	"testing"

	llmbridge "github.com/devrix/devrix/internal/bridges/llm"
	"github.com/devrix/devrix/internal/bootstrap"
	"github.com/devrix/devrix/internal/layers/communication/capture"
	"github.com/devrix/devrix/internal/layers/contextengine"
	mockctx "github.com/devrix/devrix/internal/layers/contextengine/mock"
	"github.com/devrix/devrix/internal/layers/llmgateway/budget"
	"github.com/devrix/devrix/internal/layers/llmgateway/configure"
	"github.com/devrix/devrix/internal/layers/llmgateway/protect"
	llmgw "github.com/devrix/devrix/internal/layers/llmgateway/stream"
	"github.com/devrix/devrix/internal/layers/llmgateway/stream/adapter"
	"github.com/devrix/devrix/internal/layers/multiagent"
	multiagentprovision "github.com/devrix/devrix/internal/layers/multiagent/provision"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/orchestration/coordinator"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/config"
)

// D7StackOptions configures WireD7 integration test wiring.
type D7StackOptions struct {
	LLMStub       *D7LLMStub
	ExecutionFlow bool
	Delegate      bool
	MultiAgent    bool

	// OverrideOrchestratePath replaces the lazy default OrchestratePath
	// (which uses a zero-deps WaveScheduler that panics on Start). Tests
	// that drive the IntentOrchestrate branch must inject a custom path
	// backed by a fake scheduler (see D7Stack.OrchestratePath entry in
	// tests/integration/d7/d7_orthogonal_dispatch_test.go).
	OverrideOrchestratePath *coordinator.OrchestratePath
}

// D7TestStack holds a production-like D1+D2+D3+D7 wiring for integration tests.
type D7TestStack struct {
	Obs       *observability.Observability
	ObsBridge *observability.Bridge
	Gateway   *capture.CommunicationGateway
	Handler   *MockEventHandler
	Engine    *contextengine.ContextEngine
	LLMStub   *D7LLMStub
	WorkDir   string
}

// NewD7TestStack wires bootstrap.WireD7 with mock LLM and context engine.
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
	llmCfg.DefaultModel = llmCfg.Providers["deepseek"].DefaultModel
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
	workmodel.InitGlobalTaskManager(ctxCfg.Tasks, obsBridge)

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

	engine := contextengine.NewContextEngine(contextengine.EngineDeps{
		LLM:          bridge,
		TokenCounter: counter,
		Tools:        &mockctx.ToolRunner{},
		ToolsReg:     toolsReg,
		Permission:   mockctx.AllowAllPermission{},
		Config:       ctxCfg,
		ObsBridge:    obsBridge,
	})

	store, err := capture.NewFileSessionStore(workDir)
	if err != nil {
		t.Fatalf("session store: %v", err)
	}
	handler := NewMockEventHandler()
	permMgr := capture.NewPermissionManager(&config.DefaultConfig().Permission)
	gw := capture.NewCommunicationGateway(store, handler, permMgr, config.DefaultConfig())
	gw.SetObservability(obs)

	if opt.MultiAgent {
		maCfg := config.DefaultMultiAgentConfig()
		factory := multiagentprovision.NewAgentFactory(
			multiagent.AgentDeps{Engine: engine},
			maCfg,
		)
		gw.SetAgentFactory(factory)
	}

	if opt.ExecutionFlow {
		bootstrap.WireExecutionFlow(ctxCfg, gw, obsBridge)
	}
	if opt.Delegate {
		maCfg := config.DefaultMultiAgentConfig()
		maCfg.Delegate.Enabled = true
		if toolReg, ok := toolsReg.(*contextengine.ToolRegistry); ok {
			bootstrap.WireDelegate(ctxCfg, maCfg, gw, engine, toolReg)
		} else {
			t.Fatal("delegate wiring requires *contextengine.ToolRegistry")
		}
	}

	if err := bootstrap.WireD7("", gw, engine, obsBridge, llmStack); err != nil {
		t.Fatalf("WireD7: %v", err)
	}

	if opt.OverrideOrchestratePath != nil {
		entry, ok := gw.OrchestrationEntry().(*coordinator.Entry)
		if !ok {
			t.Fatalf("OverrideOrchestratePath requires *coordinator.Entry, got %T", gw.OrchestrationEntry())
		}
		entry.SetOrchestratePath(opt.OverrideOrchestratePath)
	}

	return &D7TestStack{
		Obs:       obs,
		ObsBridge: obsBridge,
		Gateway:   gw,
		Handler:   handler,
		Engine:    engine,
		LLMStub:   stub,
		WorkDir:   workDir,
	}
}
