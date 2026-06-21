# Proposal: D7 Metrics Naming Alignment & Concurrency Hardening

**Change ID:** `devrix-d7-metrics-and-concurrency-hardening`
**Demand ID:** DM-20260622-001
**Status:** S2_Proposal
**Priority:** P1
**Date:** 2026-06-22
**Author:** Deep review session (DM-20260621-009 follow-up)

---

## 1. Background

2026-06-21 用户发起 "D7 编排层深度 review"（DM-20260621-009），综合评价 **A-**，识别 15+ 改进点。该 review 之后：

- **PR-A/B/C 已合并**（DM-20260621-010 → PR #152/#153/#154/#155, S7_Archived 2026-06-21）：
  - HandleInterrupt 错误聚合（errors.Join + InterruptMetrics 5 字段）
  - Worktree 全链路 metrics 化（SchedulerMetrics +4 字段）
  - TaskManager.publishCompletion panic 兜底
  - Sandbox cleanup hardening（freefork + execute）

- **本次 review（2026-06-22）**：在前次 review 基础上重新扫描 `internal/layers/orchestration/`，综合评价从 **A- 降至 B+**，新发现：

  - **HIGH-1**：PR-B 已交付的 5 个 counter 中，**2 个存在 spec/code 命名漂移**（dispatch_wakeup vs spec dispatch_loop_wakeups；worker_panic vs spec worker_panics）。D5 dashboard 按 spec 名过滤将看到零流量。
  - **HIGH-2**：PR-B 落地的 `sandbox_exit_failed` 在 spec D7-S6-A12-T01 声明但 D7 wavescheduler 无对应 incMetric 调用（D4 `multiagent/execute/worker.go` 才有）—— 跨域归属未澄清。
  - **HIGH-3**：`wavescheduler/scheduler.go:343` `state.cancels = append(state.cancels, cancel)` 累积 cancel funcs，但 `cancelWaveLocked`（`:606-617`）完成后**永不清理**，长会话多 wave 重入时 slice 单调增长（无 metric）。
  - **HIGH-4**：`wavescheduler/conflict.go:74` 提供原子 `AllowAndRegister`（4 个 P0 T 点 IMPLEMENTED），但 `scheduler.go:283+352` 热路径仍用 split `Allow + Register`，TOCTOU 窗口未闭合（DM-20260621-009 P1 遗留）。
  - **MEDIUM-5**：`sessionorchestrator/command_handler.go:100-101` `out <- &contracts.EngineEvent{...}` 是无 select-default 的阻塞 send（buffered 32），消费方挂掉会永久阻塞。

本 change 是 review 立即修复层（"< 1 周"范围），闭合 DM-20260621-009 全部 P0/P1 遗留 + 修正 PR-B 的 spec/code 漂移。

**与已有 change 的关系**：
- **直接修补** `devrix-d7-error-aggregation-and-metrics`（S7_Archived 2026-06-21, PR #152-#155）—— PR-B 已实施但未对齐 spec，本 change 是 "改名 + 加固" 增量
- **互补** `devrix-d6-evolution-review-fixes`（S7_Archived 2026-06-21, PR #156-#159）—— 同期姊妹 change，姊妹 PR-B 涉及 orch_* → guard_* 6 metric rename，可参考模式
- **不重复** `devrix-error-handling-tier1-tier2`（S7_Archived 2026-06-20）—— 那是 sharederrors 公共域
- **不接入** `devrix-d7-certainty-architecture`（DM-20260620-005 P0 遗留，独立 change）

**与 tech-debt 的关系**：
- `openspec/tech-debt/worktree-v2-deferred.md` 中 TD-WT-01（AdaptiveThreshold 接入 RunTurn）仍延期 v3.0
- 本 change 不动 AdaptiveThreshold

## 2. Problem Statement

### Problem 1 (HIGH): PR-B 落地的 2 个 counter 与 spec 命名漂移

**位置**：
- `wavescheduler/scheduler.go:306, 309` `incMetric("dispatch_wakeup")` —— spec 是 `dispatch_loop_wakeups`
- `wavescheduler/scheduler.go:414` `incMetric("worker_panic")` —— spec 是 `worker_panics`

