package harness_test

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/harness"
	"github.com/devrix/devrix/internal/shared/config"
)

// T: D2-S9-A03-T05
func TestToolPoolFilter_should_apply_simple_mode(t *testing.T) {
	all := []harness.ToolDesc{
		{Name: "bash"},
		{Name: "read_file"},
		{Name: "write_file"},
		{Name: "call_claude-code"},
	}
	filter := harness.NewToolPoolFilter(config.ToolPoolConfig{SimpleMode: true, IncludeMCP: true})
	out := filter.Filter(all)
	if len(out) != 3 {
		t.Fatalf("simple mode: got %d tools want 3", len(out))
	}
}

// T: D2-S9-A03-T05
func TestToolPoolFilter_should_exclude_mcp_when_disabled(t *testing.T) {
	all := []harness.ToolDesc{
		{Name: "bash"},
		{Name: "mcp_filesystem"},
	}
	filter := harness.NewToolPoolFilter(config.ToolPoolConfig{IncludeMCP: false})
	out := filter.Filter(all)
	if len(out) != 1 || out[0].Name != "bash" {
		t.Fatalf("unexpected filter result: %+v", out)
	}
}
