package escape

import (
	"fmt"
	"sync"
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/plan"
)

// --- L1-15: PlanKindSwitchCount ExceedLimit ------------------------------------

// L1-15: 守护 PlanKindSwitchCount 4/5 累计边界 (design §5.2 SoT)
// 场景: Constrained policy (ExplorationPlan 起步), 4 次切换 → OK, 第 5 次 → ForceExit
func TestPlanKindSwitchCount_ExceedLimit(t *testing.T) {
	tracker := NewPlanKindSwitchTracker()
	sessionID := "sess-count-limit"

	// 第一次: KindUnset → ExplorationPlan, 首次建立 (不算切换), count=0
	d0 := tracker.RecordSwitch(sessionID, plan.ExplorationPlan)
	if !d0.Allowed || d0.Count != 0 {
		t.Errorf("first call: Allowed=%v Count=%d, want true/0", d0.Allowed, d0.Count)
	}

	// 第 1 次切换: ExplorationPlan → ProtocolPlan, count=1, OK
	d1 := tracker.RecordSwitch(sessionID, plan.ProtocolPlan)
	if !d1.Allowed || d1.Count != 1 || d1.Exceeded {
		t.Errorf("switch 1: Allowed=%v Count=%d Exceeded=%v, want true/1/false", d1.Allowed, d1.Count, d1.Exceeded)
	}

	// 第 2 次: ProtocolPlan → ExplorationPlan, count=2, OK
	d2 := tracker.RecordSwitch(sessionID, plan.ExplorationPlan)
	if !d2.Allowed || d2.Count != 2 {
		t.Errorf("switch 2: Allowed=%v Count=%d, want true/2", d2.Allowed, d2.Count)
	}

	// 第 3 次: ExplorationPlan → ProtocolPlan, count=3, OK
	d3 := tracker.RecordSwitch(sessionID, plan.ProtocolPlan)
	if !d3.Allowed || d3.Count != 3 {
		t.Errorf("switch 3: Allowed=%v Count=%d, want true/3", d3.Allowed, d3.Count)
	}

	// 第 4 次: ProtocolPlan → ExplorationPlan, count=4, OK (边界)
	d4 := tracker.RecordSwitch(sessionID, plan.ExplorationPlan)
	if !d4.Allowed || d4.Count != 4 {
		t.Errorf("switch 4: Allowed=%v Count=%d, want true/4 (boundary)", d4.Allowed, d4.Count)
	}

	// 第 5 次: ExplorationPlan → ProtocolPlan, count=5, ForceExit
	d5 := tracker.RecordSwitch(sessionID, plan.ProtocolPlan)
	if d5.Allowed || !d5.Exceeded || d5.Count != 5 {
		t.Errorf("switch 5: Allowed=%v Exceeded=%v Count=%d, want false/true/5", d5.Allowed, d5.Exceeded, d5.Count)
	}
	if d5.Policy != SwitchConstrained {
		t.Errorf("switch 5: Policy=%s, want constrained", d5.Policy)
	}
}

// --- L1-16: ZeroStart ---------------------------------------------------------

// L1-16: 守护 0 次切换 → OK (首次切换合法)
// 场景: KindUnset → 任何 Kind, 首次建立不算切换, count=0
func TestPlanKindSwitchCount_ZeroStart(t *testing.T) {
	tracker := NewPlanKindSwitchTracker()

	tests := []struct {
		name string
		kind plan.PlanKind
	}{
		{"Commitment", plan.CommitmentPlan},
		{"Protocol", plan.ProtocolPlan},
		{"Scenario", plan.ScenarioPlan},
		{"Exploration", plan.ExplorationPlan},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sid := "sess-zero-" + tc.name
			d := tracker.RecordSwitch(sid, tc.kind)
			if !d.Allowed {
				t.Errorf("first call (%s): Allowed=%v, want true", tc.name, d.Allowed)
			}
			if d.Count != 0 {
				t.Errorf("first call (%s): Count=%d, want 0", tc.name, d.Count)
			}
			if d.Exceeded {
				t.Errorf("first call (%s): Exceeded=true, want false", tc.name)
			}
		})
	}
}

// --- L1-17: Forbidden_NoSwitch ------------------------------------------------

