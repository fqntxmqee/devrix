# Tasks: D7 Metrics Naming Alignment & Concurrency Hardening

**Change ID:** devrix-d7-metrics-and-concurrency-hardening
**Demand ID:** DM-20260622-001
**Status:** S2_Proposal
**Date:** 2026-06-22

---

## Phase 0: Setup（不在 PR-A 内）

- [x] 创建 change 目录 `openspec/changes/devrix-d7-metrics-and-concurrency-hardening/`
- [x] `.openspec.yaml` 元数据
- [x] `proposal.md`（S2，本提案）
- [x] `tasks.md`（S4，本文件）
- [x] `specs/d7-orchestration/spec_delta.md`（Gherkin 验收）
- [x] `specs/d7-orchestration/t-registry_delta.md`（6 个新 P0 T 点）
- [ ] 分支 `feat/devrix-d7-metrics-and-concurrency-hardening` 从 master 拉出

---

## PR-A: Metrics Naming Alignment + Concurrency Hardening

### A1. Metric 命名 spec/code 对齐（PR-A step 1）

- [ ] **A1.1** 修改 `wavescheduler/scheduler.go:306, 309` 把 `incMetric("dispatch_wakeup")` 改为 `incMetric("dispatch_loop_wakeups")`
- [ ] **A1.2** 修改 `wavescheduler/scheduler.go:414` 把 `incMetric("worker_panic")` 改为 `incMetric("worker_panics")`
- [ ] **A1.3** 修改 `wavescheduler/scheduler.go:418` slog tag `"metric", "worker_panic"` → `"worker_panics"`
- [ ] **A1.4** 修改 `wavescheduler/scheduler_metrics_test.go:24, 33` 测试断言字符串（`"dispatch_wakeup"` → `"dispatch_loop_wakeups"`，`"worker_panic"` → `"worker_panics"`）
- [ ] **A1.5** 验证 `wavescheduler/metrics.go` `incMetric` switch case 的 string literal 与新名一致
- [ ] **A1.6** 跑 `grep -n "incMetric(" wavescheduler/scheduler.go` 确认无遗漏旧名

### A2. sandbox_exit_failed 归属澄清（PR-A step 2，spec only）

- [ ] **A2.1** 修改 `openspec/specs/d7-orchestration/spec.md` D7-S6-A12 章节
  - 在 T01 处加注释 "**Counter 由 D4 multiagent/execute/worker.go 提供（见 D4-S6-A12-Txx），D7 wavescheduler 不重复声明**"
  - 新增 §"跨域 reference" 子章节列 D4 对应 T 点
- [ ] **A2.2** 修改 `openspec/specs/d7-orchestration/t-registry.md`
  - D7-S6-A12-T01 改 status 为 `OBSOLETE`（保留作为历史记录 + cross-ref）
  - 加 cross-ref 指向 D4 域对应 T 点

### A3. state.cancels bound cleanup（PR-A step 3）

- [ ] **A3.1** 修改 `wavescheduler/scheduler.go:549-560` `markWaveDone` 函数
  - 在 wave 完成判定后追加 `state.cancels = nil`（紧跟 wave status 更新）
  - 验证 `state.handles` map 同步清理逻辑
- [ ] **A3.2** 新增 `wavescheduler/scheduler_test.go::TestStateCancels_NilAfterWaveDone`
  - 构造 wave 调度 → 等 wave 完成
  - 验证 `s.state.cancels == nil`（或 len == 0）
  - 验证 `s.state.handles` 同步空
- [ ] **A3.3** 新增 `wavescheduler/scheduler_test.go::TestStateCancels_NoLeakAcrossWaves`
  - 构造同一 session 多次 wave 重入（mock）
  - 验证每次 wave 完成时 slice 被清空，不累积

### A4. AllowAndRegister 热路径接入（PR-A step 4）

- [ ] **A4.1** 修改 `wavescheduler/scheduler.go:283` `dispatchLoop` 内
  - 当前 `s.guard.Allow(node, s.guard.Running())` —— 改用 `s.guard.AllowAndRegister(node, s.guard.Running())` **在 dispatchOne 内**
- [ ] **A4.2** 修改 `wavescheduler/scheduler.go:352` `dispatchOne` 内
  - 删除独立的 `s.guard.Register(...)` 调用（已被 A4.1 合并）
  - 验证 `slotID` 在 AllowAndRegister 时已确定（需重构获取 slot 的逻辑）
- [ ] **A4.3** 新增 `wavescheduler/scheduler_orch_test.go::TestDispatchLoop_UsesAllowAndRegister_OnHotPath`
  - 构造并发 dispatch（mock 3+ ready nodes）
  - 验证 hot path 只调用 `AllowAndRegister`（spy count = N），不调用 split `Allow` + `Register`
  - 验证无 TOCTOU race（用 `go test -race` 跑 100 次）
- [ ] **A4.4** 跑 `go test -race ./internal/layers/orchestration/wavescheduler/...` 确认无 race detector 报警

### A5. command_handler.go select-default 加固（PR-A step 5）

- [ ] **A5.1** 修改 `sessionorchestrator/command_handler.go:90-101`
  - 把 `out <- &contracts.EngineEvent{...}` 改为 select-default 模式：
    ```go
    select {
    case out <- &contracts.EngineEvent{...}:
    default:
        slog.Warn("command_handler: out channel full, drop event",
            "type", event.Type, "session", sessionID)
    }
    ```
- [ ] **A5.2** 新增 `sessionorchestrator/command_handler_test.go::TestCommandHandler_OutChannelFull_DropsEvent`
  - mock 满 channel（pre-fill 32 events）
  - 触发新 event
  - 验证 emit 不阻塞（timeout < 100ms）
  - 验证 slog.Warn 被调用
