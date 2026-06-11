package contextengine_test

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine"
	"github.com/devrix/devrix/internal/shared/types"
)

// Covers: L5-CTX-37
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