// L1-17: 守护 CommitmentPlan 0 次切换 → OK
// 场景: KindUnset → CommitmentPlan, 首次建立, count=0, OK
func TestPlanKindSwitchPolicy_Forbidden_NoSwitch(t *testing.T) {
	tracker := NewPlanKindSwitchTracker()
	sid := "sess-forbidden-no"

	// 首次建立: KindUnset → CommitmentPlan
	d0 := tracker.RecordSwitch(sid, plan.CommitmentPlan)
	if !d0.Allowed || d0.Count != 0 || d0.Exceeded {
		t.Errorf("first call: Allowed=%v Count=%d Exceeded=%v, want true/0/false", d0.Allowed, d0.Count, d0.Exceeded)
	}
	if d0.Policy != SwitchForbidden {
		t.Errorf("first call: Policy=%s, want forbidden", d0.Policy)
	}

	// 同 Kind 重选: CommitmentPlan → CommitmentPlan, 不算切换, count 不变
	d1 := tracker.RecordSwitch(sid, plan.CommitmentPlan)
	if !d1.Allowed || d1.Count != 0 || d1.Exceeded {
		t.Errorf("same kind reselect: Allowed=%v Count=%d Exceeded=%v, want true/0/false", d1.Allowed, d1.Count, d1.Exceeded)
	}
}

// --- L1-18: Forbidden_OneSwitch -----------------------------------------------

// L1-18: 守护 CommitmentPlan 1 次切换 → ForceExit
// 场景: CommitmentPlan 起步, 任何一次切换都触发 Forbidden
func TestPlanKindSwitchPolicy_Forbidden_OneSwitch(t *testing.T) {
	tracker := NewPlanKindSwitchTracker()
	sid := "sess-forbidden-one"

	// 首次建立
	d0 := tracker.RecordSwitch(sid, plan.CommitmentPlan)
	if !d0.Allowed {
		t.Fatalf("first call should be allowed, got %+v", d0)
	}

	// 切换 1 次: CommitmentPlan → ProtocolPlan, 应 ForceExit
	d1 := tracker.RecordSwitch(sid, plan.ProtocolPlan)
	if d1.Allowed || !d1.Exceeded {
		t.Errorf("1st switch from Commitment: Allowed=%v Exceeded=%v, want false/true", d1.Allowed, d1.Exceeded)
	}
	if d1.Policy != SwitchForbidden {
		t.Errorf("1st switch: Policy=%s, want forbidden", d1.Policy)
	}
	if d1.Count != 1 {
		t.Errorf("1st switch: Count=%d, want 1", d1.Count)
	}
}

// --- L1-19: Constrained_Boundary ----------------------------------------------

// L1-19: 守护 Constrained 4/5 边界 (设计稿 §5.2 SoT: 4 次切换 → OK, 5 次切换 → ForceExit)
// 场景: ProtocolPlan (Constrained) 起步, 5 次切换全部走 ProtocolPlan <-> ExplorationPlan
func TestPlanKindSwitchPolicy_Constrained_Boundary(t *testing.T) {
	tracker := NewPlanKindSwitchTracker()
	sid := "sess-constrained-boundary"

	// 首次建立: ProtocolPlan
	d0 := tracker.RecordSwitch(sid, plan.ProtocolPlan)
	if !d0.Allowed {
		t.Fatalf("first call should be allowed, got %+v", d0)
	}

	// 4 次切换: 交替 Protocol <-> Exploration, 都应在 Constrained 下被允许
	for i := 1; i <= 4; i++ {
		newKind := plan.ExplorationPlan
		if i%2 == 0 {
			newKind = plan.ProtocolPlan
		}
		d := tracker.RecordSwitch(sid, newKind)
		if !d.Allowed || d.Exceeded {
			t.Errorf("switch %d to %s: Allowed=%v Exceeded=%v, want true/false", i, newKind, d.Allowed, d.Exceeded)
		}
		if d.Count != i {
			t.Errorf("switch %d: Count=%d, want %d", i, d.Count, i)
		}
	}

	// 第 5 次切换: 应 ForceExit
	d5 := tracker.RecordSwitch(sid, plan.ExplorationPlan)
	if d5.Allowed || !d5.Exceeded {
		t.Errorf("switch 5: Allowed=%v Exceeded=%v, want false/true (boundary violation)", d5.Allowed, d5.Exceeded)
	}
	if d5.Policy != SwitchConstrained {
		t.Errorf("switch 5: Policy=%s, want constrained", d5.Policy)
	}
}

