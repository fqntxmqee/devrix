# Tasks: D6 Evolution Review Fixes

**Change ID:** devrix-d6-evolution-review-fixes
**Demand ID:** DM-20260621-011
**Status:** S4_Tasks
**Date:** 2026-06-21

---

## Phase 0: Setup（不在 PR-A 内）

- [x] 创建 change 目录 `openspec/changes/devrix-d6-evolution-review-fixes/`
- [x] `.openspec.yaml` 元数据
- [x] `proposal.md`（S2）
- [x] `design.md`（S3）
- [x] `tasks.md`（S4，本文件）
- [x] `specs/d6-evolution/spec.md`（Gherkin 验收 — S5 时补充，本阶段空目录占位）
- [x] 分支 `feat/devrix-d6-evolution-review-fixes` 从 master 拉出

---

## PR-A: panic 修复 + 静默吞错（H-2 + H-3，最小风险）

### A1. verify/_invariant.go panic → log.Fatal

- [ ] **A1.1** 修改 `internal/layers/evolution/verify/_invariant.go`
  - `mustParseVerifyInvariants()` 改为 `parseVerifyInvariants() (ltllite.InvariantSet, error)`
  - 移除 `panic("ltllite: verify invariant parse failed: ...")`
  - 新增 `init()` 函数调用 `parseVerifyInvariants()`，失败走 `log.Fatalf`
  - `var verifyInvariantSet ltllite.InvariantSet` 改为 package-level var
- [ ] **A1.2** 确认 `verify/plan.go` 等其他文件对 `verifyInvariantSet` 的引用无需改签名（仅 init 时机变化）
- [ ] **A1.3** 新建 `verify/_invariant_test.go`
  - `TestParseVerifyInvariants_BadStruct_ReturnsError` — 标签错误返回 error
  - `TestParseVerifyInvariants_GoodStruct_Succeeds` — 默认 verifyInvariants{} 可解析
  - `TestVerifyInvariants_InitSucceeds` — init() 不 panic（默认 struct 合法）

### A2. guard/intervention.go silent swallow → metric + errors.Join

- [ ] **A2.1** 修改 `internal/layers/evolution/guard/metrics.go`
  - `orchMetrics` struct 新增 `WaitFailed atomic.Int64` + `TaskFailFailed atomic.Int64`
  - init 函数初始化这两个 atomic.Int64（无需 meter 注册，Go 原生 atomic）
- [ ] **A2.2** 修改 `internal/layers/evolution/guard/intervention.go:74`
  - `_, _ = current.Wait(ctx)` → 拆分为 waitErr + metric.Inc + slog.Warn + append to errs
  - `_ = ie.tasks.Fail(...)` → 同上模式
  - 函数末尾 `return errors.Join(errs...)`（若有 errs）或 `return nil`
- [ ] **A2.3** 检查 `intervention.go` 其他静默吞错点（grep `_ = ` 全文），按需修
- [ ] **A2.4** 修改 `InterventionExecutor` 结构体，加 `metrics *orchMetrics` 字段（如尚未注入）
- [ ] **A2.5** 修改 `NewInterventionExecutor` 注入 metrics 引用
- [ ] **A2.6** 新建 `guard/intervention_test.go`
  - `TestInterventionExecutor_WaitFailure_RecordsMetric` — Wait 失败 → WaitFailed=1 + 错误非 nil + errors.Is
  - `TestInterventionExecutor_TaskFailFailure_RecordsMetric` — tasks.Fail 失败 → TaskFailFailed=1
  - `TestInterventionExecutor_TerminateFailure_ReturnsPartialErr` — Terminate 失败 + Wait 成功 → 只包含 Terminate error
  - `TestInterventionExecutor_AllSuccess_ReturnsNil` — 三步全成功 → nil

### A3. 验证

- [ ] **A3.1** `cd /Users/fukai/workspace/devrix && go vet ./internal/layers/evolution/...`
- [ ] **A3.2** `go test -race ./internal/layers/evolution/...`
- [ ] **A3.3** 全仓 `git grep "_, _ = current.Wait\|_, _ = ie.tasks.Fail"` 验证 0 命中

---

## PR-B: bridge 删除 + Orchestration* → Guard* 重命名（C-1 + H-1）

### B1. bridge.go 删除

- [ ] **B1.1** 全仓 grep `eval/bridge` + `orchestration/bridge` —— 期望仅命中要删除的 2 个文件 + 文档历史
- [ ] **B1.2** 删除 `internal/layers/evolution/eval/bridge.go`
- [ ] **B1.3** 删除 `internal/layers/evolution/orchestration/bridge.go`
- [ ] **B1.4** 检查 `internal/layers/observability/bridge.go` 是否有 `Orchestration*` re-export，若有同步删除
- [ ] **B1.5** `git ls-files internal/layers/evolution/eval/ internal/layers/evolution/orchestration/` 确认目录清空后可一并删除（git rm）
- [ ] **B1.6** `go build ./...` + `go test ./internal/layers/evolution/...` 验证 0 编译错误

### B2. Orchestration* → Guard* 类型/函数重命名

