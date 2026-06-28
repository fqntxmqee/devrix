# Proposal: D7 多轮 session 串行化与 complete 时机修正 — WaitForTurnCompletion + turn 上下文自动注入

**Change ID:** devrix-d7-multiturn-session-state
**Demand ID:** DM-20260628-003
**Status:** S2_Design
**Priority:** P0
**Reporter:** 2026-06-28 17:31 sess_1782638991113_5000 panic hotfix（PR #271）现场跟踪 — multi-turn session 体验缺陷

---

## 1. Background

2026-06-28 17:31 sess_1782638991113_5000 用户发送 `review d2 领域 kernel` 后追问 "你并没有给出 review 结论？哪里有问题吗"，devrix 在第二轮 panic 退出。DM-20260628-002（PR #271）已修复 send-on-closed-channel panic，但 panic hotfix 后的飞书实测暴露两个**更深层的 session 生命周期缺陷**：

### 1.1 complete 事件触发太早（用户感知："devrix 还没说完就 Done 了"）

- 18:34:28 turn 1 `complete` 事件已 emit，飞书卡片显示 ✅ Done
- 18:34:28 → 18:35:42 期间 turn 1 仍在跑后续子 WorkItem（review 展开的 spawn decompose 子任务）
- 18:35:42 才真正 finalText 落地

### 1.2 turn N+1 缺乏 turn N 上下文（用户感知："第二条没第一条记忆"）

