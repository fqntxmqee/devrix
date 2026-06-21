# Proposal: D6 Evolution Review Fixes

**Change ID:** devrix-d6-evolution-review-fixes
**Demand ID:** DM-20260621-011
**Status:** S7_Archived (2026-06-21)
**Priority:** P1
**Date:** 2026-06-21

---

## 1. Background

2026-06-21 完成 D6 演化域深度 review（`openspec/changes/d6-deep-review/d6-review-report.md`，395 行）。review 覆盖 `internal/layers/evolution/**`（D6-S3/S4/S5 + bridge 兼容层），从代码质量、命名一致性、错误处理、测试覆盖、S4-Gate 检查清单 5 个维度扫描。

**review 结论**：⚠️ **Changes Requested** —— 1 CRITICAL + 6 HIGH + 7 MEDIUM + 5 LOW = 19 个问题需修复方可通过 S4-Gate。

**与已有 change 的关系**：
- 本 change 是 review 的 **Phase 1（阻塞合并层）** —— 修复 C-1 + H-1 + H-2 + H-3 四个最关键问题
- **不重复** `devrix-d6-sa-refine`（S4 Canonical 重排，DM-20260615-002）—— 那是 v1.0 → v2.0 的物理路径迁移 + canonical_s 引入
- **不重复** `devrix-spec-sync-d6-evolution-registration`（DM-20260619-003）—— 那是 spec v2.2.0 → v2.3.0 文档同步
- **姊妹篇** `devrix-d7-error-aggregation-and-metrics`（DM-20260621-010）—— 同日完成的 D7 编排层深度 review 修复，3 PR 联动 + S6 归档已闭环，本 change 复用其 PR 拆分模式

## 2. Problem Statement

### Problem 1 (P0 — CRITICAL): 兼容桥接 bridge.go 与 spec v2.2.0 矛盾

**位置**：
- `internal/layers/evolution/eval/bridge.go`
- `internal/layers/evolution/orchestration/bridge.go`

**根因**：
- D6 spec.md v2.2.0 / design.md §目录结构 已明确声明「`bridge.go` 桥接文件在 v2.0.1 cleanup 后全部删除（11 个）」
- v2.0 物理路径迁移完成（DM-20260615-003）时 `eval/` → `evaluate/`、`orchestration/` → `guard/`，bridge 文件本应同步清除
- **实际仍有 2 个残留**：eval/bridge.go + orchestration/bridge.go，导致外部代码可继续走旧 import 路径，绕过 v2.0 命名规范

**影响**：
- 命名一致性受损：6 处 `Orchestration*` 类型仍被 bridge re-export，调用方可继续用旧名
- spec/code 一致性 6 处偏离无法收敛（H-1 的前置阻塞）
- 后续删除需批量 import 迁移

### Problem 2 (P1 — HIGH): v2.0 重命名未完成（6 处类型 + 6 个指标）

**位置**：
- `internal/layers/evolution/guard/config.go`: `OrchestrationConfig`
- `internal/layers/evolution/guard/observer.go`: `OrchestrationObserver`、`NewOrchestrationObserver`、`RuntimeOrchestrationValidator`
- `internal/layers/evolution/guard/validator.go`: `RuntimeOrchestrationValidator`、`NewRuntimeOrchestrationValidator`
- `internal/layers/evolution/guard/metrics.go`: `orchMetrics` struct + 6 个 `orch_*` 指标名

**根因**：
- v2.0 物理路径迁移（DM-20260615-003）只完成了 `orchestration/` → `guard/` 目录迁移，**未完成类型 / 函数 / 指标名同步重命名**
- spec.md v2.3.0 / design.md v2.2.0 已要求 `Guard*` 命名（"曾因重命名误删从 42bf1d7 恢复"），但 6 处类型仍为 `Orchestration*`
- bridge.go re-export 让外部代码可继续用旧名，掩盖了重命名未完成的真相

**影响**：
- 阅读代码混淆：grep `Orchestration` 命中 6 处实际属于 Guard 域
- OpenTelemetry 仪表盘 / SLO 命名不一致（`orch_decisions_total` vs spec 要求 `guard_decisions_total`）
- bridge 删除（C-1）后必须完成此重命名，否则 import 错误

### Problem 3 (P1 — HIGH): panic 误用 `verify/_invariant.go:24`

**位置**：`internal/layers/evolution/verify/_invariant.go:24`

```go
func mustParseVerifyInvariants() ltllite.InvariantSet {
    set, err := ltllite.ParseStruct(verifyInvariants{})
    if err != nil {
        panic("ltllite: verify invariant parse failed: " + err.Error())
    }
    return set
}
```

