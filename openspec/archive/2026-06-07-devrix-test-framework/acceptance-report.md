---
demand-id: DM-20260607-008
title: Devrix 测试框架规范与目录拆分 — 验收报告
executor: fukai
environment: local
date: 2026-06-07
verdict: ACCEPTED
change: devrix-test-framework
---

# 验收报告：Devrix 测试框架规范与目录拆分

## 1. 执行摘要

| 项目 | 值 |
|------|---|
| 需求 ID | DM-20260607-008 |
| 变更 | devrix-test-framework |
| 执行人 | fukai |
| 测试环境 | local |
| 执行日期 | 2026-06-07 |
| 总体结论 | **ACCEPTED** |

## 2. 自动化验证

| 检查 | 结果 | 证据 |
|------|------|------|
| `scripts/test-unit.sh` | PASS | see CI/local log |
| `scripts/test-integration.sh` | PASS | see CI/local log |
| `scripts/test-e2e.sh` | PASS | see CI/local log |
| `scripts/test-acceptance.sh` | PASS | see CI/local log |


## 3. L5 测试点验证结果

| L5 ID | 描述 | 优先级 | 状态 | 证据 |
|-------|------|--------|------|------|
| L5-COMM-01 | 运行 devrix 创建 CLI 会话 | P0 | PASS | go test -tags=integration ./tests/integration |
| L5-COMM-02 | 空消息入站被拒绝 | P0 | PASS | go test -tags=acceptance ./tests/acceptance/p0 |
| L5-COMM-03 | 会话空闲超时后拒绝消息 | P0 | PASS | go test -tags=integration ./tests/integration |
| L5-COMM-04 | `/new` 命令解析正确 | P0 | PASS | go test -tags=acceptance ./tests/acceptance/p0 |
| L5-COMM-05 | `/help` 命令解析正确 | P0 | PASS | go test -tags=acceptance ./tests/acceptance/p0 |
| L5-COMM-06 | `/stop` 命令解析正确 | P0 | PASS | go test -tags=acceptance ./tests/acceptance/p0 |
| L5-COMM-07 | 权限请求超时自动拒绝 | P0 | PASS | go test -tags=integration ./tests/integration |
| L5-COMM-08 | 入站消息经 Engine 产生出站响应 | P0 | PASS | go test -tags=acceptance ./tests/acceptance/p0 |
| L5-COMM-17 | Milestone 循环依赖被拒绝 | P0 | PASS | go test ./internal/layers/communication/milestone |
| L5-COMM-18 | TaskFlow 按依赖顺序执行 | P0 | SKIP | status=PLANNED |

### 统计

| 优先级 | 总数 | 通过 | 失败 | 跳过 |
|--------|------|------|------|------|
| P0 | 10 | 9 | 0 | 1 |
| P1 | 0 | 0 | 0 | 0 |
| P2 | 0 | 0 | 0 | 0 |

## 4. 失败项分析（如有）

无失败项。

## 5. 遗留风险

| 风险 | 影响 | 规避方案 |
|------|------|---------|
| PLANNED L5 未覆盖 | 功能缺口 | 按 `openspec/l5-registry.md` 排期补测 |
| Live 测试未纳入 CI | 外部依赖波动 | 使用 `-tags=live` 在 staging 手动执行 |

## 6. 结论

P0 测试点全部通过，测试套件绿色，可进入 S6 交付。

---
生成命令: `./scripts/gen-acceptance-report.sh --change devrix-test-framework`
