package gateway

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/devrix/devrix/internal/shared/contracts"
)

const usageSplitSep = "/"

// ComputeCtxPct is a thin shim kept for backward compatibility with any caller
// that still imports the function from D1. The canonical implementation lives
// in shared/contracts (L5-0-0-02: cross-layer contract surface must be free of
// D{N}→D{N} imports). D2 PEV/QueryLoop has been migrated to call contracts.ComputeCtxPct
// directly. New code should import the shared helper.
func ComputeCtxPct(promptTokens, maxContextTokens int) int {
	return contracts.ComputeCtxPct(promptTokens, maxContextTokens)
}

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

// parseDurationMs accepts canonical millisecond strings and legacy second floats.
func parseDurationMs(raw string) int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	if ms := parseInt64Safe(raw); ms > 0 || raw == "0" {
		return ms
	}
	if sec, err := strconv.ParseFloat(raw, 64); err == nil && sec >= 0 {
		return int64(sec*1000 + 0.5)
	}
	return 0
}

// parseUsageTokens accepts total token count or legacy "prompt/completion" pairs.
func parseUsageTokens(raw string) int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	if strings.Contains(raw, usageSplitSep) {
		parts := strings.SplitN(raw, usageSplitSep, 2)
		return parseInt64Safe(parts[0]) + parseInt64Safe(parts[1])
	}
	return parseInt64Safe(raw)
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
	dur := formatDuration(parseDurationMs(durationStr))
	usage := formatTokens(parseUsageTokens(usageStr))
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