**根因**：
- `mustXxx` 命名 + `panic` 是 v1.0 风格的"启动期致命错误即终止"模式
- 当前 Devrix 规范要求业务错误一律 SentinelError，**panic 仅用于 `init()` 检测编程错误**
- `ltllite.ParseStruct` 是用户配置解析（来自 `verifyInvariants{}` struct 标签），不是编程错误；标签格式错误应在启动期 fail-fast 但走 `log.Fatal` 而非 panic

**影响**：
- 进程被 panic 污染，recover 链路过深时可能绕过 shutdown hooks
- 与 D5/D7 域的 fatal 处理风格不一致（`shared/log/fatal.go` 提供 `log.Fatal`）

**额外发现**：`_invariant.go` 文件以 `_` 开头，Go 工具链（`go build`/`go test`/`go install`）**会忽略此类文件**。这意味着：
- `CheckVerifyInvariants` / `verifyInvariantSet` / `parseVerifyInvariants` 这些 export **从未被编译进 verify 包二进制**
- 整个 `_invariant.go` 是 latent dead code（spec.md / design.md 都假设该文件 active）
- 必须重命名为 `invariant.go`（去掉 `_` 前缀）才能真正进入包

PR-A 同时修复 **panic 反模式** + **dead code 激活**。

### Problem 4 (P1 — HIGH): 静默吞错 `intervention.go:74`

**位置**：`internal/layers/evolution/guard/intervention.go:74`

```go
if current != nil {
    if err := current.Terminate(ctx); err != nil {
        slog.Warn("terminate current agent on reroute", "error", err)
    }
    _, _ = current.Wait(ctx)  // ← 静默吞错
}

if iv.MilestoneFail || iv.TaskFail {
    taskID := iv.FailReason
    _ = ie.tasks.Fail(taskID, iv.Reason)  // ← 同样静默吞
}
```

**根因**：
- 与 DM-20260621-010（D7 error aggregation）相同的"warn+nil"反模式 —— D7 已固化为 atomic counter + slog + errors.Join 三联
- D6 域未跟进同一规范

**影响**：
- 干预执行失败无 metric 计数（intervention 实际成功率不可观测）
- 与 D5 v2.1 terminal 的 `errors.Join` 风格不一致

## 3. Proposed Solution

按 review 的 **Phase 1** 范围分 **3 PR 联动**：

### PR-A: panic 修复 + 静默吞错（H-2 + H-3）

**目标**：最小风险独立单元，2 个文件修改，与 D7 error aggregation PR-A 镜像

- 修改 `verify/_invariant.go:24` panic → `slog.Error + os.Exit(1)`（启动期致命，进程退出由 systemd 接管）
- 修改 `guard/intervention.go:74` `_, _ = current.Wait(ctx)` → 引入 `InterventionMetrics.WaitFailed` atomic.Int64 + slog.Warn + errors.Join 上抛
- 新增 `guard/metrics.go` 扩展 `WaitFailed` 字段（**复用现有 orchMetrics struct，H-1 rename 时一起处理**）
- 新增 `verify/_invariant_test.go::TestMustParseVerifyInvariants_BadStruct_ReturnsError`（重构后签名变化）
- 新增 `guard/intervention_test.go::TestInterventionExecutor_WaitFailure_RecordsMetric`

### PR-B: bridge 删除 + Orchestration* 重命名（C-1 + H-1）

**目标**：spec/code 一致性收敛，6 文件修改 + 2 文件删除

- 删除 `internal/layers/evolution/eval/bridge.go`
- 删除 `internal/layers/evolution/orchestration/bridge.go`
- `guard/config.go`: `OrchestrationConfig` → `GuardConfig`（保留旧名 type alias 1 个版本，标注 DEPRECATED）
- `guard/observer.go`: `OrchestrationObserver` → `GuardObserver`、`NewOrchestrationObserver` → `NewGuardObserver`、`RuntimeOrchestrationValidator` → `RuntimeGuardValidator`（含 type alias）
- `guard/validator.go`: `RuntimeOrchestrationValidator` → `RuntimeGuardValidator`、`NewRuntimeOrchestrationValidator` → `NewRuntimeGuardValidator`（含 type alias）
- `guard/metrics.go`: `orchMetrics` → `guardMetrics` struct rename + 6 个指标名 `orch_*` → `guard_*`
- `internal/layers/observability/bridge.go`：如有 `Orchestration*` re-export 同步删除

**风险控制**：
- type alias 保留 1 个版本（v2.4.0 → v2.5.0 移除），grep 0 命中旧名（除 alias 定义点）后再彻底删除
- 全仓 `git grep Orchestration` 应仅命中 alias 定义 + comments + 文档历史

