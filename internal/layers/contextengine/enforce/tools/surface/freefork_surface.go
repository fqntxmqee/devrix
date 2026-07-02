package surface

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce/tools"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// FreeForkSurface exposes the free_fork tool. It holds an explicit
// FreeForkerFunc (NOT the package-level globalFreeForker), so the
// dependency is visible in the constructor. This is the replacement for
// tools.globalFreeForker / SetFreeForker.
//
// TOOL-SURFACE-1-A01-F07 (DM-20260618-002): permGate is the
// IPermissionGate used by CheckPermission. A nil gate is the
// conservative default (Ask).
type FreeForkSurface struct {
	forker   tools.FreeForkerFunc
	permGate contracts.IPermissionGate
}

// NewFreeForkSurface wires a forker into the surface.
func NewFreeForkSurface(f tools.FreeForkerFunc) *FreeForkSurface {
	return &FreeForkSurface{forker: f}
}

// NewFreeForkSurfaceWithGate wires both the forker and the permission
// gate. Use this in production (DM-002 requires a gate); the
// 1-arg constructor is the legacy compat path for unit tests.
func NewFreeForkSurfaceWithGate(f tools.FreeForkerFunc, gate contracts.IPermissionGate) *FreeForkSurface {
	return &FreeForkSurface{forker: f, permGate: gate}
}

// Name implements contracts.ToolSurface.
func (s *FreeForkSurface) Name() string { return "free_fork" }

// Tools implements contracts.ToolSurface.
func (s *FreeForkSurface) Tools(_ context.Context, _, _ string) []contracts.ToolSpec {
	rOnly, dest, openW, concSafe := OrthogonalFlagFor("free_fork")
		spec := contracts.ToolSpec{

		Name:            "free_fork",
		Description:     "Batch fork N child agents (1..5) under a parent session. Each child runs in an isolated worker directory sandbox by default.",
		Parameters:      `{"parent_session": "<id>", "requests": [{"name": "...", "prompt": "...", "sandbox": true, "mode": "brief|fork|full (default: brief)"}]}`,
		Risk:            types.RiskLevelHigh,
		ReadOnly:        rOnly,
		Destructive:     dest,
		OpenWorld:       openW,
		ConcurrencySafe: concSafe,
	
}
	ApplyV3Metadata(&spec, "free_fork")
	return []contracts.ToolSpec{spec}
}

// InterruptBehavior implements contracts.ToolSurface. free_fork is the only
// long-run tool in devrix and MUST return InterruptCancel so the surface
// selects on ctx.Done() inside Execute when the user issues a new message.
func (s *FreeForkSurface) InterruptBehavior(name string) contracts.InterruptMode {
	return InterruptBehaviorFor(name)
}

// CheckPermission implements contracts.ToolSurface. free_fork spawns
// up to 5 child agents — a multi-agent blast radius — so the
// permission decision is delegated to the global IPermissionGate
// (which knows about plan_mode, YOLO, and per-user policy). A nil
// gate returns Ask (conservative default — the LLM should not be
// able to spawn child agents in unattended mode).
//
// TOOL-SURFACE-1-A01-F07 (DM-20260618-002).
func (s *FreeForkSurface) CheckPermission(ctx context.Context, spec contracts.ToolSpec, _ json.RawMessage) contracts.Decision {
	if s.permGate == nil {
		return contracts.DecisionAsk
	}
	return s.permGate.CheckPermission(ctx, spec)
}

// IsConcurrencySafe implements contracts.ToolSurface v4. free_fork is a
// multi-agent spawn — NEVER concurrency safe (DSAFT: D2-S15-A02-T17).
func (s *FreeForkSurface) IsConcurrencySafe(_ json.RawMessage) bool {
	return false
}

// ToAutoClassifierInput implements contracts.ToolSurface v4. P2 stub
// default — returns "" to skip in classifier transcript.
func (s *FreeForkSurface) ToAutoClassifierInput(input json.RawMessage) string {
	return DefaultToAutoClassifierInputFor("free_fork", input)
}

// RiskLevel implements contracts.ToolSurface.
func (s *FreeForkSurface) RiskLevel(name string) types.RiskLevel {
	if name == "free_fork" {
		return types.RiskLevelHigh
	}
	return ""
}

// freeforkInput / Request / Output / Handle mirror the tools types.
type freeforkInput struct {
	ParentSession string            `json:"parent_session"`
	Requests      []freeforkRequest `json:"requests"`
}

type freeforkRequest struct {
	Name     string `json:"name"`
	Prompt   string `json:"prompt"`
	Sandbox  bool   `json:"sandbox"`
	Worktree bool   `json:"worktree"`
	Mode     string `json:"mode,omitempty"`
}

type freeforkOutput struct {
	SpawnedCount int      `json:"spawned_count"`
	AgentIDs     []string `json:"agent_ids"`
}

const freeforkMaxRequests = 5

// Execute implements contracts.ToolSurface. Behavior is identical to the
// tools.freeforkRunner it replaces.
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
	dtos := make([]tools.FreeForkRequestDTO, 0, len(in.Requests))
	for _, r := range in.Requests {
		if r.Name == "" {
			return &contracts.ToolResult{Error: "free_fork: each request must have a non-empty name"}, nil
		}
		dtos = append(dtos, tools.FreeForkRequestDTO{
			Name:     r.Name,
			Prompt:   r.Prompt,
			Sandbox:  r.Sandbox || r.Worktree,
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
