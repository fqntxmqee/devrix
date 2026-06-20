# Proposal: Error Handling Design — Tier 1 + Tier 2

**Change ID:** devrix-error-handling-tier1-tier2
**Demand ID:** DM-20260620-003
**Status:** S7_Archived
**Priority:** P0
**Date:** 2026-06-20

---

## 1. Background

2026-06-20 用户发起"整体对 devrix 项目做个 review，重点放在错误处理设计上"指令。team-architect 派单完成 1 个全仓错误处理架构 review（800+ words，4 严重度分级，5 hotspot 文件）。发现 2 个 Critical、4 个 High、5 个 Medium、4 个 Low 问题。

**关键背景**：
- devrix 已建立 `internal/shared/errors/` SentinelError 体系（CLAUDE.md §关键约定）
- 已实现 `truncateLLMUserMessage` 模式（`internal/shared/errors/context.go:60`），但仅 D2 `NewLLMUnavailableError` 一处使用
- Phase A/B/C context budget 工作刚刚收尾，已建立的 sentinel + classification 基础设施可复用
- IM 端 D1 feishu 适配器已支持 cardkit streaming + UpdateCard fallback（PR #138/PR #139），错误渲染路径相对稳定

**与已有 change 的关系**：
- 本 change 修复的是 **错误信息本身**（rendering + propagation），不是错误控制流
- 不重复 Phase A/B/C 的 budget 控制 / proactive fold / tool result cap 逻辑
- `sharederrors` 公共域是 P0 修复的载体，因此本 change 涉及跨 D1/D2/D3/D7 四个域

## 2. Problem Statement

详见 `demand.md` §2，摘录 3 个最关键问题：

### Problem 1 (Critical — C1): IM 错误渲染无脱敏

```
[LLM Provider]                [D3 llmgateway]              [D7 turn loop]              [D1 feishu adapter]            [飞书卡片]
status 401 + raw body         classify(err)                emitError(..., err)         Markdown(err.Error())          user 可见
"Bearer sk-abc...xxx"         → wrap with sentinel          "llm invoke failed: %v"    → feishu 卡片                    provider key 泄露
file /Users/fukai/.ssh/id_rsa failed: ...     →             err 包含 raw body+stack     Markdown(err.Error()) 渲染
```

### Problem 2 (High — H1): Sentinel 链跨域丢失

```go
// D2 Prepare() 返回 *SentinelError (CodeLLMUnavailable)
err := o.context.Prepare(ctx, ...)
// D7 runLoop 走 emitError(..., fmt.Sprintf("prepare failed: %v", err))
o.emitError(out, req.SessionID, fmt.Sprintf("prepare failed: %v", err))
// D1 拿到的 event.Content 是 "prepare failed: <err.Error()>" 字符串
// D1 无法 errors.Is(err, ErrLLMUnavailable) 区分 → 只能显示固定文案
```

### Problem 3 (High — H2): nil-sentinel 包装可触发 retry 死循环

```go
// internal/layers/llmgateway/protect/retry.go:91-93
if lastErr == nil {
    lastErr = sharederrors.NewProviderUnavailableError(nil)  // ← wrap nil
}
return nil, lastErr
// 上游 IsRetryable 检查时：
//   if errors.Is(lastErr, ErrProviderUnavailable) {  // 命中
//     return true  ← 又进 retry loop
//   }
```

## 3. Proposed Solution

### 3.1 总体方案

| Tier | 范围 | 工作量 | 依赖 |
|------|------|--------|------|
| **Tier 1 (紧急)** | C1 + H1 + H2 — IM 脱敏 + sentinel 链恢复 + retry nil-sentinel | 小 | — |
| **Tier 2 (必须)** | C2 + H3 + M1..M4 — 类型合并 + silent fallback + ctx inject + invariant migration | 中到中大 | Tier 1 落地 |

### 3.2 关键决策

#### Decision 1: SanitizeForUser 落地位置

**选项对比：**

| 方案 | 优点 | 缺点 |
|------|------|------|
| A. 在 IM 适配器入口（feishu.go）各自实现 | 灵活 | 各 adapter 重复实现，遗漏风险 |
| **B. 在 `sharederrors` 提供 `SanitizeForUser(err) string` 公共函数（选）** | **单一 SoT；所有 D1 适配器统一调用；可单元测试** | 公共函数需谨慎设计 |
| C. 在 `conclusion.EmitError` 出口统一包装 | 集中 | 只覆盖 emit 路径，遗漏 feishu.go:827 replyAckAck 路径 |

