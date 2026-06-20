---
demand-id: DM-20260620-003
title: Error Handling Design — Tier 1 (IM sanitize + sentinel chain) + Tier 2 (Sentinel/LLM type merge + silent fallback)
priority: P0
status: S1_Proposal
dsaft_domain: communication, context-engine, llm-gateway, orchestration
created: 2026-06-20
---

# Error Handling Design — Tier 1 + Tier 2

## 1. 背景

2026-06-20 一次"整体 review devrix 项目（重点：错误处理设计）"的专项审查由 team-architect 完成，产出一份 4 严重度分级、5 hotspot 文件的架构 review（详见 `openspec/changes/devrix-error-handling-tier1-tier2/proposal.md` §1）。

发现的核心问题：
- 错误在 D7 turn loop 经过 `emitError(... string)` 后**sentinel 链断裂为字符串**，导致 D1 IM 渲染层无法 `errors.Is/As` 区分错误类型做差异化提示。
- IM 错误渲染（`feishu.go:237` + `feishu.go:827`）**直接透传 `err.Error()`** 原始字符串，存在**安全风险**：上游 LLM 错误 body、provider stack trace、tool 执行失败时的用户文件路径、用户 prompt 中粘贴的 API key 都有可能泄漏到飞书卡片。
- `internal/shared/errors/` 内部 `LLMError` 与 `SentinelError` 是两个字段完全相同的独立类型，跨域错误处理必须双查。
- D2 prepare / D7 run loop / sub-agent 错误传播多处用 `%v` 而非 `%w`，**sentinel 链在源头就丢失**；`workmodel/task_manager.go:127` 返回 `(*Task, error)` 但实际只返回 `*Task`，**静默吞错**让"创建失败"和"创建成功"无法区分。

## 2. 问题陈述

| ID | 严重度 | 问题 | 影响 |
|----|--------|------|------|
| **C1** | Critical | IM 错误渲染无脱敏：`Markdown(err.Error())` / `+err.Error()` 直接喂给飞书卡片 | 用户可见 provider 内部错误、文件路径、潜在 API key 泄漏 |
| **C2** | High | `LLMError` + `SentinelError` 双类型冗余，跨域错误处理必须双查 | 增加 cognitive load；分类器 / IsRetryable 漏查即静默失败 |
| **H1** | High | 6 处 `%v` 而非 `%w`（orchestrator.go × 5 + subturn.go × 2 + orchestrate_path.go × 1） | sentinel 链在源头丢失 → D1 渲染无 sentinel info |
| **H2** | High | `retry.go:91-93` 在 retry 全部失败时 wrap nil 进 `NewProviderUnavailableError` | `errors.Is/As` 拿到 wrap nil 的 err → 静默 retry 死循环风险 |
| **H3** | High | 5+ 处 silent fallback（`task_manager.go:127` nil 返回、`classifier_fallback.go:73` rule fallback 静默、`EnsureGoal` 双下划线吞、freefork 错误聚合、`recover()` 静默恢复） | 错误不再传播，QA 无法定位 |
| **H4** | High | `emitError(... content string)` 不接 error 类型；D2 prepare 错误的 sentinel code 跨域丢失 | D1 IM 无法按错误类型渲染差异化提示 |
| **M1** | Medium | `turn_adapter` 私有 `ErrInvariantViolation` 未注册到 `shared/errors` | 跨包错误处理不一致 |
| **M2** | Medium | 错误日志混用 `slog` / `fmt.Println` / `log` | 结构化日志不一致 |
| **M3** | Medium | `Gateway.Stream` classify 后未 inject Classification 到 ctx | D7 拿不到 classification 对象 |
| **M4** | Medium | init-time `panic` 命名应显式（`MustParse` / `MustNew`） | 未来 reviewer 误标为业务 panic |

## 3. 验收标准

