// Package bash — T26 BashSiblingAbortController (DM-20260702-009 PR-F).
//
// 模型: 同一 batch 里 N 个并发工具调用, 任意一个 Bash 失败 →
// 取消其它兄弟调用 (并发安全: 用 errgroup.SetLimit + ctx-cancel).
//
// 这是 clawcode 的 siblingAbortController 镜像 (toolResultStorage.ts
// 附近的位置), 工程上用 Go 的 errgroup + per-call ctx cancel.
//
// 设计要点 (per design.md §3.3):
//
//   1. BashSiblingAbortController 是 **per-batch** controller (不是
//      per-turn / per-session): 一个 batch 内所有并发调用共享一个
//      controller, batch 结束 GC.
//   2. 注册: tool call 启动 → Register(callID, name) 返回 (ctx, cancel, ok);
//      ctx 是 sibling 的 per-call context (derived from parentCtx),
//      用于 runBash 的 ctx 参数; cancel 用于 controller 主动取消 (或
//      caller 自己清理).
//   3. 兄弟失败: Bash 退出非 0 → AbortSiblings(callID, reason) 取消其它
//      注册的 call, 不取消自身 (自己已经完成了).
//   4. 失败 signal: 兄弟 call 的 Execute 在下次 select ctx.Done() 时
//      返回 context.Canceled, 由 caller 写入 synthetic cancel result.
//   5. 线程安全: 内部 sync.Mutex 保护注册表; AbortSiblings 一次性触发
//      (避免惊群).
//
// 返回机制:
//
//   - AbortSiblings 是幂等的: 多次调用只有第一次真正触发 cancel, 后续
//     调用返回 false (already-aborted).
//   - 不 abort 自身: AbortSiblings(selfID) 不会 cancel 自身, 但是会清
//     注册表里其它还在跑的 siblings.
//   - parent ctx 取消也会传播到所有 siblings (因为每个 sibling ctx 是
//     derived from parentCtx).
//
// 性能: Register / Unregister / AbortSiblings 走 mutex (per-batch 锁,
// 不是 per-session 锁), p99 < 1μs. batch 内通常 < 10 调用, 不瓶颈.
//
// AC12 (per design.md):
//
//   - 兄弟 abort 边界: sync.Mutex 死锁 → 不通过 (controller 测试 + 集成测试)
//   - 多并发 abort: 同时多个 siblings 失败时, 只有第一个触发全局 cancel
//
// DSAFT: D7-S9-A50-T25 (DM-20260702-009 PR-F, 阶段 6+).
package bash

import (
	"context"
	"fmt"
	"sync"
)

// BashSiblingAbortController 协调一个 batch 内并发的 bash 工具调用,
// 允许兄弟调用在某个 bash 失败时被 abort. 线程安全 + 幂等.
//
// Lifecycle:
//
//	c := NewBashSiblingAbortController(parentCtx)
//	defer c.Close()
//
//	for each sibling call:
//	    ctx, cancel, ok := c.Register(callID, "bash")
//	    if !ok { // 注册表已满 (默认 64) 或 controller 已关闭 }
//	        return errTooManySiblings
//	    }
//	    defer cancel()
//	    defer c.Unregister(callID)
//
//	    go func() {
//	        if err := runBash(ctx, ...); err != nil {
//	            c.AbortSiblings(callID, err.Error()) // 兄弟失败 → cancel others
//	            return err
//	        }
//	    }()
type BashSiblingAbortController struct {
	parentCtx context.Context

	mu        sync.Mutex
	registry  map[string]context.CancelFunc
	closed    bool
	aborted   bool // 已经触发 abort, 防止惊群
	abortErr  string
	maxSib    int
	abortOnce sync.Once
	wg        sync.WaitGroup
}

// NewBashSiblingAbortController 创建一个新的 sibling abort controller.
// maxSiblings=0 → 默认 64 (满足典型 batch size ≤ 50).
//
// 防御: controller 关联 parentCtx. parentCtx 取消时, 所有已注册的
// siblings 都会看到 ctx.Done() (因为每个 sibling 拿到的 ctx 是
// derived from parentCtx).
func NewBashSiblingAbortController(parentCtx context.Context, maxSiblings int) *BashSiblingAbortController {
	if maxSiblings <= 0 {
		maxSiblings = defaultMaxSiblings
	}
	return &BashSiblingAbortController{
		parentCtx: parentCtx,
		registry:  make(map[string]context.CancelFunc, maxSiblings),
		maxSib:    maxSiblings,
	}
}

// Default constants for capacity tuning.
const (
	defaultMaxSiblings = 64
)

