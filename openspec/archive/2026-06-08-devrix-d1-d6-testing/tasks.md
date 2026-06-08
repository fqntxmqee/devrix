# Tasks: D1 & D6 Testing Coverage

**Demand ID:** DM-20260608-011
**Change ID:** devrix-d1-d6-testing

---

## T1: D1 命令解析 (L5-1-3-01~03)

- [x] **T1.1** `ParseCommand` 支持 TrimSpace + EqualFold
- [x] **T1.2** `command_test.go` 表驱动用例 + Covers 标注
- [x] **T1.3** `comm_commands_test.go` 验收层同步

## T2: D1 Gateway (L5-1-1-01)

- [x] **T2.1** `comm_gateway_flow_test.go` 会话创建拒绝（DM-009 已交付，本变更验收确认）

## T3: D1 Adapters (L5-1-2-01)

- [x] **T3.1** `feishu_test.go` Covers 标注（DM-009 已交付）

## T4: D1 ShortId (L5-1-8-01)

- [x] **T4.1** `shortid_test.go` 1000 次唯一性 + Covers 标注

## T5: D6 Evolution (L5-6-1-01, L5-6-2-01)

- [x] **T5.1** 确认 evolution 层未实现 → 保留 PLANNED + PlannedVersion
- [x] **T5.2** acceptance-report 记录 P2 例外

## T6: OpenSpec 交付

- [x] **T6.1** proposal.md / tasks.md / acceptance-report.md
- [x] **T6.2** 更新 l5-registry 状态汇总
- [x] **T6.3** S7 归档 + demand-archive-index
