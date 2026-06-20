package stream

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/observability/diagnose/incident"
)

// T: D3-S3-A01-F04-T01 (tools_json trace point)
//
// The LLM request span payload must surface what the provider was
// offered, not just a count. The `tools_json` field is the diagnostic
// signal for "why did the LLM call the wrong tool / hallucinate
// arguments" — without it on the span, post-mortem is blind.
//
// These tests pin the contract of buildStreamRequestInfo: names always
// present, parameters gated on observability.llm.log_content, and
// the wire-shape `tools` array present in the JSON payload regardless
// of the gate (name + description are always useful, parameters are
// the optional heavy part).
func TestBuildStreamRequestInfo_AlwaysIncludesToolNamesAndDescriptions(t *testing.T) {
	req := &llmgateway.Request{
		Model: "deepseek-v4-flash",
		Tools: []llmgateway.ToolSchema{
			{Name: "bash", Description: "execute a shell command", Parameters: `{"type":"object"}`},
			{Name: "read", Description: "read a file", Parameters: `{"type":"object","properties":{"path":{"type":"string"}}}`},
		},
	}
	info, toolNames := buildStreamRequestInfo(req)

	if got, want := toolNames, "bash,read"; got != want {
		t.Errorf("toolNames = %q, want %q", got, want)
	}
	if got, want := info["tool_count"], 2; got != want {
		t.Errorf("info[tool_count] = %v, want %v", got, want)
	}

	tools, ok := info["tools"].([]map[string]interface{})
	if !ok {
		t.Fatalf("info[tools] type = %T, want []map[string]interface{}", info["tools"])
	}
	if len(tools) != 2 {
		t.Fatalf("len(tools) = %d, want 2", len(tools))
	}
	if tools[0]["name"] != "bash" || tools[1]["name"] != "read" {
		t.Errorf("tools[].name order: %+v", tools)
	}
	if tools[0]["description"] != "execute a shell command" {
		t.Errorf("tools[0].description = %v, want %q", tools[0]["description"], "execute a shell command")
	}
	// LogContent off by default: parameters must NOT leak in the
	// summary payload (they are heavy and may include vendor-specific
	// schema fragments).
	if _, present := tools[0]["parameters"]; present {
		t.Errorf("LogContent off: tools[0] should not carry parameters, got %+v", tools[0])
	}
}

// T: D3-S3-A01-F04-T02
//
// When observability.llm.log_content is on, the full parameters JSON
// Schema must be included so the trace has a complete `tools_json`
// mirror of the request body.
func TestBuildStreamRequestInfo_FullModeIncludesParameters(t *testing.T) {
	prev := incident.CurrentLLMLogSettings()
	incident.ConfigureLLMLogging(incident.LLMLogSettings{LogContent: true})
	t.Cleanup(func() {
		incident.ConfigureLLMLogging(prev)
	})

	req := &llmgateway.Request{
		Model: "deepseek-v4-flash",
		Tools: []llmgateway.ToolSchema{
			{
				Name:       "bash",
				Description: "execute a shell command",
				Parameters: `{"type":"object","properties":{"cmd":{"type":"string"}},"required":["cmd"]}`,
			},
		},
	}
	info, _ := buildStreamRequestInfo(req)

	tools, ok := info["tools"].([]map[string]interface{})
	if !ok {
		t.Fatalf("info[tools] type = %T", info["tools"])
	}
	params, present := tools[0]["parameters"]
	if !present {
		t.Fatalf("LogContent on: tools[0] should carry parameters, got %+v", tools[0])
	}
	// Parameters must round-trip as a JSON object (not the raw string).
	bz, _ := json.Marshal(params)
	rt, ok := params.(map[string]interface{})
	if !ok {
		t.Fatalf("parameters type = %T, want map[string]interface{}", params)
	}
	if _, hasCmd := rt["properties"]; !hasCmd {
		t.Errorf("parameters missing properties: %s", string(bz))
	}
	if !strings.Contains(string(bz), `"cmd"`) {
		t.Errorf("parameters should contain cmd property, got %s", string(bz))
	}
}

// T: D3-S3-A01-F04-T03
//
// Long descriptions are truncated to 200 chars in summary mode so a
// turn offering many tools (plan mode can offer 20+) does not bloat
// the span payload.
func TestBuildStreamRequestInfo_SummaryTruncatesLongDescriptions(t *testing.T) {
	long := strings.Repeat("a", 500)
	req := &llmgateway.Request{
		Model: "deepseek-v4-flash",
		Tools: []llmgateway.ToolSchema{
			{Name: "huge", Description: long, Parameters: `{}`},
		},
	}
	info, _ := buildStreamRequestInfo(req)
	tools := info["tools"].([]map[string]interface{})
	desc, _ := tools[0]["description"].(string)
	if !strings.HasSuffix(desc, "...") {
		t.Errorf("long description should end with ellipsis, got len=%d", len(desc))
	}
	if len(desc) > 210 {
		t.Errorf("truncated description len=%d, want ≤210 (200 + '...')", len(desc))
	}
}

// T: D3-S3-A01-F04-T04
//
// A tool whose Parameters string fails to parse is still surfaced in
// the trace — the wire-format-failed case is exactly the one
// operators need to see. We keep the raw string as `parameters_raw`
// so the trace is never silently empty.
func TestBuildStreamRequestInfo_FullModeKeepsRawParametersOnParseError(t *testing.T) {
	prev := incident.CurrentLLMLogSettings()
	incident.ConfigureLLMLogging(incident.LLMLogSettings{LogContent: true})
	t.Cleanup(func() {
		incident.ConfigureLLMLogging(prev)
	})

	req := &llmgateway.Request{
		Model: "deepseek-v4-flash",
		Tools: []llmgateway.ToolSchema{
			{Name: "broken", Description: "bad schema", Parameters: `{this is not json`},
		},
	}
	info, _ := buildStreamRequestInfo(req)
	tools := info["tools"].([]map[string]interface{})
	raw, present := tools[0]["parameters_raw"]
	if !present {
		t.Fatalf("parse-failed tool should keep raw parameters, got %+v", tools[0])
	}
	if raw.(string) != `{this is not json` {
		t.Errorf("parameters_raw = %q, want original", raw)
	}
	if _, leaked := tools[0]["parameters"]; leaked {
		t.Errorf("parse-failed tool should not also have parameters, got %+v", tools[0])
	}
}
