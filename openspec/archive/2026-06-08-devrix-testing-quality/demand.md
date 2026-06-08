---
demand-id: DM-20260608-004
title: 测试质量增强（边界/VCR/断言/性能基线）
source: L5 注册表测试审计
priority: P1
status: S7_ARCHIVED
l1-domain: testing-quality
created: 2026-06-08
---

# 测试质量增强

## 1. 原始描述

> 现有测试覆盖偏 Happy Path：Verify 超时/退出码、Shell injection、PEV 并发隔离缺失；LLM 依赖 Mock 缺少 429/SSE 错误场景；断言深度不足；无性能基线。

## 2. 澄清记录

### Q1: 是否修改生产代码？

**A**: 本变更仅新增/增强测试与 fixture，不修改业务逻辑。 — 2026-06-08

### Q2: 性能测试是否进 CI？

**A**: 使用 `-tags=performance` 独立标签，默认 CI 不运行。 — 2026-06-08

### Q3: VCR 策略？

**A**: 标准库 `http.RoundTripper` + JSON fixture 回放，CI 默认回放模式。 — 2026-06-08

## 3. L1–L5 映射草案

| 层级 | 资产 |
|------|------|
| L4 | 边界测试、VCR 集成、严格断言、性能基线 |
| L5 | L5-2-1-09~12, L5-2-2-08, L5-3-1-03, L5-3-2-06, L5-3-5-03, L5-5-2-06~07 |

## 4. 验收标准

- P0：Verify/Shell/PEV 边界测试全绿
- P1：VCR 回放 + Token 中文准确性
- P2：性能基线本地可执行
