package adapter

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/llmgateway"
)

func TestBuildOpenAIChatRequest_should_include_tools(t *testing.T) {
	req, err := buildOpenAIChatRequest(&llmgateway.Request{
		Model: "deepseek-v4-pro",
		Tools: []llmgateway.ToolSchema{{
			Name: "bash", Description: "run", Parameters: `{"type":"object","properties":{}}`,
		}},
		Stream: true,
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if len(req.Tools) != 1 || req.Tools[0].Function.Name != "bash" {
		t.Errorf("tools: %+v", req.Tools)
	}
	if !req.Stream {
		t.Error("expected stream true")
	}
}
