package contextengine_test

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

// T: D2-S10-A01-T37
func TestWriteFile_plan_mode_denies_non_plan_path(t *testing.T) {
	reg, err := contextengine.NewBuiltinToolRegistry(nil)
	if err != nil {
		t.Fatalf("NewBuiltinToolRegistry: %v", err)
	}
	sc := &types.SessionContext{
		SessionID:      "sess_plan",
		WorkDir:        t.TempDir(),
		PermissionMode: types.PermissionPlan,
		PlanFilePath:   t.TempDir() + "/plan.md",
	}
	ctx := contextengine.ToolContext(context.Background(), sc)
	res, err := reg.Execute(ctx, contextengine.ToolCall{
		Name:  "write_file",
		Input: `{"path":"other.md","content":"x"}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Error == "" {
		t.Fatal("expected plan mode write denial")
	}
}

// T: D2-S18-A03 (RegisterPlanModeTools nil-arg defensive paths)
// Migrated from contextengine_test_helper_test.go during D2 root test
// cleanup (2026-06-19). Validates enforce.RegisterPlanModeTools short-circuits
// when registry or config is nil instead of dereferencing.
func TestRegisterPlanModeTools_NilRegistry(t *testing.T) {
	err := contextengine.RegisterPlanModeTools(nil, config.DefaultContextEngineConfig())
	if err != nil {
		t.Errorf("RegisterPlanModeTools nil reg: %v", err)
	}
}

func TestRegisterPlanModeTools_NilConfig(t *testing.T) {
	reg := contextengine.NewToolRegistry()
	err := contextengine.RegisterPlanModeTools(reg, nil)
	if err != nil {
		t.Errorf("RegisterPlanModeTools nil cfg: %v", err)
	}
}
