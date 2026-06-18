// Package tracker — D5-S23 文件诊断追踪器的 LTL-Lite invariant (W15)。
//
// 这些 invariant 表达 tracker 跨切面约束:
//   - LRU 容量上限 (默认 500): 必须 ≤ MaxCapacity, 防 OOM
//   - Linter 路由: 已知扩展名必须有 linter (go→goVetLinter, ts→tscLinter, sh→shellcheck)
//   - WatchedFiles: 监控集合与 TickOnce 路径一致 (异步 fire-and-forget 不丢失)
//   - Recent buffer 容量: ≤ RecentBufferSize (256) 防内存爆炸
package tracker

import "github.com/devrix/devrix/internal/shared/ltllite"

type trackerInvariants struct {
	LRUCap          string `invariant:"lru_used => lru_cap_within_max"`
	LinterRouted    string `invariant:"known_ext => linter_available"`
	WatchConsistent string `invariant:"watched_set => tick_covers_all"`
	RecentBounded   string `invariant:"recent_used => recent_within_cap"`
}

var trackerInvariantSet = mustParseTrackerInvariants()

func mustParseTrackerInvariants() ltllite.InvariantSet {
	set, err := ltllite.ParseStruct(trackerInvariants{})
	if err != nil {
		panic("ltllite: tracker invariant parse failed: " + err.Error())
	}
	return set
}

// CheckTrackerInvariants 验证 tracker 状态是否满足所有 invariant。
func CheckTrackerInvariants(state ltllite.State) []ltllite.Violation {
	return ltllite.Check(trackerInvariantSet, state)
}
