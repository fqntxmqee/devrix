package conclusion

import (
	"fmt"
	"strconv"
	"strings"
)

const usageSplitSep = "/"

// BuildCompletionSummary assembles the terminal completion card summary (S16-A02-F).
func BuildCompletionSummary(durationStr, usageStr, model, ctxPctStr string) string {
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
