package toolrunner

// W8 — D5-S23-A02 (alias G6) query_diagnostics LLM tool wiring。
//
// AC6:
//   - LLM 调用 query_diagnostics tool → 返回最近累积的 diagnostic 列表
//   - 无 tick 时（recent 为空）→ 返回 empty 标记
//   - tracker 未注入 → 拒绝
//
// input 格式 (JSON, 可空):
//   {"limit": 50, "file": "optional", "severity": "error|warning|info"}
// limit 缺省 50, max 256;file 过滤只返回该文件的 diagnostic;
// severity 过滤指定严重级。

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/devrix/devrix/internal/layers/observability/diagnose/tracker"
	"github.com/devrix/devrix/internal/shared/types"
)

// queryDiagLimitCap 单次 query 最多返回的 diagnostic 数（与 recent buffer 上限对齐）。
const queryDiagLimitCap = 256

// trackerRunner 实现 PluginRunner 接口：包装 tracker.GlobalTracker 的 Recent 累积。
type trackerRunner struct{}

func (t *trackerRunner) Name() string { return "query_diagnostics" }

func (t *trackerRunner) RiskLevel() types.RiskLevel { return types.RiskLevelLow }

func (t *trackerRunner) Schema() ToolSchema {
	return ToolSchema{
		Name: "query_diagnostics",
		Description: "Query the recent file-diagnostic buffer maintained by the periodic linter tick. " +
			"Returns up to `limit` (default 50) diagnostics with file/line/severity/source/message. " +
			"Optional `file` and `severity` filters narrow the result.",
		Parameters: `{"limit": 50, "file": "<optional>", "severity": "<error|warning|info>"}`,
	}
}

type queryDiagInput struct {
	Limit    int    `json:"limit"`
	File     string `json:"file"`
	Severity string `json:"severity"`
}

type queryDiagOutput struct {
	Count       int                   `json:"count"`
	TotalInBuf  int                   `json:"total_in_buffer"`
	Diagnostics []tracker.Diagnostic  `json:"diagnostics"`
}

func (t *trackerRunner) Execute(ctx context.Context, _ /*workDir*/, input string) (*ToolResult, error) {
	tr := tracker.GlobalTracker()
	if tr == nil {
		return &ToolResult{Error: "query_diagnostics: global tracker not initialized"}, nil
	}
	var in queryDiagInput
	if input != "" {
		if err := json.Unmarshal([]byte(input), &in); err != nil {
			return &ToolResult{Error: fmt.Sprintf("query_diagnostics: invalid input JSON: %s", err.Error())}, nil
		}
	}
	limit := in.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > queryDiagLimitCap {
		limit = queryDiagLimitCap
	}
	all := tr.Recent()
	out := queryDiagOutput{
		TotalInBuf: len(all),
		Diagnostics: make([]tracker.Diagnostic, 0, limit),
	}
	for _, d := range all {
		if in.File != "" && d.File != in.File {
			continue
		}
		if in.Severity != "" && d.Severity != in.Severity {
			continue
		}
		if len(out.Diagnostics) < limit {
			out.Diagnostics = append(out.Diagnostics, d)
		}
	}
	out.Count = len(out.Diagnostics)
	bz, _ := json.Marshal(out)
	return &ToolResult{Output: string(bz)}, nil
}
