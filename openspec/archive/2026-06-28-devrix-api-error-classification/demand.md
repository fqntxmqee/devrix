---
demand-id: DM-20260628-001
title: D3 LLM Gateway API 错误分类与可恢复语义 — 状态码分桶 + Fallback 兜底
priority: P0
status: S1_Proposal
dsaft_domain: llm-gateway
created: 2026-06-28
reporter: clawcode v2.1.88 对比分析（2026-06-28）
---

# D3 LLM Gateway API 错误分类与可恢复语义

## 1. 背景

devrix 当前在 LLM 调用错误处理层只做了**字符串级别的 sanitize**（`internal/shared/errors/SanitizeForUser`），所有来自 Anthropic / minimax / DeepSeek 的 5xx/4xx 错误被压成同一句"网络异常请重试"喂给 D1 IM 适配器。D7 orchestrator 收到错误后**没有类型信息**，一律走 `emitError` → session 终止路径。

这种粗粒度处理在以下场景直接劣化用户体验：

| 场景 | 用户感知 | 实际根因 |
|------|----------|----------|
| minimax M2.7-highspeed 偶发 529 | session 中断，需手动重发 | 主模型过载（可重试/可 fallback） |
| 用户配置 key 过期 401 | session 中断 + 文案误导 | 配置错误（不可重试，需提示更新） |
| 长 session 累计超 200K prompt | session 中断 + 丢失全部上下文 | 可恢复（折叠后继续） |
| minimax 流中途断连 | session 中断 | 可重试 3 次内通常恢复 |

clawcode v2.1.88 在 `src/services/api/errors.ts:1163-1182` 已经把同类问题按 status code 分桶为 `rate_limit` / `authentication_failed` / `server_error` / `media_size` / `prompt_too_long` / `unknown` 六类，并在 `src/query.ts:894-953` 提供 `FallbackTriggeredError` 自动切 fallback 路径。devrix 6 月 27 日 hotfix（`devrix-d7-llm-intent-only-and-feishu-verdict-hotfix`）已经先行把 `error_code` 字段暴露到 `EngineEvent.Metadata`，但**没有定义 code 的取值集合与分类规则**——本需求补齐这一层。

## 2. 问题陈述

1. **错误类型信息丢失**：`internal/layers/llmgateway/stream/adapter/*.go` 在 HTTP 5xx/4xx 时返回 `*llmgateway.APIError` 但只携带 `Status` 和 `Message`，无 `code` 字段；orchestrator 拿到后无法区分"主模型过载"与"配置错误"。
2. **不可恢复错误不可识别**：401/403/用户的 key 失效时，文案与 529 完全相同，用户看到"网络异常"去查网络。
3. **可恢复错误直接终止**：`prompt_too_long` / `media_size` 错误经过 `emitError` 立即终止 session，丢失所有已完成的工作。clawcode 通过 `withhold-then-recover` 模式（错误先压住，先尝试压缩/折叠）保住 session。
4. **fallback 机制缺失**：`OrchestratorDeps` 没有 `FallbackModel` 字段，主模型 529 时无法自动切到备用模型重试。
5. **D1 IM 适配器差异化能力受限**：`feishu.go` / `cli.go` 只能基于 `Event.Type == "error"` 走统一文案；2026-06-27 hotfix 暴露的 `error_code` 字段暂无受控枚举。

## 3. 验收标准

