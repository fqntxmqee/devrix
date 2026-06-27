package sessionorchestrator

import (
	"testing"
)

// T: D7-S2-A06-T03 (DM-20260617-004 devrix-d7-tool-ctx-inject)
// orchestrationToolSchemas (LoopFirst injected tool list) must include the
// free_fork tool so users saying "用 free_fork 启 N 个 worker" reach a
// real registered tool.
func TestOrchestrationToolSchemas_ExposesFreeFork(t *testing.T) {
	schemas := orchestrationToolSchemas()
	names := make(map[string]bool, len(schemas))
	for _, s := range schemas {
		names[s.Name] = true
	}
	for _, want := range []string{"enter_plan_mode", "free_fork"} {
		if !names[want] {
			t.Errorf("orchestrationToolSchemas missing %q; got names=%v", want, names)
		}
	}
}

// T: D7-S2-A06-T03 — free_fork schema declares parent_session + requests with
// required name/prompt, matching the D2 freeforkRunner input contract.
func TestOrchestrationToolSchemas_FreeFork_Parameters(t *testing.T) {
	schemas := orchestrationToolSchemas()
	var ff *ToolSchema
	for i := range schemas {
		if schemas[i].Name == "free_fork" {
			ff = &schemas[i]
			break
		}
	}
	if ff == nil {
		t.Fatal("free_fork schema not found in orchestrationToolSchemas")
	}
	if ff.Description == "" {
		t.Error("free_fork schema: description is empty")
	}
	props, ok := ff.Parameters["properties"].(map[string]any)
	if !ok {
		t.Fatalf("free_fork schema: properties missing or wrong type: %+v", ff.Parameters)
	}
	for _, want := range []string{"parent_session", "requests"} {
		if _, ok := props[want]; !ok {
			t.Errorf("free_fork schema: missing property %q; got=%v", want, props)
		}
	}
	required, ok := ff.Parameters["required"].([]any)
	if !ok {
		t.Fatalf("free_fork schema: required missing or wrong type")
	}
	wantReq := map[string]bool{"parent_session": false, "requests": false}
	for _, r := range required {
		if s, ok := r.(string); ok {
			if _, present := wantReq[s]; present {
				wantReq[s] = true
			}
		}
	}
	for k, seen := range wantReq {
		if !seen {
			t.Errorf("free_fork schema: required[%q] missing", k)
		}
	}
}

// T: DM-20260620-001-B / B.2.3 (D4-S4-A07-T02) — free_fork schema declares
// `mode` enum [brief|fork|full] in request items, default brief, with
// description referencing Phase B sub-agent isolation.
func TestOrchestrationToolSchemas_FreeFork_ModeEnum(t *testing.T) {
	schemas := orchestrationToolSchemas()
	var ff *ToolSchema
	for i := range schemas {
		if schemas[i].Name == "free_fork" {
			ff = &schemas[i]
			break
		}
	}
	if ff == nil {
		t.Fatal("free_fork schema not found in orchestrationToolSchemas")
	}
	props, ok := ff.Parameters["properties"].(map[string]any)
	if !ok {
		t.Fatalf("free_fork schema: properties missing: %+v", ff.Parameters)
	}
	requests, ok := props["requests"].(map[string]any)
	if !ok {
		t.Fatalf("free_fork schema: requests missing or wrong type: %+v", props["requests"])
	}
	items, ok := requests["items"].(map[string]any)
	if !ok {
		t.Fatalf("free_fork schema: requests.items missing: %+v", requests)
	}
	itemProps, ok := items["properties"].(map[string]any)
	if !ok {
		t.Fatalf("free_fork schema: requests.items.properties missing")
	}
	mode, ok := itemProps["mode"].(map[string]any)
	if !ok {
		t.Fatalf("free_fork schema: requests.items.mode missing")
	}
	enum, ok := mode["enum"].([]any)
	if !ok {
		t.Fatalf("free_fork schema: mode.enum missing or wrong type: %+v", mode)
	}
	got := map[string]bool{}
	for _, e := range enum {
		if s, ok := e.(string); ok {
			got[s] = true
		}
	}
	for _, want := range []string{"brief", "fork", "full"} {
		if !got[want] {
			t.Errorf("free_fork schema: mode.enum missing %q; got %v", want, enum)
		}
	}
	if def, _ := mode["default"].(string); def != "brief" {
		t.Errorf("free_fork schema: mode.default = %q, want \"brief\"", def)
	}
}
