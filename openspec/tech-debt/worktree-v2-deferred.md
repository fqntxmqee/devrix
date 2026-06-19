# Tech Debt: Unified Work Tree v2.1+ Deferred Items

**TD ID:** TD-WT-DEF
**Status:** PARTIAL CLOSED (TD-WT-02/03 closed DM-20260619-005; TD-WT-01/04/05/06 remain OPEN)
**Severity:** Low–Medium（功能已可用；遗留为演进与清理债）
**Created:** 2026-06-18
**Owner:** —（待指派）
**Linked Change:** `devrix-unified-work-tree` (DM-20260617-009)
**Related:** PR #83–#87（v1.0–v2.0 核心交付）；`openspec/archive/2026-06-17-devrix-unified-work-tree/version-roadmap.md`

---

## 1. 债务描述

DM-20260617-009 于 2026-06-18 完成 v1.0–v2.0 核心交付（WorkItem/WorkTree/RunRegistry 分离、统一工具 alias、RunTurn resolve/decompose/blocking await）。以下项按 `version-roadmap.md` 有意 defer 至后续里程碑，登记为技术债务以便跟踪。

## 2. 债务清单

| TD 子项 | 来源 AC / Phase | 描述 | 目标版本 | 优先级 |
|---------|----------------|------|----------|--------|
| **TD-WT-01** | AC33–AC36 / Phase 8 | **自演化 optimizer 接线** — `AdaptiveThreshold` API 与 hysteresis 已实现，但未接入 RunTurn decompose 决策；需 10+ Session 数据积累后激活 | v3.0 | P2 |
| **TD-WT-02** | v2.0 T5.5 | **删除 wave.TaskNode 持久化 SoT** — Wave 仍持有独立 TaskGraph 投影；WorkTree 已是语义 SoT，TaskNode 应退化为 dispatch 投影 | v2.1 | P1 |
| **TD-WT-03** | v2.0 Deprecated | **sc.Todos 权威降级** — `todo_write` 已写 WorkTree checklist ephemeral 节点；SessionContext `sc.Todos` 仍可作为读投影，不应再作为写入 SoT | v2.1 | P1 |
| **TD-WT-04** | Phase 7 / v2.1 | **飞书跨 Session UI** — `cross_session.go` baseline 只读查询已有；卡片展示「上次 N 个 task 完成/未完成，是否继续」未实现 | v2.1 | P2 |
| **TD-WT-05** | AC22 | **Decompose 超限人工 review 门控** — 24h 内同 kind decompose > 5 时当前仅返回 `ErrDecomposeDailyLimit`；spec 要求触发 `task_await` 人工 review 流程 | v2.1 | P2 |
| **TD-WT-06** | AC42 | **并发 terminal parent re-resolve 幂等** — 多子项同时 terminal 时 parent 可能多次 re-evaluate；需 `sync.Once` 或 CAS 保证单次 bubble | v2.1 | P2 |

## 3. 现状证据

| 子项 | 已实现 baseline | 缺口 |
|------|----------------|------|
| TD-WT-01 | `uncertainty.go::AdaptiveThreshold` + 单测 | 无 Session 历史采集、无 optimizer 循环 |
| TD-WT-02 | `SyncWaveNodes` 挂树 | `wave.TaskNode` 仍独立持久化/调度 |
| TD-WT-03 | `TodoWriteBackend` → WorkTree checklist | D2 session scratch `Todos` 仍可能被误作 SoT |
| TD-WT-04 | `cross_session_test.go` FindByItemID | D1 Feishu card 无跨 session 摘要 UI |
| TD-WT-05 | `decompose.go` daily limit 5/24h | 无 review 门控 / task_await 人工介入 |
| TD-WT-06 | `ReevaluateParentAfterChild` | 无并发 terminal 去重 |

## 4. 解除条件

### TD-WT-01（自演化）
- [ ] Session 级 decompose/spawn 结果 metrics 持久化
- [ ] `AdaptiveThreshold` 接入 `ResolveHint` / decompose 阈值
- [ ] 10+ Session 数据验证 hysteresis 不震荡

### TD-WT-02 / TD-WT-03（Legacy 清理）
- [ ] Wave 调度仅读 WorkTree，TaskNode 不写盘
- [ ] `sc.Todos` 标记 Deprecated，写入路径 audit 为零
- [ ] `scripts/audit-property-rights.sh` 无 WorkTree 产权 WARN

### TD-WT-04（跨 Session UI）
- [ ] D1 Feishu adapter 读取 `QueryWorkPlan(historical_session_id)`
- [ ] 卡片模板 + E2E 验收

### TD-WT-05 / TD-WT-06（约束硬化）
- [ ] Daily limit 超限 → review WorkItem + FocusHint 引导
- [ ] Parent re-eval CAS 单测覆盖并发 terminal

## 5. 关联 PR（已完成部分）

| PR | 内容 |
|----|------|
| #83 | WorkItem/WorkTree/RunRegistry 核心 |
| #84 | 归档冲突标记 hotfix |
| #85 | task_write/spawn/await 统一 alias + FocusHint |
| #86 | decompose + ResolveHint + depth/daily limits |
| #87 | RunTurn blocking await (`ResolveAwaiter`) |

## 6. 建议后续 Change ID

- `devrix-worktree-legacy-cleanup` — TD-WT-02, TD-WT-03
- `devrix-worktree-cross-session-ui` — TD-WT-04
- `devrix-worktree-self-evolving` — TD-WT-01（依赖数据基线）
