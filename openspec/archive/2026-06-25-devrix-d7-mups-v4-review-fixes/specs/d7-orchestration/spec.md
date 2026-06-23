# D7 Orchestration Spec Delta — MUPS v4 Review 修复

**Change ID:** devrix-d7-mups-v4-review-fixes
**Demand ID:** DM-20260625-002
**Delta Type:** PATCH (3 Critical + 10 High fixes, 1 doc)
**Spec Version:** v4.7.0 → v4.7.1

---

## 1. 修改总览

本 change 是 MUPS v4 7 个 Phase 全部 S7_Archived 之后，对落地代码做的**缺陷修复**。不改架构、不加新功能，只修复 14 个 isolated bug。

| 修复点 | 节点 | 严重度 | 行为变化 |
|--------|------|--------|----------|
| 1. clamp01OrFallback NaN | D7-S10 | Critical | NaN input 走 fallback，不再污染 reputation |
| 2. aggregateMeta 溯源 | D7-S10 | Critical | 聚合 Verdict 保留所有原始信息 |
| 3. rollback context 隔离 | D7-S9 | Critical | rollback 用独立 ctx，不被 outer cancel 打断 |
| 4. PersistScope fail-fast | D7-S5 | High | 空 PersistScope 不再通过 PP-3 |
| 5. NewPlanID UUID+SHA256 | D7-S5 | High | Plan ID 唯一性保证 |
| 6. ErrChannelStepInvalid | D7-S9 | High | 新错误类型，区分字段错 vs 数量错 |
| 7. CommitChannel 用新错 | D7-S9 | High | CommitChannel 改用新错误 |
| 8. sync.WaitGroup 模式 | D7-S9 | High | ExplorationChannel 死锁消除 |
| 9. mostInformativeError | D7-S9 | High | 失败时返回最有信息量的 error |
| 10. LP-3 Reputation 顺序 | D7-S11 | High | Reputation 写入先于 Memory |
| 11. ScheduledMemory 深拷贝 | D7-S11 | High | ListDue 返回深拷贝，避免 race |
| 12. Auto-Close 异步 Learn | D7-S13 | High | Learn 不阻塞 channel close |
| 13. Auto-Close test 500ms | D7-S13 test | High | 异步 Learn 后测试等待时间调整 |
| 14. pipeline-architecture.md | doc | High | 5 节点管道端到端总图（新文件） |

---

## 2. 行为变化详述

### 2.1 Critical 修复行为变化

**#1 NaN handling**：Wilson Score 边界场景（n=0 或 p=0/1）原本可能传播 NaN 到 reputation 注入；现在 NaN → fallback（0.5 或 prior 值），不再污染下一轮 Observe。

**#2 aggregateMeta 溯源**：多 Verdict 聚合后 `SourceID` 是 dedup join 后的 ID 列表（如 `verdict_1,verdict_3`），`IndeterminateReason` 取最长（信息量最大），`SystemAnomaly` 是 OR 聚合（任一为 true 则为 true）。

**#3 rollback context**：ProtocolPlan 失败时，rollback 用 `context.WithTimeout(context.Background(), cfg.Timeout)` 独立 ctx，外层 cancel 不再影响 rollback。返回的 error 是 first non-nil（之前是 last）。

### 2.2 High 修复行为变化

**#4 PersistScope fail-fast**：Plan.PersistScope == "" 之前能通过 Validate；现在 fail-fast 抛 `ErrPlanPersistScopeInvalid` (PLAN_PERSIST_8012)。已有 4 个 plan_test 适配。

**#5 NewPlanID**：Plan ID 格式从 `plan_<sessionID>_<timestamp>` 变为 `plan_<uuid[:8]>_<sha256[:8]>`。旧 ID 仍可读（不删除），但不再生成。

**#6-7 新错误类型**：
- 旧：`ErrChannelStepCountMismatch`（被滥用为字段错）
- 新：`ErrChannelStepInvalid`（字段错）+ `ErrChannelToolCallTimedOut`（超时）
- 错误码：EXEC_CHANNEL_9005（StepInvalid）+ EXEC_CHANNEL_9006（ToolCallTimedOut）

**#8 sync.WaitGroup**：ExplorationChannel 在 `MaxParallel < len(p.Steps)` 场景下不再死锁（之前必 hang）。

**#9 mostInformativeError**：全部失败时，Summary 显示最长 error message（包含完整 stack trace）。

**#10 LP-3 顺序**：`DefaultLearner.Learn` 内部顺序：先 `ReputationStore.Update` → 后 `Memory.Store`。Reputation 写入幂等保证重试安全。

**#11 深拷贝**：`ScheduledMemory.ListDue` 返回独立的 `ScheduledRetry` 结构体，调用方修改不影响内部 map。

**#12 异步 Learn**：Auto-Close 收到 channel close 后，**先 close(out) unblock consumer**，**再**异步触发 `learner.Learn`（独立 `context.Background() + 10s`）。3 层 fail-safe 防 panic。

**#13 test wait**：Auto-Close 测试 Round 2 加 500ms wait loop（Learn 在 goroutine 中跑）。

**#14 文档**：新增 `openspec/specs/d7-orchestration/pipeline-architecture.md`（589 行）作为 5 节点管道端到端权威图谱。

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
- `coordinator/aliases.go` 130 行 shim → 单独 Change
- `hubspoke/aliases.go` 80 行 shim → 单独 Change
- 1 个 false positive（Wilson margin formula 已验证正确）

---

## 5. References

- `openspec/archive/2026-06-25-devrix-d7-mups-v4-review-fixes/proposal.md`
- `openspec/archive/2026-06-25-devrix-d7-mups-v4-review-fixes/design.md`
- `openspec/archive/2026-06-25-devrix-d7-mups-v4-review-fixes/tasks.md`
- `openspec/archive/2026-06-25-devrix-d7-mups-v4-review-fixes/acceptance-report.md`
- 7 个 MUPS Phase archive（9 个 change-id）
- `openspec/specs/d7-orchestration/pipeline-architecture.md`（fix 14 新增）