// --- L1-20: Allowed_NoLimit ---------------------------------------------------

// L1-20: 守护 Allowed 无上限 (100 次同 Kind 重选 → count 永远 0)
// 场景: ScenarioPlan (Allowed) 起步, 100 次同 Kind 重选不计数
// 关键不变式: Allowed policy 不施加额外 count 上限 (与 Constrained ≤4 对比)
func TestPlanKindSwitchPolicy_Allowed_NoLimit(t *testing.T) {
	tracker := NewPlanKindSwitchTracker()
	sid := "sess-allowed-100"

	// 首次建立: ScenarioPlan
	d0 := tracker.RecordSwitch(sid, plan.ScenarioPlan)
	if !d0.Allowed {
		t.Fatalf("first call should be allowed, got %+v", d0)
	}

	// 100 次同 Kind 重选: 同 Kind 不算切换, count 永远 0
	for i := 0; i < 100; i++ {
		d := tracker.RecordSwitch(sid, plan.ScenarioPlan)
		if !d.Allowed || d.Exceeded {
			t.Errorf("reselect %d: Allowed=%v Exceeded=%v, want true/false", i, d.Allowed, d.Exceeded)
		}
	}

	// 验证: 100 次同 Kind 重选后 count 应为 0
	if count := tracker.GetCount(sid); count != 0 {
		t.Errorf("after 100 reselects: Count=%d, want 0 (same-kind = no count)", count)
	}

	// 验证: 切换出 ScenarioPlan → Allowed (因为 prev=ScenarioPlan), 不受 count 影响
	dSwitch := tracker.RecordSwitch(sid, plan.ProtocolPlan)
	if !dSwitch.Allowed || dSwitch.Exceeded {
		t.Errorf("switch out of Scenario: Allowed=%v Exceeded=%v, want true/false", dSwitch.Allowed, dSwitch.Exceeded)
	}
	if dSwitch.Policy != SwitchAllowed {
		t.Errorf("switch out of Scenario: Policy=%s, want allowed", dSwitch.Policy)
	}
}

// --- L1-21: PreReset_Boundary -------------------------------------------------

// L1-21: 守护 Reset 后首次切换合法 (累计 state 应被清空)
// 场景: 4 次切换后 Reset, 首次切换应 OK (count 重新从 0 开始)
func TestPlanKindSwitchPolicy_PreReset_Boundary(t *testing.T) {
	tracker := NewPlanKindSwitchTracker()
	sid := "sess-pre-reset"

	// 首次建立
	tracker.RecordSwitch(sid, plan.ProtocolPlan)

	// 累计 4 次切换 (达到 Constrained 上限)
	for i := 1; i <= 4; i++ {
		k := plan.ExplorationPlan
		if i%2 == 0 {
			k = plan.ProtocolPlan
		}
		tracker.RecordSwitch(sid, k)
	}

	// 第 5 次 → ForceExit
	d5 := tracker.RecordSwitch(sid, plan.ExplorationPlan)
	if !d5.Exceeded {
		t.Fatalf("pre-Reset: 5th switch should be Exceeded, got %+v", d5)
	}

	// Reset
	if !tracker.Reset(sid) {
		t.Fatal("Reset should return true for existing session")
	}

	// 验证 state 已清空
	if got := tracker.GetCount(sid); got != 0 {
		t.Errorf("after Reset: Count=%d, want 0", got)
	}
	if got := tracker.GetPrevKind(sid); got != plan.KindUnset {
		t.Errorf("after Reset: PrevKind=%d, want KindUnset=0", got)
	}

	// Reset 后首次建立 → OK (等同于首次调用, 不算切换)
	dReset := tracker.RecordSwitch(sid, plan.ExplorationPlan)
	if !dReset.Allowed || dReset.Count != 0 || dReset.Exceeded {
		t.Errorf("post-Reset first call: Allowed=%v Count=%d Exceeded=%v, want true/0/false", dReset.Allowed, dReset.Count, dReset.Exceeded)
	}

	// Reset 后第一次真正的切换 → OK, count=1 (前一次建立不计为切换)
	dSwitch := tracker.RecordSwitch(sid, plan.ProtocolPlan)
	if !dSwitch.Allowed || dSwitch.Count != 1 || dSwitch.Exceeded {
		t.Errorf("post-Reset first real switch: Allowed=%v Count=%d Exceeded=%v, want true/1/false", dSwitch.Allowed, dSwitch.Count, dSwitch.Exceeded)
	}
}

