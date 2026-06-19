package surface_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce/toolrunner"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce/toolrunner/surface"
	"github.com/devrix/devrix/internal/layers/observability/diagnose/tracker"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// ----- FreeForkSurface -----

func okForker(_ context.Context, _ string, _ []toolrunner.FreeForkRequestDTO) ([]toolrunner.FreeForkHandleDTO, error) {
	return []toolrunner.FreeForkHandleDTO{{AgentID: "a1"}, {AgentID: "a2"}, {AgentID: "a3"}}, nil
}

func failForker(_ context.Context, _ string, _ []toolrunner.FreeForkRequestDTO) ([]toolrunner.FreeForkHandleDTO, error) {
	return nil, errors.New("factory failure")
}

// T: TOOL-SURFACE-1-T04 — FreeForkSurface.Execute happy path: 3 batch → spawned_count=3.
func TestFreeForkSurface_Execute_Batch3(t *testing.T) {
	s := surface.NewFreeForkSurface(okForker)
	input := `{"parent_session":"sess-x","requests":[{"name":"r1","prompt":"p1"},{"name":"r2","prompt":"p2"},{"name":"r3","prompt":"p3"}]}`
	res, err := s.Execute(context.Background(), "free_fork", input, "")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Error != "" {
		t.Fatalf("Error = %q, want empty", res.Error)
	}
	var out struct {
		SpawnedCount int      `json:"spawned_count"`
		AgentIDs     []string `json:"agent_ids"`
	}
	if err := json.Unmarshal([]byte(res.Output), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.SpawnedCount != 3 || len(out.AgentIDs) != 3 {
		t.Errorf("got spawned_count=%d agent_ids=%v, want 3/3", out.SpawnedCount, out.AgentIDs)
	}
}

// T: TOOL-SURFACE-1-T04 — FreeForkSurface rolls back when factory fails.
func TestFreeForkSurface_Execute_Rollback(t *testing.T) {
	s := surface.NewFreeForkSurface(failForker)
	input := `{"parent_session":"sess-x","requests":[{"name":"r1","prompt":"p1"}]}`
	res, _ := s.Execute(context.Background(), "free_fork", input, "")
	if res.Error == "" {
		t.Error("Error empty, want 'factory failure'")
	}
	if res.Output != "" {
		t.Errorf("Output = %q, want empty (rollback)", res.Output)
	}
}

// T: TOOL-SURFACE-1-T04 — FreeForkSurface rejects requests > 5.
func TestFreeForkSurface_Execute_TooMany(t *testing.T) {
	s := surface.NewFreeForkSurface(okForker)
	requests := make([]map[string]string, 6)
	for i := range requests {
		requests[i] = map[string]string{"name": "r", "prompt": "p"}
	}
	bs, _ := json.Marshal(map[string]any{
		"parent_session": "sess-x",
		"requests":       requests,
	})
	res, _ := s.Execute(context.Background(), "free_fork", string(bs), "")
	if res.Error == "" {
		t.Error("Error empty, want 'requests count must be in [1,5]'")
	}
}

// T: TOOL-SURFACE-1-T04 — FreeForkSurface with nil forker is safe.
func TestFreeForkSurface_Execute_NilForker(t *testing.T) {
	s := surface.NewFreeForkSurface(nil)
	res, _ := s.Execute(context.Background(), "free_fork", `{"parent_session":"s","requests":[{"name":"r","prompt":"p"}]}`, "")
	if res.Error == "" {
		t.Error("Error empty, want 'forker not initialized'")
	}
}

// T: TOOL-SURFACE-1-T03 — FreeForkSurface.RiskLevel returns HIGH.
func TestFreeForkSurface_RiskLevel(t *testing.T) {
	s := surface.NewFreeForkSurface(okForker)
	if s.RiskLevel("free_fork") != types.RiskLevelHigh {
		t.Errorf("risk = %q, want HIGH", s.RiskLevel("free_fork"))
	}
	if s.RiskLevel("other") != "" {
		t.Errorf("other = %q, want empty (surface does not claim)", s.RiskLevel("other"))
	}
}

// ----- TrackerSurface -----

// T: TOOL-SURFACE-1-T04 — TrackerSurface.Execute after append returns the diagnostic.
func TestTrackerSurface_Execute_AfterAppend(t *testing.T) {
	tr := tracker.New(16)
	tr.RecordDiags([]tracker.Diagnostic{{File: "a.go", Line: 10, Severity: "error", Message: "boom"}})
	s := surface.NewTrackerSurface(tr)
	res, err := s.Execute(context.Background(), "query_diagnostics", `{"limit":10}`, "")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Error != "" {
		t.Fatalf("Error = %q", res.Error)
	}
	var out struct {
		Count       int                  `json:"count"`
		Diagnostics []tracker.Diagnostic `json:"diagnostics"`
	}
	_ = json.Unmarshal([]byte(res.Output), &out)
	if out.Count != 1 || len(out.Diagnostics) != 1 {
		t.Errorf("count = %d, len = %d, want 1/1", out.Count, len(out.Diagnostics))
	}
}

// T: TOOL-SURFACE-1-T04 — TrackerSurface with nil tracker returns "not initialized".
func TestTrackerSurface_Execute_NilTracker(t *testing.T) {
	s := surface.NewTrackerSurface(nil)
	res, _ := s.Execute(context.Background(), "query_diagnostics", `{}`, "")
	if res.Error == "" {
		t.Error("Error empty, want 'tracker not initialized'")
	}
}

// T: TOOL-SURFACE-1-T03 — TrackerSurface.Tools always returns the spec (even
// with nil tracker — schema visibility is independent of init state).
func TestTrackerSurface_Tools_AlwaysVisible(t *testing.T) {
	s := surface.NewTrackerSurface(nil)
	specs := s.Tools(context.Background(), "", "")
	if len(specs) != 1 || specs[0].Name != "query_diagnostics" {
		t.Errorf("Tools = %+v, want 1 spec for query_diagnostics", specs)
	}
}

// ----- VerifySurface -----

// T: TOOL-SURFACE-1-T04 — VerifySurface.Execute missing change_id.
func TestVerifySurface_Execute_MissingChangeID(t *testing.T) {
	s := surface.NewVerifySurface()
	res, _ := s.Execute(context.Background(), "verify_plan_execution", `{}`, "")
	if res.Error == "" {
		t.Error("Error empty, want 'change_id is required'")
	}
}

// T: TOOL-SURFACE-1-T04 — VerifySurface.Execute missing tasks file returns error.
func TestVerifySurface_Execute_MissingTasksFile(t *testing.T) {
	s := surface.NewVerifySurface()
	res, _ := s.Execute(context.Background(), "verify_plan_execution",
		`{"change_id":"nonexistent-change","repo_root":"/nonexistent"}`, "")
	if res.Error == "" {
		t.Error("Error empty, want load plan error")
	}
}

// T: TOOL-SURFACE-1-T03 — VerifySurface.Tools returns the spec.
func TestVerifySurface_Tools(t *testing.T) {
	s := surface.NewVerifySurface()
	specs := s.Tools(context.Background(), "", "")
	if len(specs) != 1 || specs[0].Name != "verify_plan_execution" {
		t.Errorf("Tools = %+v", specs)
	}
}

// T: TOOL-SURFACE-1-T04 — All 3 new surfaces satisfy contracts.ToolSurface.
func TestW4_InterfaceCompliance(t *testing.T) {
	var _ contracts.ToolSurface = surface.NewFreeForkSurface(okForker)
	var _ contracts.ToolSurface = surface.NewTrackerSurface(nil)
	var _ contracts.ToolSurface = surface.NewVerifySurface()
}
