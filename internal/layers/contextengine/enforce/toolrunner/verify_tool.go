package toolrunner

// W6 — D6-S11-A02 (alias G4) verify_plan_execution LLM tool wiring。
//
// AC4:
//   - LLM 调用 verify_plan_execution tool → 返回 verified/unverified/skipped JSON
//   - tasks.md 引用不存在的文件 → unverified 含 reason="file not found"
//
// input 格式 (JSON):
//   {"change_id": "devrix-diagnostic-tools-wiring", "repo_root": "/abs/path"}
// change_id 必填；repo_root 缺省时使用 input 解析时的工作目录。

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/devrix/devrix/internal/layers/evolution/verify"
	"github.com/devrix/devrix/internal/shared/types"
)

// verifyRunner 实现 PluginRunner 接口：包装 evolution/verify.FileVerifier。
type verifyRunner struct{}

func (v *verifyRunner) Name() string { return "verify_plan_execution" }

func (v *verifyRunner) RiskLevel() types.RiskLevel { return types.RiskLevelLow }

func (v *verifyRunner) Schema() ToolSchema {
	return ToolSchema{
		Name: "verify_plan_execution",
		Description: "Verify that all done items in the change's tasks.md have their evidence files present and (for _test.go) contain a func TestXxx(). " +
			"Returns a Report JSON with verified/unverified/skipped counts.",
		Parameters: `{"change_id": "<change-id>", "repo_root": "<optional abs path>"}`,
	}
}

// verifyInput 是 LLM 传入 JSON 的解析结构。
type verifyInput struct {
	ChangeID string `json:"change_id"`
	RepoRoot string `json:"repo_root"`
}

func (v *verifyRunner) Execute(ctx context.Context, workDir, input string) (*ToolResult, error) {
	var in verifyInput
	if input != "" {
		if err := json.Unmarshal([]byte(input), &in); err != nil {
			return &ToolResult{Error: fmt.Sprintf("verify_plan_execution: invalid input JSON: %s", err.Error())}, nil
		}
	}
	if in.ChangeID == "" {
		return &ToolResult{Error: "verify_plan_execution: change_id is required"}, nil
	}
	repoRoot := in.RepoRoot
	if repoRoot == "" {
		repoRoot = workDir
	}
	taskFile := filepath.Join(repoRoot, "openspec", "changes", in.ChangeID, "tasks.md")
	fv := verify.NewFileVerifier()
	items, err := fv.LoadPlan(taskFile)
	if err != nil {
		return &ToolResult{Error: fmt.Sprintf("verify_plan_execution: load plan: %s", err.Error())}, nil
	}
	report, err := fv.Verify(ctx, items, repoRoot)
	if err != nil {
		return &ToolResult{Error: fmt.Sprintf("verify_plan_execution: verify: %s", err.Error())}, nil
	}
	report.ChangeID = in.ChangeID // file verifier doesn't thread through ChangeID; set explicitly
	out, err := verify.FormatJSON(report)
	if err != nil {
		return &ToolResult{Error: fmt.Sprintf("verify_plan_execution: format: %s", err.Error())}, nil
	}
	return &ToolResult{Output: string(out)}, nil
}

// summarizeOutput 把 report 渲染成人类可读单行：用于 tool output 的 fallback。
func summarizeOutput(report verify.Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "verify-plan: total=%d verified=%d skipped=%d unverified=%d",
		report.Total, report.Verified, report.Skipped, len(report.Unverified))
	return b.String()
}
