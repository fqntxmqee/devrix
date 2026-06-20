# Tasks: devrix-error-handling-tier1-tier2

**Demand ID:** DM-20260620-003
**Status:** S7_Archived (2026-06-20 全部闭环)

---

## Phase A — Tier 1 (PR-A: C1 + H1 + H2 + L4)

**目标**：IM 脱敏 + Sentinel 链跨域恢复 + retry nil-sentinel 修复。

| ID | 任务 | 文件 | 估行 | 状态 |
|----|------|------|------|------|
| T-A.1 | `SanitizeForUser(err error) string` 公共函数 + 7 redaction regex | `internal/shared/errors/redact.go` | ~60 | ✅ Done |
| T-A.2 | 14 单测覆盖 Bearer / sk- / ghp_ / xoxb- / AKIA / path / truncate / idempotent / normal / whitespace / empty / long-blob / short-ids | `internal/shared/errors/redact_test.go` | ~140 | ✅ Done |
| T-A.3 | `feishu.go:237` 错误卡片渲染改 `SanitizeForUser` | `internal/layers/communication/channel/adapters/feishu.go` | +2 | ✅ Done |
| T-A.4 | `feishu.go:827` replyAck 错误回包改 `SanitizeForUser` | `internal/layers/communication/channel/adapters/feishu.go` | +2 | ✅ Done |
| T-A.5 | `orchestrator.go::emitError` 签名加 variadic `code ...string` | `internal/layers/orchestration/turn/orchestrator.go` | +6 | ✅ Done |
| T-A.6 | 5 处 emitError 调用点（256/292/371/428/568）传 `SanitizeForUser(err)` + `ErrorCode(err)` | `internal/layers/orchestration/turn/orchestrator.go` | +10 | ✅ Done |
| T-A.7 | `subturn.go:323/354` collectSubTurnResult 错误用 `sharederrors.Wrap` 保留 sentinel | `internal/layers/orchestration/turn/subturn.go` | +4 | ✅ Done |
| T-A.8 | `orchestrate_path.go:118` `%v` → `%w` 保留 sentinel 链 | `internal/layers/orchestration/sessionorchestrator/orchestrate_path.go` | +1 | ✅ Done |
| T-A.9 | `retry.go:91-93` nil-sentinel 修复：wrap `errors.New(...)` 而非 nil | `internal/layers/llmgateway/protect/retry.go` | +4 | ✅ Done |
| T-A.10 | `ErrSubagentStreamError` + `ErrSubagentStreamClosed` (AGT_STREAM_5013/5014) 工厂 | `internal/shared/errors/subturn.go` | +30 | ✅ Done |

**Quality Gate:**
- [x] AC1 PASS — `SanitizeForUser` 14 单测全绿
- [x] AC2 PASS — emitError 5 sites 全部用 `SanitizeForUser`
- [x] AC5 PASS — `retry.go:91` wrap `errors.New(...)` 真实 cause
- [x] AC9 PASS — `redact_test.go` 14 用例
- [x] PR-A #141 squash merged

**PR:** `fix(shared-errors): tier 1 — SanitizeForUser + subagent stream sentinels + retry nil-wrap fix (#141)`

---

## Phase B — Tier 2 C2 (PR-B: 类型合并)

**目标**：`LLMError` + `SentinelError` 合并为单一 `*SentinelError` + `migrate.go` deprecated alias。

| ID | 任务 | 文件 | 估行 | 状态 |
|----|------|------|------|------|
| T-B.1 | 合并 `LLMError` 与 `SentinelError`：单一 `*SentinelError{Code, Message, Err}` | `internal/shared/errors/communication.go` | ~10 | ✅ Done |
| T-B.2 | `LLMError = SentinelError` Go type alias 保 back-compat | `internal/shared/errors/llm.go` | +1 | ✅ Done |
| T-B.3 | 删除 `LLMError` 旧 `Error()` + `Unwrap()` 方法（type alias 自动继承） | `internal/shared/errors/llm.go` | -10 | ✅ Done |
| T-B.4 | `SentinelError.Error()` fallback Message → Err → Code 保留 LLMError 宽松语义 | `internal/shared/errors/communication.go` | +5 | ✅ Done |
| T-B.5 | `migrate.go` build-time guard + `IsLLMError` / `LLMCode` deprecated helper | `internal/shared/errors/migrate.go` | ~40 | ✅ Done |
| T-B.6 | 跨域调用方类型断言适配：`var llmErr *LLMError` → `*SentinelError` | 7 文件 (counter/router/deepseek/circuit_breaker/errorclass + tests) | ~30 | ✅ Done |
| T-B.7 | 6 + 7 测试文件重写：`*LLMError` → `*SentinelError` + `errors.Is/As` 验证 | 多文件 | ~80 | ✅ Done |
| T-B.8 | `IsRetryable` 简化为单一 `*SentinelError` 通道 | `internal/shared/errors/llm.go` | ~10 | ✅ Done |

