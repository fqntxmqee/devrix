# Proposal: D7 MUPS v4 整体 Review 修复

**Change ID:** devrix-d7-mups-v4-review-fixes
**Demand ID:** DM-20260625-002
**Status:** S4-Implementing (代码已落地，待 PR + S5 验收 + S6 归档)
**Priority:** P0 (3 Critical + 10 High 修复)
**Date:** 2026-06-25

---

## 1. Background

2026-06-24 完成 MUPS v4 5 节点管道全量深度 review（覆盖 Phase 1-7 全部 7 个 S7_Archived change 涉及的代码）。review 涵盖 `internal/layers/orchestration/{workmodel,plan,execute,learn,sessionorchestrator,decisionplanning,wavescheduler,orchtypes}/`，从代码质量、并发安全、错误处理、命名一致性、测试覆盖、S4-Gate 检查清单 5 个维度扫描。

**review 结论**：⚠️ **48 个问题需修复** —— 3 CRITICAL + 11 HIGH + 20 MEDIUM + 14 LOW。

**本 change 范围**：**P0 + P1 全部 14 个修复**（3 Critical + 11 High），**不**包含 Medium/Low 修复（这些留给后续的 cleanup change）。

**与已有 change 的关系**：

- **不重复** MUPS 7 个 Phase 自身的 S7_Archived change（Phase 1-7 共 9 个 archive，PR #128-191 全部 squash merge）
- **不重复** `devrix-d7-mups-v4-phase3-prc1-archived` (PR #164+#165) / `phase3-prc2-archived` (PR #168+#169) —— 那是 Phase 3 主体功能
- **不重复** `devrix-d6-evolution-review-fixes` (DM-20260621-011) —— 那是 D6 域 review，本 change 是 D7 域
- **姊妹篇** `pipeline-architecture.md` (新文件) —— 5 节点管道端到端调用链路文档，**附属于**本 change

**hotfix 路径依据**：本 change 按 `2026-06-17 反馈的 bugfix hotfix 路径`（feedback-devrix-bugfix-skip-openspec）执行 —— 跳过 S1-S3 完整立项，**直接进入 S4 实现 + S4-Gate 审查**。理由：

1. 13 个修复**代码已全部实现并通过 `go vet ./...` + `go test -race -count=1 ./internal/layers/orchestration/...`**
2. 13 个修复**都是对已落地代码的修复增强**（不是新需求），不需要 S3 设计
3. 13 个修复**都是 isolated change**（一个文件一个独立修复），不需要跨模块协调
4. **本提案本身就是后置的 S1-S3 文档**（hotfix 路径的"先 code 后 doc"原则）

---

## 2. Problem Statement

按 S4-Gate 5 维度列出 13 个 P0/P1 修复（**不**包含 1 个已验证为 false positive）：

### Critical 3 个

### Problem 1 (P0 — CRITICAL): AggregateVerdicts.clamp01OrFallback 不处理 NaN

**位置**：
- `internal/layers/orchestration/workmodel/aggregate_verdicts.go::clamp01OrFallback`

**根因**：
- `BayesianUpdate` 可能在 Wilson Score 边界场景返回 `NaN`（分母为 0 时）
- `clamp01OrFallback` 之前用 `v < 0 || v > 1` 检查，但 **NaN 与任何值比较都返回 false**，导致 NaN 直接通过

**影响**：
- `Alpha/Beta` 比例计算崩 → downstream reputation 注入污染
- 下轮 Observe 拿到 NaN prior → LLM judge 推理异常

**修复**：
- 加 `math` import
- `clamp01OrFallback` 入口先 `math.IsNaN(v)` 检查 → fallback

### Problem 2 (P0 — CRITICAL): aggregateMeta 聚合丢失溯源

**位置**：
- `internal/layers/orchestration/workmodel/aggregate_verdicts.go::aggregateMeta`

**根因**：
- 多 Verdict 聚合时 `SourceID` 应该保留为 dedup join 后的 ID 列表
- 之前 `SourceID = "verdict_1" + "verdict_2" + ...` 直接 join，**丢了 IndeterminateReason 和 SystemAnomaly**

**影响**：
- 聚合后 Verdict 无法回溯单条原始 Verdict
- 触发 Phase 5 Learn 时 Bayesian 路由错（IndeterminateReason 决定 LearningClass）

**修复**：
- `SourceID` → `strings.Join(dedup(...))`
- `IndeterminateReason` → 选最长的（信息量最大）
- `SystemAnomaly` → OR 聚合

### Problem 3 (P0 — CRITICAL): ProtocolChannel.rollback 用 request context 会被 cancel

**位置**：
- `internal/layers/orchestration/execute/channel_protocol.go::rollback`

**根因**：
- `rollback(ctx, ...)` 用的是 outer request context，如果上游 cancel 或 timeout，**所有 rollback 调用都会被 cancel**，导致部分 rollback 完成部分失败的 partial state
- 多步副作用协议在最关键的事务回收环节没有自己的 context 边界

**影响**：
- ProtocolPlan 失败后副作用可能滞留（数据库写了一半 / HTTP POST 发了一半）
- 违反 PP-3 爆炸半径约束

**修复**：
- `rollback` 改用 `context.WithTimeout(context.Background(), c.cfg.Timeout)` 独立 context
- 多步 rollback **first non-nil error wins**（之前是 last error wins，可能掩盖第一个 root cause）

---

### High 10 个

### Problem 4 (P1 — HIGH): Learn 写入顺序错误（Memory 在前，Reputation 在后）

**位置**：
- `internal/layers/orchestration/learn/learner.go::DefaultLearner.Learn`

**根因**：
- 之前顺序：先 `Memory.Store` → 后 `ReputationStore.Update`
- 这导致 partial-state 窗口：crash 发生在两步之间 → 资产已在 Memory，但 Reputation 未更新
- 下轮 Observe 注入的 prior 反映旧状态 → **Inject 会用错信誉统计**

**影响**：
- LP-1 闭环间歇性失效
- Bayesian 信誉累积不准确

**修复（LP-3 不变式）**：
- 顺序改为：先 `ReputationStore.Update` → 后 `Memory.Store`
- Reputation 写入幂等（同 prior+verdict pair → 同 next state）

### Problem 5 (P1 — HIGH): ScheduledMemory.ListDue 返回共享指针

**位置**：
- `internal/layers/orchestration/learn/memory.go::ScheduledMemory.ListDue`

**根因**：
- `out = append(out, &m.entries[i].Asset)` 返回的是内部 entry 的指针
- 调用方修改返回的 `ScheduledRetry.Asset` → 内部 map 数据被修改（race condition）

**影响**：
- ScheduledTick 在 ListDue 之后修改 Asset 内容 → 下一个 Read 看到不一致状态
- 概率性触发测试 flaky

**修复**：
- `ListDue` 返回**深拷贝**的 `ScheduledRetry` envelope（独立 struct）

### Problem 6 (P1 — HIGH): Auto-Close 在 channel close 前调用 Learn

**位置**：
- `internal/layers/orchestration/sessionorchestrator/autoclose.go::processAutoClose`

**根因**：
- 之前顺序：先 `learner.Learn` → 后 `close(out)` → consumer unblock
- 但 `learner.Learn` 是同步调用，**阻塞了 Auto-Close 的 caller**
- 加上 Learn 内部 5-10s Bayesian 写入时间 → 整个 channel close 延迟

**影响**：
- caller 卡住（违反 Auto-Close "不阻塞" 设计目标）
- IntentSkip 路径下用户体验"回了一句，等 5 秒才走"

**修复**：
- `Learn` 改为 **goroutine + context.Background() + 10s timeout** 异步执行
- 配套加 `endSpanWithOnce(span, sync.Once 保护)` 防 race / double-close

### Problem 7 (P1 — HIGH): Plan.PersistScope 接受空字符串

**位置**：
- `internal/layers/orchestration/plan/plan_struct.go::Plan.Validate`

**根因**：
- `PersistScope == ""` 之前被 `!= ""` 跳过，**0 值能通过 PP-3 校验**
- BlastRadius 是关键安全控制点，0 值通过会让 Plan 失去爆炸半径约束

**影响**：
- Plan 在 Execute 阶段 SideEffect 状态不确定（`SideEffectForScope("")` 返回 `Unknown`）
- 违反 PP-3（爆炸半径）

**修复**：
- 删除 `!= ""` 短路检查
- 改用 `if !p.BlastRadius.PersistScope.Valid()` → fail-fast
- 新增 `PLAN_PERSIST_8012` sentinel error code

### Problem 8 (P1 — HIGH): PlanID 用时间戳可能冲突

**位置**：
- `internal/layers/orchestration/plan/planner.go::NewPlanID`

**根因**：
- 之前 `NewPlanID(sessionID, observationIDs)` 用 `time.Now().UnixNano()` + sessionID 拼接
- 同一 session 内同 observationIDs 同毫秒可能**生成重复 ID**（快路径场景）
- 重复 ID → ReputationStore 二次覆盖 → Bayesian 信誉计算错误

**影响**：
- 高并发场景概率触发 Bayesian 信誉计算错
- LP-5 反向追溯链断（两个 Plan 共用同一 ID）

**修复**：
- 新 `NewPlanID` helper：用 **UUID + SHA-256 hash**（sessionID + dedup sorted observationIDs）
- 格式：`plan_<uuid[:8]>_<sha256[:8]>`（保持可读性 + 唯一性）

### Problem 9 (P1 — HIGH): ErrChannelStepCountMismatch 被滥用

**位置**：
- `internal/layers/orchestration/execute/errors.go`
- `internal/layers/orchestration/execute/channel_commit.go`

**根因**：
- `CommitChannel` 用 `ErrChannelStepCountMismatch` 来标记**字段错误**（空 ToolName、缺 IdempotencyKey）
- 但 StepCountMismatch 是**数量错误**（commitment 要 1 步，来了 0/2 步）
- 字段错误和数量错误语义不同 → 错误码混乱

**影响**：
- 错误路由错（PR-C3 StrategyDecider 把字段错误当 cardinality 错误处理）
- 监控/告警无法区分

**修复**：
- 新增 2 个 sentinel：`ErrChannelStepInvalid` + `ErrChannelToolCallTimedOut`
- 新增 2 个 helper：`NewChannelStepInvalidError` + `NewChannelToolCallTimedOutError`
- `channel_commit.go` 改用新错误

### Problem 10 (P1 — HIGH): ChannelExploration 死锁模式

**位置**：
- `internal/layers/orchestration/execute/channel_exploration.go::Execute`

**根因**：
- 之前用 `sem chan struct{}` + `for _, step := range p.Steps` 同步获取信号量
- 当 `MaxParallel < len(p.Steps)` 时：主 goroutine 阻塞在 `sem <- struct{}{}`，等待 in-flight goroutine 释放 slot
- 但**释放的 slot 是被同一个主 goroutine 消费的**（循环），**不是被 in-flight goroutine 消费**
- 死锁条件：当 in-flight goroutine 都在等 `sem <-` 释放时，主 goroutine 等不到释放信号

**影响**：
- `MaxParallel < len(p.Steps)` 场景下必死锁
- `MaxParallel == 3` + `len(p.Steps) >= 4` 时**所有请求 hang 住**

**修复**：
- 改为 **spawn-all + sync.WaitGroup** 模式：所有 goroutine 立即启动，每个 goroutine 内部 `sem <- struct{}{}` + defer `<-sem`
- 配套 `out chan runOut` 带缓冲 + `wg.Wait()` 后 close
- **完全消除**主 goroutine 与 in-flight 之间的同步依赖

### Problem 11 (P1 — HIGH): top-error 选最短而非最长

**位置**：
- `internal/layers/orchestration/execute/channel_exploration.go::Execute`

**根因**：
- 全部失败时，`Summary` 用 `results[0].err.Error()`（最短的 error）
- 实际 triage 时**最长 error 最有信息量**（包含完整 stack trace + context）

**影响**：
- 失败调试效率低
- 错误报告里 stack trace 经常被截断

**修复**：
- 新增 `mostInformativeError(results []runOut) error` helper
- 选 `len(err.Error())` 最大的 error，ties 时选 first

### Problem 12 (P1 — HIGH): Auto-Close 异步 Learn 等待时间不够

**位置**：
- `internal/layers/orchestration/sessionorchestrator/autoclose.go::processAutoClose`

**根因**：
- 之前 Learn 用 outer ctx（5s timeout），如果 LLM 慢/重，加上 ReputationStore 磁盘写入 → 容易超时
- 超时后 Learn 半完成 → ReputationStore 写入不完整

**影响**：
- 概率性 Learn 失败
- Bayesian 信誉累积丢失

**修复**：
- Learn 用 **独立的 `context.Background() + 10s` timeout**（不依赖 outer ctx）
- 3 层 fail-safe：nil learner / Learn error / channel cancel 都 log+skip，不 panic

### Problem 13 (P1 — HIGH): orchestrator_autoclose_test 等待时间不够

**位置**：
- `internal/layers/orchestration/sessionorchestrator/orchestrator_autoclose_test.go`

**根因**：
- 之前 Learn 在主 goroutine 同步执行，测试只等 channel close 即可
- Learn 改为异步 goroutine 后，**channel close 不再意味着 Learn 完成**
- 现有测试断言 `learningCalls == 2`（Round 1 + Round 2），但 Round 2 时 Learn 在 goroutine 中跑
- 测试在 `Round 2` 时没等够时间就断言 → 概率 flaky

**影响**：
- 偶发测试失败
- CI 重试掩盖了真问题

**修复**：
- Round 2 加 500ms wait loop（匹配现有 Round 1 wait pattern）

### Problem 14 (P1 — MEDIUM 提升为 HIGH): spec.md 缺 5 节点管道端到端总图

**位置**：
- `openspec/specs/d7-orchestration/spec.md`（新增章节引用）
- `openspec/specs/d7-orchestration/pipeline-architecture.md`（**新文件**）

**根因**：
- MUPS 7 个 Phase 落地后，spec.md 各 Scenario 章节是**局部契约**
- 没有一张**端到端的运行时序总图**（Observe→Plan→Execute→Verify→Learn + LP-1/2/5 闭环 + Auto-Close 异步触发）
- 验收人员无法快速建立 5 节点管道的全局心智

**影响**：
- onboarding 慢
- 跨 Phase 集成测试难以理解全貌
- 7 个 Phase 各自归档后无单一权威图谱

**修复**：
- 新增 `pipeline-architecture.md`（589 行）
  - §1 5 节点管道总览
  - §2 13 个 S 场景关系图
  - §3 全局入口 D1→D7 路径
  - §4 OrchestratePath 6 步时序
  - §5 5 节点管道闭环可视化（LP-1/LP-2/LP-5）
  - §6 Cross-references（9 个 Change archive + 13 个代码位置）
- `spec.md` 在 `## Architecture` 顶部加指向 `pipeline-architecture.md` 的引用链接

---

## 3. Out of Scope

下列问题**不在本 change 范围**（Medium/Low，留给后续 cleanup change）：

- **M1-M20 Medium 修复**（20 个）：命名一致性、错误信息措辞、文档散落、dead code 标记等
- **L1-L14 Low 修复**（14 个）：code style、注释补全、const 提取、benchmark 加注等
- **`coordinator/aliases.go` 130 行 type-alias shim 清除**（11 caller 迁移）—— 单独 Change（DM-2026XXXX 立项）
- **`hubspoke/aliases.go` 80 行 type-alias shim 清除**（8 caller 迁移）—— 单独 Change
- **MUPS 36 个 T 点的 edge case 补充**（覆盖率从 88% → 95%+）—— 单独 Change

**H-1 误报跳过**：reviewer 报告"Wilson margin formula 缺 z²/(4n²) 项"，但实际公式 `z2/(4*n*n)` = z²/(4n²)，**正确**。`TestWilsonScoreInterval_BoundsInRange` 100% 通过。

---

## 4. Implementation Strategy

### 4.1 拆分原则

按你 `2026-06-20 确认的 bugfix 聚合原则`：

> **同一会话/同一类问题的多 bug fix 聚合成一个 PR（多 commit），不要一个 fix 一个 PR**

→ **1 个聚合 PR + 14 个 commit**（每个 fix 一个 commit，保持 git history 可审查）

### 4.2 Commit 排序

按依赖关系排序（先底层后上层）：

```
Commit 1: Critical #1 (clamp01OrFallback NaN)
Commit 2: Critical #2 (aggregateMeta 溯源)
Commit 3: Critical #3 (rollback context 隔离)
Commit 4: High #7 (PersistScope fail-fast)
Commit 5: High #8 (NewPlanID UUID + SHA-256)
Commit 6: High #9 (ErrChannelStepInvalid + ErrChannelToolCallTimedOut)
Commit 7: High #10 (sync.WaitGroup 模式)
Commit 8: High #11 (mostInformativeError)
Commit 9: High #4 (LP-3 Reputation 顺序)
Commit 10: High #5 (ScheduledMemory 深拷贝)
Commit 11: High #6 (Auto-Close 异步 Learn)
Commit 12: High #12 (Learn 独立 10s timeout)
Commit 13: High #13 (Auto-Close test 500ms wait)
Commit 14: High #14 (pipeline-architecture.md + spec.md 引用)
```

### 4.3 验证策略

- `go vet ./...` → exit 0
- `go test -race -count=1 ./internal/layers/orchestration/...` → all PASS
- `go test -cover ./internal/layers/orchestration/...` → coverage ≥ 88%
- 现有 4 个 plan_test + 1 个 orchestrator_autoclose_test 全部适配新行为
- 新增 `TestChannelExploration_NoDeadlock` (PR #10 配套)

---

## 5. Risk & Rollback

### 5.1 Risk

- **R1**：14 个 commit 合并可能引入意外 behavior change（虽然都有 test 覆盖）
  - Mitigation：每个 commit 独立可 revert
- **R2**：plan_test 改了 4 个 test 适配新 PersistScope 行为，可能有遗漏
  - Mitigation：run `go test ./internal/layers/orchestration/plan/... -v` 100% 通过
- **R3**：Auto-Close 异步化后 telemetry 时间戳可能偏移
  - Mitigation：endSpanWithOnce 保留原 sessionSpan，attribute 不变

### 5.2 Rollback

- 单 commit revert：`git revert <commit-sha>`
- 全量 revert：`git revert -n HEAD~14..HEAD && git commit -m "revert: devrix-d7-mups-v4-review-fixes"`
- 数据回滚：ReputationStore 写入不依赖 commit（已经是 idempotent）

---

## 6. Acceptance Criteria

| AC | 内容 | 验证方式 |
|----|------|---------|
| AC1 | `go vet ./...` 0 warning | `go vet ./...` |
| AC2 | `go test -race -count=1 ./internal/layers/orchestration/...` all PASS | `go test -race` |
| AC3 | 13 个修复每个有对应 commit | `git log --oneline -14` |
| AC4 | pipeline-architecture.md ≥ 500 行 | `wc -l` |
| AC5 | spec.md 第 110 行附近有指向 pipeline-architecture.md 的引用 | grep |
| AC6 | 7 个 Phase 全部仍 S7_Archived | `verify-archive.sh` |
| AC7 | 本 change 自身走完 S6 归档 | `archive/devrix-d7-mups-v4-review-fixes/` |

---

## 7. Open Questions

无（13 个修复都是 isolated change，无 cross-module 协调问题）。

---

## 8. References

- MUPS v4 review report（保存在 `openspec/changes/devrix-d7-mups-v4-review-fixes/review-report.md`，584 行）
- 7 个 MUPS Phase S7_Archived archive（9 个 change-id）
- pipeline-architecture.md（本 change 新增）
- feedback-devrix-bugfix-skip-openspec（hotfix 路径依据）
- feedback-devrix-bugfix-pr-grouping（PR 聚合原则）
