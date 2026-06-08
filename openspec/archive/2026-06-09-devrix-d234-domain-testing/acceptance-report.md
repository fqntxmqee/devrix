---
demand-id: DM-20260609-001
title: D2/D3/D4 域分段测试 — 验收报告
verdict: ACCEPTED
date: 2026-06-09
change: devrix-d234-domain-testing
---

# 验收报告

| AC | 结论 | 证据 |
|----|------|------|
| AC1 规范沉淀 | PASS | `domain-segmentation.md`, `spec.md`, `testing.md` |
| AC2 全量 tag | PASS | `test-integration.sh` 传齐 d1–d5,cross；不含 live |
| AC3 域脚本 | PASS | `test-domain.sh d2/d3/d4` |
| AC4 测试 green | PASS | integration + acceptance + e2e |

```text
./scripts/test-integration.sh   — PASS
./scripts/test-acceptance.sh      — PASS
./scripts/test-e2e.sh             — PASS
```

**已知例外：** `pev_synthesis_test.go` 3 个失败为既有问题，不在本变更范围。
