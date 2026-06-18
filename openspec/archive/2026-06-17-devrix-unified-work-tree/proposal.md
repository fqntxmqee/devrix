# Proposal: Unified Work Tree

**Change ID:** `devrix-unified-work-tree`  
**Demand ID:** DM-20260617-009  
**Created:** 2026-06-17  
**Status:** S7_Archived (2026-06-17)

---

## Problem Statement

Devrix 编排域把 **工作单元 (What)**、**执行句柄 (How)**、**编排图节点 (When)** 三套概念混称为 task，导致：

- LLM 需在 `task_id` / wave node / `bg_*` 之间切换语义
- `todo_write` 与 `task_*` 功能重叠、数据不同步
- Wave `TaskNode` 与持久 task board 割裂
- 子任务完成通知不完整（仅 workmodel 终态走 notify.Bus）
- 无法实现 clawcode 式 **主子任务递归 + uncertainty 驱动分解**

**受影响方**：D7 编排、D2 tool surface、LLM agent 工具调用、飞书 IM 任务进度展示。

## Proposed Solution

引入 **WorkItem 树** 作为唯一工作语义（What），与 **RunRegistry**（How，DM-011）分离挂接：

1. **WorkTree** — 父子 WorkItem、kind、依赖、持久化
2. **吸收现有入口** — todo_write→checklist、delegate→spawn、Wave→parallel implement 子树
3. **RunTurn 递归** — focus + uncertainty decompose（v2.0）
4. **Tool 面简化** — task_write / task_spawn / task_await / task_list（v2.0，含 alias）

**实施顺序**：先统一底层 WorkItem 模型（v1.0），再写路径挂树（v1.1），再 RunRegistry 挂接（v1.2），最后 tool rename + 递归（v2.0）。

## Scope

### In Scope

- WorkItem + WorkTree + DiskWorkItemStore
- TaskManager legacy adapter
- delegate / PlanMode / FlowHub / todo_write / Wave 写读 WorkTree
- WorkItem.RunRef ↔ RunRegistry（与 DM-011 联调）
- QueryWorkPlan 树形读模型
- 终态 tool 面（alias 期）
- Uncertainty + GetFocus（v2.0）

### Out of Scope

- RunRegistry 核心实现（见 DM-011 独立 change）
- 跨 session 任务可见性
- agent_tools 配置发现
- D2 QueryLoop 新能力

## Impact Analysis

| Component | Change Required | Details |
|-----------|-----------------|---------|
| workmodel | Yes | 新增 WorkItem/WorkTree；TaskManager 委托 |
| delegatetools | Yes | spawn 写 parent_id + kind |
| todo_tool | Yes | 底层改 UpsertChecklist |
| wave | Yes | decomposer 写树；scheduler 读树 |
| coordinator/workmodel | Yes | QueryWorkPlan 树形 |
| turn/orchestrator | Yes (v2.0) | focus + bubble notification |
| task_* tools | Yes (v2.0) | rename + alias |
| RunRegistry | Indirect | DM-011；本 change v1.2 挂接 |
| devrix.yaml | Minimal | 沿用 tasks.mode v2 |

## Architecture Considerations

- 对齐 DM-011：**WorkTree ≠ RunRegistry**，通过 RunRef 关联
- 对齐 DM-20260617-001：RunTurn 为 canonical 递归引擎
- 对齐 DM-20260617-008：TaskManager 显式注入，无 global singleton
- WaveScheduler 降为基础设施，不拥有任务类型
- D7 拥有 WorkTree；D2 仅 tool runner

## Success Criteria

| Milestone | AC 范围 | 核心交付 |
|-----------|---------|---------|
| A (v1.0–v1.2) 产权集中 | AC1–AC13, AC23–AC25 | 6→1 模型统一，WorkTree 为唯一真相源 |
| B (v1.5–v2.0) 递归求解 | AC14–AC22, AC27–AC29 | 最小递归 → 完整递归 + tool 统一 |
| C (v2.1–v3.0) 自演化 | AC30–AC36 | 跨会话 + 自适应学习 |

- [ ] 全量 `go test ./internal/layers/orchestration/...` 每版绿
- [ ] 详见 `version-roadmap.md` 完整演进路线

## Risks & Mitigations

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| 大爆炸重写 | Med | High | 5 阶段 PR；legacy adapter 保留 |
| DM-011 阻塞 | Med | Med | v1.0–v1.1 RunRef 可空 |
| todo_write 回归 | Med | High | sc.Todos 投影 + 集成测试 |
| ID 前缀变更 | Low | Med | 文档 + alias |

## Game Theory Analysis

### 分析文件

| 文件 | 作者 | 角色 |
|------|------|------|
| `gaming-analysis.md` | Claude | 初始博弈论分析 — 产权理论、What/How 分离、递归求解、Tool 面简化 |
| `review-gametheory-worktree.md` | Codex (MiniMax-M3) | 独立审查 — 发现 3 处概念错位、5 个结构性盲点 |
| `gaming-analysis-response.md` | Claude | 对 Codex 审查的观点 — 接受 6/7 修正，保留 1 项异议 |

### 关键修正（Claude 接受）

1. **§4.2 Spence Costly Signal → Cheap Talk** — LLM 设 uncertainty 对 LLM 无成本，separating equilibrium 论证需重建，引入 Uncertainty Anchor 机制（AC27）
2. **§2 Coase → Demsetz/Williamson** — 术语精度修正，论证结构不变
3. **产权过渡期博弈缺失** — 需新增 T0→T1→T2 两阶段博弈分析
4. **递归深度硬上限缺失** — AC20-22
5. **§7.2 防御从 CR 升级到 CI 自动化** — AC23-25

### 保留异议

- **AC26 (empty RunRef → block spawn)**: 不同意 v1.1 block，改为 rate-limited warn + v1.2 hard dependency

## Version Roadmap

详见 `version-roadmap.md` — 完整演进路线 v1.0 → v3.0：

```
v1.0 ─→ v1.1 ─→ v1.2 ─→ v1.5 ⭐ ─→ v2.0 ─→ v2.1 ─→ v3.0
产权      写入      执行      最小       完整       跨会话     自演化
集中      统一      观测      递归       递归                 任务系统
```

三个里程碑，每个独立交付价值：
- **A (v1.0–v1.2):** 产权集中，6→1 模型，WorkTree 唯一真相源
- **B (v1.5–v2.0):** 递归求解，最小递归 MVP → 完整递归引擎
- **C (v2.1–v3.0):** 自演化，跨会话记忆 → 任务模板学习

## Next Steps

1. S3-Gate Review：`gaming-analysis.md` v2 + `version-roadmap.md` + 双边共识
2. 通过后 → `/openspec-apply devrix-unified-work-tree` 从 Phase 0 开始
3. 并行推进 DM-011 RunRegistry Phase 1（解锁 v1.2）— ✅ 内联实现完成
4. v1.5 最小递归作为独立里程碑验证核心循环 — ✅ PR #85–#87
5. v2.1+ 演进项登记 tech-debt — ✅ `openspec/tech-debt/worktree-v2-deferred.md`

---

## Archive Information

**Archived:** 2026-06-18  
**Outcome:** Successfully implemented (v1.0–v2.0); v2.1+ deferred to tech-debt  
**PRs:** #83, #84, #85, #86, #87  
**Specs Updated:** `specs/d7-orchestration/spec.md` v3.8.0, `d7-domain.md` v1.1.0, `t-registry.md`
