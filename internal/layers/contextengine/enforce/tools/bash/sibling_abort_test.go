// T: D7-S9-A50-T25 — BashSiblingAbortController 单测 (DM-20260702-009 PR-F).
//
// 9 个单测覆盖 lifecycle / 边界 / 并发:
//   1. TestSiblingAbort_RegisterAndUnregister    — basic lifecycle
//   2. TestSiblingAbort_FailureAbortsSiblings   — 1 个失败 → 其它 ctx.Done()
//   3. TestSiblingAbort_SelfIDNotAborted        — AbortSiblings(self) 不 cancel 自身
//   4. TestSiblingAbort_Idempotent              — 第二次 AbortSiblings 返 false
//   5. TestSiblingAbort_RegisterAfterAbortDenied — abort 后拒绝新注册
//   6. TestSiblingAbort_ConcurrentRegAndAbort   — 并发注册 + abort 无 deadlock / panic
//   7. TestSiblingAbort_ParentCtxCancel         — parent ctx 取消传播到所有 sibling ctx
//   8. TestSiblingAbort_CloseCancelsAll         — Close 后所有 sibling cancel
//   9. TestSiblingAbort_MaxSiblingsCap          — 注册表满后拒绝新注册
//  10. TestSiblingAbort_DefaultMaxSiblingsIs64  — 默认 max=64
package bash

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// 1. basic lifecycle: Register → sibling count++ → Unregister → count--.
func TestSiblingAbort_RegisterAndUnregister(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := NewBashSiblingAbortController(parent, 0)
	defer c.Close()

	_, _, ok1 := c.Register("call_a", "bash")
	if !ok1 {
		t.Fatal("first Register must succeed")
	}
	if got := c.SiblingCount(); got != 1 {
		t.Errorf("after 1 Register, SiblingCount=%d, want 1", got)
	}

	_, _, ok2 := c.Register("call_b", "bash")
	if !ok2 {
		t.Error("second Register must succeed")
	}
	if got := c.SiblingCount(); got != 2 {
		t.Errorf("after 2 Register, SiblingCount=%d, want 2", got)
	}

	c.Unregister("call_a")
	if got := c.SiblingCount(); got != 1 {
		t.Errorf("after Unregister, SiblingCount=%d, want 1", got)
	}
}

// 2. AC12 core invariant: a sibling call's failure should cancel all other
// siblings' ctx. The failing call itself is allowed to "finish" (its
// goroutine already exited; we only cancel others).
func TestSiblingAbort_FailureAbortsSiblings(t *testing.T) {
	parent, cancelParent := context.WithCancel(context.Background())
	defer cancelParent()

	c := NewBashSiblingAbortController(parent, 0)
	defer c.Close()

	// 3 个 sibling, 模拟 A 失败 → B/C 看到 ctx.Done().
	ctxA, cancelA, ok := c.Register("call_a", "bash")
	if !ok {
		t.Fatal("Register A failed")
	}
	ctxB, cancelB, ok := c.Register("call_b", "bash")
	if !ok {
		t.Fatal("Register B failed")
	}
	ctxC, cancelC, ok := c.Register("call_c", "bash")
	if !ok {
		t.Fatal("Register C failed")
	}
	_ = cancelA

	// A 失败:
	if !c.AbortSiblings("call_a", "bash exit 1") {
		t.Error("first AbortSiblings must return true")
	}
	if !c.Aborted() {
		t.Error("after AbortSiblings, Aborted() must be true")
	}
	if r := c.AbortReason(); r != "bash exit 1" {
		t.Errorf("AbortReason=%q, want %q", r, "bash exit 1")
	}

	// B/C 的 sibling ctx 必须 Done (controller 内部 cancelFn 已触发).
	select {
	case <-ctxB.Done():
		// pass — B 看到兄弟 abort
	case <-time.After(100 * time.Millisecond):
		t.Error("sibling B ctx must be cancelled after AbortSiblings")
	}
	select {
	case <-ctxC.Done():
		// pass — C 看到兄弟 abort
	case <-time.After(100 * time.Millisecond):
		t.Error("sibling C ctx must be cancelled after AbortSiblings")
	}

	// A 自身 ctx **不**应被 cancel (SelfIDNotAborted 不变).
	select {
	case <-ctxA.Done():
		t.Error("sibling A (self) must NOT be cancelled by AbortSiblings(self)")
	case <-time.After(20 * time.Millisecond):
		// pass — A 的 ctx 仍 alive
	}

	// Wrap-cancel 是 sync.Once-wrapped, 重复调用是 no-op, 不 panic.
	cancelB()
	cancelC()

	// Idempotency: 第二次 AbortSiblings 返 false.
	if c.AbortSiblings("call_b", "second") {
		t.Error("second AbortSiblings must return false (already aborted)")
	}
	if r := c.AbortReason(); r != "bash exit 1" {
		t.Errorf("AbortReason=%q, want %q (first call's reason preserved)", r, "bash exit 1")
	}

	// AbortSiblings 不 unregister: SiblingCount 仍是 3 (caller 自己清理).
	if c.SiblingCount() != 3 {
		t.Errorf("AbortSiblings does not unregister; SiblingCount=%d, want 3", c.SiblingCount())
	}
	c.Unregister("call_a")
	c.Unregister("call_b")
	c.Unregister("call_c")
}

