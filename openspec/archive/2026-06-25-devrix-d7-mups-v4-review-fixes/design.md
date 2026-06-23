# Design: D7 MUPS v4 Review 修复

**Change ID:** devrix-d7-mups-v4-review-fixes
**Demand ID:** DM-20260625-002
**Status:** S4-Implementing

---

## 1. 修复总览

本 change 包含 **14 个独立 fix**（3 Critical + 10 High + 1 doc），全部为 isolated change，无 cross-module 协调。按 5 节点管道节点组织：

| Fix | 节点 | 严重度 | 文件 | 行数 |
|-----|------|--------|------|------|
| 1. clamp01OrFallback NaN | Verify | Critical | workmodel/aggregate_verdicts.go | +5 |
| 2. aggregateMeta 溯源 | Verify | Critical | workmodel/aggregate_verdicts.go | +60 |
| 3. rollback context 隔离 | Execute | Critical | execute/channel_protocol.go | +20 |
| 4. PersistScope fail-fast | Plan | High | plan/plan_struct.go | +5 |
| 5. NewPlanID UUID+SHA256 | Plan | High | plan/planner.go | +50 |
| 6. ErrChannelStepInvalid/Timeout | Execute | High | execute/errors.go | +34 |
| 7. CommitChannel 用新错误 | Execute | High | execute/channel_commit.go | +3 |
| 8. sync.WaitGroup 消除死锁 | Execute | High | execute/channel_exploration.go | +50 |
| 9. mostInformativeError | Execute | High | execute/channel_exploration.go | +22 |
| 10. LP-3 Reputation 顺序 | Learn | High | learn/learner.go | +60 |
| 11. ScheduledMemory 深拷贝 | Learn | High | learn/memory.go | +15 |
| 12. Auto-Close 异步 Learn | 闭环 | High | sessionorchestrator/autoclose.go | +50 |
| 13. Auto-Close test 500ms | 测试 | High | sessionorchestrator/orchestrator_autoclose_test.go | +11 |
| 14. pipeline-architecture.md | 文档 | High | d7-orchestration/pipeline-architecture.md (NEW) | +589 |

---

## 2. 关键修复的设计决策

### 2.1 Fix 3：rollback context 隔离

**原代码**（problem）：
```go
func (c *ProtocolChannel) rollback(p *plan.Plan, executedSteps []int) error {
    for i := len(executedSteps) - 1; i >= 0; i-- {
        // 用 outer ctx，cancel 时 rollback 全挂
        r, err := c.runner.Invoke(ctx, ...)
        ...
    }
    return lastErr
}
```

**新代码**（fix）：
```go
func (c *ProtocolChannel) rollback(p *plan.Plan, executedSteps []int) error {
    rollbackCtx, cancel := context.WithTimeout(context.Background(), c.cfg.Timeout)
    defer cancel()
    var firstErr error
    for i := len(executedSteps) - 1; i >= 0; i-- {
        r, err := c.runner.Invoke(rollbackCtx, ...)
        if err != nil && firstErr == nil {
            firstErr = fmt.Errorf("rollback step %d (%s): %w", idx, step.ToolName, err)
        }
    }
    return firstErr
}
```

**Why**：
- 独立 ctx 保证即使 outer cancel 也能完成 rollback（事务原子性）
- `first non-nil error wins`：保留 root cause，last error 可能掩盖 first

### 2.2 Fix 8：sync.WaitGroup 消除死锁

**原代码**（problem）：
```go
sem := make(chan struct{}, c.cfg.MaxParallel)
out := make(chan runOut, len(p.Steps))
// 同步等信号量：死锁场景
for i, step := range p.Steps {
    sem <- struct{}{}  // 主 goroutine 等 in-flight 释放
    go func(idx int, s plan.Step) {
        defer func() { <-sem }()
        ...
    }(i, step)
}
```

**新代码**（fix）：
```go
sem := make(chan struct{}, c.cfg.MaxParallel)
out := make(chan runOut, len(p.Steps))
var wg sync.WaitGroup
for i, step := range p.Steps {
    wg.Add(1)
    go func(idx int, s plan.Step) {
        defer wg.Done()
        sem <- struct{}{}
        defer func() { <-sem }()
        r, err := c.runner.Invoke(ctx, ...)
        out <- runOut{idx, s, r, err}
    }(i, step)
}
go func() { wg.Wait(); close(out) }()
```

**Why**：
- Spawn-all：主 goroutine 不再同步等信号量
- wg.Wait() + close(out) 保证所有 worker 完成才 drain
- 完全消除主 ↔ in-flight 同步依赖

### 2.3 Fix 10：LP-3 Reputation 顺序

