# D1 Communication — DSAFT 层计数（Stub）

**Capability:** d1-communication
**Status:** Deprecated — 内容已迁移
**Version:** 2.0.0
**Last Updated:** 2026-06-16
**Superseded By:** `d1-domain.md` · `terminal-state-guide.md` · `observability-guide.md`

---

> **本文件仅为历史入口保留。** 领域边界、跨域流程、IntentKind 时序、Span Runbook 请读下方迁移表，勿在本文件追加内容。

## 迁移去向

| 原内容 | 现 SoT |
|--------|--------|
| North Star / Out of Scope / 6S 承诺 | `d1-domain.md` |
| S/A 明细、代码路径 | `a-registry.md` |
| A→F 编排树、主路径、时序图 | `terminal-state-guide.md` |
| Span↔T、必达演练、P0 Runbook | `observability-guide.md` |
| Gherkin 验收 | `spec.md` |
| EventBus / CardKit 实现细节 | `design.md` |

## DSAFT 五层计数（Canonical）

| Layer | D1 Mapping |
|-------|------------|
| **D** | D1 Communication |
| **S** | S13–S18（6） |
| **A** | 16 |
| **F** | 18 |
| **T** | 56（26 P0） |

## 修订记录

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-15 | Initial DSAFT architecture analysis |
| 1.1.0 | 2026-06-16 | 跨域表同步 DM-007 |
| **2.0.0** | **2026-06-16** | **收敛为 Stub**；明细迁至 `d1-domain.md` + Guides |
