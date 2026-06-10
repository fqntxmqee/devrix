---
demand-id: DM-20260610-004
title: 可观察层文档同步 — Canonical Trace Tree 与 Coverage 指南
source: observability P0-P2 归档后续
priority: P2
status: ACCEPTED
l1-domain: observability
created: 2026-06-10
parent-demand: DM-20260610-003
---

# 可观察层文档同步

P0–P2 代码已交付，但 `docs/observability-design.md` 仍标记大量「缺失」项。本 change 将文档与 `openspec/specs/observability/spec.md` v1.7.0 及 registry 对齐。

## 验收

- `docs/observability-design.md` 含 Canonical Trace Tree（R1–R5）与当前 metrics/CLI 清单
- `docs/coverage.md` 含按 Layer 验收说明与条件触发 zero-hit 解读
