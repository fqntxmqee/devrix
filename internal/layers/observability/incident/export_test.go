package incident_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/devrix/devrix/internal/layers/observability/incident"
)

// Covers: L5-OBS-EXPORT-01
func TestBuildBundle_should_export_valid_json_schema_v1(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "sess_test.jsonl")
	line := `{"timestamp":"2026-06-10T12:00:00Z","session_id":"sess_test","phase":"request","trace_id":"abc","span_id":"def","data":{}}`
	if err := os.WriteFile(logPath, []byte(line+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	bundle, err := incident.BuildBundle("sess_test", incident.ExportOptions{LLMLogDir: dir})
	if err != nil {
		t.Fatalf("BuildBundle: %v", err)
	}
	if bundle.SchemaVersion != "1.0" {
		t.Fatalf("schema_version: got %q", bundle.SchemaVersion)
	}
	if bundle.SessionID != "sess_test" {
		t.Fatalf("session_id: got %q", bundle.SessionID)
	}
	if len(bundle.LLMRounds) != 1 {
		t.Fatalf("llm_rounds: got %d want 1", len(bundle.LLMRounds))
	}
	if bundle.Trace == nil || bundle.Trace.TraceID != "abc" {
		t.Fatalf("trace_id: got %+v", bundle.Trace)
	}

	data, err := incident.MarshalJSON(bundle)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, string(data))
	}
	for _, key := range []string{"schema_version", "session_id", "exported_at", "llm_rounds"} {
		if _, ok := raw[key]; !ok {
			t.Fatalf("missing key %q", key)
		}
	}
}
