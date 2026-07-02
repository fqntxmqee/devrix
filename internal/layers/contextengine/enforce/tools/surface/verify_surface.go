package surface

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/devrix/devrix/internal/layers/evolution/verify"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// VerifySurface exposes the verify_plan_execution tool. The verifier is
// constructed on demand (verify.NewFileVerifier is stateless), so this
// surface holds no instance state — every call is independent.
type VerifySurface struct{}

// NewVerifySurface returns a stateless verify surface.
func NewVerifySurface() *VerifySurface { return &VerifySurface{} }

// Name implements contracts.ToolSurface.
func (s *VerifySurface) Name() string { return "verify" }

// Tools implements contracts.ToolSurface.
func (s *VerifySurface) Tools(_ context.Context, _, _ string) []contracts.ToolSpec {
	rOnly, dest, openW, concSafe := OrthogonalFlagFor("verify_plan_execution")
		spec := contracts.ToolSpec{

		Name:            "verify_plan_execution",
		Description:     "Verify that all done items in the change's tasks.md have their evidence files present and (for _test.go) contain a func TestXxx(). Returns a Report JSON with verified/unverified/skipped counts.",
		Parameters:      `{"change_id": "<change-id>", "repo_root": "<optional abs path>"}`,
		Risk:            types.RiskLevelLow,
		ReadOnly:        rOnly,
		Destructive:     dest,
		OpenWorld:       openW,
		ConcurrencySafe: concSafe,
	
}
	ApplyV3Metadata(&spec, "verify_plan_execution")
	return []contracts.ToolSpec{spec}
}

// InterruptBehavior implements contracts.ToolSurface.
func (s *VerifySurface) InterruptBehavior(name string) contracts.InterruptMode {
	return InterruptBehaviorFor(name)
}

// CheckPermission implements contracts.ToolSurface.
// verify_plan_execution is a read-only verification query — always Allow.
func (s *VerifySurface) CheckPermission(_ context.Context, _ contracts.ToolSpec, _ json.RawMessage) contracts.Decision {
	return contracts.DecisionAllow
}

// IsConcurrencySafe implements contracts.ToolSurface v4. verify_plan_execution
// is read-only but NOT concurrency safe (v2 static bool = false) — verification
// state can race between parallel calls (DSAFT: D2-S15-A02-T17).
func (s *VerifySurface) IsConcurrencySafe(_ json.RawMessage) bool {
	return false
}

// ToAutoClassifierInput implements contracts.ToolSurface v4. P2 stub
// default — returns "" to skip in classifier transcript.
func (s *VerifySurface) ToAutoClassifierInput(input json.RawMessage) string {
	return DefaultToAutoClassifierInputFor("verify_plan_execution", input)
}

// RiskLevel implements contracts.ToolSurface.
func (s *VerifySurface) RiskLevel(name string) types.RiskLevel {
	if name == "verify_plan_execution" {
		return types.RiskLevelLow
	}
	return ""
}

// verifyInput mirrors tools.verifyInput.
type verifyInput struct {
	ChangeID string `json:"change_id"`
	RepoRoot string `json:"repo_root"`
}

// Execute implements contracts.ToolSurface. Behavior is identical to the
// tools.verifyRunner it replaces.
func (s *VerifySurface) Execute(ctx context.Context, _, input, workDir string) (*contracts.ToolResult, error) {
	var in verifyInput
	if input != "" {
		if err := json.Unmarshal([]byte(input), &in); err != nil {
			return &contracts.ToolResult{Error: fmt.Sprintf("verify_plan_execution: invalid input JSON: %s", err.Error())}, nil
		}
	}
	if in.ChangeID == "" {
		return &contracts.ToolResult{Error: "verify_plan_execution: change_id is required"}, nil
	}
	repoRoot := in.RepoRoot
	if repoRoot == "" {
		repoRoot = workDir
	}
	taskFile := filepath.Join(repoRoot, "openspec", "changes", in.ChangeID, "tasks.md")
	fv := verify.NewFileVerifier()
	items, err := fv.LoadPlan(taskFile)
	if err != nil {
		return &contracts.ToolResult{Error: fmt.Sprintf("verify_plan_execution: load plan: %s", err.Error())}, nil
	}
	report, err := fv.Verify(ctx, items, repoRoot)
	if err != nil {
		return &contracts.ToolResult{Error: fmt.Sprintf("verify_plan_execution: verify: %s", err.Error())}, nil
	}
	report.ChangeID = in.ChangeID
	out, err := verify.FormatJSON(report)
	if err != nil {
		return &contracts.ToolResult{Error: fmt.Sprintf("verify_plan_execution: format: %s", err.Error())}, nil
	}
	return &contracts.ToolResult{Output: string(out)}, nil
}