// 3. abort(self) must NOT cancel self (self already finished).
func TestSiblingAbort_SelfIDNotAborted(t *testing.T) {
	parent, cancelParent := context.WithCancel(context.Background())
	defer cancelParent()

	c := NewBashSiblingAbortController(parent, 0)
	defer c.Close()

	ctxSelf, cancelSelf, ok := c.Register("call_a", "bash")
	if !ok {
		t.Fatal("Register A failed")
	}
	ctxB, _, ok := c.Register("call_b", "bash")
	if !ok {
		t.Fatal("Register B failed")
	}

	// A 自己 abort (尽管正常不会发生, 但合约: 不 cancel self).
	if !c.AbortSiblings("call_a", "self") {
		t.Error("AbortSiblings(self) must succeed")
	}

	// self 的 ctx **不**应被 cancel.
	select {
	case <-ctxSelf.Done():
		t.Error("self ctx must NOT be cancelled by AbortSiblings(self)")
	case <-time.After(20 * time.Millisecond):
		// pass
	}

	// B (其它 sibling) 应该被 cancel.
	select {
	case <-ctxB.Done():
		// pass
	case <-time.After(50 * time.Millisecond):
		t.Error("sibling B ctx must be cancelled when A aborts")
	}

	// SiblingCount: A 还在注册表里 (没 Unregister), B 也还在.
	// AbortSiblings 不主动 unregister.
	if c.SiblingCount() != 2 {
		t.Errorf("AbortSiblings does not unregister; SiblingCount=%d, want 2", c.SiblingCount())
	}

	c.Unregister("call_a")
	c.Unregister("call_b")
	_ = cancelSelf
}

// 4. abort is idempotent — second call returns false.
func TestSiblingAbort_Idempotent(t *testing.T) {
	parent, cancelParent := context.WithCancel(context.Background())
	defer cancelParent()

	c := NewBashSiblingAbortController(parent, 0)
	defer c.Close()

	c.Register("call_a", "bash")
	c.Register("call_b", "bash")

	if !c.AbortSiblings("call_a", "first") {
		t.Error("first AbortSiblings must succeed")
	}
	if c.AbortSiblings("call_b", "second") {
		t.Error("second AbortSiblings must return false (already aborted)")
	}
	if r := c.AbortReason(); r != "first" {
		t.Errorf("AbortReason=%q, want %q (first call's reason preserved)", r, "first")
	}
}

// 5. abort 后再 Register 必须失败 (省一次无意义的 goroutine 启动).
func TestSiblingAbort_RegisterAfterAbortDenied(t *testing.T) {
	parent, cancelParent := context.WithCancel(context.Background())
	defer cancelParent()

	c := NewBashSiblingAbortController(parent, 0)
	defer c.Close()

	c.Register("call_a", "bash")
	c.AbortSiblings("call_a", "failure")

	ctx, cancel, ok := c.Register("call_x", "bash")
	if ok || ctx != nil || cancel != nil {
		t.Error("Register after AbortSiblings must be denied (return nil, nil, false)")
	}
}

// 6. 并发注册 + abort: 没有 deadlock / panic.
// 这是 design.md 风险章节明确点出的 "sync.Mutex 死锁" 边界.
func TestSiblingAbort_ConcurrentRegAndAbort(t *testing.T) {
	parent, cancelParent := context.WithCancel(context.Background())
	defer cancelParent()

	c := NewBashSiblingAbortController(parent, 32)
	defer c.Close()

	var wg sync.WaitGroup
	const workers = 20

	// 一半 worker 在 Register + sleep + Unregister
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := string(rune('A' + i))
			_, cancel, ok := c.Register(id, "bash")
			if !ok {
				return
			}
			time.Sleep(time.Duration(i) * time.Microsecond)
			_ = cancel
			c.Unregister(id)
		}(i)
	}

	// 一个 worker 试图 abort 在并发 Register 中.
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(5 * time.Microsecond)
		c.AbortSiblings("", "race")
	}()

	wg.Wait()

	// 不崩溃 = 通过.
	t.Logf("concurrent reg+abort completed without deadlock; Aborted=%v, count=%d",
		c.Aborted(), c.SiblingCount())
}