**Quality Gate:**
- [x] AC3 PASS — 单一类型；migrate.go alias 兼容
- [x] AC10 PASS — `llm_test.go` + `communication_test.go` 全部重写通过
- [x] PR-B #142 squash merged

**PR:** `refactor(shared-errors): unify LLMError + SentinelError via type alias (#142)`

---

## Phase C — Tier 2 H3 + M1..M4 (PR-C: misc)

**目标**：silent fallback 修复 + invariant migration + ctx inject + observability Wrap。

| ID | 任务 | 文件 | 估行 | 状态 |
|----|------|------|------|------|
| T-C.1 | `TaskManager.Create` 签名改 `(*Task, error)`；span 记录 `task.create.error` + `task.ensure_goal.error` | `internal/layers/orchestration/workmodel/task_manager.go` | +20 | ✅ Done |
| T-C.2 | `resolveDelegateTaskID` 签名改 `(string, error)`；caller IIFE 适配 | `internal/layers/orchestration/delegatetools/delegate_tools.go` | +5 | ✅ Done |
| T-C.3 | `CreateTask` + `CreateWorkPlan` 适配新签名 | `internal/layers/orchestration/sessionorchestrator/workmodel.go` | ~10 | ✅ Done |
| T-C.4 | `sessionorchestrator/orchestrate_path.go` local `emitError` 用 `SanitizeForUser` | `internal/layers/orchestration/sessionorchestrator/orchestrate_path.go` | +5 | ✅ Done |
| T-C.5 | `turn_adapter.ErrInvariantViolation` 迁移到 `sharederrors.ErrInvariantViolation` (AGT_INVARIANT_5013) + deprecated alias | `internal/layers/orchestration/turn_adapter/ltl_hook.go` | +10 | ✅ Done |
| T-C.6 | `classifyAndWrap` + `Gateway.classify` 签名加 ctx 参数 | `internal/layers/llmgateway/stream/classify_helpers.go` + `gateway.go` | +10 | ✅ Done |
| T-C.7 | `classifyResultKey{}` + `ClassifyResultFromCtx` helper 让下游 span 读 cached Classification | `internal/layers/llmgateway/stream/classify_helpers.go` | +10 | ✅ Done |
| T-C.8 | `Observability.Shutdown` 用 `errors.Join` + `%w` 保 typed chain | `internal/layers/observability/observability.go` | +10 | ✅ Done |
| T-C.9 | `LLMFallbackClassifier.Classify` LLM 失败加 `slog.Warn` 结构化日志 | `internal/layers/orchestration/decisionplanning/classifier_fallback.go` | +10 | ✅ Done |
| T-C.10 | `workmodel.Create` 新签名 test 适配（counter/router/deepseek/circuit_breaker/task_manager/work_tree/cross_session/disk_store + hub + integration） | 8 + 5 测试文件 | ~150 | ✅ Done |

**Quality Gate:**
- [x] AC4 PASS — silent fallback 修复；3 P0 T 点（T18/T19/emitError）覆盖
- [x] AC6 PASS — `turn_adapter.ErrInvariantViolation` 迁移 + alias
- [x] AC7 PASS — `Observability.Shutdown` Wrap
- [x] AC8 PASS — `Gateway.Stream` `errorclass.InjectClassification(ctx, c)`
- [x] AC11 PASS — `retry_test.go` + `task_manager_test.go` 覆盖新签名
- [x] PR-C #143 squash merged

**PR:** `fix(d7/shared-errors): tier 2 H3+M1..M4 — silent fallback + invariant migration + ctx inject + observability Wrap (#143)`

---

## Phase D — docs + t-registry + 归档 (PR-D)

**目标**：跨切面文档 + t-registry 同步 + S6 归档。