**原代码**（problem）：
```go
// 错误顺序：crash 发生在两步之间 → 资产在 Memory 但 Reputation 未更新
asset := l.AssetBuilder.Build(class, content)
if err := l.Memory.Store(ctx, asset); err != nil { ... }
if l.Reputation != nil { ... l.Reputation.Update(ctx, next) ... }
```

**新代码**（fix）：
```go
// LP-3：先 Reputation 后 Memory
if l.Reputation != nil && l.BayesianUpdater != nil {
    prior, _ := l.Reputation.Get(ctx, req.SessionID)
    if prior == nil { prior, _ = NewReputationEvidence(req.SessionID, TrackModeDeveloper) }
    next := l.BayesianUpdater(prior, req.Verdict)
    if err := l.Reputation.Update(ctx, next); err != nil { ... }
}
asset := l.AssetBuilder.Build(class, content)
if err := l.Memory.Store(ctx, asset); err != nil { ... }
```

**Why**：
- Reputation 写入幂等（同 prior+verdict pair → 同 next state）
- "过计数"（over-count）是 benign 统计伪影，"欠计数"（under-count）会让 Inject 忽略一个已被系统确认的 Learn
- LP-3 不变式：Reputation 永远先于 Memory

### 2.4 Fix 12：Auto-Close 异步 Learn

**原代码**（problem）：
```go
func (o *SessionOrchestrator) processAutoClose(ctx, sessionID, ch) error {
    go func() {
        for ev := range ch { out <- ev }
        // close 之前同步 Learn
        if _, err := o.learner.Learn(ctx, req); err != nil { ... }
        close(out)
    }()
}
```

**新代码**（fix）：
```go
func (o *SessionOrchestrator) processAutoClose(ctx, sessionID, ch) error {
    go func() {
        var lastEvent *contracts.EngineEvent
        for ev := range ch { lastEvent = ev; out <- ev }
        close(out)  // 立即 close，consumer unblock
        // 异步 Learn，独立 ctx
        go func() {
            learnCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
            defer cancel()
            if _, err := o.learner.Learn(learnCtx, req); err != nil { ... }
        }()
    }()
}
```

**Why**：
- 异步 Learn 不阻塞 channel close（用户秒回）
- 独立 10s ctx 不受 outer cancel 影响（事务原子性）
- 3 层 fail-safe：nil learner / Learn error / channel cancel 都 log+skip

### 2.5 Fix 14：pipeline-architecture.md

**Why**（独立 fix，不依赖其他 13 个）：
- MUPS 7 个 Phase 落地后没有一张端到端总图
- 5 节点管道（Observe→Plan→Execute→Verify→Learn）+ LP-1/2/5 闭环 + Auto-Close 异步触发需要单一权威图谱
- 验收人员 onboarding 慢

**结构**（6 章节）：
- §1 5 节点管道总览（架构图 + 4 类对应表 + 3 项不变式 + LP-1/2/5）
- §2 13 个 S 场景关系图
- §3 全局入口 D1→D7 路径
- §4 OrchestratePath 6 步时序
- §5 5 节点管道闭环可视化（LP-1/LP-2/LP-5）
- §6 Cross-references（9 个 archive + 13 个代码位置）

---

## 3. 兼容性

| 维度 | 影响 | 缓解 |
|------|------|------|
| PersistScope fail-fast | 4 个 plan_test 失败 | 已加 `BlastRadius: BlastRadius{PersistScope: ...}` |
| Auto-Close 异步化 | 1 个 orchestrator_autoclose_test 失败 | 已加 500ms wait |
| NewPlanID 格式 | DB 里旧 ID 仍可读 | 旧 ID 不删除，只是不再生成 |
| LP-3 顺序 | Bayesian 信誉累积更快 | 用户不可见 |
| rollback ctx 隔离 | ProtocolPlan 失败时副作用更彻底清理 | 用户不可见 |
| sync.WaitGroup | ExplorationChannel 性能提升 | 用户不可见 |
| 异步 Learn | ReputationStore 写入延迟最高 +10s | 用户不可见 |
| pipeline-architecture.md | 文档新增 | spec.md 加 1 行引用 |

---

## 4. 不在范围内

- M1-M20 Medium 修复 → 后续 cleanup change
- L1-L14 Low 修复 → 后续 cleanup change
- `coordinator/aliases.go` 130 行 shim → 单独 Change（DM-2026XXXX）
- `hubspoke/aliases.go` 80 行 shim → 单独 Change

---

## 5. References

- `openspec/changes/devrix-d7-mups-v4-review-fixes/proposal.md`
- `openspec/changes/devrix-d7-mups-v4-review-fixes/tasks.md`
- `openspec/changes/devrix-d7-mups-v4-review-fixes/review-report.md`（reviewer 报告）
- 7 个 MUPS Phase archive（9 个 change-id）
- pipeline-architecture.md（fix 14 新增）
