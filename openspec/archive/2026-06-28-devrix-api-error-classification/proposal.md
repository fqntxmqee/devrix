# Proposal: D3 LLM Gateway API 错误分类与可恢复语义 — 529/429/401/PromptTooLong 分桶 + Fallback 兜底

**Change ID:** devrix-api-error-classification
**Demand ID:** DM-20260628-001
**Status:** S7_Archived (2026-06-28, PR #265 squash merged, S6 archive)
**Reporter:** clawcode v2.1.88 对比分析（2026-06-28）
**Priority:** P0

---

## 1. Background

devrix 当前在 LLM 调用错误处理层只做了**字符串级别的 sanitize**（`internal/shared/errors/SanitizeForUser`），所有来自 Anthropic / minimax / DeepSeek 的 5xx/4xx 错误被压成同一句"网络异常请重试"喂给 D1 IM 适配器。D7 orchestrator 收到错误后**没有类型信息**，一律走 `emitError` → session 终止路径。

clawcode v2.1.88 在 `src/services/api/errors.ts:1163-1182` 已经把同类问题按 status code 分桶为 `rate_limit` / `authentication_failed` / `server_error` / `media_size` / `prompt_too_long` / `unknown` 六类，并在 `src/query.ts:894-953` 提供 `FallbackTriggeredError` 自动切 fallback 路径。devrix 2026-06-27 hotfix（`devrix-d7-llm-intent-only-and-feishu-verdict-hotfix`）已经先行把 `error_code` 字段暴露到 `EngineEvent.Metadata`，但**没有定义 code 的取值集合与分类规则**——本需求补齐这一层。

参考实现与依赖：

- 借鉴来源：clawcode `src/services/api/errors.ts:1163-1182` `categorizeRetryableAPIError`
- 先行基础：DM-20260627-001 已暴露 `error_code` 字段
- v1.0 错误处理骨架：DM-20260620-003 `devrix-error-handling-tier1-tier2` 落地的 `SentinelError` + `sharederrors.WithCode` 已就位

## 2. Problem Statement

1. **错误类型信息丢失**：`internal/layers/llmgateway/stream/adapter/*.go` 在 HTTP 5xx/4xx 时返回 `*llmgateway.APIError` 但只携带 `Status` 和 `Message`，无 `code` 字段；orchestrator 拿到后无法区分"主模型过载"与"配置错误"。
2. **不可恢复错误不可识别**：401/403/用户的 key 失效时，文案与 529 完全相同，用户看到"网络异常"去查网络。
3. **可恢复错误直接终止**：`prompt_too_long` / `media_size` 错误经过 `emitError` 立即终止 session，丢失所有已完成的工作。clawcode 通过 `withhold-then-recover` 模式（错误先压住，先尝试压缩/折叠）保住 session。
4. **fallback 机制缺失**：`OrchestratorDeps` 没有 `FallbackModel` 字段，主模型 529 时无法自动切到备用模型重试。
5. **D1 IM 适配器差异化能力受限**：`feishu.go` / `cli.go` 只能基于 `Event.Type == "error"` 走统一文案；2026-06-27 hotfix 暴露的 `error_code` 字段暂无受控枚举。

## 3. Proposed Solution

### 3.1 方案对比

| 方案 | 优点 | 缺点 | 结论 |
|------|------|------|------|
| **A. 完整分层 (推荐)** — `APIErrorCode` 枚举 + HTTP status 映射 + `llmgateway.APIError.Code` 字段 + `OrchestratorDeps.FallbackModel` 字段 + `withheld=true` 标记 + IM 差异化文案 | 一致性最好；所有后续 follow-up（P0-2/3）只补逻辑不动契约；`error_code` 字段真正成为 IM 端可消费的受控枚举 | 范围略大（4 个 adapter + orchestrator + 2 个 IM 适配器）；1 个新枚举 + 1 个新结构 + 字段预留 | ✅ **采用** |
| B. 仅做错误分类（不预留 fallback 字段） | 范围最小 | 后续 P0-2 启动时要再次改 `OrchestratorDeps` 结构，影响 IM/Orchestrator 签名 | ❌ |
| C. 完整实现（含 streaming fallback 自动切换） | 一步到位 | 与 P0-2 范围重叠，scope creep 风险高；S4 工作量翻倍 | ❌ |

### 3.2 核心架构

```
D3 Adapter (HTTP 4xx/5xx)
    ↓ 构造 llmgateway.APIError{Status, Message, Code}
sharederrors.WithCode(code, msg, cause)  ←─ 枚举常量闭集
    ↓ sharederrors.Code(err) → APIErrorCode
D7 SessionOrchestrator.emitError
    ↓ Event.Metadata["error_code"] = code.String()  (受控枚举)
D1 feishu.go / cli.go
    ↓ switch code → 差异化文案
User
```

### 3.3 三层契约

1. **闭集枚举层**：`sharederrors.APIErrorCode` 7 类常量（RateLimit/AuthenticationFailed/ServerError/MediaSize/PromptTooLong/ImageSize/Unknown）；`sharederrors.NewAPIErrorCode(status) APIErrorCode` 工厂方法按 HTTP status 自动映射（详见 design.md §2）。
2. **传播层**：`llmgateway.APIError` 结构体加 `Code APIErrorCode` 字段（保留 `Message` / `Status` 字段作为向后兼容）；`sharederrors.Code(err) APIErrorCode` 提取；`sharederrors.IsCode(err, code) bool` 判断。
3. **消费层**：`feishu.go` / `cli.go` 基于 `Event.Metadata["error_code"]` 走差异化文案；`OrchestratorDeps.FallbackModel string` 字段预留（**仅字段，完整逻辑放 P0-2**）。

### 3.4 关键决策

#### Decision: 错误码常量的归属包

**选项:**
| 方案 | 优点 | 缺点 |
|------|------|------|
| A. 放在 `internal/shared/errors/`（sharederrors） | 与 DM-20260620-003 SentinelError 同包；30+ 调用点零迁移；`sharederrors.IsCode` 模式与 `errors.Is` 一致 | 略增加 sharederrors 表面积 |
| B. 放在 `internal/layers/llmgateway/` 单独包 | 紧贴 LLM 域 | 跨域消费（D7/D1）要反向 import llmgateway，破坏分层 |

**选择:** A
**理由:** DM-20260620-003 已经把 `sharederrors` 建成跨域错误基础设施；`Code` / `IsCode` 是通用错误查询 API，归属 sharederrors 与现有 `WithCode` 同源；B 违反 layering 约束。

#### Decision: `withheld=true` 状态的持久化策略

**选项:**
| 方案 | 优点 | 缺点 |
|------|------|------|
| A. in-memory only（不持久化） | 实现简单；session 重启天然重置 | session 重启后若再触发 prompt_too_long 会重新走 fold 路径（其实等价） |
| B. 写入 `internal/layers/communication/capture/transcript/` | 重启后可见 | 跨 session 状态污染；transcript 设计是只读审计；可能引入并发风险 |

**选择:** A
**理由:** withhold 是 turn-internal 临时状态，session 重启后无意义；`AC4` 明确"仅限 in-memory state"。

#### Decision: `FallbackModel` 字段是否在 S4 启用

**选项:**
| 方案 | 优点 | 缺点 |
|------|------|------|
| A. 仅字段预留，日志打 `fallback_model_set_but_not_yet_wired` | P0-2 启动时零结构改动 | 用户以为能 fallback 实际不能（已写入需求风险表） |
| B. 字段 + 完整重试循环 | 一步到位 | 与 P0-2 范围重叠 |

**选择:** A
**理由:** 与方案 A 的 "P0-2 留 follow-up" 一致；日志显式标注避免用户误解。

## 4. Success Metrics

| 维度 | 指标 | 目标值 | 测量方法 |
|------|------|--------|----------|
| 错误分类覆盖率 | 6 类 APIErrorCode 全部走通 HTTP status 映射 | 100% | `internal/shared/errors/api_code_test.go` 12+ case |
| IM 差异化文案准确率 | 5 类 code 各自独立文案 | 5/5 PASS | `internal/layers/communication/feishu_test.go` |
| 向后兼容 | 现有 30+ `SanitizeForUser` 调用点零行为变化 | 0 regression | `go test ./...` 全绿 |
| 错误信息泄漏防护 | API key/Bearer/路径/凭证 substring 不出现在 IM 文案 | 0 leak | `sharederrors.SanitizeForUser` 单测 + AC6 验证 |
| 闭集约束 | 编译期禁止动态新增 code | Go const 强约束 | 0 runtime panic 风险 |

## 5. Implementation Plan

### 5.1 实施步骤（无工时估算，详见 `tasks.md`）

1. **Step 1 — 枚举与映射层**
   - 新增 `internal/shared/errors/api_code.go`：`APIErrorCode` 7 类 const + `String()` + `ParseAPIErrorCode(s) (APIErrorCode, error)` + `NewAPIErrorCodeFromStatus(status int) APIErrorCode` 工厂（401/403→AuthFailed, 408/413→PromptTooLong, 429→RateLimit, 5xx→ServerError, 529→ServerError）
   - 新增 `WithCode(code APIErrorCode, msg string, cause error) error` / `Code(err error) APIErrorCode` / `IsCode(err error, code APIErrorCode) bool` 三 API
   - 新增 `internal/shared/errors/api_code_test.go` ≥ 12 case（7 枚举 + 5 status 映射边界）

2. **Step 2 — llmgateway 错误结构扩展**
   - 新增 `internal/layers/llmgateway/api_error.go`：`APIError` struct `{ Status int; Message string; Code sharederrors.APIErrorCode; Cause error }` + `NewAPIError(status int, msg string) *APIError` 工厂（自动调 `NewAPIErrorCodeFromStatus`）
   - 新增 `Error() string` / `Unwrap() error` 实现 errors 接口
   - 4 个 adapter（minimax/deepseek/anthropic/openai）HTTP 错误构造点改用 `NewAPIError`
   - 单元测试覆盖 4 adapter

3. **Step 3 — OrchestratorDeps 扩展 + emitError 路径**
   - `internal/layers/orchestration/sessionorchestrator/orchestrator_deps.go`（或同等文件）加 `FallbackModel string` 字段
   - `internal/layers/orchestration/sessionorchestrator/turn_orchestrator.go`（或 `emitError` 所在文件）的 `emitError` 路径用 `sharederrors.Code(err)` 填 `Metadata["error_code"]`
   - 当 `Code == RateLimit || ServerError` 且连续 2 次时，log `fallback_trigger_candidate` + 暂时 fallback_model 未 wire 时 log `fallback_model_set_but_not_yet_wired`
   - `withheld` 字段加到 `TurnState`（or 等价状态结构）；prompt_too_long 错误标记 `withheld=true` 而不 emit error

4. **Step 4 — D1 IM 适配器差异化文案**
   - `internal/layers/communication/feishu.go` 按 `Event.Metadata["error_code"]` 走差异化文案
   - `internal/layers/communication/cli.go` 同步
   - 5 个 code 各自独立文案（`RateLimit` / `AuthenticationFailed` / `PromptTooLong` / `MediaSize` / `ImageSize` / `ServerError` / `Unknown`）

5. **Step 5 — 端到端集成测试**
   - `tests/integration/llm_fallback_e2e_test.go`：mock 主模型 3 次 529 → fallback 切到次模型 → 完成 turn → 飞书卡片显示最终结论

### 5.2 关键文件清单

| 类型 | 路径 | 说明 |
|------|------|------|
| 新增 | `internal/shared/errors/api_code.go` | APIErrorCode 枚举 + 3 API |
| 新增 | `internal/shared/errors/api_code_test.go` | 12+ 单测 |
| 新增 | `internal/layers/llmgateway/api_error.go` | APIError struct + 工厂 |
| 新增 | `internal/layers/llmgateway/api_error_test.go` | struct 单测 |
| 新增 | `tests/integration/llm_fallback_e2e_test.go` | E2E |
| 修改 | `internal/layers/llmgateway/stream/adapter/minimax.go` | HTTP 错误构造 |
| 修改 | `internal/layers/llmgateway/stream/adapter/deepseek.go` | HTTP 错误构造 |
| 修改 | `internal/layers/llmgateway/stream/adapter/anthropic.go` | HTTP 错误构造 |
| 修改 | `internal/layers/llmgateway/stream/adapter/openai.go` | HTTP 错误构造 |
| 修改 | `internal/layers/orchestration/sessionorchestrator/orchestrator_deps.go` | `FallbackModel` 字段 |
| 修改 | `internal/layers/orchestration/sessionorchestrator/turn_orchestrator.go` | `emitError` 用 `sharederrors.Code` |
| 修改 | `internal/layers/communication/feishu.go` | 差异化文案 |
| 修改 | `internal/layers/communication/cli.go` | 差异化文案 |
| 同步 | `openspec/specs/d3-llm-gateway/t-registry.md` | v3.2.0 → v3.3.0 +3 T |
| 同步 | `openspec/specs/d7-orchestration/t-registry.md` | v4.8.0 → v4.9.0 +2 T |
| 同步 | `openspec/specs/d1-communication/t-registry.md` | v3.0.0 → v3.1.0 +1 T |
| 同步 | `openspec/t-registry.md` | v5.6.0 → v5.7.0 增量 note |
| 同步 | `openspec/specs/d3-llm-gateway/spec.md` | Gherkin ADDED Requirements |
| 同步 | `openspec/demand-archive-index.md` | active changes 表 |

### 5.3 T 层预登记（PLANNED）

**D3 域（v3.2.0 → v3.3.0，+3 P0 T）**：

| T ID | 描述 | A/F | Test 位置 |
|------|------|-----|-----------|
| D3-S1-A01-T04 | `NewAPIErrorCodeFromStatus` HTTP status → APIErrorCode 映射覆盖 401/403/408/413/429/529/5xx/4xx-unknown 7 类 | D3-S1-A01-F01 MatchRouting（错误分类）| `internal/shared/errors/api_code_test.go` |
| D3-S1-A01-T05 | `sharederrors.IsCode(err, code)` 正确识别包装链（WithCode → Unwrap → bare APIError）| D3-S1-A01-F01 | `internal/shared/errors/api_code_test.go` |
| D3-S3-A01-T17 | 4 adapter（minimax/deepseek/anthropic/openai）HTTP 错误构造点全部走 `NewAPIError`，不再用字符串拼接 | D3-S3-A01-F02 RecordOutcome | `internal/layers/llmgateway/stream/adapter/*_test.go` |

**D7 域（v4.8.0 → v4.9.0，+2 P0 T）**：

| T ID | 描述 | A/F | Test 位置 |
|------|------|-----|-----------|
| D7-S2-A50-T05 | `OrchestratorDeps.FallbackModel` 字段就位 + `withheld` 状态在 `TurnState` 就位 + emitError 路径用 `sharederrors.Code(err)` 填 `Metadata["error_code"]` | D7-S2-A50 RunTurnLoop | `internal/layers/orchestration/sessionorchestrator/orchestrator_test.go` |
| D7-S2-A50-T06 | 主模型 2 次连续 RateLimit/ServerError 触发 `fallback_trigger_candidate` 日志（fallback_model 未 wire 场景）+ prompt_too_long 错误标记 `withheld=true` 不 surface | D7-S2-A50 | `internal/layers/orchestration/sessionorchestrator/turn_orchestrator_test.go` |

**D1 域（v3.0.0 → v3.1.0，+1 P0 T）**：

| T ID | 描述 | A/F | Test 位置 |
|------|------|-----|-----------|
| D1-S3-A08-T01 | feishu / cli IM 适配器基于 `Event.Metadata["error_code"]` 走差异化文案（5 类 code 各自独立文案 + 兜底 Unknown）| D1-S3-A08 EmitError | `internal/layers/communication/feishu_test.go` + `cli_test.go` |

## 6. Risks & Mitigations

| 风险 | 影响 | 缓解 |
|------|------|------|
| 新错误码枚举破坏现有 D1 IM 适配器 | 中 | AC5 单测覆盖 5 类 code；新增适配器（如 web）只需补 case |
| `FallbackModel` 字段预留但未启用 → 用户以为能 fallback 实际不能 | 低 | 文档明确"AC3 字段已加但完整切换逻辑在 P0-2 follow-up"，日志打 `fallback_model_set_but_not_yet_wired` |
| `withheld=true` 状态未在 session 重启后恢复 → 用户重新触发 prompt_too_long | 中 | `withheld` 仅限 in-memory state，不持久化；session 重启重新进入 withhold 路径 |
| 切换 fallback 时 thinking blocks 未清 → Anthropic 报 "thinking blocks cannot be modified" | 中 | P0-2 follow-up 切 fallback 前调 `stripThinkingTags` 兜底（`textutil` 已实现） |
| `APIErrorCode` 枚举值与现有 `sharederrors.ErrorCode` 函数返回的字符串冲突 | 低 | `api_code.go` 用独立前缀 `APICode_*`，`sharederrors.ErrorCode` 改造为薄包装层 |
| Fallback 模型与主模型 prompt 模板不同 → fallback 收到的 system prompt 异常 | 低 | P0-2 follow-up 解决；S4 仅预留字段 |
| 4 个 adapter 改造 diff 大 → 合并冲突 | 低 | 4 adapter 的 HTTP 错误构造模式高度相似（4 模式：ParseError/HTTPError/StreamError/AuthError），统一走 `NewAPIError` 后冲突面收敛 |

## 7. Out of Scope

- ❌ **Streaming fallback 自动切换的完整实现**（AC3 仅做字段预留与一次性兜底检测，完整重试循环放 P0-2 `devrix-streaming-fallback`）
- ❌ **Prompt-too-long 折叠恢复闭环**（AC4 仅做错误 withhold 与 `withheld=true` 标记，fold→retry 链路放 P0-3 `devrix-withhold-then-recover`）
- ❌ **Per-tool `maxResultSizeChars` 字段化**（放 P1-4）
- ❌ **`additionalWorkingDirectories` 多工作区**（放 P1-5）
- ❌ **Sandbox 隔离**（放 P1-6）
- ❌ **D2 context engine / D4 multi-agent / D5 observability**（本需求不涉及）
- ❌ **D3 Provider 适配扩展**（v1.1 已闭环，本需求不重新切 S/A）
- ❌ **错误监控埋点新增**（只复用现有 `telemetry`/`audit` 通道，不新加 metric）

## 8. 检查清单（S2 完成前）

- [x] `.openspec.yaml` 所有字段已填写
- [x] `dsaft_scenarios` 已标注涉及的 DSAFT 场景 ID
- [x] `proposal.md` 包含方案对比与风险评估
- [x] T 层测试点在 T 层注册表预登记（PLANNED）— 根索引 `openspec/t-registry.md`，域明细 `openspec/specs/d{N}-*/t-registry.md`
- [x] `dsaft_activities` 已在 `.openspec.yaml` 标注
- [x] `t_points` 已在 `.openspec.yaml` 标注
- [x] decision 记录在 §3.4
- [x] Out of Scope 明确声明 8 项
- [x] `demand-archive-index.md` active changes 入表

---

**S2 完成。等待用户确认后进入 S3（design.md + spec.md Gherkin）。**