**选择**: B
**理由**: 公共函数 + 单一 SoT；D1 各 adapter（feishu / cli / future）都调；可在 source 端（emitError）和 sink 端（IM render）双调做防御纵深。

#### Decision 2: LLMError + SentinelError 合并

**选项对比：**

| 方案 | 优点 | 缺点 |
|------|------|------|
| A. 完全删除 LLMError，迁到 SentinelError | 简单 | 破坏现有调用方 |
| **B. 合并为新 `*Error{Code, Message, Err}`；旧 LLMError / SentinelError 保留为 type alias + 工厂函数（选）** | **类型统一；调用方逐步迁移；`errors.Is/As` 兼容** | 需 deprecation period |
| C. 保留双类型，文档化"何时用哪个" | 零迁移 | 治标不治本 |

**选择**: B
**理由**: 治本；errors.Is(err, ErrLLMAuthFailed) 走 type assertion 到 `*Error{Code, Msg, Err}`，与 LLMError / SentinelError 平起平坐；migrate.go 提供 1 个 minor release 的 alias。

#### Decision 3: silent fallback 修复方式

**选项对比：**

| 方案 | 优点 | 缺点 |
|------|------|------|
| A. 全改返回 `(*Task, error)` 等二元签名 | 类型安全 | 改动面大 |
| **B. 关键路径（A 涉及控制流）改签名；observability-only 路径加 slog.Warn（选）** | **最小破坏；observability 路径不变** | 需逐 case 评估 |
| C. 全部加 slog.Warn 不改签名 | 最小改动 | QA 仍无法程序化处理错误 |

**选择**: B
**理由**: `task_manager.go:127` 是"create task"控制流关键路径，必须改签名；`EnsureGoal` / `classifier_fallback` 是 observability-only，加 slog.Warn + metadata 即可。

### 3.3 实施顺序

| PR | 范围 | 风险 | 依赖 |
|----|------|------|------|
| **PR-A** | Tier 1: C1 + H1 + H2 — `SanitizeForUser` + 6 处 emitError/render 替换 + retry nil-sentinel 修复 | Low | — |
| **PR-B** | Tier 2: C2 — 类型合并 `*Error` + `migrate.go` deprecated alias + 所有调用方适配 | Med | PR-A |
| **PR-C** | Tier 2: H3 + M1..M4 — silent fallback 修复 + invariant 迁移 + ctx inject + observability Wrap | Low | PR-B |
| **PR-D** | docs + t-registry + acceptance-report + S6 归档 | Low | PR-A-C |

## 4. Success Metrics

| 指标 | 当前 | 目标 | 度量方式 |
|------|------|------|----------|
| IM 错误泄漏敏感信息 | 5+ 处 `Markdown(err.Error())` | 0 处直传 | grep `err.Error\(\)` in `adapters/*.go` |
| Sentinel 链跨域丢失 | 6+ 处 `%v` | 0 处（除 user-safe 字符串） | grep `fmt.Sprintf.*%v.*err` in `orchestration/turn/*.go` |
| silent fallback 数量 | 5+ 处 | 0 处（除非注释说明） | grep `_ = ` + `return.*nil` in business code |
| retry 死循环风险 | nil-sentinel wrap bug 1 处 | 0 | unit test `TestRetry_NilLastErr` |
| SentinelError 类型数 | 2（LLMError + SentinelError） | 1 | `internal/shared/errors/*.go` 中 struct 类型计数 |
| P0 T 层测试通过率 | — | 100% | `./scripts/test-unit.sh` + acceptance |

## 5. Implementation Plan

### 5.1 Tier 1 (PR-A) — 紧急 hotfix

| 文件 | 改动 | LoC 预估 |
|------|------|----------|
| `internal/shared/errors/redact.go` (新) | `SanitizeForUser(err error) string` + 5+ 正则 | +50 |
| `internal/shared/errors/redact_test.go` (新) | 8+ 用例 | +100 |
| `internal/layers/communication/channel/adapters/feishu.go:237/827` | 改 `SanitizeForUser(err)` | +2 |
| `internal/layers/orchestration/turn/orchestrator.go:256/292/371/428/568` | emitError 改 `SanitizeForUser` | +5 |
| `internal/layers/orchestration/turn/subturn.go:323/354` | 改 `sharederrors.Wrap` 保留 sentinel | +4 |
| `internal/layers/orchestration/sessionorchestrator/orchestrate_path.go:118` | `%v` → `%w` | +1 |
| `internal/layers/llmgateway/protect/retry.go:91-93` | nil-sentinel 修复 | +4 |

**PR-A 合计**: 6 文件 +166 LoC, 1 PR

