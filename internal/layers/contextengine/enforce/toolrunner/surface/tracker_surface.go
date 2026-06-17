package surface

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/devrix/devrix/internal/layers/observability/diagnose/tracker"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// queryDiagLimitCap mirrors toolrunner.queryDiagLimitCap.
const queryDiagLimitCap = 256

// TrackerSurface exposes the query_diagnostics tool. The tracker is held
// explicitly (no GlobalTracker read at Execute time) — this is the
// replacement for the package-level global singleton that
// tracker.SetGlobalTracker used to install.
//
// If tr is nil, the surface is still safe to call: Tools() returns the
// spec, but Execute returns "tracker not initialized" so the LLM can see
// the tool exists but won't silently get an empty result.
type TrackerSurface struct {
	tr *tracker.Tracker
}

// NewTrackerSurface constructs a tracker surface with the given Tracker
// instance. Pass the same instance the bootstrap tick goroutine uses.
func NewTrackerSurface(tr *tracker.Tracker) *TrackerSurface {
	return &TrackerSurface{tr: tr}
}

// Name implements contracts.ToolSurface.
func (s *TrackerSurface) Name() string { return "tracker" }

// Tools implements contracts.ToolSurface. Always returns the spec; the
// nil-tracker case is reported at Execute time so the tool is still
// discoverable (otherwise the LLM would silently lose the schema).
func (s *TrackerSurface) Tools(_ context.Context, _, _ string) []contracts.ToolSpec {
	return []contracts.ToolSpec{{
		Name:        "query_diagnostics",
		Description: "Query the recent file-diagnostic buffer maintained by the periodic linter tick. Returns up to `limit` (default 50) diagnostics with file/line/severity/source/message. Optional `file` and `severity` filters narrow the result.",
		Parameters:  `{"limit": 50, "file": "<optional>", "severity": "<error|warning|info>"}`,
		Risk:        types.RiskLevelLow,
	}}
}

// RiskLevel implements contracts.ToolSurface.
func (s *TrackerSurface) RiskLevel(name string) types.RiskLevel {
	if name == "query_diagnostics" {
		return types.RiskLevelLow
	}
	return types.RiskLevelLow
}

// queryDiagInput mirrors toolrunner.queryDiagInput.
type queryDiagInput struct {
	Limit    int    `json:"limit"`
	File     string `json:"file"`
	Severity string `json:"severity"`
}

// queryDiagOutput mirrors toolrunner.queryDiagOutput.
type queryDiagOutput struct {
	Count       int                  `json:"count"`
	TotalInBuf  int                  `json:"total_in_buffer"`
	Diagnostics []tracker.Diagnostic `json:"diagnostics"`
}

// Execute implements contracts.ToolSurface. Behaves identically to the
// toolrunner.trackerRunner it replaces.
func (s *TrackerSurface) Execute(_ context.Context, _, input, _ string) (*contracts.ToolResult, error) {
	if s.tr == nil {
		return &contracts.ToolResult{Error: "query_diagnostics: tracker not initialized"}, nil
	}
	var in queryDiagInput
	if input != "" {
		if err := json.Unmarshal([]byte(input), &in); err != nil {
			return &contracts.ToolResult{Error: fmt.Sprintf("query_diagnostics: invalid input JSON: %s", err.Error())}, nil
		}
	}
	limit := in.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > queryDiagLimitCap {
		limit = queryDiagLimitCap
	}
	all := s.tr.Recent()
	out := queryDiagOutput{
		TotalInBuf:  len(all),
		Diagnostics: make([]tracker.Diagnostic, 0, limit),
	}
	for _, d := range all {
		if in.File != "" && d.File != in.File {
			continue
		}
		if in.Severity != "" && d.Severity != in.Severity {
			continue
		}
		if len(out.Diagnostics) < limit {
			out.Diagnostics = append(out.Diagnostics, d)
		}
	}
	out.Count = len(out.Diagnostics)
	bz, _ := json.Marshal(out)
	return &contracts.ToolResult{Output: string(bz)}, nil
}
