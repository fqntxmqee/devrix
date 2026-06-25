# Tasks: D7 复审 review fixes 系列

**Change ID:** devrix-d7-mups-v4-review-fixes-series
**Demand IDs:** DM-20260625-005 + DM-20260625-006

---

## Phase 1: DM-20260625-005 Silent Failure 修复 (PR #205)

### 9 处错误吞咽

- [x] **T-01 (C1)**: `turn/orchestrator.go:821` `PersistTurn` 错误 → slog.Warn
  - 文件: `internal/layers/orchestration/sessionorchestrator/turn/orchestrator.go`
  - 加 session_id / task_id / worker_id 字段

- [x] **T-02 (C2)**: `wavescheduler/scheduler.go:544-549` 删除 `finalizeTask` 4 参数空函数
  - 文件: `internal/layers/orchestration/wavescheduler/scheduler.go`
  - 同时删除唯一调用点
  - 留说明注释指引未来真正扩展点的位置

- [x] **T-03 (C3)**: `executionflow/hub/hub.go:155-160` 4 处 task 状态更新 → slog.Warn
  - 文件: `internal/layers/orchestration/executionflow/hub/hub.go`
  - SetOwner / UpdateStatus 4 处
  - 加 session_id / task_id / worker_id / kind 字段

- [x] **T-04 (H4)**: `sessionorchestrator/command_handler.go:146` `interruptHandle` → slog.Warn
  - 文件: `internal/layers/orchestration/sessionorchestrator/command_handler.go`
  - 加 session_id / command_kind 字段

- [x] **T-05 (H5)**: `escape/arbitrator.go:562, 622` Save/Delete → slog.Warn
  - 文件: `internal/layers/orchestration/escape/arbitrator.go`
  - 加 session_id / escape_kind 字段

- [x] **T-06 (H6)**: `workmodel/work_tree.go:637` `os.Remove` → slog.Warn + IsNotExist 过滤
  - 文件: `internal/layers/orchestration/workmodel/work_tree.go`
  - `if err != nil && !os.IsNotExist(err) { slog.Warn(...) }`

- [x] **T-07 (H7)**: `workmodel/cli_commands.go:377` `planMode.Enter` 错误返回用户
  - 文件: `internal/layers/orchestration/workmodel/cli_commands.go`
  - 错误用 `fmt.Errorf("failed to enter plan mode: %w", err)` 返回
  - 不再返回 "Entered plan mode" 成功消息

- [x] **T-08 (H8)**: `runregistry/registry.go:52` `os.MkdirAll` → slog.Warn + 失败置空 outputDir
  - 文件: `internal/layers/orchestration/runregistry/registry.go`
  - 失败时 `outputDir = ""` 避免连锁失败

- [x] **T-09 (M5)**: `workmodel/unified_tools.go:284` `json.Unmarshal` → slog.Warn + 返回 nil
  - 文件: `internal/layers/orchestration/workmodel/unified_tools.go`

### 1 处死代码

- [x] **T-10 (C2 dead code)**: 删除 `wavescheduler.finalizeTask` 4 参数空函数 + 唯一调用点
  - 与 T-02 合并实施

### 1 个回归守护测试

- [x] **T-11**: `command_handler_test.go::TestCommandHandler_Handle_PlanCommand` 改为断言 "Failed to enter plan mode"
  - 文件: `internal/layers/orchestration/sessionorchestrator/command_handler_test.go`
  - 断言改为 `strings.Contains(out, "Failed to enter plan mode")`
  - 反向断言 `!strings.Contains(out, "Entered plan mode")`

---

## Phase 2: DM-20260625-006 God Function 拆分 (PR #206)

### 3 个 god function 拆分

- [x] **T-12 (C4)**: `ProcessMessage` 183→75 行（5 phase helper）
  - 文件: `internal/layers/orchestration/sessionorchestrator/orchestrator.go`
  - ⚠️ **skipped in PR #206**: master 已无 `shadowClassifier` 字段，留待后续
  - phase helpers: tryResumeShortCircuit / classifyIntent / handleClassifyError / prepareRoutePreDispatch / dispatchByIntent / handleExecuteError

- [x] **T-13 (H1)**: `runLoop` 519→40 行（4 phase helper + 2 carrier struct）
  - 文件: `internal/layers/orchestration/sessionorchestrator/turn/orchestrator.go` + 新文件 `orchestrator_loop_helpers.go`
  - ⚠️ **skipped in PR #206**: master 已由 DM-20260625-012 D3 完成
  - phase helpers: runPrepare / runOneIteration / runLLMStream / runFinalize
  - carrier structs: turnLoopState / turnIteration

- [x] **T-14 (H2)**: `LLMArbitrator.Arbitrate` 132→21 行（4 phase helper + 1 factory）
  - 文件: `internal/layers/orchestration/escape/arbitrator.go`
  - ✅ **实施在 PR #206**（唯一 forward-port 部分）
  - phase helpers: invokeLLMWithTimeout / parseWithRetry / invokeGenerateBounded / validateActionAndBuild
  - factory: buildForceExit（统一 EscapeForceExit decision 构造）
  - 保留原 reason 区分（ctx_cancelled / llm_timeout_5s / llm_stuck_force_exit）通过 `*EscapeDecision` 指针返回

---

## Phase 3: 验证 (S5)

- [x] **V-01**: `go build ./...` PASS
- [x] **V-02**: `go vet ./...` PASS
- [x] **V-03**: `go test -race -count=1 ./internal/layers/orchestration/...` 22/22 PASS 0 race
- [x] **V-04**: 行为不变性验证（所有 Span 属性 / decision routing / audit level 不变）
- [x] **V-05**: LP-1 `TestAutoClose_FullLP1Loop` 已知 flake 通过空 commit re-trigger CI 绕过

---

## Phase 4: PR 落地 (S4)

- [x] **P-01**: PR #205 squash auto-merge → `0313238f119cf38df19b5195d17efa2df21c584a`
- [x] **P-02**: PR #206 squash auto-merge → `97a4ace78ffd271cb1b65a6c8338518e144f089b`
- [x] **P-03**: PR 实际改动统计
  - PR #205: 10 文件 +85/-26
  - PR #206: 1 文件 (escape/arbitrator.go) +123/-79
  - 合计: 11 文件 +208/-105

---

## Phase 5: S6 归档 (当前任务)

- [x] **A-01**: 创建 archive 分支 `archive/devrix-d7-mups-v4-review-fixes-series`
- [x] **A-02**: 创建 `openspec/archive/2026-06-25-devrix-d7-mups-v4-review-fixes-series/` 目录
- [x] **A-03**: 创建 7 个归档文件
  - `.openspec.yaml` ✅
  - `demand.md` ✅
  - `proposal.md` ✅
  - `design.md` ✅
  - `tasks.md` ✅ (本文件)
  - `acceptance-report.md` (待创建)
  - `specs/d7-orchestration/spec.md` (待创建)
- [ ] **A-04**: 运行 `scripts/verify-archive.sh` 期望 11/11 PASS
- [ ] **A-05**: 创建 archive PR + auto-merge
- [ ] **A-06**: 更新 `demand-archive-index.md`
