// Package notify — G3 后台任务完成通知总线，对标 clawcode TaskOutputTool 的
// <task-notification> 行为。
//
// 工作流：
//   - workmodel.TaskManager 完成任务时调用 bus.Publish
//   - task_output tool 监听 Subscribe channel 提前返回（不等 sleep 轮询）
//   - 下回合 prepareTurn 阶段，剩余未消费 event 附加到 system reminder
package notify

import (
	"strings"
	"sync"
	"time"
)

// CompletionEvent 单条任务完成事件。
type CompletionEvent struct {
	TaskID    string        `json:"task_id"`
	Kind      string        `json:"kind"` // "bash" | "agent" | "remote"
	ExitCode  int           `json:"exit_code,omitempty"`
	Duration  time.Duration `json:"duration_ns"`
	TailLines []string      `json:"tail_lines,omitempty"`
	Error     string        `json:"error,omitempty"`
	// Summary 任务简短摘要，由发布方填写，consumer 直接显示。
	Summary string `json:"summary,omitempty"`
	Time    time.Time `json:"time"`
}

// Bus 通知总线接口。
type Bus interface {
	Publish(sessionID string, evt CompletionEvent)
	Subscribe(sessionID string) <-chan CompletionEvent
	// Drain 一次性读出 sessionID 的全部未消费 event，consumer 在 prepareTurn 阶段调用。
	Drain(sessionID string) []CompletionEvent
	// Len 未消费 event 数。
	Len(sessionID string) int
	// Close 关闭 session 关联的 channel。
	Close(sessionID string)
}

// InMemoryBus 默认实现：每 session 一个 buffered channel + 累积 list。
type InMemoryBus struct {
	mu      sync.Mutex
	chans   map[string]chan CompletionEvent
	pending map[string][]CompletionEvent
	bufSize int
}

// NewInMemoryBus 构造 bus，bufSize 单 channel 缓冲（避免阻塞 producer）。
func NewInMemoryBus(bufSize int) *InMemoryBus {
	if bufSize <= 0 {
		bufSize = 64
	}
	return &InMemoryBus{
		chans:   make(map[string]chan CompletionEvent),
		pending: make(map[string][]CompletionEvent),
		bufSize: bufSize,
	}
}

func (b *InMemoryBus) channel(sessionID string) chan CompletionEvent {
	b.mu.Lock()
	defer b.mu.Unlock()
	ch, ok := b.chans[sessionID]
	if !ok {
		ch = make(chan CompletionEvent, b.bufSize)
		b.chans[sessionID] = ch
	}
	return ch
}

// Publish 发送一条 event；channel 满时降级到 pending list。
func (b *InMemoryBus) Publish(sessionID string, evt CompletionEvent) {
	if evt.Time.IsZero() {
		evt.Time = time.Now()
	}
	ch := b.channel(sessionID)
	select {
	case ch <- evt:
	default:
		b.mu.Lock()
		b.pending[sessionID] = append(b.pending[sessionID], evt)
		b.mu.Unlock()
	}
}

// Subscribe 拿消费 channel（惰性创建）。
func (b *InMemoryBus) Subscribe(sessionID string) <-chan CompletionEvent {
	return b.channel(sessionID)
}

// Drain 读出 session 全部未消费 event。
func (b *InMemoryBus) Drain(sessionID string) []CompletionEvent {
	b.mu.Lock()
	pending := b.pending[sessionID]
	delete(b.pending, sessionID)
	b.mu.Unlock()

	ch := b.channel(sessionID)
	out := make([]CompletionEvent, 0, len(pending)+len(ch))
	out = append(out, pending...)
drain:
	for {
		select {
		case e, ok := <-ch:
			if !ok {
				break drain
			}
			out = append(out, e)
		default:
			break drain
		}
	}
	return out
}

// Len 返回 session 未消费 event 数。
func (b *InMemoryBus) Len(sessionID string) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.pending[sessionID]) + len(b.chans[sessionID])
}

// Close 关闭 sessionID 的 channel，consumer 收到零值。
func (b *InMemoryBus) Close(sessionID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if ch, ok := b.chans[sessionID]; ok {
		close(ch)
		delete(b.chans, sessionID)
	}
}

// FormatReminder 把 events 列表渲染为 LLM 可消费的 <task-notification> block。
func FormatReminder(events []CompletionEvent) string {
	if len(events) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("<task_notifications>\n")
	for _, e := range events {
		// 单行：time kind task_id summary
		b.WriteString("  - [")
		b.WriteString(e.Time.Format("15:04:05"))
		b.WriteString("] ")
		b.WriteString(e.Kind)
		b.WriteString(" ")
		b.WriteString(e.TaskID)
		if e.Summary != "" {
			b.WriteString(": ")
			b.WriteString(e.Summary)
		} else if e.Error != "" {
			b.WriteString(" ERROR: ")
			b.WriteString(e.Error)
		}
		b.WriteString("\n")
	}
	b.WriteString("</task_notifications>\n")
	return b.String()
}
