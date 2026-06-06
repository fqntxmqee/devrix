package renderers

import (
	"fmt"
	"strings"

	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

// CLRenderer renders CLI output with ANSI colors
type CLRenderer struct {
	ansi config.ANSIConfig
}

// NewCLIRenderer creates a new CLRenderer
func NewCLIRenderer(ansi config.ANSIConfig) *CLRenderer {
	return &CLRenderer{ansi: ansi}
}

// RenderMessage renders a message with appropriate coloring
func (r *CLRenderer) RenderMessage(msg *types.OutboundMessage) {
	var color string
	switch msg.Role {
	case types.MessageRoleUser:
		color = r.ansi.User
	case types.MessageRoleAssistant:
		color = r.ansi.Assistant
	default:
		color = ""
	}

	if color != "" {
		fmt.Printf("%s%s%s\n", color, msg.Content, r.ansi.Reset)
	} else {
		fmt.Printf("%s\n", msg.Content)
	}
}

// RenderStreamingMessage renders a streaming message (incremental)
func (r *CLRenderer) RenderStreamingMessage(content string, isComplete bool) {
	if isComplete {
		fmt.Printf("%s%s%s\n", r.ansi.Assistant, content, r.ansi.Reset)
	} else {
		// Clear line and overwrite
		fmt.Printf("\r%s%s%s", r.ansi.Assistant, content, r.ansi.Reset)
	}
}

// RenderError renders an error message
func (r *CLRenderer) RenderError(err error) {
	fmt.Printf("%sError: %s%s\n", r.ansi.Error, err.Error(), r.ansi.Reset)
}

// RenderStatus renders a status message
func (r *CLRenderer) RenderStatus(state types.SessionState) {
	var statusText string
	var color string

	switch state {
	case types.SessionStateThinking:
		statusText = "Thinking..."
		color = r.ansi.Warning
	case types.SessionStateStreaming:
		statusText = "Generating..."
		color = r.ansi.Assistant
	case types.SessionStateToolExecuting:
		statusText = "Executing tool..."
		color = r.ansi.Warning
	case types.SessionStateWaitingPermission:
		statusText = "Waiting for permission..."
		color = r.ansi.Warning
	case types.SessionStateCompleted:
		statusText = "Done"
		color = r.ansi.Assistant
	case types.SessionStateFailed:
		statusText = "Failed"
		color = r.ansi.Error
	default:
		statusText = "Idle"
		color = ""
	}

	if color != "" {
		fmt.Printf("%s[%s]%s ", color, statusText, r.ansi.Reset)
	} else {
		fmt.Printf("[%s] ", statusText)
	}
}

// RenderPermissionRequest renders a permission request card
func (r *CLRenderer) RenderPermissionRequest(req *types.PermissionRequest) {
	border := strings.Repeat("-", 50)
	riskColor := r.getRiskColor(req.RiskLevel)

	fmt.Println()
	fmt.Println(border)
	fmt.Printf("%s⚠️  Permission Required%s\n", riskColor, r.ansi.Reset)
	fmt.Println(border)
	fmt.Printf("Tool: %s\n", req.ToolName)
	if req.Description != "" {
		fmt.Printf("Description: %s\n", req.Description)
	}
	if req.InputPreview != "" {
		fmt.Printf("Input Preview:\n%s\n", indent(req.InputPreview, "  "))
	}
	fmt.Printf("Risk Level: %s%s%s\n", riskColor, req.RiskLevel, r.ansi.Reset)
	fmt.Println(border)
}

// RenderToolCall renders a tool call
func (r *CLRenderer) RenderToolCall(toolName string, args string) {
	fmt.Printf("%s🔧 Tool: %s%s\n", r.ansi.Warning, toolName, r.ansi.Reset)
	if args != "" {
		fmt.Printf("Args:\n%s\n", indent(args, "  "))
	}
}

// RenderToolResult renders a tool result
func (r *CLRenderer) RenderToolResult(output string, err error) {
	if err != nil {
		fmt.Printf("%s✗ Error: %s%s\n", r.ansi.Error, err.Error(), r.ansi.Reset)
	} else {
		fmt.Printf("%s✓ Success%s\n", r.ansi.Assistant, r.ansi.Reset)
		if output != "" {
			fmt.Printf("%s\n", indent(output, "  "))
		}
	}
}

// RenderComplete renders a completion message with stats
func (r *CLRenderer) RenderComplete(usage map[string]int) {
	fmt.Printf("%s✓ Complete%s\n", r.ansi.Assistant, r.ansi.Reset)
	if usage != nil {
		fmt.Println("Usage:")
		for k, v := range usage {
			fmt.Printf("  %s: %d\n", k, v)
		}
	}
}

// getRiskColor returns the color for a risk level
func (r *CLRenderer) getRiskColor(level types.RiskLevel) string {
	switch level {
	case types.RiskLevelCritical:
		return r.ansi.Error
	case types.RiskLevelHigh:
		return r.ansi.Warning
	case types.RiskLevelMedium:
		return r.ansi.Warning
	default:
		return ""
	}
}

// indent indents a multi-line string
func indent(s string, prefix string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}