| ID | 标准 | 优先级 |
|----|------|--------|
| **AC1** | `errors.SanitizeForUser(err) string` helper 在 `internal/shared/errors/` 实现并在所有 D1 IM 渲染入口（`feishu.go:237` + `feishu.go:827`）替换 `err.Error()` 直传；redact API key / bearer token / `/Users/...` `/home/...` 路径 / 长度截断 240 | P0 |
| **AC2** | D7 turn loop 5 处 `emitError(..., fmt.Sprintf("...: %v", err))` 全部改用 `emitError(..., errors.SanitizeForUser(err))`；sub-agent 错误（subturn.go × 2）改用 `sharederrors.Wrap` 模式保留 sentinel 链 | P0 |
| **AC3** | `internal/shared/errors/` 合并 `LLMError` 与 `SentinelError` 为单一 `*Error{Code, Message, Err}`；提供 `IsRetryable` / `ErrorCode` 单一通道；所有调用方通过编译时类型断言统一 | P0 |
| **AC4** | 5+ 处 silent fallback 修复：`task_manager.go:127` 改返回 `(*Task, error)`；`classifier_fallback.go:73` 至少 `slog.Warn`；`EnsureGoal` 改写为 `(bool, error)`；freefork 错误聚合打印结构化日志；`recover()` 改为 `slog.Error` | P0 |
| **AC5** | `retry.go:91-93` 的 nil-sentinel 包装 bug 修复：当所有 retry 尝试均无 lastErr 时返回 `fmt.Errorf("llm: all retry attempts exhausted without captured error")` 而非 wrap nil 的 sentinel | P0 |
| **AC6** | `turn_adapter` 的 `ErrInvariantViolation` 移到 `shared/errors` 注册 AGT_INVARIANT_5013 code | P1 |
| **AC7** | 错误日志统一：`internal/layers/observability/observability.go:164` 等剩余 `fmt.Errorf` 改 `sharederrors.Wrap`；CLI 渲染 `RenderError` 加 `slog.Error` audit trail | P1 |
| **AC8** | `Gateway.Stream` 在 `classifyAndWrap` 之前调用 `errorclass.InjectClassification(ctx, c)`，让下游 `FromContext` 可读 | P1 |
| **AC9** | 单元测试：新增 `errors/redact_test.go`（8+ 用例覆盖 API key / path / model name / 长 err / 嵌套 err / nil / 已 sanitized err）；原有 4 处 `emitError` 测试保持 + 新增 sanitize 集成测试 | P0 |
| **AC10** | `errors/llm_test.go` + `errors/communication_test.go` 重新编写适配合并后的 `*Error`；保留全部原 behavior | P0 |
| **AC11** | `retry_test.go` + `task_manager_test.go` 覆盖 AC5 nil-sentinel + AC4 错误签名变更 | P0 |
| **AC12** | t-registry 注册（根索引 + 域 D1/D3/D7/shared.errors）：约 8 P0 T 点 | P0 |
| **AC13** | docs：`docs/error-handling.md` 新建（错误处理设计综述 + SanitizeForUser 使用模式 + 类型合并迁移指南） | P1 |
| **AC14** | go vet + `./scripts/test-unit.sh` + `go test -race ./...` 全绿；P0 T 层 100% PASS；覆盖率 ≥ 80% | P0 |

## 4. 依赖与约束

| 类型 | 内容 |
|------|------|
| 依赖 | 已完成：`internal/shared/errors/` 现有 SentinelError 体系、`llmgateway/protect/` 错误分类器 + 熔断 + retry、`conclusion.EmitError` IM 渲染入口 |
| 约束 | 不动 D5 observability 自身的错误流（仅消费其 redactor） |
| 约束 | 不引入新外部依赖（SanitizeForUser 纯 stdlib + 正则） |
| 约束 | 类型合并（C2）须保 `errors.Is/As` 兼容旧调用方，必要时 `internal/shared/errors/internal/migrate.go` 加 deprecated alias |
| 约束 | D1 IM 渲染脱敏（AC1）必须在所有 D1 适配器入口（feishu + cli + future）统一调用，不能各 adapter 重复实现 |
| 约束 | 改动文件数 ≤ 15（focus on error flow） |

## 5. 变更范围

### 新增

- `internal/shared/errors/redact.go` — SanitizeForUser helper + 5+ 正则
- `internal/shared/errors/redact_test.go` — 8+ 测试用例
- `internal/shared/errors/error.go` — 合并后的 `*Error{Code, Message, Err}` 类型 + `IsRetryable` / `ErrorCode` 单一通道
- `internal/shared/errors/migrate.go` — LLMError/SentinelError 的 deprecated alias（保留 1 个 minor release）
- `docs/error-handling.md` — 错误处理设计综述
- 测试：`internal/shared/errors/llm_test.go` + `communication_test.go` 重写

