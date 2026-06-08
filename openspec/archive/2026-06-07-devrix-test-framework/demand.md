---
demand-id: DM-20260607-008
title: Devrix 测试框架规范与目录拆分
source: 技术团队 / AI Agent
priority: P0
status: DELIVERED
l1-domain: devrix
created: 2026-06-07
---

# Devrix 测试框架规范与目录拆分

## 1. 原始描述

> 基于现有代码和 OpenSpec 交付管线，设计并落地 Devrix 测试框架：
> - 单元测试保留 Go 惯例（同目录 `*_test.go`）
> - 集成 / E2E / 验收测试拆分到 `tests/`
> - L5 测试点可追溯，支撑 S5 自动化验收
> - 规范沉淀为 OpenSpec SoT，后续开发必须遵守

## 2. 澄清记录

### Q1: 测试文件是否全部迁出 internal？
**A**: 否。仅集成/E2E/验收迁出；单元测试保留同目录。 — 2026-06-07

### Q2: Mock 放哪里？
**A**: 正式包禁止 `mock_*.go`；同包测未导出符号用 `*_mock_test.go`；跨包用 `tests/testutil/`。 — 2026-06-07

### Q3: 如何与 OpenSpec S5 对齐？
**A**: `openspec/l5-registry.md` 登记 L5，`// Covers: L5-*` 追溯，验收跑 `./scripts/test-acceptance.sh`。 — 2026-06-07

## 3. 澄清范围

### 3.1 L1-L5 映射

| 层级 | 资产 ID | 名称 | 状态 |
|------|---------|------|------|
| L1 | devrix | 开发大脑 | 已有 |
| L2 | L2-DEVRIX-01 | 规格驱动交付 | 已有 |
| L3-BE | L3-BE-DEVRIX-01 | 自动化测试与验收 | 新增 |
| L4-BE | L4-BE-DEVRIX-TEST | 测试框架 | 新增 |
| L5 | L5-COMM-* 等 | 通信层测试点 | 草拟→正式 |

### 3.2 范围

**In Scope**:
- 测试目录拆分（tests/integration, e2e, acceptance）
- OpenSpec testing-framework spec
- L5 注册表
- 测试执行脚本
- Cursor 规则约束

**Out of Scope**:
- CI workflow（后续变更）
- `gen-acceptance-report.sh` 自动生成（后续变更）