| ID | 标准 | 优先级 | 可验证方式 |
|----|------|--------|-----------|
| AC1 | `shared/errors/` 新增 `APIErrorCode` 枚举常量：`RateLimit` / `AuthenticationFailed` / `ServerError` / `MediaSize` / `PromptTooLong` / `ImageSize` / `Unknown`；按 HTTP status code 自动映射 | P0 | 单元测试覆盖 401/403/408/413/429/529 五类 status 映射 |
| AC2 | `llmgateway.APIError` 增加 `Code APIErrorCode` 字段，adapter 层在构造错误时按 status 填入；orchestrator 端能从 `errors.IsCode(err, codes.RateLimit)` 判断 | P0 | adapter 层单测 + orchestrator 层单测 |
| AC3 | `OrchestratorDeps` 增加 `FallbackModel string` 字段；当主模型连续 2 次返回 `RateLimit` 或 `ServerError` 时，自动切到 `FallbackModel` 重试当前 turn；重试上限 3 次 | P0 | 注入 mock LLM 模拟 3 次 529 后切到 fallback 的集成测试 |
| AC4 | `prompt_too_long` / `media_size` 类错误**不立即 emit error**，先压住标记 `withheld=true`，下一轮 `prepareContext` 触发 `proactiveFold` 或 `CompressHint`，恢复成功则清掉 `withheld`，仍失败才 surface | P0 | 注入 mock LLM 模拟 prompt_too_long → fold → 成功的集成测试 |
| AC5 | `feishu.go` IM 适配器基于 `Event.Metadata["error_code"]` 走差异化文案：`RateLimit` → "模型繁忙，请稍候重试"；`AuthenticationFailed` → "API key 失效，请检查 ~/.devrix/config.yaml"；`PromptTooLong` → "会话过长，已尝试压缩"；其他 → 现有通用文案 | P1 | feishu adapter 单测覆盖每类 code |
| AC6 | 新增 `internal/shared/errors/api_code.go`，导出 `WithCode(code, msg, cause)` / `Code(err) APIErrorCode` / `IsCode(err, code) bool` 三个 API，向后兼容现有 `SanitizeForUser` 调用点 | P0 | 现有 `SanitizeForUser` 30+ 调用点回归测试 |
| AC7 | `EngineEvent.Metadata["error_code"]` 字段有受控枚举约束（不再允许任意字符串），由 `sharederrors.ErrorCode(err)` 统一返回 | P0 | D7 orchestrator emitError 路径单测 |
| AC8 | 端到端集成测试：模拟主模型 3 次 529 → fallback 切到次模型 → 完成 turn → 飞书卡片显示最终结论 | P0 | `tests/integration/llm_fallback_e2e_test.go` |

## 4. 依赖与约束

| 类型 | 内容 |
|------|------|
| 外部依赖 | clawcode `src/services/api/errors.ts` 的分桶表（仅参考，不复用代码） |
| 内部依赖 | 2026-06-27 hotfix 已暴露的 `Event.Metadata["error_code"]` 通道 |
| 约束 1 | 错误码常量必须是闭集（不允许动态新增），由 Go `const ( ... )` 强约束 |
| 约束 2 | `SanitizeForUser` 现有 30+ 调用点必须保留语义兼容（不删除，仅允许内部增强） |
| 约束 3 | Fallback model 切换不能引入"thinking signature 污染"风险——切换前需清空已累积的 thinking blocks（参考 clawcode `stripSignatureBlocks`） |
| 约束 4 | 本需求只做"分类 + withhold"两件事；fallback 自动切换放 P0-2 follow-up，prompt_too_long fold 闭环放 P0-3 follow-up |

## 5. 变更范围

### 5.1 新增

- `internal/shared/errors/api_code.go` — `APIErrorCode` 枚举 + `WithCode` / `Code` / `IsCode` 三 API
- `internal/shared/errors/api_code_test.go` — 状态码→枚举映射单测（≥ 12 case）
- `internal/layers/llmgateway/api_error.go` — `APIError` 结构 + `NewAPIError(status, msg)` 工厂
- `tests/integration/llm_fallback_e2e_test.go` — AC8 端到端

### 5.2 修改

- `internal/layers/llmgateway/stream/adapter/{minimax,deepseek,anthropic,openai}.go` — HTTP 错误构造点加 `Code` 字段
- `internal/layers/orchestration/sessionorchestrator/turn_orchestrator.go` — `emitError` 路径用 `sharederrors.Code(err)` 填 metadata（替换现有字符串拼接）
- `internal/layers/orchestration/sessionorchestrator/orchestrator_deps.go`（或同等文件）— `OrchestratorDeps` 加 `FallbackModel string` 字段（AC3 留 placeholder，完整逻辑放 P0-2）
- `internal/layers/communication/feishu.go` — 按 `error_code` 走差异化文案（AC5）
- `internal/layers/communication/cli.go` — 同步 AC5 行为

