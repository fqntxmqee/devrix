//go:build integration && d2

package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	milestonebridge "github.com/devrix/devrix/internal/bridges/milestone"
	"github.com/devrix/devrix/internal/layers/communication/gateway"
	"github.com/devrix/devrix/internal/layers/communication/milestone"
	"github.com/devrix/devrix/internal/layers/contextengine"
	mockctx "github.com/devrix/devrix/internal/layers/contextengine/mock"
	"github.com/devrix/devrix/internal/layers/contextengine/registry"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

type planLLMGateway struct {
	response string
}

func (p *planLLMGateway) ChatStream(_ context.Context, req *contextengine.LLMRequest) (<-chan contextengine.LLMChunk, error) {
	ch := make(chan contextengine.LLMChunk, 2)
	text := "Done."
	if strings.Contains(req.SystemPrompt, "task planner") {
		text = p.response
	}
	go func() {
		ch <- contextengine.LLMChunk{Content: text, Done: true}
		close(ch)
	}()
	return ch, nil
}

// Covers: L5-CTX-20, L5-CTX-21, L5-CTX-24
func TestIntegration_PlanMilestoneProgressEvents(t *testing.T) {
	msSvc := milestone.NewMilestoneService(nil)
	planner := milestonebridge.NewPlannerAdapter(msSvc)

	ctxCfg := config.DefaultContextEngineConfig()
	ctxCfg.Plan.Enabled = true
	ctxCfg.Plan.AutoDetect = false

	planJSON := `{
		"task_id": "task_integration",
		"milestones": [
			{"id": "ms_1", "name": "analyze", "description": "read code", "dependencies": []},
			{"id": "ms_2", "name": "fix", "description": "apply fix", "dependencies": ["ms_1"]}
		]
	}`

	engine := contextengine.NewContextEngine(contextengine.EngineDeps{
		LLM:        &planLLMGateway{response: planJSON},
		Tools:      &mockctx.ToolRunner{},
		ToolsReg:   registry.NewBuiltinRegistry(),
		Permission: mockctx.AllowAllPermission{},
		Planner:    planner,
		Config:     ctxCfg,
	})

	session := &types.Session{
		SessionID: "sess_plan",
		WorkDir:   t.TempDir(),
		Model:     "mock",
	}

	ch := engine.Process(context.Background(), session, "/plan refactor auth module")
	var events []*gateway.EngineEvent
	for ev := range ch {
		events = append(events, ev)
	}

	var progressCount int
	for _, ev := range events {
		if ev.Type == "milestone_progress" {
			progressCount++
			if ev.Metadata["milestone_id"] == "" {
				t.Fatal("milestone_progress missing milestone_id")
			}
			if ev.Metadata["task"] == "" {
				t.Fatal("milestone_progress missing task")
			}
		}
	}
	if progressCount < 2 {
		t.Fatalf("expected >=2 milestone_progress events, got %d", progressCount)
	}
}

// Covers: L5-CTX-24
func TestIntegration_PlanDisabledUsesV2Path(t *testing.T) {
	ctxCfg := config.DefaultContextEngineConfig()
	ctxCfg.Plan.Enabled = false

	engine := contextengine.NewContextEngine(contextengine.EngineDeps{
		LLM:        &mockctx.LLMGateway{Response: "hello without plan"},
		Tools:      &mockctx.ToolRunner{},
		ToolsReg:   registry.NewBuiltinRegistry(),
		Permission: mockctx.AllowAllPermission{},
		Config:     ctxCfg,
	})

	session := &types.Session{SessionID: "sess_v2", WorkDir: t.TempDir(), Model: "mock"}
	ch := engine.Process(context.Background(), session, "/plan should be ignored")

	deadline := time.After(2 * time.Second)
	var gotText bool
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				if !gotText {
					t.Fatal("expected text event from V2 path")
				}
				return
			}
			if ev.Type == "text" {
				gotText = true
			}
			if ev.Type == "milestone_progress" {
				t.Fatal("plan disabled should not emit milestone_progress")
			}
		case <-deadline:
			t.Fatal("timeout waiting for V2 response")
		}
	}
}