### PR-C: spec 同步 + t-registry + acceptance-report + S6 归档

**目标**：文档 / 规范 / 注册表 / 归档 一次性同步

- `openspec/specs/d6-evolution/spec.md` v2.3.0 → v2.4.0
  - 删除 §目录结构 中 bridge 章节（已删）
  - 新增 "Orchestration* → Guard* rename history" Revision History 章节
  - 新增 D6-S12-A02/D6-S12-A03 需求项
- `openspec/specs/d6-evolution/t-registry.md` v3.1.0 → v3.2.0
  - 新增 D6-S11-A02-T09（panic 修复）、D6-S12-A01-T01~T03（干预 metrics + 错误聚合）、D6-S12-A02-T04（bridge 删除回归测试）、D6-S12-A03-T03~T05（rename 全量回归）
- `openspec/specs/d6-evolution/design.md` v2.2.0 → v2.3.0
  - §目录结构 删除 eval/ + orchestration/ + bridge 章节
  - 新增 "v2.0 → v2.4 命名迁移路径" 章节
- `openspec/changes/devrix-d6-evolution-review-fixes/acceptance-report.md`（S5 验收凭证）

## 4. Acceptance Criteria

| AC ID | 描述 | 验证方式 |
|-------|------|----------|
| AC1 | `verify/_invariant.go` 不再 panic；ParseStruct 失败走 log.Fatal | `verify/_invariant_test.go` 单测 |
| AC2 | `intervention.go` Wait 失败有 metric + slog.Warn + errors.Join | `intervention_test.go` 单测 + grep 0 `_, _ =` |
| AC3 | `eval/bridge.go` + `orchestration/bridge.go` 完全删除 | `git ls-files` 0 命中 + 全仓 import 迁移 |
| AC4 | guard/ 内 0 处 `Orchestration*`（除 type alias 定义点） | `git grep` 仅命中 alias + comments |
| AC5 | 6 个指标名 `orch_*` → `guard_*` 与 spec v2.4.0 一致 | `metrics.go` 单测 + Jaeger 验证 |
| AC6 | spec.md / t-registry / design.md / acceptance-report 全部同步 | verify-archive.sh 11/11 ✓ |
| AC7 | D6 t-registry v3.1.0 → v3.2.0，新增 ≥ 5 P0 T 点全 IMPLEMENTED | t-registry grep |
| AC8 | D5 spans P95 不退化（intervention metrics 不引入额外开销） | `go test -bench` 或 D5-spans 复跑 |

## 5. Out of Scope

- **Phase 2 (后续 Change)**：H-4 SentinelError 引入 + H-5/6 函数拆分 + M-2/M-3 死代码 + M-4 字段复用
- **Phase 3 (后续 Change)**：M-1/M-6/M-7 + L-1~L-5 风格优化
- 不动 11 个已注册 probe 的 `ID()` 命名（M-5 弱问题，Phase 3）
- 不动 `aggregatedScore` buckets 语义（M-6，Phase 3）

## 6. Risk Assessment

| 风险 | 等级 | 缓解 |
|------|------|------|
| C-1 bridge 删除导致外部 import 失败 | Medium | PR-B 一次性迁移 + 全仓 grep + type alias 兼容 |
| H-1 重命名外部 13 调用方断链 | Medium | 同上 + 保留 type alias + type alias 定义点单测 |
| H-2 panic → log.Fatal 启动失败掩盖 | Low | 仅 fatal 路径启动期触发，进程退出码非 0 由 systemd 重启 |
| H-3 metrics 引入额外开销 | Low | atomic.Int64 单次 Add < 10ns，D5 spans P95 复跑验证 |
| D6 domain 跨域调用（D5 observability bridge 同步） | Low | PR-B 同步检查 `observability/bridge.go` |

## 7. Related Work

- 完整 review 报告：`openspec/changes/d6-deep-review/d6-review-report.md`（19 个问题）
- 姊妹 change：`devrix-d7-error-aggregation-and-metrics`（DM-20260621-010，S7_Archived 2026-06-21）
- 模板 change：`devrix-d5-v2-terminal`（DM-20260619-006，S7_Archived 2026-06-19）的 bridge 清债 + 物理路径归位

## 8. References

- `openspec/changes/d6-deep-review/d6-review-report.md` — 完整 review 报告
- `openspec/specs/d6-evolution/spec.md` — D6 域规范 v2.3.0
- `openspec/specs/d6-evolution/design.md` — D6 设计 v2.2.0
- `openspec/specs/d6-evolution/t-registry.md` — D6 T 注册表 v3.1.0
- `openspec/specs/d6-evolution/d6-domain.md` — D6 物理路径映射（DM-20260619-003 新建）