### 修改

- `internal/layers/orchestration/turn/orchestrator.go:256/292/371/428/568` — emitError 改 sanitize
- `internal/layers/orchestration/turn/orchestrator.go:692` — emitError 签名加 `code string` 注入 metadata
- `internal/layers/orchestration/turn/subturn.go:323/354` — 改 sharederrors.Wrap
- `internal/layers/orchestration/sessionorchestrator/orchestrate_path.go:118` — `%v` → `%w`
- `internal/layers/communication/channel/adapters/feishu.go:237/827` — 改 SanitizeForUser
- `internal/layers/llmgateway/protect/retry.go:91-93` — 修 nil-sentinel 包装
- `internal/layers/orchestration/workmodel/task_manager.go:127` — 返回签名改 `(*Task, error)`
- `internal/layers/orchestration/decisionplanning/classifier_fallback.go:73-75` — 加 slog.Warn
- `internal/layers/orchestration/sessionorchestrator/orchestrator.go:270` — EnsureGoal 错误处理
- `internal/layers/llmgateway/stream/gateway.go:110` — classifyAndWrap 之前 inject ctx
- `internal/layers/orchestration/turn_adapter/ltl_hook.go:28` — 移到 shared/errors
- `internal/layers/observability/observability.go:164` — fmt.Errorf 改 Wrap
- 所有跨域使用 `errors.Is(err, sharederrors.ErrXxx)` 调用方 — 适配合并后的 `*Error`

### 不变更

- D5 observability 自身错误流（tracer / metric 上报失败处理）
- D2 contextengine 内部压缩 / enforce 错误（仅看入口 prepare 错误）
- D4 multiagent 内部 fork / join 错误（仅看 freefork 错误聚合点）
- IM 适配器 feishu / cli / future 的非错误消息路径

## 6. 风险评估

| 风险 | 概率 | 影响 | 缓解 |
|------|------|------|------|
| 类型合并 C2 破坏第三方 / 内部调用方 | Med | High | migrate.go 加 deprecated alias 1 minor release；逐个调用方编译时类型断言改造 |
| SanitizeForUser 误删有用信息（如 provider 错误体） | Low | Med | 默认 truncate 240 + redact 列表仅命中明确敏感 pattern；测试覆盖 happy path |
| silent fallback 修复暴露历史 latent bug | Med | Med | 走 PR 一次性 + 测试覆盖新签名；个别 case 可分 commit 渐进 |
| `Gateway.Stream` ctx inject 改变 D7 调用语义 | Low | Med | `errorclass.InjectClassification` 已是 noop when not set；下游 FromContext 缺 key 返回 zero value 安全 |
| 改动文件 > 15 上限 | Low | Low | 拆分 4 PR（tier1 紧急 + tier1 docs + tier2 合并 + tier2 修复） |
| `EnsureGoal` 错误传播可能让 orchestration 路径炸 | Low | High | 仅加 `slog.Warn` + 落 metadata，不改控制流；CI 全绿即安全 |

## 7. Out of Scope

- D4 multiagent fork / join 错误（仅看 freefork 错误聚合点）
- D2 contextengine 内部 compression / enforce 错误（仅看入口 prepare 错误）
- D5 observability 自身错误（仅消费其 redactor）
- Anthropic provider 接入（Phase A 已 defer）
- 跨 session 上下文共享
- 错误国际化（i18n）—— 当前 SentinelError message 已中英混合，留作后续
- Performance profiling of error path
- 全量 OpenTelemetry 错误 attribute（已有 span RecordError）

## 8. 元数据

- **DM ID**: DM-20260620-003
- **Change ID**: `devrix-error-handling-tier1-tier2`
- **Priority**: P0（Critical + High 涉及安全 + 跨域 sentinel 丢失）
- **Domains**: D1, D2, D3, D7（+ shared/errors 公共域）
- **DSAFT 场景**: D1-S2, D2-S15, D3-S4, D7-S2（错误处理跨场景）
