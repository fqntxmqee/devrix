package surface

import (
	"github.com/devrix/devrix/internal/shared/ltllite"
)

// LTL-Lite invariant declarations for BashASTPolicy (DM-20260618-007 W15)。
//
// Bash surface 的 4 条跨切面约束:
//   - bashASTPolicy.DenyList: 5 个 deny rules (rm -rf / / dd / mkfs / sudo / chmod 777 /)
//   - 集成 bash.Policy (W5): sandboxast + 22+ zsh rules + heredoc audit
//   - CheckPermission: bash tool 必须先经 bashAST.Check (TOOL-SURFACE-1-A01-F07)
//   - RiskLevel: bash → High (sandbox-mediated, 非 LOW)
type bashSurfaceInvariants struct {
	DenyRules        string `invariant:"has_deny_rules => deny_list_non_empty"`
	PolicyIntegrated string `invariant:"bash_policy_injected => sandboxast_enabled"`
	PermissionHook   string `invariant:"permission_required => checkpolicy_called"`
	RiskNotLow       string `invariant:"bash_callable => risk_not_low"`
}

var bashSurfaceInvariantSet = mustParseBashInvariants()

func mustParseBashInvariants() ltllite.InvariantSet {
	set, err := ltllite.ParseStruct(bashSurfaceInvariants{})
	if err != nil {
		panic("ltllite: BashASTPolicy invariant parse failed: " + err.Error())
	}
	return set
}

// CheckBashInvariants 验证 BashASTPolicy 状态是否满足所有 invariant。
func CheckBashInvariants(state ltllite.State) []ltllite.Violation {
	return ltllite.Check(bashSurfaceInvariantSet, state)
}
