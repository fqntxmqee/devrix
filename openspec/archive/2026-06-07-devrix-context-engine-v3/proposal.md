# Proposal: Context Engine V3

**Change ID:** devrix-context-engine-v3
**Layer:** 2 - Context Engine
**Type:** Enhancement
**Status:** S7 Archived
**Based on:** `devrix-context-engine-v2` (archived 2026-06-07), Communication V3 milestone.Service
**Demand:** DM-20260607-006

---

## Archive Information

**Archived:** 2026-06-07
**Outcome:** Successfully implemented
**Canonical Spec:** `openspec/specs/context-engine/spec.md` v3.0.0

---

## Problem Statement

V2 补齐了压缩与验证能力，但 **任务编排** 与 **跨会话记忆** 仍为占位：

1. **PEV 缺 Plan** — 仅 Execute→Verify，复杂任务无法分解；通信层 `milestone_progress` 无 L2 生产者
2. **LongTerm stub** — `Recall` 返回 `FeatureNotImplemented`，多 Session 开发无法复用项目上下文
3. **ShortTerm 无 Milestone 引用** — 快照不含 DAG 状态，Session 恢复后任务进度丢失

## Proposed Solution

| 能力 | V3 方案 |
|------|---------|
| PEV Plan | LLM 生成结构化 Milestone JSON → `IMilestonePlanner` 创建 DAG |
| Milestone 驱动执行 | 按拓扑序逐 milestone Execute→Verify，发射 `milestone_progress` |
| LongTerm Memory | SQLite `~/.devrix/memory.db`，Recall（topic/LIKE）+ 自动 Store |
| 层间边界 | `shared/contracts.IMilestonePlanner` + `bridges/milestone` 适配器 |

## Goals

| Goal | V2 | V3 |
|------|----|----|
| PEV 完整三阶段 | Execute→Verify | Plan→Execute→Verify |
| Milestone DAG | ❌ | ✅ LLM 生成 + 环检测 |
| milestone_progress 事件 | 通信层占位 | ✅ PEV 发射 |
| LongTerm Memory | stub | ✅ SQLite |
| plan.enabled=false 回退 | — | ✅ 等同 V2 |

## Capabilities

| Capability | L4 映射 | 说明 |
|------------|---------|------|
| pev-plan | L4-CTX-PLAN | LLM 任务分解 + DAG 校验 |
| milestone-driven-pev | L4-CTX-PEV | 按 milestone 顺序执行 |
| longterm-memory | L4-CTX-MEMORY | SQLite 跨 Session 记忆 |
| plan-observability | L4-CTX-OBS | IPEVObserver Plan/Milestone 扩展 |

## Alternatives Considered

| 方案 | 结论 |
|------|------|
| L2 直接 import `communication/milestone` | 拒绝 — 违反层间依赖；用 contracts + bridge |
| Plan 每次强制执行 | 拒绝 — 简单问答不应多一轮 LLM；启发式 + 配置开关 |
| LongTerm 用文件 JSON | 拒绝 — 查询与并发差；SQLite 满足 V3 规模 |
| 向量检索 Recall | 拒绝 V3 — 复杂度高；LIKE + topic 足够 MVP |

## Impact

| 组件 | 变更 |
|------|------|
| `pev/plan.go` | **新增** Plan 阶段 |
| `pev/milestone_runner.go` | **新增** 按 DAG 驱动循环 |
| `memory/longterm.go` | **新增** 替换 stub |
| `memory/longterm_stub.go` | 保留为 disabled 模式 |
| `shared/contracts/milestone.go` | **新增** `IMilestonePlanner` |
| `bridges/milestone/wire.go` | **新增** L1→L2 适配 |
| `shared/types/context.go` | +`PEVPhasePlan` |
| `contracts.go` | `IPEVObserver` 扩展 |
| `shared/config/contextengine_v3.go` | **新增** plan/longterm 配置 |
| `engine.go` | Plan 入口 + LongTerm Recall 注入 |
| `cmd/devrix/main.go` | Wire MilestonePlanner + LongTerm |
| `openspec/l5-registry.md` | +L5-CTX-19~25 |
| `openspec/specs/context-engine/spec.md` | S7 合并 delta → v3.0.0 |

## Scope

**In Scope:** 见 `demand.md` §3.2

**Out of Scope:** 快照加密、异步 Autocompact、向量检索、Multi-Agent

## Dependencies

```
devrix-context-engine-v2 (V2 archived)
        │
        ├── devrix-llm-gateway (Plan/Execute LLM)
        ├── communication/milestone.Service (via bridge)
        └── devrix-observability (Plan span/metrics，可并行)
```

## Success Criteria (S3 准出)

- [x] proposal / design / specs / tasks 四件套完整
- [x] L5-CTX-19 ~ L5-CTX-25 已登记 `l5-registry.md`
- [x] 每个新增 L4 至少 1 个 L5 测试点
- [x] 利益相关方 sign-off（Grill 2026-06-07，见 design.md §九）

## Timeline (估算)

| 阶段 | 工期 |
|------|------|
| S3 规划（本 PR） | 1d |
| S4 实现（M1–M5） | 8–10d |
| S5 验收 | 2d |
