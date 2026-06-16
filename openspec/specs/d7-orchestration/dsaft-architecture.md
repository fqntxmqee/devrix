# D7 Orchestration — DSAFT 层计数（Stub）

**Capability:** d7-orchestration
**Status:** Deprecated — 内容已迁移
**Version:** 2.0.0
**Last Updated:** 2026-06-16
**Superseded By:** `d7-domain.md` · `terminal-state-guide.md` · `observability-guide.md`

---

> **本文件仅为历史入口保留。** 领域边界、IntentKind 四链、Span Runbook 请读下方迁移表，勿在本文件追加内容。

## 迁移去向

| 原内容 | 现 SoT |
|--------|--------|
| North Star / Out of Scope / 5S 承诺 | `d7-domain.md` |
| S/A 明细、代码路径 | `a-registry.md` |
| IntentKind 四链、A→F 编排树、跨域时序 | `terminal-state-guide.md` |
| Span↔T、Trace 树、P0 Runbook | `observability-guide.md` |
| Gherkin 验收 | `spec.md` |
| Wave / Hub / PlanMode 实现细节 | `design.md` |
| Review R1/R2 完整澄清 | `d7-requirements-clarifications.md` |

## DSAFT 五层计数（Canonical）

| Layer | D7 Mapping |
|-------|------------|
| **D** | D7 Orchestration |
| **S** | S1–S5（5） |
| **A** | 24 |
| **F** | 51（Legacy 44 + Canonical 7，见 `f-registry.md`） |
| **T** | 66（44 P0） |

## 修订记录

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-15 | Initial DSAFT architecture analysis |
| **2.0.0** | **2026-06-16** | **收敛为 Stub**；明细迁至 `d7-domain.md` + Guides |
