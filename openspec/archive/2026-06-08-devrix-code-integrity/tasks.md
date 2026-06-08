# Tasks: Devrix 代码健康规范

**Change ID:** devrix-code-integrity
**Demand ID:** DM-20260608-009
**Status:** S5 Accepted — 待 S7 归档

---

## Phase 1: 规范发布 (Group A)

- [x] **T1.1** `openspec/specs/project/coding.md` §9 代码完整性
- [x] **T1.2** `CLAUDE.md` 不可变性条款修正
- [x] **T1.3** `review-code.md` 增加不可变性/CQS 检查项

## Phase 2: 安全修复 (Group B)

- [x] **T2.1** `connection/manager.go` emitEvent type switch
- [x] **T2.2** `connection/manager_test.go` L5-0-1-03~05

## Phase 3: D1 L5 测试 (Group C)

- [x] **T3.1** L5-1-1-01 / L5-0-1-06 — `comm_gateway_flow_test.go`
- [x] **T3.2** L5-1-3-01~03 / L5-0-1-07 — `comm_commands_test.go`
- [x] **T3.3** L5-1-2-01 / L5-0-1-08 — `feishu_test.go` Covers 标注
- [x] **T3.4** L5-1-8-01 — 已有 `shortid_test.go`，registry IMPLEMENTED

## Phase 4: D6 排期 (Group D)

- [x] **T4.1** D6 L5 标注 PlannedVersion v2.1.0 / v2.2.0

## Phase 5: 命名/异味 (Group E)

- [x] **T5.1** `CLRenderer` → `CLIRenderer`
- [x] **T5.2** 删除自定义 `min()`，使用 built-in
- [x] **T5.3** `GetInstances` 消除副作用 + CQS 单测

## Phase 6: 验收与归档

- [x] **T6.1** `l5-registry.md` 更新 IMPLEMENTED
- [x] **T6.2** `acceptance-report.md`
- [x] **T6.3** S7 归档

## Completion Checklist

- [x] AC1~AC3, AC5~AC8 满足
- [x] AC4 D6 排期标注（非立即实现）
- [x] `go test ./internal/layers/communication/...` 全绿
- [x] `go test -tags=acceptance ./tests/acceptance/p0/...` 全绿
