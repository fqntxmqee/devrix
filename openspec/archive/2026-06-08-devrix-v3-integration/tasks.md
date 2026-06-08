# Tasks: Communication V3 集成补全

**Change ID:** devrix-v3-integration
**Demand ID:** DM-20260608-010
**Status:** S5 Accepted（待 S7 归档）

---

## Phase 1: 规格与 L5 登记（S3 → S4 门禁）

- [x] **T1.1** 确认 `demand.md` L5 映射表与 proposal 一致
- [x] **T1.2** 更新 `openspec/l5-registry.md`
- [x] **T1.3** DM-008 归档包只读追溯
- [x] **T1.4** 更新 `demand-archive-index.md` Active Changes

## Phase 2: 测试补全（S4）

- [x] **T2.1** 环检测单测 — L5-1-5-01
- [x] **T2.2** 多里程碑 TaskFlow 链 — L5-1-5-02
- [x] **T2.3** ProgressBar + StatusBadge — L5-1-8-02
- [x] **T2.4** dingtalk Covers L5-1-2-02
- [x] **T2.5** milestone 出站渲染 — L5-1-2-03
- [x] **T2.6** Register/Unregister — L5-1-1-02

## Phase 3: 热路径接线（S4）

- [x] **T3.1** DingTalkAdapter OnMessage 渲染分支
- [x] **T3.2** Gateway milestone_progress render metadata
- [x] **T3.3** devrix-dingtalk Instance Registry
- [x] **T3.4** devrix-feishu Instance Registry
- [x] **T3.5** 删除 `task_flow.go` — L5-1-5-03

## Phase 4: 文档同步（S4/S6）

- [x] **T4.1** config-environment.md dingtalk 入口
- [ ] **T4.2** project.md D1-S2 补充钉钉（P2 可选）
- [x] **T4.3** acceptance-report.md

## Phase 5: 验收与归档（S5–S7）

- [x] **T5.1** L5 逐项验收
- [x] **T5.2** l5-registry IMPLEMENTED
- [x] **T5.3** S7 归档
- [x] **T5.4** demand-archive-index 归档条目

## Completion Checklist

- [x] 全部 P1 L5 IMPLEMENTED
- [x] P2 L5 IMPLEMENTED
- [x] `go test ./internal/layers/communication/...` 全绿
- [x] S7 归档完成

---

## 预估

| Phase | 估时 |
|-------|------|
| Phase 2 测试 | 2h |
| Phase 3 接线 | 3h |
| Phase 4 文档 | 1h |
| Phase 5 验收归档 | 1h |
| **合计** | **~7h** |