### 5.3 不变更（明确 Out of Scope）

- ❌ **Streaming fallback 自动切换的完整实现**（AC3 仅做字段预留与一次性兜底检测，完整重试循环放 P0-2 `devrix-streaming-fallback`）
- ❌ **Prompt-too-long 折叠恢复闭环**（AC4 仅做错误 withhold 与 `withheld=true` 标记，fold→retry 链路放 P0-3 `devrix-withhold-then-recover`）
- ❌ **Per-tool `maxResultSizeChars` 字段化**（放 P1-4）
- ❌ **`additionalWorkingDirectories` 多工作区**（放 P1-5）
- ❌ **Sandbox 隔离**（放 P1-6）
- ❌ **D2 context engine / D4 multi-agent / D5 observability**（本需求不涉及）
- ❌ **D3 Provider 适配扩展**（v1.1 已闭环，本需求不重新切 S/A）
- ❌ **错误监控埋点新增**（只复用现有 `telemetry`/`audit` 通道，不新加 metric）

## 6. 风险评估

| 风险 | 影响 | 缓解 |
|------|------|------|
| 新错误码枚举破坏现有 D1 IM 适配器 | 中 | AC5 单测覆盖 5 类 code；新增适配器（如 web）只需补 case |
| `FallbackModel` 字段预留但未启用 → 用户以为能 fallback 实际不能 | 低 | 文档明确"AC3 字段已加但完整切换逻辑在 P0-2 follow-up"，日志打 `fallback_model_set_but_not_yet_wired` |
| `withheld=true` 状态未在 session 重启后恢复 → 用户重新触发 prompt_too_long | 中 | `withheld` 仅限 in-memory state，不持久化；session 重启重新进入 withold 路径 |
| 切换 fallback 时 thinking blocks 未清 → Anthropic 报 "thinking blocks cannot be modified" | 中 | 切 fallback 前调 `stripThinkingTags` 兜底（`textutil` 已实现） |
| `APIErrorCode` 枚举值与现有 `sharederrors.ErrorCode` 函数返回的字符串冲突 | 低 | `api_code.go` 用独立前缀 `APICode_*`，`sharederrors.ErrorCode` 改造为薄包装层 |
| Fallback 模型与主模型 prompt 模板不同 → fallback 收到的 system prompt 异常 | 低 | fallback 复用同一 `SystemPrompt`，仅切换 `Model` 字段；S2 阶段细化 |

## 7. 关联记忆与归档

- **借鉴来源**：clawcode `src/services/api/errors.ts:1163-1182` `categorizeRetryableAPIError`
- **先行基础**：DM-20260627-001 `devrix-d7-llm-intent-only-and-feishu-verdict-hotfix` 已暴露 `error_code` 字段
- **DSAFT 域**：D3（主）+ D7 + D1；DSAFT 类型 D3=public / D7=core / D1=core
- **后续 follow-up 候选**（已记入 `.openspec.yaml` followup_candidates）：
  - P0-2 `devrix-streaming-fallback` — 完整 streaming fallback model 切换
  - P0-3 `devrix-withhold-then-recover` — prompt_too_long fold 闭环

## 8. 检查清单（S1 完成前）

- [x] DM ID 已分配：`DM-20260628-001`（grep `demand-archive-index.md` 无冲突）
- [x] `.openspec.yaml` 必填字段已填（id / demand_id / status / priority / domains / dsaft_scenarios）
- [x] demand.md 包含背景、问题、验收标准、依赖、范围、风险
- [x] 8 个 AC 中 7 个 P0、1 个 P1（满足"至少 1 个 P0"）
- [x] Out of Scope 明确声明 7 项
- [x] DSAFT 域标注：`llm-gateway`（主）+ `orchestration` + `communication`
- [x] `dsaft_scenarios` 预登记（D3-S1-A03 / D7-S2-A50 / D1-S3-A08）

---

**S1 完成。等待用户确认后进入 S2（proposal.md + 完整方案对比）。**