- [ ] **A5.3** 验证现有 `/help`, `/stop` 路径行为不变（regression）

### A6. spec.md 更新

- [ ] **A6.1** 在 `openspec/specs/d7-orchestration/spec.md` 新增 §D7-S6-A14
  - 命名: Metrics Naming Alignment & Concurrency Hardening
  - 描述: 5 个修复点的 Gherkin scenario
- [ ] **A6.2** 修改 §D7-S6-A12 中 metric 名（保留 plural 形式）

### A7. t-registry 更新

- [ ] **A7.1** 在 `openspec/specs/d7-orchestration/t-registry.md` 新增 6 个 P0 T 点：
  - **D7-S6-A14-T01**: dispatch_wakeup → dispatch_loop_wakeups rename
  - **D7-S6-A14-T02**: worker_panic → worker_panics rename
  - **D7-S6-A14-T03**: sandbox_exit_failed 跨域归属（D7 不重复声明）
  - **D7-S6-A14-T04**: state.cancels nil reset after wave done
  - **D7-S6-A14-T05**: AllowAndRegister 热路径接入（hot path 改用原子调用）
  - **D7-S6-A14-T06**: command_handler out send select-default 加固
- [ ] **A7.2** 把 6 个 T 点 status 从 `PLANNED` 改为 `IMPLEMENTED`（PR merge 后）

### A8. PR-A 验收

- [ ] **A8.1** `go vet ./...`
- [ ] **A8.2** `go test -race ./internal/layers/orchestration/wavescheduler/...`
- [ ] **A8.3** `go test -race ./internal/layers/orchestration/sessionorchestrator/...`
- [ ] **A8.4** `go test -race ./internal/layers/multiagent/...`
- [ ] **A8.5** `./scripts/test-unit.sh` 全量通过（≥80% 覆盖率）
- [ ] **A8.6** 提交 commit `feat(d7): metrics naming alignment + concurrency hardening (DM-20260622-001 PR-A)`
- [ ] **A8.7** 创建 PR `devrix-d7-metrics-and-concurrency-hardening PR-A` → 启用 auto-merge
- [ ] **A8.8** PR merge 后跑 `./scripts/verify-archive.sh devrix-d7-metrics-and-concurrency-hardening`

---

## Phase S5: Acceptance

### S5.1 P0 T 层验收

- [ ] 跑 `./scripts/test-acceptance.sh --change devrix-d7-metrics-and-concurrency-hardening`
- [ ] 验证 6 个新 P0 T 点全部 PASS：
  - D7-S6-A14-T01..T06
- [ ] 写 `acceptance-report.md` 总结 6 个 T 点 PASS 状态

### S5.2 综合评分验证

- [ ] 跑下一轮 D7 deep review（独立 session）验证综合评价回到 **A-**
- [ ] 5 个 counter 命名与 spec 100% 对齐（grep 验证）
- [ ] state.cancels 无 leak（新 metric 加 `state_cancels_cumulative` 也可）
- [ ] AllowAndRegister hot path 覆盖率 100%（grep 验证）
- [ ] command_handler select-default 单元测试覆盖

---

## Phase S6: Archive

### S6.1 归档脚本

- [ ] 提交 commit `docs(d7): spec sync + t-registry + acceptance report (DM-20260622-001 S6)`
- [ ] 创建 PR `devrix-d7-metrics-and-concurrency-hardening S6 (archive)` → 启用 auto-merge
- [ ] PR merge 后执行归档：
  - `mv openspec/changes/devrix-d7-metrics-and-concurrency-hardening openspec/archive/2026-06-22-devrix-d7-metrics-and-concurrency-hardening`
- [ ] 更新 `openspec/specs/d7-orchestration/spec.md` History（v4.0.0 → v4.1.0）
- [ ] 更新 `openspec/t-registry.md` 根索引加 change-id
- [ ] 更新 `openspec/demand-archive-index.md` 加 entry
- [ ] 通知用户验收完成 + 提供综合评分证据

---

## 总览

| Phase | PR | 任务数 | 文件数 | LoC 预估 | 风险 |
|-------|----|----|--------|----------|------|
| PR-A | Metrics + Concurrency Hardening | ~30 | ~8 | +200 / -30 | Low |
| S5 | Acceptance | ~6 | ~3 | +50 / 0 | Low |
| S6 | Archive | ~7 | ~4 | +30 / -5 | Low |
| **总计** | **单 PR × 1-2 天** | **~43 任务** | **~15 文件** | **+280 / -35** | **Low** |

**关键交付**：
- 6 个新 P0 T 点（D7-S6-A14-T01..T06）
- metric 命名 100% spec/code 对齐（D5 dashboard 立即可读）
- TOCTOU 窗口关闭（conflict guard hot path 改用 AllowAndRegister）
- state.cancels 无界增长修复（防止长会话 leak）
- command_handler 阻塞 send 加固
- spec.md / t-registry.md 同步更新
- 综合评价从 B+ 回到 A-

**关键非目标**（明确不做）：
- coordinator/aliases.go 退役 —— 独立 change
- turn/orchestrator.go 1349 行拆分 —— 独立 change
- UncertaintyCoord + AdaptiveThreshold wiring —— 独立 change（DM-20260620-005 P0）
- 三流格式统一 —— spec.md:210 defer

---

## 完成 Checklist

- [ ] A1-A8 全部勾选
- [ ] S5 acceptance 报告生成
- [ ] S6 归档通过 verify-archive.sh
- [ ] 通知用户验收 + 准备下一轮 review 验证 A- 评分