- [ ] **B2.1** 修改 `internal/layers/evolution/guard/config.go`
  - `type OrchestrationConfig = config.OrchestrationConfig` → `type GuardConfig = config.GuardConfig`
  - 保留旧名 type alias：`//go:deprecated type OrchestrationConfig = config.GuardConfig`
- [ ] **B2.2** 修改 `internal/layers/evolution/guard/config/config.go`（底层配置包）
  - `type OrchestrationConfig struct {...}` → `type GuardConfig struct {...}`
  - 字段名同步（若需要）：`OrchestrationTimeout` → `GuardTimeout` 等
- [ ] **B2.3** 修改 `internal/layers/evolution/guard/observer.go`
  - `type OrchestrationObserver struct {...}` → `type GuardObserver struct {...}`
  - `func NewOrchestrationObserver(...) *OrchestrationObserver` → `func NewGuardObserver(...) *GuardObserver`
  - 方法接收者 `(o *OrchestrationObserver)` → `(o *GuardObserver)`
  - 保留 type alias：`//go:deprecated type OrchestrationObserver = GuardObserver` + 旧 `NewOrchestrationObserver` 函数
- [ ] **B2.4** 修改 `internal/layers/evolution/guard/validator.go`
  - `type RuntimeOrchestrationValidator struct {...}` → `type RuntimeGuardValidator struct {...}`
  - `func NewRuntimeOrchestrationValidator(...) *RuntimeOrchestrationValidator` → `func NewRuntimeGuardValidator(...) *RuntimeGuardValidator`
  - 方法接收者 + 类型引用全量替换
  - 保留 type alias
- [ ] **B2.5** 修改 `internal/layers/evolution/guard/judge_adapter.go`
  - `RuntimeOrchestrationValidator` 引用 → `RuntimeGuardValidator`
- [ ] **B2.6** 修改 `internal/layers/evolution/guard/intervention.go`
  - `RuntimeOrchestrationValidator` 引用 → `RuntimeGuardValidator`
- [ ] **B2.7** 修改 `internal/layers/evolution/guard/types.go`
  - `OrchestrationConfig` 引用 → `GuardConfig`
- [ ] **B2.8** 全仓 grep `RuntimeOrchestrationValidator\|NewOrchestrationObserver\|OrchestrationObserver\|OrchestrationConfig` —— 期望仅命中 alias 定义点

### B3. orch_* → guard_* 指标重命名

- [ ] **B3.1** 修改 `internal/layers/evolution/guard/metrics.go`
  - `type orchMetrics struct {...}` → `type guardMetrics struct {...}`（PR-A 已加 WaitFailed + TaskFailFailed）
  - 6 个指标名：
    - `orch_decisions_total` → `guard_decisions_total`
    - `orch_validations_total` → `guard_validations_total`
    - `orch_interventions_total` → `guard_interventions_total`
    - `orch_judge_latency_seconds` → `guard_judge_latency_seconds`
    - `orch_observer_active` → `guard_observer_active`
    - `orch_decisions_by_stage` → `guard_decisions_by_stage`
  - `initOrchMetrics` → `initGuardMetrics`
  - 保留 `orchMetrics` type alias + 旧 `initOrchMetrics` 函数（v2.5 删除）
- [ ] **B3.2** 检查 Jaeger / Prometheus dashboard JSON，6 个指标名同步更新（v2.4 迁移期双注册，v2.5 删旧）
- [ ] **B3.3** 全仓 grep `orch_decisions_total\|orch_validations_total\|orch_interventions_total\|orch_judge_latency\|orch_observer_active\|orch_decisions_by_stage` —— 期望仅命中 alias 定义点 + comments

### B4. 测试

- [ ] **B4.1** 新建 `tests/integration/d6_bridge_absence_test.go`
  - `TestBridge_FilesRemoved` —— `os.Stat` 两个 bridge.go 路径，期望 `os.IsNotExist(err) == true`
  - `TestBridge_NoExternalImporters` —— `go/packages` 扫描全仓，无 `eval/bridge` 或 `orchestration/bridge` import
- [ ] **B4.2** 新建 `tests/integration/d6_rename_test.go`
  - `TestRename_NoOrchestrationUsage` —— AST 扫描 guard/ 包，0 处 `Orchestration*`（除 alias 定义点）
  - `TestRename_OldMetricNamesDeprecated` —— 全仓 grep 6 个旧指标名仅命中 alias 注释
- [ ] **B4.3** 现有 `guard/validator_test.go` 测试更新 —— `NewRuntimeOrchestrationValidator` → `NewRuntimeGuardValidator`
- [ ] **B4.4** 现有 `guard/metrics_test.go`（如有）测试更新 —— 指标名引用

### B5. CI guard

- [ ] **B5.1** 新建 `scripts/check-orch-rename.sh`
  - grep `Orchestration` 在 `internal/layers/evolution/**/*.go` 中，期望仅命中 alias 定义点（≤ 6 处）
  - grep `orch_` 在 `*.go` 中，期望仅命中 metrics.go 旧名注释 + alias（≤ 6 处）
  - exit code 1 即 fail

### B6. 验证

