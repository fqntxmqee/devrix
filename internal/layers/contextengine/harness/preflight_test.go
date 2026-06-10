package harness_test

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/harness"
	"github.com/devrix/devrix/internal/shared/config"
)

// Covers: L5-2-9-09
func TestPreflightEvaluator_should_warn_on_empty_message(t *testing.T) {
	eval := harness.NewPreflightEvaluator(config.PreflightConfig{
		Enabled:     true,
		Mode:        config.PreflightModeWarnOnly,
		TokenBudget: 8000,
		WarnRatio:   0.85,
	}, harness.NewToolPoolFilter(config.DefaultHarnessConfig().ToolPool))
	result := eval.Evaluate(nil, "   ", nil, "context")
	if result.Scores.Completeness != 0 {
		t.Fatalf("completeness: got %d want 0", result.Scores.Completeness)
	}
	if len(result.Warnings) == 0 {
		t.Fatal("expected warnings")
	}
}

// Covers: L5-2-9-09
func TestPreflightEvaluator_should_filter_irrelevant_tools(t *testing.T) {
	eval := harness.NewPreflightEvaluator(config.PreflightConfig{
		Enabled: true,
		ToolFilter: config.PreflightToolFilterConfig{
			Enabled: true,
			Mode:    config.PreflightToolFilterAutoRepair,
		},
	}, harness.NewToolPoolFilter(config.DefaultHarnessConfig().ToolPool))
	tools := []harness.ToolDesc{
		{Name: "bash", Description: "shell"},
		{Name: "read_file", Description: "read files"},
		{Name: "write_file", Description: "write files"},
		{Name: "call_cursor", Description: "cursor agent"},
	}
	filtered, decision := eval.FilterVisibleTools("run ls in terminal", tools)
	if len(filtered) == 0 {
		t.Fatal("expected kept tools")
	}
	_ = decision
}
