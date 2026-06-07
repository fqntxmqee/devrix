package bootstrap

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/communication/milestone"
	"github.com/devrix/devrix/internal/layers/contextengine/memory"
	"github.com/devrix/devrix/internal/shared/config"
)

func TestWireContextV3_should_wire_planner_when_plan_enabled(t *testing.T) {
	ctxCfg := config.DefaultContextEngineConfig()
	ctxCfg.Plan.Enabled = true
	msSvc := milestone.NewMilestoneService(nil)

	planner, longTerm := WireContextV3(ctxCfg, msSvc)
	if planner == nil {
		t.Fatal("expected planner when plan.enabled=true")
	}
	if longTerm == nil {
		t.Fatal("expected long-term memory instance")
	}
}

func TestWireContextV3_should_skip_planner_when_plan_disabled(t *testing.T) {
	ctxCfg := config.DefaultContextEngineConfig()
	ctxCfg.Plan.Enabled = false
	ctxCfg.LongTerm.Enabled = false

	planner, longTerm := WireContextV3(ctxCfg, milestone.NewMilestoneService(nil))
	if planner != nil {
		t.Fatal("expected nil planner when plan disabled")
	}
	if _, ok := longTerm.(*memory.DisabledLongTermMemory); !ok {
		t.Fatalf("expected disabled long-term memory, got %T", longTerm)
	}
}
