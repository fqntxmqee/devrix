---
demand-id: DM-20260614-004
title: S5-P2 Tail-only LLM Classify Shadow — v1.1 兜底冷启动准备
source: devrix-d7-orchestration-domain R2 决议 P1 #8 (命题 C)
priority: P1
status: S1_Requirement
dsaft_domain: D7, D3
created: 2026-06-14
last-updated: 2026-06-14
---

# S5-P2 Tail-only LLM Classify Shadow

## 1. 原始描述

`devrix-d7-orchestration-domain` R2 决议 §5 命题 C 明确指出：

> S5-P2 规则+command 优先存在"演化博弈局部最优"风险：v1.0 用户消息种群中，~80% 被规则匹配掉，规则策略成为 ESS（演化稳定策略）。LLM 策略到 v1.1 引入时**冷启动**。

**R2 决议**：v1.0 同时跑 LLM 分类（结果仅入日志、不入决策），收集置信度分布与训练样本。具体范围：

- **仅对规则未命中 tail（~20%）异步 LLM classify**
- **结果只写日志/样本库**
- v1.0 决策路径仍为规则+command-first
- shadow 为 v1.1 兜底冷启动准备
- 列入 v1.0 release 后第一个 P1

**现状**：

- `internal/layers/d7/classifier.go` 仅 `RuleClassifier`，无 LLM 接口
- `internal/layers/d7/orchestrator.go` `classifier IntentClassifier` 仅实现 `Classify(ctx, message) (IntentClassification, error)`
- 无 `LLMIntentClassifier` 接口定义
- 无 `ShadowClassifier` 包装
- 无配置项 `D7Config.ShadowLLMClassify` / `D7Config.ShadowLLMTimeoutMs`
- 无 `orchestration.intent.classify.shadow.*` metric

**目标**：实现 v1.0 LLM 兜底冷启动准备。具体：

1. 定义 `LLMIntentClassifier` 接口（与 d7 域解耦）
2. 实现 `ShadowClassifier` 包装 `RuleClassifier` + `LLMIntentClassifier`
3. 接入 `SessionOrchestrator`（替换 `classifier` 字段或新增 `shadowClassifier`）
4. 配置项：`ShadowLLMClassify bool` (默认 false) + `ShadowLLMTimeoutMs int` (默认 500)
5. Metric：`orchestration.intent.classify.shadow.{match, mismatch, error, llm_latency}` 
6. Span：`d7.intent.classify.shadow` 含 rule_kind / llm_kind / llm_match / llm_latency_ms

## 2. 验收标准

| ID | 标准 | 优先级 |
|----|------|--------|
| AC1 | `LLMIntentClassifier` 接口定义，1 方法 `ClassifyIntent(ctx, message) (IntentClassification, error)` | **P0** |
| AC2 | `ShadowClassifier` 包装 `IntentClassifier` (rule) + `LLMIntentClassifier` (llm) + `ShadowMetrics` + `timeoutMs` | **P0** |
| AC3 | 当 rule 返回 `IntentSkip` / `IntentCommand` / `IntentFast` 时，**LLM 不调用**（tail-only） | **P0** |
| AC4 | 当 rule 返回 `IntentOrchestrate` 时，**异步触发 LLM** classify；不阻塞 rule 决策路径 | **P0** |
| AC5 | LLM 调用超时（默认 500ms）时，记 error metric；不传播错误 | **P0** |
| AC6 | LLM 返回结果与 rule 一致 → match counter +1 | **P0** |
| AC7 | LLM 返回结果与 rule 不一致 → mismatch counter +1 + span 属性 | **P0** |
| AC8 | `ShadowLLMClassify=false` 时，**不调用 LLM**（hot path zero-cost） | **P0** |
| AC9 | 单元测试覆盖 AC3~AC8 全部场景 | **P0** |
| AC10 | 配置可注入（`Config.ShadowLLMClassify` + `Config.ShadowLLMTimeoutMs`） | P1 |