// --- L1-22: Concurrent --------------------------------------------------------

// L1-22: 守护并发安全 (100 并发同 Kind 重选, count 不变)
// 场景: 100 goroutine 同时对同一 session RecordSwitch 同 Kind, 无 race
// 关键不变式: 同 Kind 重选不计数, 即使高并发下 state 也应保持一致
func TestPlanKindSwitchPolicy_Concurrent(t *testing.T) {
	tracker := NewPlanKindSwitchTracker()
	sid := "sess-concurrent"

	// 首次建立: ProtocolPlan
	d0 := tracker.RecordSwitch(sid, plan.ProtocolPlan)
	if d0.Count != 0 {
		t.Fatalf("initial: Count=%d, want 0", d0.Count)
	}

	// 100 个 goroutine 并发做同 Kind 重选
	var wg sync.WaitGroup
	const goroutines = 100
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			tracker.RecordSwitch(sid, plan.ProtocolPlan)
		}()
	}
	wg.Wait()

	// 100 次同 Kind 重选 → count 应保持 0 (无 race-induced lost updates)
	if count := tracker.GetCount(sid); count != 0 {
		t.Errorf("concurrent same-kind: Count=%d, want 0 (race-safe)", count)
	}
	if prev := tracker.GetPrevKind(sid); prev != plan.ProtocolPlan {
		t.Errorf("concurrent same-kind: PrevKind=%d, want ProtocolPlan", prev)
	}
}

// L1-22b: 守护并发跨 session 隔离 (100 并发不同 session, state 不串)
// 场景: 100 goroutine 各自独立 session, 互不影响
func TestPlanKindSwitchPolicy_Concurrent_SessionIsolation(t *testing.T) {
	tracker := NewPlanKindSwitchTracker()

	var wg sync.WaitGroup
	const goroutines = 100
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(i int) {
			defer wg.Done()
			sid := fmt.Sprintf("sess-iso-%d", i)
			// 每个 session 首次建立 → count=0
			d := tracker.RecordSwitch(sid, plan.ProtocolPlan)
			if d.Count != 0 || !d.Allowed {
				t.Errorf("session %s: first call failed: %+v", sid, d)
			}
		}(i)
	}
	wg.Wait()

	// 验证: 100 个 session 都应处于 count=0, prev=ProtocolPlan 状态
	for i := 0; i < goroutines; i++ {
		sid := fmt.Sprintf("sess-iso-%d", i)
		if c := tracker.GetCount(sid); c != 0 {
			t.Errorf("session %s: Count=%d, want 0 (isolation)", sid, c)
		}
		if p := tracker.GetPrevKind(sid); p != plan.ProtocolPlan {
			t.Errorf("session %s: PrevKind=%d, want ProtocolPlan", sid, p)
		}
	}
}

// --- L1-23: Integration_With_Planner -----------------------------------------

// L1-23: 守护与 planner.MatchKind 集成 (DetermineSwitchPolicy 应与 MatchKind 输出兼容)
// 场景: MatchKind 输出的 4 种 PlanKind 都能正确映射到对应 policy
func TestPlanKindSwitchPolicy_Integration_With_Planner(t *testing.T) {
	// 模拟 MatchKind 4 种输出 (PR-B1 MatchKind_4Rules)
	scenarios := []struct {
		name        string
		quantized   string
		stepCount   int
		anomalies   int
		wantKind    plan.PlanKind
		wantPolicy  PlanKindSwitchPolicy
	}{
		// Rule 1: orchestrate / anomalies>=3 → ExplorationPlan → Constrained
		{"orchestrate", "intent_orchestrate", 5, 0, plan.ExplorationPlan, SwitchConstrained},
		{"high-anomaly", "intent_command", 5, 3, plan.ExplorationPlan, SwitchConstrained},
		// Rule 2: stepCount==1 → CommitmentPlan → Forbidden
		{"single-step", "intent_command", 1, 0, plan.CommitmentPlan, SwitchForbidden},
		// Rule 3: command / steps<=3 → ProtocolPlan → Constrained
		{"multi-step-command", "intent_command", 3, 0, plan.ProtocolPlan, SwitchConstrained},
		{"multi-step-small", "intent_query", 3, 0, plan.ProtocolPlan, SwitchConstrained},
		// Rule 4: default → ScenarioPlan → Allowed
		{"default-scenario", "intent_query", 10, 0, plan.ScenarioPlan, SwitchAllowed},
	}

	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			kind := plan.MatchKind(sc.quantized, sc.stepCount, sc.anomalies)
			if kind != sc.wantKind {
				t.Fatalf("MatchKind(%q, %d, %d)=%s, want %s", sc.quantized, sc.stepCount, sc.anomalies, kind, sc.wantKind)
			}
			gotPolicy := DetermineSwitchPolicy(kind)
			if gotPolicy != sc.wantPolicy {
				t.Errorf("DetermineSwitchPolicy(%s)=%s, want %s", kind, gotPolicy, sc.wantPolicy)
			}
		})
	}
}

