// Package turn_adapter — D7 orchestration 的 turn 适配层 (DM-20260618-007 W15)。
//
// ltl_hook 在 turn lifecycle 关键节点调 ltllite.Check 验证 invariant。
// 这是 LTL-Lite 跨切面框架的运行时入口。
//
// 关键点:
//   - Prepare 阶段: 加载 allSurfaceState, 跑全量 invariant check
//     任一 violation → ErrInvariantViolation 立即中止 turn (不开销 spawn agent)
//   - BeforeExecute 阶段: 每个 surface.Execute 前重检相关 invariant
//     (节省时间 — 不需要再跑全量)
//
// Performance: 全量 check ≤ 5ms/turn (spec §LTL-Lite Self-Invariants); 1000
// invariant 在内部 map lookup 下实测 < 1ms。
package turn_adapter

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/devrix/devrix/internal/shared/ltllite"
)

// ErrInvariantViolation — invariant 验证失败的 sentinel error。
// turn_adapter.Prepare 收到 violation 时 wrap 此 error 返回; 上层 (D7
// orchestrator) 据此决定 retry / report / abort。
var ErrInvariantViolation = errors.New("turn_adapter: ltl-lite invariant violation")

// SurfaceHook 注册一个 surface 的 invariant set + state provider。
//
// Provider 是把 surface runtime state 映射为 ltllite.State 的回调。
// 例如 LSPToolSurface 的 provider 应返回:
//   - "is_typed_method": true (5 typed method)
//   - "read_only":       true (所有 5 method 都是 ReadOnly)
//   - "is_concurrent_safe": false (单 lsp_run instance 串行)
//   - "low_risk":        true (Risk=LOW)
type SurfaceHook struct {
	Name     string
	InvSet   ltllite.InvariantSet
	Provider func() ltllite.State
}

// HookRegistry 收集所有 surface hooks, 并发安全。
type HookRegistry struct {
	mu    sync.RWMutex
	hooks []SurfaceHook
}

// NewHookRegistry 构造空 registry。
func NewHookRegistry() *HookRegistry { return &HookRegistry{} }

// Register 注册一个 surface hook (按注册顺序评估)。
func (r *HookRegistry) Register(h SurfaceHook) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.hooks = append(r.hooks, h)
}

// Count 返回注册 hook 数。
func (r *HookRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.hooks)
}

// Prepare 阶段: 跑全量 invariant check。
//
// 返回 violations (空 = 全部成立); 调用方据此决定 turn 是否继续。
// 任一 violation 被 wrap 到 ErrInvariantViolation 让 errors.Is 识别。
func (r *HookRegistry) Prepare() error {
	r.mu.RLock()
	hooks := make([]SurfaceHook, len(r.hooks))
	copy(hooks, r.hooks)
	r.mu.RUnlock()

	var allViolations []ltllite.Violation
	for _, h := range hooks {
		if h.Provider == nil {
			continue
		}
		state := h.Provider()
		vs := ltllite.Check(h.InvSet, state)
		allViolations = append(allViolations, vs...)
	}
	if len(allViolations) == 0 {
		return nil
	}
	// wrap violations 到 error message
	msg := fmt.Sprintf("%d invariant violations:", len(allViolations))
	for _, v := range allViolations {
		msg += "\n  - " + v.String()
	}
	return fmt.Errorf("%w: %s", ErrInvariantViolation, msg)
}

// BeforeExecute 阶段: 单 surface.Execute 前的快速重检。
//
// 只跑指定 surfaceName 的 hook, 不影响其他 surface 的 invariant。
// 用于"invariant 是 dynamic 的, Execute 期间状态可能变化"的场景
// (e.g. tracker.LRUUsed 在不同 tick 间变化)。
func (r *HookRegistry) BeforeExecute(surfaceName string) error {
	r.mu.RLock()
	hooks := make([]SurfaceHook, len(r.hooks))
	copy(hooks, r.hooks)
	r.mu.RUnlock()

	for _, h := range hooks {
		if h.Name != surfaceName {
			continue
		}
		if h.Provider == nil {
			continue
		}
		state := h.Provider()
		vs := ltllite.Check(h.InvSet, state)
		if len(vs) == 0 {
			continue
		}
		msg := fmt.Sprintf("%s invariant violation:", surfaceName)
		for _, v := range vs {
			msg += "\n  - " + v.String()
		}
		return fmt.Errorf("%w: %s", ErrInvariantViolation, msg)
	}
	return nil
}

// PrepareTimed 返回 Prepare + 实测 latency (用于 benchmark + SLO 监控)。
func (r *HookRegistry) PrepareTimed() (time.Duration, error) {
	start := time.Now()
	err := r.Prepare()
	return time.Since(start), err
}
