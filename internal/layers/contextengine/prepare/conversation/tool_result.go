package conversation

import (
	"fmt"
	"strings"
)

const maxToolErrorForLLM = 512

// FormatToolResultContent builds the tool-role payload sent to the LLM.
// Errors are shortened and stripped of operator-only hints (config paths, YOLO notes).
func FormatToolResultContent(toolName, output, errMsg string) string {
	if strings.TrimSpace(errMsg) == "" {
		return output
	}
	clean := SanitizeToolErrorForLLM(toolName, errMsg)
	if strings.TrimSpace(output) == "" {
		return clean
	}
	return output + "\n[error] " + clean
}

// SanitizeToolErrorForLLM shortens tool errors before they enter the LLM context.
func SanitizeToolErrorForLLM(toolName, errMsg string) string {
	msg := strings.TrimSpace(errMsg)
	msg = strings.ReplaceAll(msg, " (add to tool.allowlist in config).", ".")
	msg = strings.ReplaceAll(msg, "add to tool.allowlist in config", "")
	msg = strings.ReplaceAll(msg, "This is a sandbox policy (not permission/YOLO); use relative paths under WorkDir or read_file/glob/list_dir for files.", "")
	msg = strings.ReplaceAll(msg, "use relative paths under WorkDir or read_file/glob/list_dir for files.", "")
	msg = strings.TrimSpace(msg)
	if msg == "" {
		msg = "tool execution failed"
	}
	if toolName != "" && !strings.HasPrefix(msg, toolName+":") && !strings.HasPrefix(msg, "bash:") && !strings.HasPrefix(msg, "glob:") {
		msg = fmt.Sprintf("%s: %s", toolName, msg)
	}
	if len(msg) > maxToolErrorForLLM {
		msg = msg[:maxToolErrorForLLM] + "…"
	}
	return msg
}