// --- L1-24: SameKindSwitch -----------------------------------------------------

// L1-24: 守护"同 Kind 重选"语义 (重选不计数 vs 误计数)
// 场景: 连续多次 RecordSwitch 同一 Kind, count 应保持不变
func TestPlanKindSwitchPolicy_EdgeCase_SameKindSwitch(t *testing.T) {
	tracker := NewPlanKindSwitchTracker()
	sid := "sess-same-kind"

	// 首次建立
	d0 := tracker.RecordSwitch(sid, plan.ExplorationPlan)
	if d0.Count != 0 {
		t.Fatalf("first call: Count=%d, want 0", d0.Count)
	}

	// 连续 5 次同 Kind 重选
	for i := 1; i <= 5; i++ {
		d := tracker.RecordSwitch(sid, plan.ExplorationPlan)
		if d.Count != 0 {
			t.Errorf("same-kind reselect %d: Count=%d, want 0 (no switch)", i, d.Count)
		}
		if !d.Allowed {
			t.Errorf("same-kind reselect %d: Allowed=%v, want true", i, d.Allowed)
		}
	}

	// 最后真正切换一次: count 应只 +1 (累积 5 次同 Kind 重选不算)
	dFinal := tracker.RecordSwitch(sid, plan.ProtocolPlan)
	if dFinal.Count != 1 {
		t.Errorf("after 5x same-kind + 1 real switch: Count=%d, want 1", dFinal.Count)
	}
}

// --- L1-25: Allowed_NoUpperLimit -----------------------------------------------

// L1-25: 守护 Allowed 1000 次同 Kind 重选仍 OK (极端边界)
// 场景: ScenarioPlan 起步, 1000 次同 Kind 重选 → 全部 OK, count 永远 0
// 关键不变式: Allowed policy 不施加 count 上限 (与 Constrained ≤4 形成对照)
func TestPlanKindSwitchPolicy_Allowed_NoUpperLimit(t *testing.T) {
	tracker := NewPlanKindSwitchTracker()
	sid := "sess-allowed-1000"

	// 首次建立: ScenarioPlan
	tracker.RecordSwitch(sid, plan.ScenarioPlan)

	const reselects = 1000
	allowedCount := 0
	for i := 0; i < reselects; i++ {
		d := tracker.RecordSwitch(sid, plan.ScenarioPlan)
		if d.Allowed && !d.Exceeded {
			allowedCount++
		}
	}

	if allowedCount != reselects {
		t.Errorf("Allowed scenario: %d/%d reselects allowed, want all %d", allowedCount, reselects, reselects)
	}
	if final := tracker.GetCount(sid); final != 0 {
		t.Errorf("final count after 1000 reselects: %d, want 0 (same-kind no count)", final)
	}
}

// --- DetermineSwitchPolicy basic table tests -----------------------------------

// Extra: 守护 DetermineSwitchPolicy 4 个 PlanKind 都映射正确
func TestDetermineSwitchPolicy_AllKinds(t *testing.T) {
	tests := []struct {
		kind  plan.PlanKind
		want  PlanKindSwitchPolicy
	}{
		{plan.ExplorationPlan, SwitchConstrained},
		{plan.ScenarioPlan, SwitchAllowed},
		{plan.ProtocolPlan, SwitchConstrained},
		{plan.CommitmentPlan, SwitchForbidden},
		{plan.KindUnset, SwitchConstrained}, // unknown → conservative
		{plan.PlanKind(99), SwitchConstrained}, // truly unknown → conservative
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("kind=%d", tc.kind), func(t *testing.T) {
			got := DetermineSwitchPolicy(tc.kind)
			if got != tc.want {
				t.Errorf("DetermineSwitchPolicy(%d)=%s, want %s", tc.kind, got, tc.want)
			}
		})
	}
}