- [ ] **B6.1** `go vet ./...` 全仓 0 错
- [ ] **B6.2** `go build ./...` 全仓 0 错
- [ ] **B6.3** `go test -race ./internal/layers/evolution/...` 全绿
- [ ] **B6.4** `go test -race ./tests/integration/...`（含新增的 d6_bridge_absence_test + d6_rename_test）全绿
- [ ] **B6.5** `bash scripts/check-orch-rename.sh` exit 0

---

## PR-C: spec 同步 + t-registry + acceptance-report + S6 归档

### C1. spec.md v2.3.0 → v2.4.0

- [ ] **C1.1** 修改 `openspec/specs/d6-evolution/spec.md`
  - 版本号 v2.3.0 → v2.4.0
  - Last Updated 2026-06-19 → 2026-06-21
  - §目录结构：删除 eval/ + orchestration/ + bridge 章节
  - §DSAFT 结构：S12 韧性域新增 A02/A03 需求项
  - §Revision History：新增 "v2.4.0 命名迁移（DM-20260621-011）"
  - 新增 D6-S12-A02（Guard 名空间收敛）+ D6-S12-A03（Verify Invariant fail-safe）需求项

### C2. t-registry.md v3.1.0 → v3.2.0

- [ ] **C2.1** 修改 `openspec/specs/d6-evolution/t-registry.md`
  - 版本号 v3.1.0 → v3.2.0
  - Last Updated 2026-06-18 → 2026-06-21
  - Change 头部新增 "devrix-d6-evolution-review-fixes (DM-20260621-011)"
  - IMPLEMENTED 计数更新：35 → 41（+6 新 P0 T 点）
  - 新增 T 点：
    - **D6-S11-A02-T09** verify/_invariant.go ParseStruct 失败不再 panic (`verify/_invariant_test.go`)
    - **D6-S12-A01-T01** intervention.go Wait 失败 metric + slog + errors.Join (`guard/intervention_test.go`)
    - **D6-S12-A01-T02** intervention.go tasks.Fail 失败 metric + slog (`guard/intervention_test.go`)
    - **D6-S12-A02-T01** bridge.go 完全删除（git ls-files 0 命中）(`tests/integration/d6_bridge_absence_test.go`)
    - **D6-S12-A03-T01** guard/ 内 0 处 Orchestration*（除 alias 定义点）(`tests/integration/d6_rename_test.go`)
    - **D6-S12-A03-T02** 6 个指标 orch_* → guard_* 重命名 (`guard/metrics_test.go`)

### C3. design.md v2.2.0 → v2.3.0

- [ ] **C3.1** 修改 `openspec/specs/d6-evolution/design.md`
  - 版本号 v2.2.0 → v2.3.0
  - Last Updated 2026-06-19 → 2026-06-21
  - §目录结构：删除 eval/ + orchestration/ + bridge 章节
  - §命名规范 新增 "v2.0 → v2.4 命名迁移路径"
  - §错误处理 新增 "三联固化：atomic counter + slog + errors.Join"（与 D7 对齐）

### C4. acceptance-report.md

- [ ] **C4.1** 新建 `openspec/changes/devrix-d6-evolution-review-fixes/acceptance-report.md`
  - S5 验收凭证：8 个 AC 全 PASS + 6 个新 P0 T 点 IMPLEMENTED
  - 引用 PR-A + PR-B + PR-C 的 PR 编号
  - go vet + go test -race + layer-lint 全绿
  - D5 spans P95 复跑不退化

### C5. S6 归档

- [ ] **C5.1** `git mv openspec/changes/devrix-d6-evolution-review-fixes/ openspec/archive/2026-06-21-devrix-d6-evolution-review-fixes/`
- [ ] **C5.2** 修改 `openspec/archive/2026-06-21-devrix-d6-evolution-review-fixes/.openspec.yaml`
  - `status: S2_Proposal` → `status: s7_archived`
  - `domains: [D6]` 字段保留（verify-archive.sh §2.3 必读）
- [ ] **C5.3** 修改 `openspec/archive/2026-06-21-devrix-d6-evolution-review-fixes/proposal.md`
  - `**Status:** S2_Proposal` → `**Status:** S7_Archived`
- [ ] **C5.4** 修改 `openspec/demand-archive-index.md`
  - 主表新增 `DM-20260621-011` 行（PR #XXX + summary）
  - Archive Locations 表新增 `devrix-d6-evolution-review-fixes` 行
- [ ] **C5.5** 运行 `./scripts/verify-archive.sh devrix-d6-evolution-review-fixes` 期望 11 ✓ 0 ✗ 2 ⚠

---

## 跨 PR 验收清单

- [ ] 8 个 AC 全 PASS（proposal.md §4）
- [ ] 6 个新 P0 T 点 IMPLEMENTED（t-registry.md v3.2.0）
- [ ] `git grep "Orchestration"` 仅命中 alias 定义点（≤ 6 处）
- [ ] `git ls-files internal/layers/evolution/eval/ internal/layers/evolution/orchestration/` 0 命中
- [ ] go vet + go test -race + layer-lint + check-orch-rename.sh 全绿
- [ ] verify-archive.sh 11/11 PASS