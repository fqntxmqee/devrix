package surface

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce/toolrunner"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// FreeForkSurface exposes the free_fork tool. It holds an explicit
// FreeForkerFunc (NOT the package-level globalFreeForker), so the
// dependency is visible in the constructor. This is the replacement for
// toolrunner.globalFreeForker / SetFreeForker.
type FreeForkSurface struct {
	forker toolrunner.FreeForkerFunc
}

// NewFreeForkSurface wires a forker into the surface.
func NewFreeForkSurface(f toolrunner.FreeForkerFunc) *FreeForkSurface {
	return &FreeForkSurface{forker: f}
}

// Name implements contracts.ToolSurface.
func (s *FreeForkSurface) Name() string { return "free_fork" }

// Tools implements contracts.ToolSurface.
func (s *FreeForkSurface) Tools(_ context.Context, _, _ string) []contracts.ToolSpec {
	return []contracts.ToolSpec{{
		Name:        "free_fork",
		Description: "Batch fork N child agents (1..5) under a parent session. Each child inherits the parent's session id but runs in an isolated worktree (default). Returns agent_ids list.",
		Parameters:  `{"parent_session": "<id>", "requests": [{"name": "...", "prompt": "...", "worktree": true, "mode": "default"}]}`,
		Risk:        types.RiskLevelHigh,
	}}
}

// RiskLevel implements contracts.ToolSurface.
func (s *FreeForkSurface) RiskLevel(name string) types.RiskLevel {
	if name == "free_fork" {
		return types.RiskLevelHigh
	}
	return types.RiskLevelLow
}

// freeforkInput / Request / Output / Handle mirror the toolrunner types.
type freeforkInput struct {
	ParentSession string            `json:"parent_session"`
	Requests      []freeforkRequest `json:"requests"`
}

type freeforkRequest struct {
	Name     string `json:"name"`
	Prompt   string `json:"prompt"`
	Worktree bool   `json:"worktree"`
	Mode     string `json:"mode,omitempty"`
}

type freeforkOutput struct {
	SpawnedCount int      `json:"spawned_count"`
	AgentIDs     []string `json:"agent_ids"`
}

const freeforkMaxRequests = 5

// Execute implements contracts.ToolSurface. Behavior is identical to the
// toolrunner.freeforkRunner it replaces.
func (s *FreeForkSurface) Execute(ctx context.Context, _, input, _ string) (*contracts.ToolResult, error) {
	if s.forker == nil {
		return &contracts.ToolResult{Error: "free_fork: forker not initialized"}, nil
	}
	var in freeforkInput
	if err := json.Unmarshal([]byte(input), &in); err != nil {
		return &contracts.ToolResult{Error: fmt.Sprintf("free_fork: invalid input JSON: %s", err.Error())}, nil
	}
	if in.ParentSession == "" {
		return &contracts.ToolResult{Error: "free_fork: parent_session is required"}, nil
	}
	if n := len(in.Requests); n < 1 || n > freeforkMaxRequests {
		return &contracts.ToolResult{Error: fmt.Sprintf("free_fork: requests count must be in [1,%d], got %d", freeforkMaxRequests, n)}, nil
	}
	dtos := make([]toolrunner.FreeForkRequestDTO, 0, len(in.Requests))
	for _, r := range in.Requests {
		if r.Name == "" {
			return &contracts.ToolResult{Error: "free_fork: each request must have a non-empty name"}, nil
		}
		dtos = append(dtos, toolrunner.FreeForkRequestDTO{
			Name:     r.Name,
			Prompt:   r.Prompt,
			Worktree: r.Worktree,
			Mode:     r.Mode,
		})
	}
	handles, err := s.forker(ctx, in.ParentSession, dtos)
	if err != nil {
		return &contracts.ToolResult{Error: fmt.Sprintf("free_fork: %s", err.Error())}, nil
	}
	out := freeforkOutput{
		SpawnedCount: len(handles),
		AgentIDs:     make([]string, 0, len(handles)),
	}
	for _, h := range handles {
		out.AgentIDs = append(out.AgentIDs, h.AgentID)
	}
	bz, _ := json.Marshal(out)
	return &contracts.ToolResult{Output: string(bz)}, nil
}
