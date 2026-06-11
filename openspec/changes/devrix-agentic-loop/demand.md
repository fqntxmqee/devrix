---
demand-id: DM-20260611-001
title: D2 上下文引擎 Agentic Loop 深化 — 无限循环 + 流式工具执行 + 多层错误恢复
source: devrix-harness-architecture-audit
priority: P0
status: S1_Proposal
l1-domain: context-engine
created: 2026-06-11
---

# D2 上下文引擎 Agentic Loop 深化

## 1. 背景

当前 PEV 引擎使用固定次数循环（`MaxIterations=3`），没有真正的 Agentic Loop 能力。Claude Code Harness 的 `queryLoop()` 使用 while-true 循环，每次迭代由 `tool_use` 出现自动驱动继续，并内置 5 种压缩策略和 StreamingToolExecutor 流式并发执行。当前 PEV 在这三个维度上均有代差。

## 2. 问题陈述

### 2.1 PEV 循环深度不足

| 问题 | 影响 | 证据 |
|------|------|------|
| PEV 硬编码 3 次迭代 | 复杂任务需要数十轮工具调用时强制截断 | `pev_engine.go` `MaxIterations: 3` |
| 无动态 `needsFollowUp` 信号 | 无法根据 `tool_use` 出现自动判断是否继续 | 循环不由工具调用驱动 |
| 无无线 continuation | 不存在 while-true 模式 | 固定 for 循环 |

### 2.2 流式工具执行缺失

| 问题 | 影响 |
|------|------|
| 工具调用串行：全部收集 → 逐一执行 → 汇总返回 | LLM 流式完成前无法并发执行 |
| 无 StreamingToolExecutor | 工具调用总延迟 = 模型输出时间 + 工具总执行时间 |
| tool_call/tool_result 配对脆弱 | 手动 JSON 序列化 + `time.Now().UnixNano()` 作 ID |

### 2.3 错误恢复机制空白

| 场景 | 当前行为 | 应有行为 |
|------|----------|----------|
| Context 超限 (413) | 直接失败 | Collapse Drain → Reactive Compact |
| max_output_tokens | 无感知 | 64k 扩容 → 3 次 recovery message |
| 模型超载 | 无 fallback | 自动切换 fallback 模型 |
| 中断保护 | 无 | 孤儿 tool_use → tool_result 补偿 |

### 2.4 Claude Code 对照

Claude Code 的 `query.ts:queryLoop()` 是 while-true 异步生成器，核心差异体现在三个层面：

| 维度 | Claude Code | Devrix PEV |
|------|-------------|-----------|
| 循环驱动 | tool_use 出现自动继续；无 tool_use 时自然结束 | 硬编码 3 次，不感知 tool_use |
| 工具执行 | StreamingToolExecutor 流式执行：LLM 仍在输出时已完成工具调用并注入结果 | 串行收集→执行→汇总，LLM 等待期间空转 |
| 压缩策略 | 多策略流水线（microCompact→contextCollapse→autoCompact→reactiveCompact→snipCompact），413 时依次触发 collapse drain → reactive compact → 重试 | 无恢复路径，413 直接失败 |
| 输出限制 | 64k 扩容 + 3 次 recovery message 渐进恢复 | 无感知 |
| 模型容错 | API 超载时自动切换 fallback 模型 | 无 fallback |

**关键设计理念**：Claude Code 将 loop、工具执行、压缩、错误恢复视为一个统一问题的不同方面，统一在 `queryLoop()` 中解决。Devrix 将这些拆分为 PEVEngine、ToolMessages、CompressionPipeline 等独立组件且互不感知，导致复杂任务场景下出现裂隙。

## 3. 验收标准

### P0 (阻止合并)

- [ ] PEV 引擎支持无限 while-true 循环，由 `tool_use` 出现自动驱动继续
- [ ] 实现流式工具执行：LLM 仍在输出时并行执行已完成的 tool_use
- [ ] tool_call ↔ tool_result 使用强类型 ID 映射，禁止手动 JSON 序列化
- [ ] 实现 Prompt Too Long (413) 恢复：压缩降级 → 重试

### P1 (必须完成)

- [ ] 实现 max_output_tokens 自动恢复（扩容 + recovery message）
- [ ] 实现 fallback 模型切换（API 超载时自动降级）
- [ ] 中断保护：中断时自动生成孤儿 tool_use 的 tool_result
- [ ] max_turns 硬限制兜底，防止无限循环
- [ ] 实现 Agentic Loop 评测探针（`LoopDepthProbe`）：对比新旧循环不同 max_turns 下的任务完成率，注册到 D6 Eval 框架
- [ ] 实现流式工具执行评测探针（`ToolConcurrencyProbe`）：测量并对比 StreamingToolExecutor 与串行执行的端到端延迟，注册到 D6 Eval 框架

### P2 (建议完成)

- [ ] Reactive Compact：按需摘要压缩后重试
- [ ] Agentic Loop 运行时指标：迭代次数、工具并发数、各类型恢复事件（413 / max_tokens / fallback / 中断）计数，对接 D5 可观察层

## 4. 领域映射

| 子域 | 影响范围 | 预期工作量 |
|------|----------|-----------|
| `contextengine/pev` | PEVEngine 重写 | 高 |
| `contextengine/contracts` | ILLMGateway 扩展 | 中 |
| `contextengine/tool_messages` | tool_call 序列化重写 | 中 |
| `contextengine/compression` | 响应式压缩 | 低 |
| `shared/contracts` | EngineEvent 扩展 | 低 |
| `observability/metrics` | Agentic Loop 运行时指标 | 低 |
| `d6/eval` | LoopDepth / ToolConcurrency 探针 | 低 |

## 5. 回归风险

- 现有 PEV 固定循环行为需保持兼容或提供迁移路径
- 流式工具执行可能引入并发竞争，需 `-race` 测试门禁
- 无限循环必须受 `max_turns` 硬限制保护