**证据**：

```
openspec/archive/2026-06-21-devrix-d7-error-aggregation-and-metrics/specs/d7-orchestration/spec.md:
  Line 95:    And metric "worker_panics" increments by 1
  Line 146: - D7-S6-A12-T02: Worker panic → metric.Inc("worker_panics") + slog.Error

vs

wavescheduler/scheduler.go:414    s.incMetric("worker_panic")   ← 单数
wavescheduler/scheduler.go:306, 309 s.incMetric("dispatch_wakeup") ← spec 名 "dispatch_loop_wakeups"
```

**根因**：PR-B 编写时按代码侧命名（singular / 短名），未与 spec 验收文案对齐。

**影响**：
- D5 observability dashboard 按 spec 名 `worker_panics` / `dispatch_loop_wakeups` 过滤 → 永远显示 0
- t-registry D7-S6-A12-T02 / T05 测试虽然 PASS（测试也用单数名），但**测的是错误名**而非 spec 名
- 实际生产 metrics 不可观测

**修法**：rename 2 处 string 字面量 + 更新对应测试断言字符串 + 更新 `scheduler_metrics_test.go` 用例名。

### Problem 2 (HIGH): `sandbox_exit_failed` 跨域归属未澄清

**位置**：
- spec D7-S6-A12-T01 声明 `metric.Inc("sandbox_exit_failed")` 应由 Forker.Fork 路径触发
- 代码：实际 incMetric 在 `multiagent/execute/worker.go:54-61`（D4 域），D7 wavescheduler **零命中**

**根因**：PR-A 实施时 `sandbox_exit_failed` counter 加到了 D4 worker.go（`multiagent/execute/metrics.go::ExecutorMetrics`），但 spec 在 D7 域 doc 声明—— spec/code 域归属错位。

**影响**：
- 跨域 spec 错误归属，D5 dashboard 配置按 D7 spec 加 alert 但 D7 域无 metric 触发
- Forker.Fork 路径（`internal/layers/multiagent/provision/freefork/forker.go`）的 sandbox Exit 失败**仍未被 metric 覆盖**（仅 slog.Warn）
- ForkerMetrics 在 PR-A 已加 SandboxExitFailed 字段，但代码侧 incMetric 调用散在 worker.go 而非 forker.go

**修法（两步）**：
1. spec.md 显式标注 `sandbox_exit_failed` 由 D4 `multiagent/execute/worker.go` 提供，D7 spec 移除该 T 点（或注明 "out of D7 scope, see D4-S6-*")
2. t-registry 删除 D7-S6-A12-T01，新增跨域 reference 到 D4 域对应 T 点

### Problem 3 (HIGH): `state.cancels` 无界增长

**位置**：`wavescheduler/scheduler.go:343, 365`

```go
state.cancels = append(state.cancels, cancel)   // line 343：累积 cancel funcs
// ...
state.cancels = append(state.cancels, ...)     // line 365：再次累积
// cancelWaveLocked (line 606-617) 完成后，slice 永不清空
```

**根因**：设计上依赖 `CancelAll` 显式清理（PR 注释自承），但实际从未在 cancel 完成后 reset slice。

**影响**：
- 长会话多 wave 重入（如 LLM 长任务反复 trigger Plan Mode → wave 重启）时，slice 单调增长
- 每个 cancel func 持有 `context.WithCancel` 资源（含 trace detach 注册项）
- 无 metric 监控 leak 数

**修法**：`cancelWaveLocked`（或新增 `clearWaveCancels`）完成后 `state.cancels = nil`；考虑同步 `state.handles` map 的清理逻辑。

### Problem 4 (HIGH): `AllowAndRegister` 存在但热路径用 split Allow+Register（TOCTOU 复发）

**位置**：
- `wavescheduler/conflict.go:74` 提供原子 `AllowAndRegister(candidate, slotID, running)` —— 通过测试 `conflict_test.go:91-141` 4 个 P0 T 点 IMPLEMENTED
- **但 `wavescheduler/scheduler.go:283` + `:352` 热路径仍用 split**：