| ID | 任务 | 文件 | 估行 | 状态 |
|----|------|------|------|------|
| T-D.1 | `docs/error-handling.md` 9 节综述（Overview / Core Types / Code Conventions / Layer Responsibilities / Sentinel Migration / Patterns / Testing / Cheatsheet / References） | `docs/error-handling.md` | ~270 | ✅ Done |
| T-D.2 | D7 t-registry 新增 D7-S1-T18/T19 + D7-S2-A06-T24/T25/T26/T27 + D7-S2-A02-T18 | `openspec/specs/d7-orchestration/t-registry.md` | +50 | ✅ Done |
| T-D.3 | D3 t-registry 新增 D3-S3-A01-T16 retry nil-sentinel | `openspec/specs/d3-llm-gateway/t-registry.md` | +5 | ✅ Done |
| T-D.4 | D5 t-registry 新增 D5-S23-A06-T03 Observability.Shutdown errors.Join | `openspec/specs/d5-observability/t-registry.md` | +5 | ✅ Done |
| T-D.5 | 根索引 `openspec/t-registry.md` v4.5.0 → v4.6.0 + 增量行 | `openspec/t-registry.md` | +5 | ✅ Done |
| T-D.6 | `openspec/demand-archive-index.md` 加 DM-20260620-003 行 + Archive Location | `openspec/demand-archive-index.md` | +5 | ✅ Done |
| T-D.7 | `acceptance-report.md` (S5) | `openspec/changes/devrix-error-handling-tier1-tier2/acceptance-report.md` | ~250 | ✅ Done |
| T-D.8 | `tasks.md` (S4 任务拆解) | `openspec/changes/devrix-error-handling-tier1-tier2/tasks.md` | ~150 | ✅ Done (本文件) |
| T-D.9 | S6 归档：`openspec/changes/...` → `openspec/archive/2026-06-20-devrix-error-handling-tier1-tier2/` + `.openspec.yaml status: s7_archived` | `openspec/archive/...` | — | ✅ Done |
| T-D.10 | `verify-archive.sh devrix-error-handling-tier1-tier2` 通过 | `scripts/verify-archive.sh` | — | ✅ Done |

**Quality Gate:**
- [x] AC12 PASS — t-registry 根索引 + D3 + D5 + D7 + shared.errors 全部同步
- [x] AC13 PASS — `docs/error-handling.md` v1.0 落地
- [x] AC14 PASS — go vet + go test -race + layer-lint 全绿；P0 T 100% PASS
- [x] verify-archive.sh 全通过

---

## AC 验收汇总

| AC | 状态 | 证据 |
|----|------|------|
| AC1 SanitizeForUser + IM 替换 | ✅ PASS | `internal/shared/errors/redact.go` 14 单测；`feishu.go:237/827` |
| AC2 emitError 5 sites + subturn sentinel | ✅ PASS | `orchestrator.go:emitError` + 5 callsites + `subturn.go:323/354` |
| AC3 LLMError+SentinelError 合并 | ✅ PASS | `internal/shared/errors/communication.go` + `migrate.go` |
| AC4 silent fallback 修复 | ✅ PASS | `task_manager.go:Create` 改 `(*Task, error)`；`resolveDelegateTaskID` 改 `(string, error)`；`classifier_fallback.go:slog.Warn` |
| AC5 retry nil-sentinel | ✅ PASS | `retry.go:91` wrap `errors.New(...)` |
| AC6 invariant migration | ✅ PASS | `turn_adapter/ltl_hook.go` 用 `sharederrors.ErrInvariantViolation` + alias |
| AC7 observability Wrap | ✅ PASS | `observability.go:Shutdown` 用 `errors.Join` |
| AC8 ctx inject for classify | ✅ PASS | `Gateway.Stream` `errorclass.InjectClassification(ctx, c)` + `ClassifyResultFromCtx` |
| AC9 redact_test.go 14 cases | ✅ PASS | `internal/shared/errors/redact_test.go` 14 PASS |
| AC10 llm_test + communication_test 重写 | ✅ PASS | 合并类型 6 + 7 = 13 测试文件重写 |
| AC11 retry + task_manager test | ✅ PASS | `retry_test.go` + `task_manager_test.go` 覆盖新签名 |
| AC12 t-registry 注册 | ✅ PASS | D7 +7, D3 +1, D5 +1 = 9 T 点 |
| AC13 docs/error-handling.md | ✅ PASS | 9 节综述 v1.0 |
| AC14 go vet + test + 覆盖率 ≥ 80% | ✅ PASS | go vet 0 错；go test -race 全绿 |

**T 层验收**：9 P0 T 点全 IMPLEMENTED
- D3-S3-A01-T16 (retry nil-sentinel)
- D5-S23-A06-T03 (Shutdown errors.Join)
- D7-S1-T18 (TaskManager.Create `(*Task, error)`)
- D7-S1-T19 (resolveDelegateTaskID `(string, error)`)
- D7-S2-A02-T18 (emitError variadic code)
- D7-S2-A06-T24 (invariant migration)
- D7-S2-A06-T25 (subturn error_code wrap)
- D7-S2-A06-T26 (subturn channel-closed sentinel)
- D7-S2-A06-T27 (retry nil-sentinel defensive wrap)

S5 验收通过，S7_Archived。