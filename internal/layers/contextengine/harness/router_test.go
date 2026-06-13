package harness_test

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/harness"
	"github.com/devrix/devrix/internal/shared/config"
)

// T: D2-S9-A01-T06
func TestPromptRouter_should_score_matching_tools(t *testing.T) {
	router := harness.NewPromptRouter(config.RoutingConfig{Enabled: true, MaxMatches: 3})
	tools := []harness.ToolDesc{
		{Name: "bash", Description: "run shell commands"},
		{Name: "read_file", Description: "read workspace files"},
	}
	hint := router.Route("please read the config file", tools, 3)
	if len(hint.Tools) == 0 {
		t.Fatal("expected matched tools")
	}
	if hint.Scores["read_file"] == 0 {
		t.Fatal("expected read_file score > 0")
	}
}
