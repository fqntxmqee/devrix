package contextengine

import (
	"github.com/devrix/devrix/internal/shared/types"
)

// ContextAssembler 结构化地组装 LLM 输入上下文
// 业界最佳实践：将上下文分为 System、Tools、History、Current 四层
type ContextAssembler struct{}

// NewContextAssembler creates a new context assembler
func NewContextAssembler() *ContextAssembler {
	return &ContextAssembler{}
}

// ContextLayers 定义 LLM 输入的层次结构
type ContextLayers struct {
	SystemPrompt string        // 系统提示层
	Tools        []ToolSchema // 工具定义层
	History      []types.Message // 历史消息层
	CurrentTurn  *CurrentTurnInput // 当前轮次输入
}

// CurrentTurnInput 当前轮次的输入
type CurrentTurnInput struct {
	UserMessage   string                // 用户消息
	ToolCalls    []types.ToolCallRecord // 本轮工具调用（如果有）
	ToolResults  []types.ToolCallRecord // 本轮工具结果（如果有）
}

// AssembleContext 组装完整的 LLM 输入上下文
// 这是业界最佳实践的上下文组装方式
func (ca *ContextAssembler) AssembleContext(
	systemPrompt string,
	tools []ToolSchema,
	history []types.Message,
	userMessage string,
	toolCallHistory []types.ToolCallRecord,
) *LLMRequest {
	// 构建历史消息，附加本轮的工具调用
	messages := ca.buildMessageHistory(history, toolCallHistory)
	
	return &LLMRequest{
		Model:        "", // 由调用方设置
		SystemPrompt: systemPrompt,
		Messages:     messages,
		Tools:        tools,
	}
}

// buildMessageHistory 构建消息历史，插入工具调用和结果
func (ca *ContextAssembler) buildMessageHistory(
	history []types.Message,
	toolCallHistory []types.ToolCallRecord,
) []types.Message {
	if len(history) == 0 && len(toolCallHistory) == 0 {
		return nil
	}
	
	// 复制历史消息
	result := make([]types.Message, 0, len(history)+len(toolCallHistory)*2)
	result = append(result, history...)
	
	// 追加本轮的工具调用和结果
	for _, tc := range toolCallHistory {
		// 添加 assistant 的工具调用消息
		tcJSON := ca.formatToolCall(tc)
		result = append(result, types.Message{
			Role:     types.MessageRoleAssistant,
			Content:  "",
			Metadata: map[string]string{"tool_calls": tcJSON},
		})
		
		// 添加 tool 的结果消息
		content := tc.Output
		if tc.Error != "" {
			content = "Error: " + tc.Error
		}
		result = append(result, types.Message{
			Role:     types.MessageRoleTool,
			Content:  content,
			Metadata: map[string]string{"tool_call_id": ca.toolCallID(tc)},
		})
	}
	
	return result
}

// formatToolCall 将 ToolCallRecord 格式化为 JSON
func (ca *ContextAssembler) formatToolCall(tc types.ToolCallRecord) string {
	// 使用与 OpenAI tool_calls 相同的格式
	return ` [{"type":"function","function":{"name":"` + tc.ToolName + `","arguments":"` + escapeJSON(tc.Input) + `"}}]`
}

// toolCallID 生成工具调用的 ID
func (ca *ContextAssembler) toolCallID(tc types.ToolCallRecord) string {
	return "call_" + tc.ToolName
}

// escapeJSON 转义 JSON 字符串
func escapeJSON(s string) string {
	// 简单的 JSON 转义
	result := make([]byte, 0, len(s)*2)
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"':
			result = append(result, '\\', '"')
		case '\\':
			result = append(result, '\\', '\\')
		case '\n':
			result = append(result, '\\', 'n')
		case '\r':
			result = append(result, '\\', 'r')
		case '\t':
			result = append(result, '\\', 't')
		default:
			result = append(result, s[i])
		}
	}
	return string(result)
}

// FormatContextForLogging 格式化上下文用于日志输出（人类可读）
func (ca *ContextAssembler) FormatContextForLogging(req *LLMRequest) string {
	summary := "=== LLM Input Context ===\n"
	summary += "System Prompt: " + truncate(req.SystemPrompt, 200) + "\n\n"
	summary += "Tools (" + itoa(len(req.Tools)) + "): "
	for _, t := range req.Tools {
		summary += t.Name + ", "
	}
	summary += "\n\n"
	summary += "Messages (" + itoa(len(req.Messages)) + "):\n"
	for i, m := range req.Messages {
		role := string(m.Role)
		content := m.Content
		if len(m.Metadata) > 0 {
			if tc, ok := m.Metadata["tool_calls"]; ok {
				summary += itoa(i) + ". [assistant] [tool_calls]: " + truncate(tc, 100) + "\n"
				continue
			}
			if tcID, ok := m.Metadata["tool_call_id"]; ok {
				summary += itoa(i) + ". [tool:" + tcID + "] " + truncate(content, 200) + "\n"
				continue
			}
		}
		summary += itoa(i) + ". [" + role + "] " + truncate(content, 300) + "\n"
	}
	return summary
}


// itoa 将 int 转换为 string
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	if i < 0 {
		return "-" + itoa(-i)
	}
	result := ""
	for i > 0 {
		result = string('0'+byte(i%10)) + result
		i /= 10
	}
	return result
}