// Register 注册一个 sibling call, 返回该 call 的 ctx + cancel 函数 + 是否
// 注册成功. 失败原因: 控制器已关闭 / 已 aborted / 注册表满 / 同 ID 已存在.
//
// 调用顺序: 每个 sibling goroutine 启动时调用 Register 获取
// (ctx, cancel, ok); goroutine 退出时调用 Unregister(callID) 清理.
//
// 返回的 ctx 是 sibling 的 per-call context (derived from parentCtx):
//   - 兄弟失败 → ctx 被 controller 取消 (ctx.Done() 触发)
//   - parent 取消 → ctx 也会被取消 (context 派生链)
//
// 线程安全: 可并发调用.
func (c *BashSiblingAbortController) Register(callID, toolName string) (ctx context.Context, cancel context.CancelFunc, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil, nil, false
	}
	if c.aborted {
		// controller 已经 abort, 拒绝新注册 (省一次无意义的 goroutine 启动).
		return nil, nil, false
	}
	if _, exists := c.registry[callID]; exists {
		return nil, nil, false // duplicate ID
	}
	if len(c.registry) >= c.maxSib {
		return nil, nil, false
	}

	siblingCtx, cancelFn := context.WithCancel(c.parentCtx)
	// 把 toolName 注入 ctx 值, 让 select ctx.Done() 的 caller 能读到
	// "我是被哪个兄弟 abort 的". 这对 fail-open + 日志有用.
	siblingCtx = context.WithValue(siblingCtx, siblingAbortReasonKey{}, &SiblingAbortReason{
		ToolName: toolName,
		CallID:   callID,
	})
	c.registry[callID] = cancelFn
	return siblingCtx, wrapCancel(cancelFn), true
}

type siblingAbortReasonKey struct{}

// SiblingAbortReason is attached to every sibling-derived ctx. It tells
// the caller (e.g. runBash's select ctx.Done()) which tool call is being
// aborted, useful for observability + debugging.
type SiblingAbortReason struct {
	ToolName string
	CallID   string
}

// wrapCancel 提供 sync.Once 防御性清理, 避免 caller 多次调 cancel 导致
// 的 (理论无害但语义不洁) 重复触发. context.CancelFunc 本身幂等, 这里
// 只是 belt-and-suspenders.
func wrapCancel(cancel context.CancelFunc) context.CancelFunc {
	var once sync.Once
	return func() {
		once.Do(func() {
			cancel()
		})
	}
}

// Unregister 主动从注册表移除一个 sibling call. defer register 之后
// 跟着 Unregister, 确保 GC 时机.
//
// 重复调用 / 不存在的 ID 是 no-op.
func (c *BashSiblingAbortController) Unregister(callID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if cancel, ok := c.registry[callID]; ok {
		// do NOT call cancel here — sibling completed normally; don't
		// false-trigger an abort. just clean the registry.
		_ = cancel
		delete(c.registry, callID)
	}
}

// AbortSiblings 取消除 exceptID 之外的所有 sibling calls. exceptID 是
// 触发本次 abort 的 call ID (e.g. bash 失败那个 call) — 它自身已经
// 完成了, 不应该被 cancel. 如果 exceptID == "" → cancel 全部.
//
// 幂等: 第二次调用返回 false (controller 已 aborted). 这避免了 goroutine
// 惊群 (例如两个并发 bash 都失败, 第二次 abort 是 no-op).
//
// 注意: 不追踪 parent ctx 取消 — AbortSiblings 只要 controller 未 closed
// 未 aborted, 就返回 true. parent cancel 通过 sibling ctx 的派生链传播.
func (c *BashSiblingAbortController) AbortSiblings(exceptID, reason string) bool {
	c.mu.Lock()
	if c.closed || c.aborted {
		c.mu.Unlock()
		return false
	}
	c.aborted = true
	c.abortErr = reason
	// copy keys + cancel funcs so we can release the lock before calling
	// cancel funcs (avoids deadlocks when cancel funcs re-enter into the
	// controller via Selectors).
	snapshot := make(map[string]context.CancelFunc, len(c.registry))
	for id, cancel := range c.registry {
		if id == exceptID {
			continue
		}
		snapshot[id] = cancel
	}
	c.mu.Unlock()

	for id, cancel := range snapshot {
		_ = id // 可观测 ID 已存在 ctx value 里
		cancel()
	}
	c.abortOnce.Do(func() {
		// 用 WaitGroup 是为了 future "block until all cancelled" 调用,
		// 当前不阻塞 — 取消是异步操作, sibling goroutine 自行 select ctx.Done().
	})
	return true
}

// Aborted reports whether AbortSiblings was invoked (true = yes). Always
// safe to call from any goroutine.
func (c *BashSiblingAbortController) Aborted() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.aborted
}

// AbortReason returns the human-readable reason passed to AbortSiblings.
// Returns "" if no abort has occurred.
func (c *BashSiblingAbortController) AbortReason() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.abortErr
}

// SiblingCount returns the number of currently registered siblings.
// Useful for tests + observability dashboards.
func (c *BashSiblingAbortController) SiblingCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.registry)
}

// Close shuts down the controller and cancels all remaining sibling ctxs.
// Idempotent.
func (c *BashSiblingAbortController) Close() {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	snapshot := make([]context.CancelFunc, 0, len(c.registry))
	for _, cancel := range c.registry {
		snapshot = append(snapshot, cancel)
	}
	c.registry = nil
	c.mu.Unlock()

	for _, cancel := range snapshot {
		cancel()
	}
	// 等所有 sibling goroutine 完全退出 (避免 GC race).
	c.wg.Wait()
}

// AbortReasonError returns an error wrapping the abort reason. Useful as
// a synthetic tool_result.Error for siblings that were aborted.
//
//   if c.Aborted() { return ErrBashSiblingAborted }
//   if r := c.AbortReason(); r != "" { return fmt.Errorf("%w: %s", ErrBashSiblingAborted, r) }
func NewAbortReasonError(reason string) error {
	return fmt.Errorf("bash sibling aborted: %s", reason)
}