```go
// scheduler.go:283 (dispatchLoop)
if !s.guard.Allow(node, s.guard.Running()) {
    continue
}
// ... 中间数十行 ...
// scheduler.go:352 (dispatchOne)
s.guard.Register(RunningTask{Node: node, SlotID: slotID})
```

**根因**：DM-20260621-009 当时已识别（"ConflictGuard TOCTOU"），PR-A 写了 `AllowAndRegister` 但未改 hot path。

**影响**：
- 在 Allow 通过到 Register 完成之间，另一 goroutine 拿到同一冲突组仍可进入（race window）
- 4 个 `AllowAndRegister` P0 T 点 IMPLEMENTED 但**实际生产 hot path 不使用**

**修法**：将 `:283 + :352` 合并为单次 `s.guard.AllowAndRegister(node, slotID, s.guard.Running())`。

### Problem 5 (MEDIUM): `command_handler.go:100` 阻塞 send 无保护

**位置**：`sessionorchestrator/command_handler.go:90-101`

```go
out <- &contracts.EngineEvent{...}   // buffered chan size 32，无 select-default
```

**根因**：channel 有 32 buffer，但若 consumer 持续慢 / 死锁，会永久阻塞。

**影响**：CLI /help、/stop 等 CommandHandler 命令在异常 consumer 状态下 hang，session 不可用。

**修法**：

```go
select {
case out <- &contracts.EngineEvent{...}:
default:
    slog.Warn("command_handler: out channel full, drop event", "type", event.Type)
}
```

## 3. Proposed Solution

### 3.1 总体方案

**单 PR 模式（候选 A）**：5 个修复点（2 metric rename + sandbox 归属 + state.cancels bound + AllowAndRegister 热路径 + select-default）一次性合入，加 spec / t-registry / tests。

| 项 | 工作量 | 风险 |
|---|--------|------|
| HIGH-1 metric rename（2 处 string + test） | 极小 | Low |
| HIGH-2 sandbox_exit_failed 归属澄清 | 极小（仅 spec） | Low |
| HIGH-3 state.cancels bound cleanup | 小 | Low |
| HIGH-4 AllowAndRegister hot path | 中 | Low |
| MEDIUM-5 select-default 加固 | 极小 | Low |
| spec.md + t-registry 更新 + 6 个新 T 点 | 小 | Low |

**总估算**：~8 文件，+200 / -30 LoC，1-2 天交付。

### 3.2 关键决策

#### Decision 1: metric 命名修齐 spec（不改 spec 跟代码）

**选项对比**：

| 方案 | 优点 | 缺点 |
|------|------|------|
| A. 改代码跟 spec（rename 2 string） | spec 是 SoT；测试已 PASS 但用错名 | 需改 4 处 string + 4 处测试 |
| B. 改 spec 跟代码（spec 改成单数） | 改动最小 | spec 是规范源，权威性丧失 |
| **C. 改代码 + 测试 + slog tag 全部对齐 spec 复数名（选）** | **保持 spec 权威 + D5 dashboard 按 spec 名可读** | 改动 4-5 处 |

**选择**：C
**理由**：devrix 流程明确 spec 是 "Canonical — source of truth"（见 spec.md:7），代码跟 spec 是单向绑定。PR-B 编写时未对齐是事故，修正即可。

#### Decision 2: `sandbox_exit_failed` 跨域归属

**选项对比**：

| 方案 | 优点 | 缺点 |
|------|------|------|
| A. 在 D7 wavescheduler 也加 incMetric（复制） | D7 spec 自洽 | 重复 emit，D5 重复计数 |
| **B. spec.md 注明由 D4 提供，D7 不重复声明（选）** | **跨域清晰；D5 唯一源** | 需 spec 跨域 reference |
| C. 把 D4 ExecutorMetrics 移到 D7 域 | 集中 | 违反 D7 不拥有 D4 边界 |

**选择**：B
**理由**：D7 不拥有 D4（spec.md §域边界），counter 由触发源（D4 worker.go）拥有最自然。spec 用 "see D4-S6-A12-Txx" 引用即可。

#### Decision 3: state.cancels 清理时机

**选项对比**：