- turn 1 finalText 已在 18:35:42 完成，但 prompt 落盘时**未注入**到 WorkItem.Directive
- turn 2 (18:35:17 进站）开启新 EnsureGoal 时，只看到上一轮的 directive 原话（"review d2 领域 kernel"），看不到 turn 1 finalText
- turn 2 启动新一轮 LLM ReAct loop 时直接跑 `git status && git diff --stat`，把 turn 1 当作没发生过

### 1.3 与 PR #271（panic hotfix）的关系

DM-20260628-002 修了 panic，但 panic 的**根因**（"前后 turn 的 goroutine 并发执行"）仍在：

- panic 之所以能发生，就是因为 multi-turn session 设计上就允许"前后 turn 的 goroutine 并发"
- "并发允许" 还会导致 complete 提前 + turn 上下文丢失两个体验缺陷
- 所以本次需求不是 panic hotfix 的补丁，而是**正确的 session 生命周期重构**

## 2. Problem Statement

| ID | 问题 | 影响面 | 优先级 |
|----|------|--------|--------|
| **P1** | `session_turn_loop.go:186-189` 在 `for iter` break 后立即 emit `complete`，但子 WorkItem 还在跑 | 飞书卡片 Done 提前，finalText 后到不再追加 | P0 |
| **P2** | `EnsureGoal(req.SessionID, req.Message)` 用用户原话做 directive，前一轮 finalText 完全丢失 | turn N+1 看不到 turn N 结论，等于无 multi-turn | P0 |
| **P3** | turn N+1 不等 turn N 收尾就进站 | 两个 goroutine 并发持有同 session_id 的 emitFn，是 panic 的根因（PR #271 治了症状） | P0 |
| **P4** | transcript jsonl 写入已就位（PR #137），但 D7 没有读取路径 | P2 治本所需的"读最近 N 轮 finalText" helper 缺失 | P1 |

## 3. Proposed Solution

### 3.1 方案对比

| 方案 | 优点 | 缺点 | 结论 |
|------|------|------|------|
| **A. 串行化 + 上下文注入（推荐）** — `TurnState.WaitTurn` 阻塞后续 turn + `transcript_reader` 注入 prior-output-summary + complete 时机延后到 processAutoClose 完成 | 严格串行语义对齐 chat 模型体感；治本而非补丁；改动局部（`sessionorchestrator` 包内 + 1 个 IM 适配器） | 单 session 单 goroutine（设计意图，非限制） | ✅ **采用** |
| B. 仅修 complete 时机，不动 turn 串行 | 范围最小 | 留下 panic 根因，下一次仍可能 panic | ❌ |
| C. Queue + Replay：turn N+1 不开新 WorkItem，作为 turn N 延续 | 保留多轮体感 | server 侧无法区分 turn，破坏 D7 turn-level metrics 和 transcript index | ❌ |
| D. Late Attach：turn N+1 立即开始，context 后挂 | 复杂 | 需要 partial-result commit + context merge，scope creep | ❌ |

### 3.2 核心架构

```
D1 Gateway (feishu/cli)
    ↓ ProcessMessage(sessionID, message)
D7 SessionOrchestrator.ProcessMessage
    ↓ ① TurnState.WaitTurn(sessionID) ───── 阻断 in-flight turn
    │                                        ↓ TurnInProgressError (新)
    │                                   feishu: "上一条还在处理中"
    ↓ ② EnsureGoal(sessionID, message)
    ↓ ③ transcript_reader.ReadRecent(sessionID, N=3) ── D2 transcript jsonl
    │   ↓ 拼接 <prior-output-summary>...</prior-output-summary>
    │   ↓ 注入 WorkItem.Directive
    ↓ ④ RunSessionTurnLoop(ctx, req, intent)
    │   ↓ goroutine 内：for iter { ... }
    │   ↓ break 后 emit complete ──── 此时 processAutoClose 已就绪
    ↓ ⑤ TurnState.EndTurn(sessionID)
    ↓ return channel → processAutoClose → transcript 落盘
```

### 3.3 三层契约

1. **状态层**：`TurnState` struct（in-memory，per-SessionOrchestrator）持有 `sessionID → *turnHandle` map + `sync.RWMutex`；`turnHandle` 包含 `done chan struct{}` + `turnNo int` + `startedAt time.Time`。
2. **注入层**：`transcript_reader.go` 读 `internal/layers/communication/capture/transcript/{sessionID}.jsonl`，按 `kind=final_text` 过滤，取最近 N 条，按时序拼成 `<prior-output-summary>` 块（复用 D2 `FoldAssistantOutput` 的标签语法）。
3. **时序层**：`session_turn_loop.go` goroutine 在 `defer close(out)` 之前调用 `TurnState.EndTurn(sessionID)`（保证 channel close 先于 EndTurn 信号释放，避免再触发 P3 的并发问题）。`complete` 事件本身在原位 emit（channel close 已隐含 processAutoClose 完成），不需要改 timing。

### 3.4 关键决策

#### Decision: TurnState 归属

**选项:**
| 方案 | 优点 | 缺点 |
|------|------|------|
| A. `SessionOrchestrator` 内嵌字段 | 生命周期与 orchestrator 一致；零额外依赖 | 不能跨 orchestrator 共享（够用，devrix 单进程单 orchestrator） |
| B. 独立 `TurnStateRegistry` 全局单例 | 可跨 orchestrator | 单进程单 orchestrator 用全局是 over-engineering；测试时需 reset |

**选择:** A
**理由:** devrix 是单进程架构，`SessionOrchestrator` 是 entry 唯一持有者；A 没有 B 的全局状态管理负担；测试可在 `SessionOrchestrator` 实例化时拿到 TurnState 直接构造用例。

#### Decision: WaitTurn 阻塞策略

**选项:**
| 方案 | 优点 | 缺点 |
|------|------|------|
| A. 无限等（直到 turn N done） | 最简单 | session 死锁时 turn N+1 永久等（需 ctx 取消） |
| B. 带 timeout（默认 30s） | 防止永久阻塞 | timeout 后 turn N+1 仍可能与 turn N 撞车（race） |
| C. 无限等 + ctx 取消联动 | 语义正确 + 死锁可解 | 依赖 ctx 链路正确 |

**选择:** C
**理由:** A 死锁风险真实存在（session 死循环时 turn N 永不 done），但 timeout 后强制 turn N+1 进入又会和 turn N 撞 — C 是唯一不引入新 race 的方案。实现：`WaitTurn(ctx, sessionID)` 内部 `select { case <-handle.done: ; case <-ctx.Done(): }`。

#### Decision: transcript_reader 的 N 默认值

**选项:**
| 方案 | 优点 | 缺点 |
|------|------|------|
| A. N=3（最近 3 轮 finalText） | 覆盖用户常见追问场景；3 轮 finalText ≈ 2.4K token | 更长上下文可能仍不足 |
| B. N=1（只最近 1 轮） | 最保守 | 用户"我前面问的"指代不清时不足 |
| C. 可配置 + 默认 N=3 | 灵活 | 多一个配置点 |

**选择:** C
**理由:** 通过 `OrchestratorDeps.PriorContextRounds int` 字段暴露，默认 3（与 devrix PR #149 iter3 的 `prior-output-summary` 剥离场景一致）；N=0 时等于不注入（向后兼容老路径）。本需求 S4 落地 N=3 default，配置化走纯字段预留，不引入配置加载逻辑（避免 scope creep）。

#### Decision: prior-output-summary 标签注入位置

**选项:**
| 方案 | 优点 | 缺点 |
|------|------|------|
| A. 在 `EnsureGoal` 后立即注入 directive | 改动局部；语义清晰（"directive 自带 prior context"） | EnsureGoal 不再"纯粹"，需要文档说明 |
| B. 新增 `DirectiveForTurn` helper 单独处理 | 关注点分离 | 多一层调用栈，需要修改 4-5 个 directive 拼装点 |
| C. 在 D2 `prepareContext` 层做 | 与 D2 折叠逻辑同源 | D2 当前只折叠 assistant 输出，不注入 prior context；跨域改动 |

**选择:** A
**理由:** B 多调用栈反而复杂；C 跨域且 D2 不感知 turn 边界。A 在 EnsureGoal 之后、RunSessionTurnLoop 之前，由 SessionOrchestrator.ProcessMessage 统一注入，single call site，scope 可控。

## 4. Success Metrics

| 指标 | 当前值 | 目标值 | 测量方式 |
|------|--------|--------|----------|
| multi-turn session 飞书 "Done" 提前率 | ~14%（sess_1782638991113_5000 实测） | 0% | `tests/integration/multiturn_session_e2e_test.go` 断言 complete 是最后一个事件 |
| multi-turn session 上下文注入率 | 0%（无机制） | 100%（N≥1） | LP-5 e2e 测试断言 turn 2 directive 包含 turn 1 finalText 关键词 |
| 同 session_id 并发 turn 数 | 上限 unbounded（实际可能多个） | 严格 1 | TurnState 单测覆盖 `BeginTurn` 二次调用返回 TurnInProgressError |
| panic recurrence | 0（PR #271 修复） | 0 | `verify-archive.sh` 兜底 |

## 5. Implementation Plan

| PR # | 内容 | 依赖 | 预估行数 |
|------|------|------|----------|
| PR-1 | TurnState struct + Begin/End/Wait API + 并发安全 + 单测 | 无 | +150 / -0 |
| PR-2 | transcript_reader.go + 最近 N 轮 finalText helper + 单测 | PR-1 | +120 / -0 |
| PR-3 | ProcessMessage 接入 WaitTurn + EnsureGoal 后注入 prior-output-summary + TurnInProgressError 定义 + feishu 适配 | PR-1 + PR-2 | +200 / -20 |
| PR-4 | LP-5 e2e 集成测试 + S6 归档 | PR-3 | +250 / -0 |

总预估：~720 行新增，~20 行修改。

## 6. Risks & Mitigations

| 风险 | 影响 | 缓解 |
|------|------|------|
| 多轮 session 串行化降低单 session 吞吐 | 高 | 这是**正确**语义（用户体感对齐 chat）；AC3 N=0 配置项保留高吞吐路径 |
| prior-output-summary 增加 prompt token | 中 | D2 FoldAssistantOutput 的 maxChars=800 兜底；3 轮 ≤ 2.4K token |
| TurnInProgressError 被 CLI 用户视为异常 | 中 | §5.3 明确 CLI 不识别此错误（保持兼容） |
| TurnState 全局 map 内存增长 | 低 | session 关闭（`sessiondone` 事件）显式 delete；不持久化 |
| transcript 文件持续增长 | 低 | 不在本 scope，follow-up `devrix-d2-transcript-rotate` |
| 改动 ProcessMessage 核心热路径 | 高 | S3-Gate review + 22/22 orchestration packages -race + 用户飞书实测 |

## 7. Out of Scope

- **transcript jsonl 文件 rotate** — 单独 follow-up：`devrix-d2-transcript-rotate`
- **session 级跨 turn metrics**（turn-level latency 等） — 单独 follow-up：`devrix-d7-turn-metrics`
- **session 级 UI 显示历史 turn 列表** — D1 IM 适配器范围，本需求只补 D7 端
- **CLI 适配器 turn_in_progress 友好提示** — 保持现有错误处理（与 PR #271 行为一致）
- **Streaming fallback 自动切换** — DM-20260628-001 P0-2 已有
- **prompt_too_long withhold-then-recover** — DM-20260628-001 P0-3 已有

## 8. 关联变更

- **DM-20260628-002 (PR #271)**：panic hotfix，不动 session 生命周期 — 本需求是**治本**版本
- **PR #137**：transcript jsonl 写入已就绪，本需求补**读取**路径
- **PR #149 iter3**：D1 边界剥离 `<prior-output-summary>` 标签已就绪，本需求在 D7 边界**写入**该标签（LLM 输出后被 D1 剥掉防止泄漏到 IM 卡片）
- **PR #257**：emit 链路补齐，本需求扩展 emit 时机约束
- **DM-20260626-001**：v6.0.0 域升级 14S → 6S，本需求新建 D7-S15

## 9. 备注

本次 S2 proposal 同步提交 S3 design.md + tasks.md + spec delta（跨 D7 域），符合 devrix-d7-real-closure-pr36 "按用户思路推进到整体目标" 工作模式（PR #36 一次性扫 4 cell）。