### 5.2 Tier 2 PR-B (类型合并)

| 文件 | 改动 | LoC 预估 |
|------|------|----------|
| `internal/shared/errors/error.go` (新) | 合并 `*Error{Code, Message, Err}` | +80 |
| `internal/shared/errors/migrate.go` (新) | LLMError / SentinelError → type alias + 工厂函数 | +40 |
| `internal/shared/errors/llm.go` | 改 type alias | -10 |
| `internal/shared/errors/communication.go` | 改 type alias | -10 |
| `internal/shared/errors/llm_test.go` | 重写为新类型测试 | ±0 |
| `internal/shared/errors/communication_test.go` | 重写为新类型测试 | ±0 |
| `internal/shared/errors/error_test.go` (新) | 合并类型 behavior 测试 | +120 |
| 跨域调用方 (orchestrator, llmgateway, communicate) | 适配方类型断言 | +60 |
| `docs/error-handling.md` (新) | 设计综述 + 迁移指南 | +200 |

**PR-B 合计**: 10+ 文件, 1 PR

### 5.3 Tier 2 PR-C (silent fallback + misc)

| 文件 | 改动 | LoC 预估 |
|------|------|----------|
| `internal/layers/orchestration/workmodel/task_manager.go:127` | 返回 `(*Task, error)` | +20 |
| `internal/layers/orchestration/decisionplanning/classifier_fallback.go:73` | 加 slog.Warn | +3 |
| `internal/layers/orchestration/sessionorchestrator/orchestrator.go:270` | EnsureGoal 错误传播 | +5 |
| `internal/layers/orchestration/turn_adapter/ltl_hook.go:28` | 移到 shared/errors | +5 |
| `internal/layers/llmgateway/stream/gateway.go:110` | classifyAndWrap 前 inject ctx | +3 |
| `internal/layers/observability/observability.go:164` | fmt.Errorf → Wrap | +3 |
| `internal/layers/llmgateway/protect/retry_test.go` | 加 nil-sentinel test | +30 |
| `internal/layers/orchestration/workmodel/task_manager_test.go` | 适配新签名 | +30 |

**PR-C 合计**: 8 文件, 1 PR

### 5.4 Tier 2 PR-D (docs + 归档)

| 文件 | 改动 |
|------|------|
| `docs/error-handling.md` (新建) | 设计综述 + 迁移指南 |
| `openspec/t-registry.md` (新条目) | 17 P0 T 点 |
| `openspec/specs/shared-errors/t-registry.md` (新) | 域 T 注册表 |
| `openspec/changes/devrix-error-handling-tier1-tier2/acceptance-report.md` (S5 生成) | S5 验收 |
| `openspec/changes/devrix-error-handling-tier1-tier2/ → openspec/archive/2026-06-20-devrix-error-handling-tier1-tier2/` | S6 归档 |

**PR-D 合计**: 1 PR (S6 archive)

## 6. Risks & Mitigations

详见 `demand.md` §6。补充：

| 风险 | 概率 | 影响 | 缓解 |
|------|------|------|------|
| 类型合并改动量大，调用方遗漏 | Med | High | `migrate.go` 1 minor release 兼容期；`go vet` + 编译时全量断言 |
| Silent fallback 修复暴露 latent bug | Med | Med | 逐 PR 渐进；CI 全绿即安全 |
| 4 PR 拆分 → 4 次 auto-merge → 可能中间被 master 推进冲突 | Low | Low | 每个 PR 合并后 `git pull --rebase origin master` 再开下一个 |

## 7. Out of Scope

详见 `demand.md` §7。补充：

- 不重构 D5 observability 自身的错误流
- 不动 IM 适配器非错误消息路径
- 不动 D2 contextengine 内部压缩 / enforce 错误
- 不动 D4 multiagent 内部 fork / join 错误
- 不做错误 i18n
- 不做 errors 全量 OpenTelemetry attribute
- 不做 errors 全量 metrics（仅现有 observability 接入点）

## 8. References

- `demand.md` — 完整需求（DM-20260620-003）
- `design.md` — 详细技术设计（S3）
- `docs/error-handling.md` — 错误处理综述（PR-D 新建）
- `openspec/specs/d7-orchestration/spec.md` — D7 turn loop 错误流
- `openspec/specs/d1-communication/spec.md` — D1 IM 适配器错误渲染
- `openspec/specs/d3-llm-gateway/spec.md` — D3 llm gateway 错误分类
- `internal/shared/errors/` — 公共错误库（待改造）
- Phase A/B/C 归档 — `openspec/archive/2026-06-20-devrix-context-budget-*/`