## 3. 范围

### 3.1 新增

- `internal/layers/d7/shadow_classifier.go`：
  - `LLMIntentClassifier` 接口
  - `ShadowMetrics` 结构（含 4 counter / 1 histogram 占位）
  - `ShadowClassifier` 结构 + 构造函数
  - `(*ShadowClassifier).Classify(ctx, message)` — 同步返回 rule 结果，异步触发 LLM
  - 内部 `(*ShadowClassifier).shadowAsync(ctx, message, ruleResult)` 私有方法
  - 默认 `noopLLMIntentClassifier`（用于 AC8 disabled 状态）

- `internal/layers/d7/shadow_classifier_test.go`：
  - 9 个测试覆盖 AC1~AC9

### 3.2 修改

- `internal/layers/d7/config.go`：
  - `Config` 结构新增 `ShadowLLMClassify bool` + `ShadowLLMTimeoutMs int`
  - `DefaultConfig()` 设默认 `false` + `500`

- `internal/layers/d7/orchestrator.go`：
  - 新增 `shadowClassifier *ShadowClassifier` 字段
  - 新增 `WithShadowClassifier(s *ShadowClassifier) OrchestratorOption`
  - 替换 `classifier IntentClassifier` 为 `classifier` 字段 + 内部 `shadow` 字段
  - 现有 `NewSessionOrchestrator` 默认 shadow 为 `nil`（backward compat）

- `openspec/specs/d7-orchestration/t-registry.md`：
  - 新增 `D7-S5-T07` PLANNED — Tail-only LLM classify shadow

- `openspec/t-registry.md`：
  - D7 域 Total 45 → 46

### 3.3 不变更

- `RuleClassifier` 实现（classify.go）— 行为完全不变
- `IntentClassifier` 接口签名不变
- 现有 D7-S2 路由矩阵（规则+command-first 仍为权威决策）
- 现有 FastPath 性能预算（P99 ≤ 2ms）
- D5 observability metric 注册机制（仅新增 4 个 counter）

## 4. 依赖与约束

| 类型 | 内容 |
|------|------|
| 依赖 | D3 LLM Gateway（不直接依赖，依赖抽象接口 `LLMIntentClassifier`） |
| 依赖 | D5 observability metrics（`metrics.Meter` / `Counter` / `Histogram`） |
| 约束 | 不得修改 `IntentClassifier` 接口（保持向后兼容） |
| 约束 | shadow 调用不阻塞 rule 决策路径（goroutine + select on ctx.Done） |
| 约束 | shadow 不传错误到 caller（错误只入 metric + log） |
| 约束 | 默认 disabled（`ShadowLLMClassify=false`），避免对所有用户产生 LLM 成本 |

## 5. 风险评估

| 风险 | 影响 | 缓解 |
|------|------|------|
| LLM 延迟拉低 FastPath | 中 | tail-only（仅 rule 未命中）+ 异步触发 + 严格超时 |
| LLM 成本失控 | 中 | 默认 disabled；启用时按 session 计数；后续可加 rate limit |
| shadow 调用 panic 影响 rule 决策 | 高 | goroutine 包裹 + `defer recover` + error metric |
| LLM 分类质量低于预期 | 低 | 不影响决策路径；v1.1 切换前可 A/B 对比 |
| 测试需要 mock LLM | 低 | `LLMIntentClassifier` 接口 + 测试 stub |

## 6. 后续路线（v1.1+）

1. 收集 shadow 数据后，将 LLM classify 作为 v1.1 兜底（rule 未命中时切换 LLM 决策）
2. 配置 rate limit（按 session / IP）
3. shadow 样本库持久化（JSONL 入 `var/log/devrix/shadow/`）
4. 离线评估（Devrix Eval Phase 5 — shadow 对比报告）
5. 三模型合并决策清单（命题 B）
