# Demand: D7 Layer SubContext Phase 3

**Demand ID:** DM-20260628-002  
**Created:** 2026-06-28  
**Reporter:** Layer SubContext Phase 1+2 归档 follow-up  
**Priority:** P1  
**Parent:** DM-20260627-003 (`devrix-d7-layer-subcontext`)

---

## 1. 原始诉求

Phase 1+2 归档时登记的三项 Phase 3 缺口：

1. **T33** — delegate SubTurn `brief`/`fork`/`full` 统一映射到 D2 MaterializePolicy  
2. **T34** — Wave scheduler `ContextResolver` 合并进 Materializer（`PartitionWave`）  
3. **T35** — Observe 阶段可选 LLM ObservationProposer（G3：LLM 提案 + 规则校验）

## 2. 业务目标

| ID | 目标 | 可验证承诺 |
|----|------|------------|
| **LC-P3-1** | SubTurn 统一 Materialize | brief/fork/full → fresh/fork/resume；bootstrap 默认 wired |
| **LC-P3-2** | Wave 统一 Materialize | fresh/resume/upstream 走 `PartitionWave`；legacy fallback 保留 |
| **LC-P3-3** | Observe LLM 提案层 | 结构化信号输入；ObsFact ≤ 0.85；LLM 失败 fail-safe |

## 3. Demand 级验收标准

- [x] **P0** SubTurn brief 不继承 parent history（fresh partition）
- [x] **P0** SubTurn fork 含 parent prefix；full 含 agent sidechain resume
- [x] **P0** Wave worker 三种 policy 均经 Materializer（Jaeger `D2_Context_Materialize` 可证）
- [x] **P1** ObservationProposer 不读 wi private ReAct jsonl
- [x] **P1** LLM 提案超 strength 或无 evidence 被规则 gate 丢弃
- [x] **P1** LLM 错误时 Observe 仍产出 rules-only Obs*

## 4. 关联文档

- Phase 1+2 归档：`openspec/archive/2026-06-28-devrix-d7-layer-subcontext/`
- CG2′ SoT：`openspec/specs/d7-orchestration/workitem-context-graph-design.md` v0.4.0
- D7 Observe SoT：`openspec/specs/d7-orchestration/spec.md` D7-S8 + D7-S16-A72/A74
