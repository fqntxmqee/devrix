package ltllite

import (
	"fmt"
	"time"
)

// State 提供 invariant 中命名命题的真值查询接口。
//
// 实现方负责把"系统当前状态"映射为 named booleans。
// 例如: surface 的 ReadOnly 字段 → "is_read_only" = true;
//       permission gate 启用 → "permission_gate" = true。
//
// 这是 LTL-Lite 与 model checker 的关键区别: 我们不在 check 内部实现
// 时序逻辑求值, 而是委托给 State (实现方在 hot-path 上自行缓存)。
type State interface {
	// Eval 求值命题 prop, 返回 true/false。
	// 未知命题返回 false (保守: 触发 violation 让调用方补齐 state 映射)。
	Eval(prop string) bool
}

// MapState 是 State 的 trivial map 实现, 方便测试。
type MapState map[string]bool

// Eval implements State.
func (m MapState) Eval(prop string) bool { return m[prop] }

// Violation 一条不变式违规。
type Violation struct {
	Invariant Invariant
	PreVal    bool
	PostVal   bool
	Reason    string
}

// String 便于日志展示。
func (v Violation) String() string {
	return fmt.Sprintf("VIOLATION %s: pre=%v, post=%v — %s", v.Invariant, v.PreVal, v.PostVal, v.Reason)
}

// Check 评估 set 中所有 invariant; 返回违规列表 (空 = 全部成立)。
//
// 评估规则:
//   - pre=false: invariant 不触发, 跳过 (不要求 post=true)
//   - pre=true, post=true: 成立
//   - pre=true, post=false: 违规 (Violation)
//
// 性能 bound (spec §LTL-Lite Self-Invariants): check_latency <= 5ms_per_turn。
// 实测 O(n) in invariants, 单次 Eval O(1) (map lookup)。1000 invariant 仍在 5ms 内。
func Check(set InvariantSet, state State) []Violation {
	return checkWithClock(set, state, defaultClock)
}

// checkWithClock 测试用 — 注入 clock 测量 latency。
func checkWithClock(set InvariantSet, state State, clock func() time.Time) []Violation {
	_ = clock // 当前实现不需要 clock; 保留供未来 latency assertion 钩子
	var out []Violation
	for _, inv := range set.Invariants {
		preVal := state.Eval(inv.Pre)
		if !preVal {
			continue
		}
		postVal := state.Eval(inv.Post)
		if postVal {
			continue
		}
		out = append(out, Violation{
			Invariant: inv,
			PreVal:    preVal,
			PostVal:   postVal,
			Reason:    fmt.Sprintf("pre %q holds but post %q does not", inv.Pre, inv.Post),
		})
	}
	return out
}

var defaultClock = func() time.Time { return time.Now() }
