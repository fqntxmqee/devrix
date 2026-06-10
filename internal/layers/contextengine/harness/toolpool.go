package harness

import (
	"strings"

	"github.com/devrix/devrix/internal/shared/config"
)

var simpleModeTools = map[string]struct{}{
	"bash":       {},
	"read_file":  {},
	"write_file": {},
}

// ToolPoolFilter applies harness tool pool rules.
type ToolPoolFilter struct {
	cfg config.ToolPoolConfig
}

// NewToolPoolFilter creates a tool pool filter from config.
func NewToolPoolFilter(cfg config.ToolPoolConfig) *ToolPoolFilter {
	return &ToolPoolFilter{cfg: cfg}
}

// Filter returns the visible tool schemas after applying pool rules.
func (f *ToolPoolFilter) Filter(all []ToolDesc) []ToolDesc {
	if f == nil {
		return all
	}
	out := make([]ToolDesc, 0, len(all))
	for _, tool := range all {
		if f.shouldDeny(tool.Name) {
			continue
		}
		if !f.cfg.IncludeMCP && isMCPTool(tool) {
			continue
		}
		if f.cfg.SimpleMode {
			if _, ok := simpleModeTools[tool.Name]; !ok {
				continue
			}
		}
		out = append(out, tool)
	}
	return out
}

func (f *ToolPoolFilter) shouldDeny(name string) bool {
	for _, deny := range f.cfg.DenyNames {
		if name == deny {
			return true
		}
	}
	for _, prefix := range f.cfg.DenyPrefixes {
		if prefix != "" && strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func isMCPTool(tool ToolDesc) bool {
	lowerName := strings.ToLower(tool.Name)
	if strings.Contains(lowerName, "mcp") {
		return true
	}
	return strings.Contains(strings.ToLower(tool.Description), "mcp")
}
