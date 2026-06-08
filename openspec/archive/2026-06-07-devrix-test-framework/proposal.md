# Proposal: Devrix Testing Framework

**Change ID:** devrix-test-framework
**Type:** Infrastructure / Quality
**Status:** Delivered

---

## Motivation

Devrix 已有分散的单元测试，但缺少：
- 与 OpenSpec L5 的追溯链
- 集成/E2E/验收的分层执行
- 正式包中的 Mock 污染
- 可执行的验收门禁脚本

## What Changes

1. 建立 `openspec/specs/testing-framework/spec.md` 作为测试框架 SoT
2. 建立 `openspec/l5-registry.md` 作为 L5 主注册表
3. 拆分 `tests/{integration,e2e,acceptance,testutil}/`
4. 提供 `scripts/test-*.sh` 执行门禁
5. 添加 Cursor 规则强制遵守

## Capabilities

| Capability | L1/L2 映射 | 说明 |
|------------|-----------|------|
| testing-framework | L2-DEVRIX-01 | 测试金字塔、目录、Mock、L5 追溯、执行门禁 |

## Impact

- 所有 `internal/` 新代码的测试放置方式
- 所有 `openspec/changes/*/tasks.md` 的 L5 标注要求
- PR 提交前必须跑 `test-unit.sh`

## Goals (SLO)

| 指标 | 目标 |
|------|------|
| PR 单元测试 | < 2min |
| 全量测试 | < 15min |
| P0 L5 覆盖率 | 100% 有测试用例或 PLANNED 登记 |
| 覆盖率阈值 | ≥ 80%（合入门禁） |
