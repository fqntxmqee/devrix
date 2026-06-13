package debug_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/devrix/devrix/internal/cli/debug"
)

// T: D5-S4-A01-T02
func TestRunExport_should_write_incident_bundle(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "sess_cli.jsonl")
	if err := os.WriteFile(logPath, []byte(`{"session_id":"sess_cli","phase":"request","data":{}}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(dir, "out.json")

	if err := debug.RunExport([]string{
		"--session", "sess_cli",
		"--llm-log-dir", dir,
		"--output", outPath,
	}); err != nil {
		t.Fatalf("RunExport: %v", err)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("empty export")
	}
}
