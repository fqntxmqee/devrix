package contextengine

import (
	"testing"

	"github.com/devrix/devrix/internal/shared/types"
)

func TestContextAssembler_assembleContext(t *testing.T) {
	ca := NewContextAssembler()

	// 模拟第一轮对话后的状态
	history := []types.Message{
		{Role: types.MessageRoleUser, Content: "帮我写一个 hello world"},
		{Role: types.MessageRoleAssistant, Content: "好的，我来帮你写"},
	}

	tools := []ToolSchema{
		{Name: "write_file", Description: "写文件"},
	}

	// 模拟工具调用
	toolCallHistory := []types.ToolCallRecord{
		{ToolName: "write_file", Input: `{"path":"hello.go","content":"package main\nfunc main(){ println("Hello") }"}`, Output: "File written successfully"},
	}

	req := ca.AssembleContext(
		"You are a helpful assistant",
		tools,
		history,
		"请执行",
		toolCallHistory,
	)

	// 验证
	if len(req.Messages) != 4 {
		t.Errorf("expected 4 messages, got %d", len(req.Messages))
	}

	// 验证工具调用消息
	if req.Messages[2].Role != types.MessageRoleAssistant {
		t.Errorf("expected role assistant at index 2, got %s", req.Messages[2].Role)
	}
	if _, ok := req.Messages[2].Metadata["tool_calls"]; !ok {
		t.Errorf("expected tool_calls in metadata")
	}

	// 验证工具结果消息
	if req.Messages[3].Role != types.MessageRoleTool {
		t.Errorf("expected role tool at index 3, got %s", req.Messages[3].Role)
	}
	if req.Messages[3].Content != "File written successfully" {
		t.Errorf("expected tool result content, got %s", req.Messages[3].Content)
	}

	t.Log("Context assembly test passed!")
	t.Logf("Messages: %d", len(req.Messages))
	for i, m := range req.Messages {
		t.Logf("  [%d] %s: %s", i, m.Role, m.Content[:min(50, len(m.Content))])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
