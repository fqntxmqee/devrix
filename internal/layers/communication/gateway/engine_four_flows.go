package gateway

import (
	"context"
	"log/slog"

	"github.com/devrix/devrix/internal/shared/types"
)

// FourFlowEngine is a mock engine that emits all 4 flows for testing
type FourFlowEngine struct{}

// NewFourFlowEngine creates a new four-flow test engine
func NewFourFlowEngine() *FourFlowEngine {
	return &FourFlowEngine{}
}

// Process implements IContextEngine.Process and emits events for all 4 flows
// This engine emits all 4 flows in sequence for quick validation:
// 1. Event flow: thinking -> tool_call -> tool_result -> text
// 2. Task flow: milestone_progress
// 3. Info flow: info
// 4. Complete: complete
func (e *FourFlowEngine) Process(ctx context.Context, session *types.Session, message string) <-chan *EngineEvent {
	slog.Info("fourflow: Process called", "sessionID", session.SessionID, "message", message)
	ch := make(chan *EngineEvent, 20)

	go func() {
		defer close(ch)

		// === 事件流: thinking (独立卡片) ===
		ch <- &EngineEvent{
			Type:      "thinking",
			SessionID: session.SessionID,
			Content:   "用户反馈还是没有及时确认的图标。我先检查 OK 确认逻辑是否已实现，代码在 onMessage 里通过 goroutine 异步发送。",
		}
		ch <- &EngineEvent{
			Type:      "thinking",
			SessionID: session.SessionID,
			Content:   "找到了第 619 行的 OK 确认逻辑，接下来读取完整上下文，确认它是否在正确位置。",
		}

		// === 事件流: tool call #1 ===
		ch <- &EngineEvent{
			Type:      "tool_call",
			SessionID: session.SessionID,
			ToolName:  "Grep",
			Metadata: map[string]string{
				"tool_name":     "Grep",
				"risk_level":    "LOW",
				"input":         "SendMessage.*👍|OK.*确认",
				"auto_approved": "true",
			},
		}
		ch <- &EngineEvent{
			Type:      "tool_result",
			SessionID: session.SessionID,
			Content:   "feishu.go:619: SendMessage(ctx, sessionKey, \"👍 OK 确认\")",
			ToolName:  "Grep",
		}

		// === 事件流: tool call #2 ===
		ch <- &EngineEvent{
			Type:      "tool_call",
			SessionID: session.SessionID,
			ToolName:  "Read",
			Metadata: map[string]string{
				"tool_name":     "Read",
				"risk_level":    "LOW",
				"input":         "/Users/fukai/workspace/devrix/internal/layers/communication/adapters/feishu.go",
				"auto_approved": "true",
			},
		}
		ch <- &EngineEvent{
			Type:      "tool_result",
			SessionID: session.SessionID,
			Content:   "func (a *FeishuAdapter) onMessage(...) {\n    // OK 确认逻辑\n    go a.SendMessage(ctx, sessionKey, \"👍\")\n}",
			ToolName:  "Read",
		}

		// === 任务流: milestone progress (任务进度卡) ===
		ch <- &EngineEvent{
			Type:      "milestone_progress",
			SessionID: session.SessionID,
			Metadata: map[string]string{
				"milestone_id": "m1",
				"progress":     "50%",
				"task":         "排查 OK 确认图标未显示",
			},
		}

		// === 事件流: streaming text (独立回复卡) ===
		parts := []string{
			"**分析结果**\n\n",
			"代码看起来是正确的，但图标未显示可能有三种原因：\n\n",
			"1. `onMessage` 未被调用（检查飞书消息日志）\n",
			"2. `SendMessage` 失败（检查错误日志）\n",
			"3. `sessionKey` 格式问题（消息发到了错误位置）\n\n",
			"建议先检查日志输出位置。",
		}

		for _, part := range parts {
			ch <- &EngineEvent{
				Type:      "text",
				SessionID: session.SessionID,
				Content:   part,
				Metadata:  map[string]string{"is_complete": "false"},
			}
		}

		// === 信息流: info (写入任务进度卡小结) ===
		ch <- &EngineEvent{
			Type:      "info",
			SessionID: session.SessionID,
			Content:   "代码逻辑正确，需排查运行时日志",
		}
		ch <- &EngineEvent{
			Type:      "info",
			SessionID: session.SessionID,
			Content:   "响应生成完毕 | 用时: 2.5s | 消耗: 1000 tokens",
		}

		// === 事件流: complete ===
		ch <- &EngineEvent{
			Type:      "complete",
			SessionID: session.SessionID,
			Metadata: map[string]string{
				"usage":    "1000 tokens",
				"duration": "2.5s",
			},
		}
	}()

	return ch
}
