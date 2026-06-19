// Package windowanalyzer — A5 上下文窗口分析,对标 clawcode context analyze。
//
// 接收一个 session 的 message history,按类别(系统/工具/消息/思考/提醒)拆 token 用量,
// 输出 Breakdown + ASCII bar chart,供 devrix context analyze CLI 使用。
//
// 设计参考:openspec/changes/devrix-diagnostic-tools-parity/design.md §2.13
package windowanalyzer

import (
	"fmt"
	"sort"
	"strings"

	"github.com/devrix/devrix/internal/layers/contextengine/prepare/token"
	"github.com/devrix/devrix/internal/shared/types"
)

// Breakdown 按类别拆分的 token 数。
type Breakdown struct {
	System    int `json:"system"`
	Tools     int `json:"tools"`
	Messages  int `json:"messages"`
	Thinking  int `json:"thinking"`
	Reminders int `json:"reminders"`
	Total     int `json:"total"`
}

// Category 类别标签。
type Category string

const (
	CatSystem    Category = "system"
	CatTools     Category = "tools"
	CatMessages  Category = "messages"
	CatThinking  Category = "thinking"
	CatReminders Category = "reminders"
)

// MessageView 简化 message,带分类标签。
type MessageView struct {
	Role     string
	Content  string
	Category Category
}

// Analyzer 分析器接口。
type Analyzer interface {
	Analyze(history []MessageView) Breakdown
}

// TokenAnalyzer 默认实现,使用 token.Counter 启发式计数。
type TokenAnalyzer struct {
	counter *token.Counter
}

// NewTokenAnalyzer 构造 analyzer。
func NewTokenAnalyzer() *TokenAnalyzer {
	return &TokenAnalyzer{counter: token.NewCounter()}
}

// Analyze 累计各 Category token 数。
func (a *TokenAnalyzer) Analyze(history []MessageView) Breakdown {
	var b Breakdown
	for _, m := range history {
		t := a.counter.CountText(m.Content)
		switch m.Category {
		case CatSystem:
			b.System += t + 4
		case CatTools:
			b.Tools += t + 4
		case CatThinking:
			b.Thinking += t + 4
		case CatReminders:
			b.Reminders += t + 4
		default: // 包括空 Category 和 messages
			b.Messages += t + 4
		}
	}
	b.Total = b.System + b.Tools + b.Messages + b.Thinking + b.Reminders
	return b
}

// AnalyzeMessages 便捷方法:基于 types.Message + 启发式 role 分类。
//   - role=system → CatSystem
//   - role=tool → CatTools
//   - 含 "thinking" 关键字的 content → CatThinking
//   - 含 "<reminder>" tag → CatReminders
//   - 其他 → CatMessages
func (a *TokenAnalyzer) AnalyzeMessages(msgs []types.Message) Breakdown {
	views := make([]MessageView, len(msgs))
	for i, m := range msgs {
		cat := CatMessages
		switch m.Role {
		case types.MessageRoleSystem:
			cat = CatSystem
		case types.MessageRoleTool:
			cat = CatTools
		}
		body := m.Content
		switch {
		case strings.Contains(body, "<thinking>"):
			cat = CatThinking
		case strings.Contains(body, "<reminder>") || strings.Contains(body, "<task_notifications>"):
			cat = CatReminders
		}
		views[i] = MessageView{Role: string(m.Role), Content: body, Category: cat}
	}
	return a.Analyze(views)
}

// FormatTable 渲染 Breakdown 为 ASCII 表格 + 进度条。
func FormatTable(b Breakdown) string {
	var s strings.Builder
	fmt.Fprintln(&s, "Context Window Breakdown")
	fmt.Fprintln(&s, "========================")
	rows := []struct {
		name string
		val  int
	}{
		{"system", b.System},
		{"messages", b.Messages},
		{"tools", b.Tools},
		{"thinking", b.Thinking},
		{"reminders", b.Reminders},
	}
	// 数值降序
	sort.Slice(rows, func(i, j int) bool { return rows[i].val > rows[j].val })
	maxVal := 0
	for _, r := range rows {
		if r.val > maxVal {
			maxVal = r.val
		}
	}
	const barWidth = 30
	for _, r := range rows {
		pct := 0.0
		if b.Total > 0 {
			pct = float64(r.val) / float64(b.Total) * 100
		}
		filled := 0
		if maxVal > 0 {
			filled = int(float64(r.val) / float64(maxVal) * float64(barWidth))
		}
		bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
		fmt.Fprintf(&s, "  %-10s %6d tok  %5.1f%%  %s\n", r.name, r.val, pct, bar)
	}
	fmt.Fprintf(&s, "  --------\n  %-10s %6d tok  100.0%%\n", "total", b.Total)
	return s.String()
}
