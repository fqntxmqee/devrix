---
demand-id: DM-20260609-001
title: D2/D3/D4 域分段测试（Build Tag 方案 B）
priority: P1
type: Testing Infrastructure
---

# Demand: D2/D3/D4 Domain Segmentation Testing

**Change ID:** devrix-d234-domain-testing
**Date:** 2026-06-09

## Context

现有测试金字塔按**层级**（unit / integration / acceptance）组织，缺少按 **D2 Context Engine、D3 LLM Gateway、D4 Multi-Agent** 独立运行的能力。改 D3 时仍需跑全量 integration，反馈慢且域边界不清。

## Goals

1. 引入 build tag 第二轴（`d2` / `d3` / `d4` / `cross` / `live`）
2. 提供 `./scripts/test-domain.sh {d2|d3|d4}` 分段入口
3. 沉淀至 `openspec/specs/testing-framework/domain-segmentation.md` 与 `testing.md`

## Success Criteria

| AC | 描述 |
|----|------|
| AC1 | 域 tag 注册于 testing-framework spec |
| AC2 | 全量脚本传齐域 tag，默认 CI 不含 `live` |
| AC3 | `test-domain.sh d2/d3/d4` 可独立运行 |
| AC4 | integration / acceptance / e2e 测试 green |

## Out of Scope

- 单元测试目录迁移
- D1/D5/D6 域脚本（仅 tag 登记，脚本可后续扩展）
- 修复 `pev_synthesis_test` 既有失败