| 方案 | 优点 | 缺点 |
|------|------|------|
| A. cancelWaveLocked 后 nil slice | 立即清理 | 需确认 handles map 同步 |
| **B. 在完整 wave 完成（markWaveDone）后清空（选）** | **仅清一次，与 wave 生命周期对齐** | 改动略多 |

**选择**：B
**理由**：cancel funcs 在 wave 完成时已全部 cancel 过（cancelWaveLocked 已执行），slice 使命终结。`markWaveDone`（scheduler.go:549-560）是 wave 终结点，最自然。

#### Decision 4: AllowAndRegister hot path 改造

**选项对比**：

| 方案 | 优点 | 缺点 |
|------|------|------|
| A. 在 dispatchLoop 内组合 Allow+Register 为单调用 | 改动集中 | dispatchLoop 已大（`dispatchOne` 155 行），可读性下降 |
| **B. 在 dispatchOne 内组合为单调用（选）** | **dispatchOne 是单任务派发，语义清晰** | 需拆解 Allow 早返回逻辑 |

**选择**：B
**理由**：`dispatchOne`（scheduler.go:314-468，155 行）已是单任务完整生命周期，把 split Allow+Register 合并到该函数内部更符合"原子操作"的语义。

### 3.3 实施顺序

**单 PR（PR-A）**，内部按 5 个修复点顺序：

1. **Step 1**: metric rename（2 string + 4 测试）→ 最小风险先合入
2. **Step 2**: sandbox_exit_failed 归属 spec 澄清（仅 spec.md）
3. **Step 3**: state.cancels bound cleanup（含新 unit test）
4. **Step 4**: AllowAndRegister 热路径改造（含 scheduler_orch_test.go 新 IT）
5. **Step 5**: command_handler.go select-default 加固（含 unit test）
6. **Step 6**: spec.md + t-registry.md 同步更新（6 个新 T 点注册）
7. **Step 7**: S5 acceptance 跑通 → S6 archive

## 4. Success Metrics

| 指标 | 当前 | 目标 | 度量方式 |
|------|------|------|----------|
| metric 命名 spec/code 一致性 | 60%（3/5 一致） | 100%（5/5） | `grep "incMetric(" wavescheduler/scheduler.go` vs spec D7-S6-A12-T01..T05 |
| D5 dashboard spec 名可观测 | 0 metrics visible | 5/5 visible | 部署后 dashboard test |
| state.cancels 无界增长 | 100% 会话 leak | 0% | unit test `TestStateCancels_NilAfterWaveDone` |
| AllowAndRegister hot path 覆盖率 | 0% hot path | 100% hot path | `grep "AllowAndRegister" wavescheduler/scheduler.go` ≥ 1 |
| TOCTOU 窗口 | 存在 | 关闭 | scheduler_orch_test.go 新 IT |
| CommandHandler 阻塞风险 | 高 | 低 | `select-default` 路径覆盖 |
| P0 T 点新增 | — | 6 个 | t-registry PLANNED → IMPLEMENTED |
| 综合 review 评分 | B+ | A- | 下一轮 deep review |

## 5. Implementation Plan

### 5.1 PR-A — 5 个修复点 + spec sync

**核心文件改动**：

| 文件 | 改动 | LoC 预估 |
|------|------|----------|
| `wavescheduler/scheduler.go:306,309` | rename `incMetric("dispatch_wakeup")` → `"dispatch_loop_wakeups"` | +2 / -2 |
| `wavescheduler/scheduler.go:414` | rename `incMetric("worker_panic")` → `"worker_panics"` | +1 / -1 |
| `wavescheduler/scheduler.go:418` slog tag | `"metric", "worker_panic"` → `"worker_panics"` | +1 / -1 |
| `wavescheduler/scheduler_metrics_test.go:24,33` | rename 测试断言字符串 | +4 / -4 |
| `wavescheduler/scheduler.go:343, 365, 606-617` | state.cancels nil reset + 新 `clearWaveCancels` 钩子 | +10 / -2 |
| `wavescheduler/scheduler.go:283+352` | split Allow+Register → 单次 AllowAndRegister | +3 / -5 |
| `sessionorchestrator/command_handler.go:90-101` | out send 加 select-default | +5 / -1 |
| `wavescheduler/scheduler_test.go` (新) | `TestStateCancels_NilAfterWaveDone` | +40 |
| `wavescheduler/scheduler_orch_test.go` (改) | `TestDispatchLoop_UsesAllowAndRegister_OnHotPath` | +30 |
| `sessionorchestrator/command_handler_test.go` (新或改) | `TestCommandHandler_OutChannelFull_DropsEvent` | +25 |
| `openspec/specs/d7-orchestration/spec.md` | 命名 / 归属澄清 + D7-S6-A14 章节 | +30 / -5 |
| `openspec/specs/d7-orchestration/t-registry.md` | D7-S6-A14-T01..T06 新增 + D7-S6-A12-T01 注明跨域 | +25 / -2 |

