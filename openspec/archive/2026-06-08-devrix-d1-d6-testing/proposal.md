# Proposal: D1 & D6 Testing Coverage

**Change ID:** devrix-d1-d6-testing
**Demand ID:** DM-20260608-011
**Status:** S4 Development
**Priority:** P1

---

## Motivation

覆盖率复盘（`devrix-layering-standard/coverage-review.md`）显示 D1 Communication 与 D6 Evolution 的 L5 测试点为零覆盖。D1 是用户入口域，缺少确定性测试会导致命令解析、会话拒绝、飞书入站等关键路径回归不可见。

## Relationship to DM-20260608-009

`devrix-code-integrity` 在 S4 已先行落地 D1 核心 L5（L5-1-1-01、L5-1-3-*、L5-1-2-01、L5-1-8-01）并登记 D6 排期。本变更 **闭合 OpenSpec 交付环**，并补充 design.md 中尚未覆盖的行为（命令大小写/trim、ShortId 1000 次唯一性）。

## Goals

| Goal | Priority | 成功信号 |
|------|----------|----------|
| D1 L5 6/6 IMPLEMENTED | P0 | registry + 测试 green |
| ParseCommand 行为与设计一致 | P1 | 大小写不敏感 + TrimSpace |
| D6 L5 排期明确 | P2 | L5-6-1-01 / L5-6-2-01 保留 PLANNED + PlannedVersion |

## Non-Goals

- 实现 `internal/layers/evolution/`（无 LoadAndWatch / version 模块）
- E2E 飞书 WebSocket 联调
- Gateway 多场景拒绝矩阵（rate limit / auth）— 留待后续集成测试变更

## Technical Approach

### D1 测试映射

| L5 | 测试位置 | 状态 |
|----|----------|------|
| L5-1-3-01~03 | `types/command_test.go` + `comm_commands_test.go` | IMPLEMENTED |
| L5-1-1-01 | `comm_gateway_flow_test.go` | IMPLEMENTED |
| L5-1-2-01 | `adapters/feishu_test.go` | IMPLEMENTED |
| L5-1-8-01 | `types/shortid_test.go` | IMPLEMENTED |

### ParseCommand 增强

- `strings.TrimSpace` 处理前导空白
- `strings.EqualFold` 匹配 new/stop/help

### D6 例外

`config.LoadAndWatch` 未实现 → L5-6-2-01 保留 v2.2.0 排期；L5-6-1-01 保留 v2.1.0 排期。

## Risks

| Risk | Mitigation |
|------|------------|
| 与 code-integrity 重复 | proposal 明确分工；本变更只做增量 + 归档 |
| D6 无法在本迭代交付 | acceptance 记录 P2 例外 |
