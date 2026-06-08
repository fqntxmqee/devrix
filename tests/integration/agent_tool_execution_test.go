//go:build integration && d4

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/contextengine"
	"github.com/devrix/devrix/internal/layers/multiagent/tool"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

// testAgentToolPlugin implements contextengine.PluginRunner to simulate
// how the bootstrap-level call_agent plugin bridges the tool.Registry to D2.
type testAgentToolPlugin struct {
	reg *tool.Registry
}

func (p *testAgentToolPlugin) Name() string { return "call_agent" }

func (p *testAgentToolPlugin) Schema() contextengine.ToolSchema {
	return contextengine.ToolSchema{
		Name:        "call_agent",
		Description: "Call an external agent tool",
		Parameters:  `{"type":"object","properties":{"agent_name":{"type":"string"},"task":{"type":"string"}},"required":["agent_name","task"]}`,
	}
}

func (p *testAgentToolPlugin) RiskLevel() types.RiskLevel { return types.RiskLevelHigh }

func (p *testAgentToolPlugin) Execute(ctx context.Context, workDir, input string) (*contextengine.ToolResult, error) {
	agentTool, err := p.reg.Get("echo-agent")
	if err != nil {
		return &contextengine.ToolResult{Error: err.Error()}, nil
	}
	sessionID := "integration-session"
	evtCh, err := agentTool.Execute(ctx, sessionID, tool.Request{Task: input, WorkDir: workDir})
	if err != nil {
		return &contextengine.ToolResult{Error: err.Error()}, nil
	}
	var parts []string
	for evt := range evtCh {
		switch evt.Type {
		case "text":
			parts = append(parts, evt.Content)
		case "error":
			return &contextengine.ToolResult{Error: evt.Content}, nil
		case "complete":
		}
	}
	if len(parts) == 0 {
		return &contextengine.ToolResult{Output: ""}, nil
	}
	return &contextengine.ToolResult{Output: parts[0]}, nil
}

// Covers: L5-4-6-01, L5-4-6-02, L5-4-6-07
func TestIntegration_AgentTool_RegistryToExecutionChain(t *testing.T) {
	reg := tool.NewRegistry()
	echoTool := tool.NewCLIAgentTool(tool.CLIConfig{
		Name:    "echo-agent",
		Command: "bash",
		Args:    []string{"-c", `echo '{"type":"text","content":"hello from agent"}'; echo '{"type":"complete","content":""}'`},
	})
	defer echoTool.Stop()

	if err := reg.Register(echoTool); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if _, err := reg.Get("echo-agent"); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(reg.List()) != 1 {
		t.Fatalf("List len = %d, want 1", len(reg.List()))
	}

	plugin := &testAgentToolPlugin{reg: reg}
	cfg := config.DefaultToolConfig()
	builtinReg := contextengine.NewBuiltinToolRegistry(cfg)
	if err := builtinReg.Register(plugin); err != nil {
		t.Fatalf("register plugin: %v", err)
	}

	limiter := contextengine.NewToolLimiter(10)
	toolRunner := contextengine.NewLimitedToolRunner(builtinReg, limiter)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := toolRunner.Execute(ctx, contextengine.ToolCall{
		Name: "call_agent",
		Input: `{"agent_name":"echo-agent","task":"hello","work_dir":"/tmp"}`,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("result.Error = %q", result.Error)
	}
	if result.Output != "hello from agent" {
		t.Errorf("result.Output = %q, want 'hello from agent'", result.Output)
	}
}

// Covers: L5-4-6-04, L5-4-6-07
func TestIntegration_AgentTool_SessionLifecycle(t *testing.T) {
	stateful := tool.NewCLIAgentTool(tool.CLIConfig{
		Name:    "stateful-agent",
		Command: "bash",
		Args:    []string{"-c", `while read line; do echo '{"type":"text","content":"ok"}'; echo '{"type":"complete","content":""}'; done`},
	})
	defer stateful.Stop()

	ctx := context.Background()

	// First call creates session
	ch1, err := stateful.Execute(ctx, "sess_1", tool.Request{Task: "first"})
	if err != nil {
		t.Fatalf("first Execute: %v", err)
	}
	for evt := range ch1 {
		if evt.Type == "complete" {
			break
		}
	}

	// Reuse session
	ch2, err := stateful.Execute(ctx, "sess_1", tool.Request{Task: "second"})
	if err != nil {
		t.Fatalf("reuse Execute: %v", err)
	}
	for evt := range ch2 {
		if evt.Type == "complete" {
			break
		}
	}

	// Cleanup session
	stateful.CloseSession("sess_1")

	// New session after cleanup still works
	ch3, err := stateful.Execute(ctx, "sess_2", tool.Request{Task: "third"})
	if err != nil {
		t.Fatalf("new session Execute: %v", err)
	}
	for evt := range ch3 {
		if evt.Type == "complete" {
			break
		}
	}
}

// Covers: L5-4-6-06
func TestIntegration_AgentTool_CleanupOtherSession(t *testing.T) {
	agent := tool.NewCLIAgentTool(tool.CLIConfig{
		Name:    "cleanup-agent",
		Command: "bash",
		Args:    []string{"-c", `while read line; do echo '{"type":"text","content":"ok"}'; echo '{"type":"complete","content":""}'; done`},
	})
	defer agent.Stop()

	ctx := context.Background()

	ch1, _ := agent.Execute(ctx, "sess_a", tool.Request{Task: "a"})
	for evt := range ch1 {
		if evt.Type == "complete" {
			break
		}
	}
	ch2, _ := agent.Execute(ctx, "sess_b", tool.Request{Task: "b"})
	for evt := range ch2 {
		if evt.Type == "complete" {
			break
		}
	}

	agent.CloseSession("sess_a")

	ch3, err := agent.Execute(ctx, "sess_b", tool.Request{Task: "c"})
	if err != nil {
		t.Fatalf("Execute sess_b after cleanup: %v", err)
	}
	for evt := range ch3 {
		if evt.Type == "complete" {
			break
		}
	}
}
