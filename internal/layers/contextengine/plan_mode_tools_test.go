package contextengine_test

import (
	"context"
	"os"
	"strings"
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

// T: D2-S18-A80-T01 (DM-20260630-013 RH-D2-01)
// edit_file must enforce the same plan-mode write gate as write_file.
// Without parity, plan mode could rewrite non-plan files via targeted edits.
func TestEditFile_plan_mode_denies_non_plan_path(t *testing.T) {
	workDir := t.TempDir()
	planPath := workDir + "/plan.md"
	reg, err := contextengine.NewBuiltinToolRegistry(nil)
	if err != nil {
		t.Fatalf("NewBuiltinToolRegistry: %v", err)
	}
	sc := &types.SessionContext{
		SessionID:      "sess_plan_edit",
		WorkDir:        workDir,
		PermissionMode: types.PermissionPlan,
		PlanFilePath:   planPath,
	}
	ctx := contextengine.ToolContext(context.Background(), sc)
	res, err := reg.Execute(ctx, contextengine.ToolCall{
		Name:  "edit_file",
		Input: `{"file_path":"other.md","old_string":"x","new_string":"y"}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Error == "" {
		t.Fatal("expected plan mode edit_file denial")
	}
	if !strings.Contains(res.Error, "plan mode") {
		t.Fatalf("expected plan mode error, got: %q", res.Error)
	}
}

// T: D2-S18-A80-T02 (DM-20260630-013 RH-D2-01)
// edit_file to the plan file itself must remain allowed in plan mode.
func TestEditFile_plan_mode_allows_plan_path(t *testing.T) {
	workDir := t.TempDir()
	planPath := workDir + "/plan.md"
	if err := os.WriteFile(planPath, []byte("# old body\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	reg, err := contextengine.NewBuiltinToolRegistry(nil)
	if err != nil {
		t.Fatalf("NewBuiltinToolRegistry: %v", err)
	}
	sc := &types.SessionContext{
		SessionID:      "sess_plan_edit_ok",
		WorkDir:        workDir,
		PermissionMode: types.PermissionPlan,
		PlanFilePath:   planPath,
	}
	ctx := contextengine.ToolContext(context.Background(), sc)
	res, err := reg.Execute(ctx, contextengine.ToolCall{
		Name:  "edit_file",
		Input: `{"file_path":"plan.md","old_string":"# old body","new_string":"# new body"}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Error != "" {
		t.Fatalf("expected edit to plan file to succeed, got error: %q", res.Error)
	}
	data, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "# new body") {
		t.Fatalf("plan file was not updated, contents: %q", string(data))
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
