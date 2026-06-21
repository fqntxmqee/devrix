// Package verify — D6-Evo plan verify 的 LTL-Lite invariant (DM-20260618-007 W15)。
//
// verify_plan_execution surface 的 4 条跨切面约束:
//   - ReadOnly: 验证不改 plan 文件 (FileVerifier.Verify 纯函数)
//   - EvidenceKind: 已知 kind 必须有 checker (file/test/cmd/api)
//   - Skipped: Done=false 的 items 计为 skipped (不入 verified/unverified)
//   - Report JSON: 输出 schema 稳定 (change_id + verified + unverified + skipped + summary)
package verify

import (
	"log"

	"github.com/devrix/devrix/internal/shared/ltllite"
)

type verifyInvariants struct {
	ReadOnly         string `invariant:"verify_called => no_plan_mutation"`
	EvidenceRouted   string `invariant:"known_kind => checker_available"`
	SkippedSeparated string `invariant:"skipped_counted => not_in_unverified"`
	ReportSchema     string `invariant:"report_emitted => required_fields_present"`
}

var verifyInvariantSet ltllite.InvariantSet

func init() {
	set, err := parseVerifyInvariants()
	if err != nil {
		log.Fatalf("verify: ltllite invariant parse failed: %v", err)
	}
	verifyInvariantSet = set
}

func parseVerifyInvariants() (ltllite.InvariantSet, error) {
	return ltllite.ParseStruct(verifyInvariants{})
}

// CheckVerifyInvariants 验证 verify 状态是否满足所有 invariant。
func CheckVerifyInvariants(state ltllite.State) []ltllite.Violation {
	return ltllite.Check(verifyInvariantSet, state)
}