// --- Reset() and Get methods --------------------------------------------------

// Extra: Reset() 清空指定 session
func TestPlanKindSwitchTracker_Reset(t *testing.T) {
	tracker := NewPlanKindSwitchTracker()

	tracker.RecordSwitch("sess-r1", plan.ProtocolPlan)
	// sess-r2 做一个真实切换 (不是首次建立), 让 count > 0
	tracker.RecordSwitch("sess-r2", plan.ScenarioPlan)
	tracker.RecordSwitch("sess-r2", plan.ProtocolPlan) // 真实切换, count=1

	if !tracker.Reset("sess-r1") {
		t.Error("Reset(sess-r1) should return true")
	}
	if tracker.GetCount("sess-r1") != 0 {
		t.Errorf("after Reset: sess-r1 Count=%d, want 0", tracker.GetCount("sess-r1"))
	}
	if tracker.GetPrevKind("sess-r1") != plan.KindUnset {
		t.Errorf("after Reset: sess-r1 PrevKind=%d, want 0", tracker.GetPrevKind("sess-r1"))
	}
	// sess-r2 不应被 reset, count 应为 1
	if got := tracker.GetCount("sess-r2"); got != 1 {
		t.Errorf("sess-r2 should not be reset: Count=%d, want 1", got)
	}

	// Reset 不存在的 session → false
	if tracker.Reset("non-existent") {
		t.Error("Reset(non-existent) should return false")
	}
}

// Extra: ResetAll() 清空所有 sessions
func TestPlanKindSwitchTracker_ResetAll(t *testing.T) {
	tracker := NewPlanKindSwitchTracker()

	tracker.RecordSwitch("sess-ra1", plan.ProtocolPlan)
	tracker.RecordSwitch("sess-ra2", plan.ScenarioPlan)
	tracker.RecordSwitch("sess-ra3", plan.ExplorationPlan)

	tracker.ResetAll()

	for _, sid := range []string{"sess-ra1", "sess-ra2", "sess-ra3"} {
		if c := tracker.GetCount(sid); c != 0 {
			t.Errorf("after ResetAll: %s Count=%d, want 0", sid, c)
		}
	}
}

// Extra: GetCount/GetPrevKind 未注册 session 返回零值
func TestPlanKindSwitchTracker_GetUnregistered(t *testing.T) {
	tracker := NewPlanKindSwitchTracker()

	if c := tracker.GetCount("never-seen"); c != 0 {
		t.Errorf("GetCount(never-seen)=%d, want 0", c)
	}
	if k := tracker.GetPrevKind("never-seen"); k != plan.KindUnset {
		t.Errorf("GetPrevKind(never-seen)=%d, want KindUnset", k)
	}
}

// --- ToEscapeDecision integration ---------------------------------------------

// Extra: SwitchDecision.ToEscapeDecision 转换语义
func TestSwitchDecision_ToEscapeDecision(t *testing.T) {
	sid := "sess-to-escape"

	// Allowed case → EscapeContinue
	dAllowed := SwitchDecision{Allowed: true, Count: 2, Policy: SwitchConstrained}
	escAllowed := dAllowed.ToEscapeDecision(sid, plan.ProtocolPlan)
	if escAllowed.Action != EscapeContinue {
		t.Errorf("allowed → Action=%s, want continue", escAllowed.Action)
	}
	if escAllowed.AuditLevel != 0 {
		t.Errorf("allowed → AuditLevel=%d, want 0", escAllowed.AuditLevel)
	}
	if escAllowed.SessionID != sid {
		t.Errorf("allowed → SessionID=%q, want %q", escAllowed.SessionID, sid)
	}

	// Exceeded case → EscapeForceExit + AuditLevel=2
	dExceeded := SwitchDecision{Allowed: false, Count: 5, Exceeded: true, Policy: SwitchConstrained}
	escExceeded := dExceeded.ToEscapeDecision(sid, plan.ExplorationPlan)
	if escExceeded.Action != EscapeForceExit {
		t.Errorf("exceeded → Action=%s, want force_exit", escExceeded.Action)
	}
	if escExceeded.AuditLevel != 2 {
		t.Errorf("exceeded → AuditLevel=%d, want 2", escExceeded.AuditLevel)
	}
}