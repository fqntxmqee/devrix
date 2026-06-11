package contextengine_test

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine"
	"github.com/devrix/devrix/internal/shared/types"
)

// Covers: L5-CTX-37
func TestWriteFile_yolo_allows_non_plan_path_in_plan_mode(t *testing.T) {
	reg, err := contextengine.NewBuiltinToolRegistry(nil)
	if err != nil {
		t.Fatalf("NewBuiltinToolRegistry: %v", err)
	}
	workDir := t.TempDir()
	sc := &types.SessionContext{
		SessionID:      "sess_plan_yolo",
		WorkDir:        workDir,
		PermissionMode: types.PermissionPlan,
		PlanFilePath:   workDir + "/plan.md",
	}
	ctx := contextengine.WithFilesAutoApproved(
		contextengine.ToolContext(context.Background(), sc),
		true,
	)
	res, err := reg.Execute(ctx, contextengine.ToolCall{
		Name:  "write_file",
		Input: `{"path":"other.md","content":"ok"}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Error != "" {
		t.Fatalf("unexpected error: %s", res.Error)
	}
}
