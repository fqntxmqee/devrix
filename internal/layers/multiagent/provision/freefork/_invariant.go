// Package freefork — D4-S11 FreeFork 的 LTL-Lite invariant (W15)。
//
// FreeFork 跨切面约束:
//   - Concurrency: maxConcurrent ≤ 8 (默认), W0 配置项覆盖
//   - Worktree 隔离: 每个 child agent 在独立 worktree, 防 main branch 污染
//   - Rollback on partial failure: 中途失败必须回滚已 spawn 的 handles (现有 TestFork_FailureMidBatchRollsBack)
//   - Handle.Wait 必返回 AgentResult 或 error: 无 zombie goroutine
package freefork

import "github.com/devrix/devrix/internal/shared/ltllite"

type freeforkInvariants struct {
	Concurrency       string `invariant:"fork_active => concurrency_within_max"`
	WorktreeIsolated  string `invariant:"worktree_requested => path_distinct"`
	RollbackOnFailure string `invariant:"partial_failure => all_spawned_rolled_back"`
	HandleTerminates  string `invariant:"handle_returned => agent_result_or_error"`
}

var freeforkInvariantSet = mustParseFreeForkInvariants()

func mustParseFreeForkInvariants() ltllite.InvariantSet {
	set, err := ltllite.ParseStruct(freeforkInvariants{})
	if err != nil {
		panic("ltllite: freefork invariant parse failed: " + err.Error())
	}
	return set
}

// CheckFreeForkInvariants 验证 FreeFork 状态是否满足所有 invariant。
func CheckFreeForkInvariants(state ltllite.State) []ltllite.Violation {
	return ltllite.Check(freeforkInvariantSet, state)
}