// 7. parent ctx cancel 传播到所有 sibling ctx (因为 sibling ctx 是
// derived from parentCtx). AbortSiblings 仍返回 true (controller 不追踪
// parent 取消, 只看 closed/aborted).
func TestSiblingAbort_ParentCtxCancel(t *testing.T) {
	parent, cancelParent := context.WithCancel(context.Background())

	c := NewBashSiblingAbortController(parent, 0)
	defer c.Close()

	ctxA, _, ok := c.Register("call_a", "bash")
	if !ok {
		t.Fatal("Register A failed")
	}
	ctxB, _, ok := c.Register("call_b", "bash")
	if !ok {
		t.Fatal("Register B failed")
	}

	cancelParent()

	// Parent cancel 通过 context 派生链传播到所有 sibling ctx.
	select {
	case <-ctxA.Done():
		// pass — parent cancel propagated to sibling ctx A
	case <-time.After(100 * time.Millisecond):
		t.Error("sibling ctx A must be cancelled when parent ctx cancels")
	}
	select {
	case <-ctxB.Done():
		// pass
	case <-time.After(100 * time.Millisecond):
		t.Error("sibling ctx B must be cancelled when parent ctx cancels")
	}

	// Parent cancel 不 unregister; SiblingCount 仍是 2.
	if c.SiblingCount() != 2 {
		t.Errorf("parent cancel should not affect SiblingCount, got %d", c.SiblingCount())
	}

	// AbortSiblings 仍应返 true: controller 未 closed 未 aborted.
	// 合约: parent cancel 不影响 AbortSiblings, 因为 sibling ctx 已经
	// 通过 context 派生链 Done 了 — AbortSiblings 是 sibling-only abort.
	if !c.AbortSiblings("call_a", "after-parent-cancel") {
		t.Error("AbortSiblings must return true even after parent cancel (controller not closed/aborted)")
	}
}

// 8. Close cancels all remaining sibling ctxs.
func TestSiblingAbort_CloseCancelsAll(t *testing.T) {
	parent, cancelParent := context.WithCancel(context.Background())
	defer cancelParent()

	c := NewBashSiblingAbortController(parent, 0)
	ctxA, _, ok := c.Register("call_a", "bash")
	if !ok {
		t.Fatal("Register A failed")
	}
	ctxB, _, ok := c.Register("call_b", "bash")
	if !ok {
		t.Fatal("Register B failed")
	}
	ctxC, _, ok := c.Register("call_c", "bash")
	if !ok {
		t.Fatal("Register C failed")
	}

	c.Close()

	// Close 后所有 sibling ctx 都 Done.
	for name, ctx := range map[string]context.Context{
		"call_a": ctxA, "call_b": ctxB, "call_c": ctxC,
	} {
		select {
		case <-ctx.Done():
			// pass
		case <-time.After(50 * time.Millisecond):
			t.Errorf("%s ctx must be Done after Close", name)
		}
	}

	// Close 后再次 Register 必须失败.
	_, _, ok = c.Register("call_x", "bash")
	if ok {
		t.Error("Register after Close must fail")
	}
	// 第二次 Close 是 no-op, 不 panic.
	c.Close()
	c.Close()
}

// 9. 注册表满后拒绝新注册.
func TestSiblingAbort_MaxSiblingsCap(t *testing.T) {
	parent, cancelParent := context.WithCancel(context.Background())
	defer cancelParent()

	c := NewBashSiblingAbortController(parent, 3)
	defer c.Close()

	if _, _, ok := c.Register("call_a", "bash"); !ok {
		t.Error("1st Register must succeed")
	}
	if _, _, ok := c.Register("call_b", "bash"); !ok {
		t.Error("2nd Register must succeed")
	}
	if _, _, ok := c.Register("call_c", "bash"); !ok {
		t.Error("3rd Register must succeed")
	}
	// 4th 注册必须被拒绝 (3 个 cap).
	if ctx, cancel, ok := c.Register("call_d", "bash"); ok || ctx != nil || cancel != nil {
		t.Error("4th Register must be denied (cap=3)")
	}
	if c.SiblingCount() != 3 {
		t.Errorf("SiblingCount=%d, want 3", c.SiblingCount())
	}
}

// Default capacity: 0 → 默认 64.
func TestSiblingAbort_DefaultMaxSiblingsIs64(t *testing.T) {
	parent, cancelParent := context.WithCancel(context.Background())
	defer cancelParent()

	c := NewBashSiblingAbortController(parent, 0)
	if c.maxSib != 64 {
		t.Errorf("default maxSib=%d, want 64", c.maxSib)
	}
	c.Close()
}

// atomic.Int32 sentinel — keep `sync/atomic` import live even if tests
// evolve to drop the closure-based atomic ops in future refactors.
var _ atomic.Int32