**PR-A 合计**：~8 文件 +200 / -30 LoC

### 5.2 PR-A 验收

- [ ] `go vet ./...`
- [ ] `go test -race ./internal/layers/orchestration/wavescheduler/...`
- [ ] `go test -race ./internal/layers/orchestration/sessionorchestrator/...`
- [ ] `go test -race ./internal/layers/multiagent/...`
- [ ] `./scripts/test-unit.sh` 全量通过
- [ ] 6 个新 P0 T 点全部 PLANNED → IMPLEMENTED（t-registry 更新）
- [ ] spec.md D7-S6-A14 章节增补

## 6. Risks & Mitigations

| 风险 | 概率 | 影响 | 缓解 |
|------|------|------|------|
| metric rename 破坏现有 dashboard 过滤规则 | Med | Med | 在 PR description 显式列变更；D5 团队同步通知 |
| AllowAndRegister 热路径改造引入死锁 | Low | High | 4 个 P0 T 点已有覆盖；新增 IT 单独验证 |
| state.cancels nil reset 时机错误导致 ctx 泄漏 | Low | High | 新 unit test 验证 wave 完成 → slice 空；保留 handles map 兜底 |
| CommandHandler select-default 丢弃关键事件（如 /stop stopped） | Low | High | 测试覆盖所有 4 路径；slog.Warn 留痕 |
| spec.md 跨域 reference 解析失败 | Low | Low | 加显式 D4 cross-ref + link |

## 7. Out of Scope

- `coordinator/aliases.go` 退役（130 行 shim + 12 bootstrap import）—— 独立 change
- `turn/orchestrator.go` 1349 行拆分 —— 独立 change（devrix-d7-orchestrator-decomposition）
- UncertaintyCoord + AdaptiveThreshold + VERDICT 协议 —— DM-20260620-005 P0 遗留，独立 change
- 三流格式统一（FastPath vs OrchestratePath vs CommandHandler）—— spec.md:210 defer
- CommandHandler systemPrompt 注入 —— spec.md:210 defer
- DispatchLoop 唤醒语义切分（tick vs slot-release）—— P3 优化，独立 change
- worktree_slug 与 Worktree 命名差异 —— 已确认是 stable API contract，不动

## 8. References

- 上次深度 review 报告：`~/.claude/projects/-Users-fukai-workspace/memory/devrix-d7-deep-review-2026-06-21.md`（A-）
- 本次深度 review（同 session 输出）：综合评价 B+（详见同 session review 报告 §七）
- PR-B 已交付 spec 漂移源：`openspec/archive/2026-06-21-devrix-d7-error-aggregation-and-metrics/specs/d7-orchestration/spec.md`
- 上次 change S7 归档：`~/.claude/projects/-Users-fukai-workspace/memory/devrix-d7-error-aggregation-s7-archived.md`
- 确定性架构 P0 遗留：`~/.claude/projects/-Users-fukai-workspace/memory/devrix-d7-certainty-architecture.md`
- D6 同期姊妹 change：`~/.claude/projects/-Users-fukai-workspace/memory/devrix-d6-evolution-review-fixes-pr156.md`
- D7 域规范：`openspec/specs/d7-orchestration/spec.md`
- D7 T 层注册表：`openspec/specs/d7-orchestration/t-registry.md`