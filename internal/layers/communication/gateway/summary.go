package gateway

import (
	"fmt"
	"strconv"
	"strings"
)

// formatDuration 将毫秒数格式化为人类可读时间。
//   - durationMs <= 0       → "0s"
//   - < 60 秒                → "{s}s"（例：7655ms → "8s"）
//   - >= 60 秒且余秒为 0     → "{m}m"（例：120000ms → "2m"）
//   - >= 60 秒且余秒非 0     → "{m}m{s}s"（例：122000ms → "2m2s"）
func formatDuration(durationMs int64) string {
	if durationMs <= 0 {
		return "0s"
	}
	seconds := (durationMs + 500) / 1000
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	m := seconds / 60
	s := seconds % 60
	if s == 0 {
		return fmt.Sprintf("%dm", m)
	}
	return fmt.Sprintf("%dm%ds", m, s)
}

// formatTokens 将 token 数格式化为带万分符的人类可读字符串。
//   - < 10000     → "{n} tokens"（例：1500 → "1500 tokens"）
//   - >= 10000    → "{x.x}w tokens"（例：22000 → "2.2w tokens"）
func formatTokens(tokens int64) string {
	if tokens < 0 {
		tokens = 0
	}
	if tokens < 10000 {
		return fmt.Sprintf("%d tokens", tokens)
	}
	w := float64(tokens) / 10000.0
	return fmt.Sprintf("%.1fw tokens", w)
}

// parseInt64Safe 尝试解析字符串为 int64，失败时返回 0。
func parseInt64Safe(s string) int64 {
	if s == "" {
		return 0
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// ComputeCtxPct 计算当前 PEV run 最后一次 LLM 调用的 prompt tokens
// 占上下文窗口的百分比（0-100）。当 maxContextTokens 或 promptTokens
// 任一为 0 时返回 0；超限 clamp 至 100。
//
// DM-20260611-008：与 D1 摘要的 "ctx: X%" 段口径一致。D2 PEV/QueryLoop
// 在 emit complete 时也用此函数计算 ctx_pct metadata，保证两侧一致。
func ComputeCtxPct(promptTokens, maxContextTokens int) int {
	if maxContextTokens <= 0 || promptTokens <= 0 {
		return 0
	}
	pct := promptTokens * 100 / maxContextTokens
	if pct > 100 {
		pct = 100
	}
	return pct
}

// buildCompletionSummary 拼装"任务完成"卡片的摘要字符串。
//
// 段顺序：duration → usage → ctx(可选) → model(可选)
//
// 示例：
//   - 含模型 + 含 ctx: "用时: 2m2s, 消耗: 2.2w tokens, ctx: 12%, 模型: claude-sonnet-4-6"
//   - 含模型 + 无 ctx: "用时: 8s, 消耗: 1500 tokens, 模型: claude-sonnet-4-6"
//   - 无模型 + 含 ctx: "用时: 8s, 消耗: 1500 tokens, ctx: 12%"
//   - 无模型 + 无 ctx: "用时: 8s, 消耗: 1500 tokens"
//
// ctxPctStr 取值约定：
//   - "" / "0" / 不可解析 → 省略 ctx 段
//   - 其它数字           → 渲染 "ctx: N%"
func buildCompletionSummary(durationStr, usageStr, model, ctxPctStr string) string {
	dur := formatDuration(parseInt64Safe(durationStr))
	usage := formatTokens(parseInt64Safe(usageStr))
	ctxPct := parseInt64Safe(ctxPctStr)
	parts := []string{fmt.Sprintf("用时: %s", dur), fmt.Sprintf("消耗: %s", usage)}
	if ctxPct > 0 {
		parts = append(parts, fmt.Sprintf("ctx: %d%%", ctxPct))
	}
	if model != "" {
		parts = append(parts, fmt.Sprintf("模型: %s", model))
	}
	return strings.Join(parts, ", ")
}
