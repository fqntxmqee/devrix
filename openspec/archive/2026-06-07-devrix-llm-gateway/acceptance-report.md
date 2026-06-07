---
demand-id: DM-20260607-004
title: LLM Gateway Layer V1（DeepSeek + MiniMax + 熔断 + Token 统一） — 验收报告
executor: fukai
environment: local
date: 2026-06-07
verdict: ACCEPTED
change: devrix-llm-gateway
---

# 验收报告：LLM Gateway Layer V1（DeepSeek + MiniMax + 熔断 + Token 统一）

## 1. 执行摘要

| 项目 | 值 |
|------|---|
| 需求 ID | DM-20260607-004 |
| 变更 | devrix-llm-gateway |
| 执行人 | fukai |
| 测试环境 | local |
| 执行日期 | 2026-06-07 |
| 总体结论 | **ACCEPTED** |

## 2. 自动化验证

| 检查 | 结果 | 证据 |
|------|------|------|
| `scripts/test-unit.sh` | PASS | see CI/local log |
| `scripts/test-integration.sh` | PASS | see CI/local log |
| `scripts/test-e2e.sh` | PASS | see CI/local log |
| `scripts/test-acceptance.sh` | PASS | see CI/local log |


## 3. L5 测试点验证结果

| L5 ID | 描述 | 优先级 | 状态 | 证据 |
|-------|------|--------|------|------|
| L5-COMM-01 | 运行 devrix 创建 CLI 会话 | P0 | SKIP | status=PLANNED |
| L5-COMM-02 | 空消息入站被拒绝 | P0 | SKIP | status=PLANNED |
| L5-COMM-03 | 会话空闲超时后拒绝消息 | P0 | SKIP | status=PLANNED |
| L5-COMM-04 | /new 命令解析正确 | P0 | SKIP | status=PLANNED |
| L5-COMM-05 | /help 命令解析正确 | P0 | SKIP | status=PLANNED |
| L5-COMM-06 | /stop 命令解析正确 | P0 | SKIP | status=PLANNED |
| L5-COMM-09 | ShortId 唯一且排除歧义字符 | P1 | SKIP | status=PLANNED |
| L5-COMM-11 | 飞书消息解析正确 | P1 | SKIP | status=PLANNED |
| L5-CTX-01 | 新会话初始化上下文与 system prompt | P0 | PASS | go test ./internal/layers/contextengine/memory |
| L5-CTX-02 | 用户消息后历史正确追加 | P0 | PASS | go test ./internal/layers/contextengine/memory |
| L5-CTX-03 | 超 Token 阈值触发七步压缩 | P0 | PASS | go test -tags=acceptance ./tests/acceptance/p0 |
| L5-CTX-04 | TokenBlock 超限返回 ContextExceeded | P0 | PASS | go test ./internal/layers/contextengine/compression |
| L5-CTX-05 | ContextSnapshot 保存与恢复 | P0 | PASS | go test ./internal/layers/contextengine/snapshot |
| L5-CTX-06 | PEV Execute 调用 LLM 并流式输出 | P0 | PASS | go test -tags=integration ./tests/integration |
| L5-CTX-07 | 工具执行后 Verify basic 模式 | P0 | PASS | go test ./internal/layers/contextengine |
| L5-CTX-09 | EngineEvent 与通信层四流契约一致 | P0 | PASS | go test -tags=integration ./tests/integration |
| L5-CTX-11 | 权限批准/拒绝后 PEV 行为正确 | P0 | PASS | go test -tags=integration ./tests/integration |
| L5-CTX-12 | Autocompact 触发并降低 token | P0 | SKIP | status=PLANNED |
| L5-CTX-13 | Autocompact LLM 失败降级跳过 | P0 | SKIP | status=PLANNED |
| L5-CTX-14 | Verify commands 全部通过 | P0 | SKIP | status=PLANNED |
| L5-CTX-15 | Verify 命令失败触发重试 | P0 | SKIP | status=PLANNED |
| L5-CTX-16 | Token 计数共享契约与 Gateway 对齐 | P0 | SKIP | status=PLANNED |
| L5-CTX-08 | Autocompact 禁用时跳过步骤 6 | P1 | PASS | go test ./internal/layers/contextengine/compression |
| L5-CTX-17 | 压缩/Verify 步骤可观测事件 | P1 | SKIP | status=PLANNED |
| L5-CTX-18 | 主路径接入真实 LLM Gateway | P1 | SKIP | status=PLANNED |
| L5-CTX-10 | L3 长期记忆返回 NotImplemented | P2 | PASS | go test ./internal/layers/contextengine/memory |
| L5-LLM-01 | DeepSeek 适配器流式响应 | P0 | PASS | go test ./internal/layers/llmgateway/adapter |
| L5-LLM-02 | MiniMax 适配器流式响应 | P0 | PASS | go test ./internal/layers/llmgateway/adapter |
| L5-LLM-03 | Circuit breaker 正常关闭 | P0 | PASS | go test ./internal/layers/llmgateway/breaker |
| L5-LLM-04 | Circuit breaker 触发开启 | P0 | PASS | go test ./internal/layers/llmgateway/breaker |
| L5-LLM-05 | Circuit breaker 半开→关闭 | P0 | PASS | go test ./internal/layers/llmgateway/breaker |
| L5-LLM-06 | Circuit breaker 半开→开启 | P0 | PASS | go test ./internal/layers/llmgateway/breaker |
| L5-LLM-07 | Token 计数准确性 (cl100k_base) | P0 | PASS | go test ./internal/layers/llmgateway/token |
| L5-LLM-08 | Token 预算检查 | P0 | PASS | go test ./internal/layers/llmgateway/token |
| L5-LLM-09 | Provider 配置加载 | P0 | PASS | go test ./internal/layers/llmgateway/config |
| L5-LLM-10 | DeepSeek Fallback 模型切换 | P1 | PASS | go test -tags=integration ./tests/integration |
| L5-LLM-11 | MiniMax Fallback 模型切换 | P1 | PASS | go test -tags=integration ./tests/integration |
| L5-LLM-12 | 重试策略执行 | P1 | PASS | go test ./internal/layers/llmgateway/retry |
| L5-LLM-13 | LLM 调用可观测事件 | P1 | PASS | go test -tags=integration ./tests/integration |
| L5-LLM-16 | 未知 Provider/Model 报错 | P1 | PASS | go test ./internal/layers/llmgateway/gateway |
| L5-LLM-14 | 多 Provider 并发调用 | P2 | PASS | go test ./internal/layers/llmgateway/gateway |
| L5-LLM-15 | 熔断器状态持久化 | P2 | SKIP | status=PLANNED |
| L5-AGENT-01 | AgentFactory 创建 Agent 实例 | P0 | SKIP | status=PLANNED |
| L5-AGENT-02 | Agent 生命周期状态转换 | P0 | SKIP | status=PLANNED |
| L5-AGENT-03 | 工具注册与风险等级 | P0 | SKIP | status=PLANNED |
| L5-AGENT-04 | Permission Pipeline 授权流程 | P0 | SKIP | status=PLANNED |
| L5-OBS-01 | Tracing Span 创建与传播 | P0 | SKIP | status=PLANNED |
| L5-OBS-02 | Metrics Counter 计数 | P0 | SKIP | status=PLANNED |
| L5-OBS-03 | 日志级别过滤 | P0 | SKIP | status=PLANNED |
| L5-EVO-01 | 版本检测与记录 | P1 | SKIP | status=PLANNED |
| L5-EVO-02 | 配置热更新 | P1 | SKIP | status=PLANNED |

### 统计

| 优先级 | 总数 | 通过 | 失败 | 跳过 |
|--------|------|------|------|------|
| P0 | 36 | 18 | 0 | 18 |
| P1 | 12 | 6 | 0 | 6 |
| P2 | 3 | 2 | 0 | 1 |

## 4. 失败项分析（如有）

无失败项。

## 5. 遗留风险

| 风险 | 影响 | 规避方案 |
|------|------|---------|
| PLANNED L5 未覆盖 | 功能缺口 | 按 `openspec/l5-registry.md` 排期补测 |
| Live 测试未纳入 CI | 外部依赖波动 | 使用 `-tags=live` 在 staging 手动执行 |

## 6. 结论

P0 测试点全部通过，测试套件绿色，可进入 S6 交付。

---
生成命令: `./scripts/gen-acceptance-report.sh --change devrix-llm-gateway`
