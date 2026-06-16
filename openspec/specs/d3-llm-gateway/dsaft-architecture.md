# D3 LLM Gateway — DSAFT 层计数（Stub）

**Capability:** llm-gateway
**Status:** Deprecated — 内容已迁移
**Version:** 2.0.0
**Last Updated:** 2026-06-16
**Superseded By:** `d3-domain.md` · `terminal-state-guide.md` · `observability-guide.md`

---

> **本文件仅为历史入口保留。** 领域边界、5+1 承诺、Span Runbook 请读下方迁移表。

## 迁移去向

| 原内容 | 现 SoT |
|--------|--------|
| North Star 5 承诺 + S6 | `d3-domain.md` |
| S/A/F 明细 | `a-registry.md` / `f-registry.md` |
| D7 Invoke 时序 | `terminal-state-guide.md` |
| Span↔T、P0 Runbook | `observability-guide.md` |
| Gherkin 验收 | `spec.md` |
| 实现细节 | `design.md` |

## DSAFT 五层计数（Canonical）

| Layer | D3 Mapping |
|-------|------------|
| **D** | D3 LLM Gateway |
| **S** | S1–S6（6） |
| **A** | 6 |
| **F** | 30 域内 + CROSS |
| **T** | 35（19 P0） |

## 修订记录

| Version | Date | Changes |
|---------|------|---------|
| 1.x | 2026-06-14 | V3 S/A 重切分析 |
| **2.0.0** | **2026-06-16** | **收敛为 Stub**；明细迁至 `d3-domain.md` + Guides |
