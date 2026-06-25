# D7 Orchestration — DSAFT 层计数（Stub）

**Capability:** d7-orchestration
**Status:** Active — pointer stub (counts verified 2026-06-25, MUPS v4.3 + v5 EscapeEngine 闭环)
**Version:** 3.0.0
**Last Updated:** 2026-06-25
**Superseded By:** `d7-domain.md` · `terminal-state-guide.md` · `observability-guide.md`

---

> **本文件仅为历史入口保留。** 领域边界、IntentKind 四链、Span Runbook 请读下方迁移表，勿在本文件追加内容。

## 迁移去向

| 原内容 | 现 SoT |
|--------|--------|
| North Star / Out of Scope / 5S 承诺 + 14 S 层 + MUPS 5 节点管道 | `d7-domain.md` |
| S/A 明细、代码路径（含 56 A 全部 S1-S14）| `a-registry.md` |
| IntentKind 四链、A→F 编排树、跨域时序、14 ExitReason、Auto-Close 4 规则、ResumeSession 3 决策路由 | `terminal-state-guide.md` |
| Span↔T、Trace 树、5 节点管道 trace、P0 Runbook | `observability-guide.md` |
| Gherkin 验收 | `spec.md` |
| Wave / Hub / PlanMode / MUPS 5 节点 / v5 EscapeEngine 实现细节 | `design.md` |
| Review R1/R2 完整澄清 | `d7-requirements-clarifications.md` |

## DSAFT 五层计数（Canonical，2026-06-25 闭环）

| Layer | D7 Mapping |
|-------|------------|
| **D** | D7 Orchestration |
| **S** | **S1–S14（14 个 Canonical S 层，含 MUPS 5 节点管道 S7-S11 + 横切 S6/S12/S13/S14）** |
| **A** | **56**（S1:6 · S2:7 · S3:4 · S4:5 · S5:4 · S6:1 · S7:1 · S8:1 · S9:2 · S10:4 · S11:5 · S12:3 · S13:3 · S14:3 + Legacy 7）|
| **F** | **75**（Legacy 48 + Canonical 27，见 `f-registry.md`） |
| **T** | **180**（v4.9.0 2026-06-25 闭环，含 MUPS 5 节点 + 横切硬化） |
| **Span** | **18 ops**（9 旧 + 9 MUPS/Escape 新增）+ sessionSpan 9 attributes（6 prior + 3 resume）|

### MUPS 5 节点管道 节点级计数

| 节点 | S 层 | A 数 | F 数 | T 数 |
|------|------|------|------|------|
| Observe | D7-S8 | 1 | 6 | 6 P0 |
| Plan | D7-S8 PR-B1 | 1 | 3 | 3 P0 |
| Execute | D7-S9 | 2 | 8 | 9（4 P0）|
| Verify | D7-S10 | 4 | 9 | 8 P0 |
| Learn | D7-S11 | 5 | 14 | 5 P0 |
| Observe-Learner 闭环 | D7-S12 | 3 | 6 | 6 P0 |
| Verify Auto-Close | D7-S13 | 3 | 5 | 6 P0 |
| EscapeEngine | D7-S14 | 3 | 9 | 18（17 P0）|

## 修订记录

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-15 | Initial DSAFT architecture analysis |
| 2.1.0 | 2026-06-19 | v2.0 Structure 路径对齐后计数复核（S1–S5 物理路径见 `code-layout.md` §4.2） |
| **3.0.0** | **2026-06-25** | **MUPS v4.3 5 节点管道 + v5 EscapeEngine 落地计数**：S 5 → 14；A 24 → 56；F 51 → 75；T 66 → 180；Span 9 → 18。新增 §MUPS 5 节点管道 节点级计数表（Observe/Plan/Execute/Verify/Learn + 横切 闭环/Auto-Close/EscapeEngine）。|
