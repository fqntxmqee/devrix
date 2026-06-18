# Tasks: QueryLoop Span 对齐 v1.1

**Change ID:** devrix-queryloop-spans-v1.1
**Demand ID:** DM-20260612-014

> **归档说明 (2026-06-18):** 变更在 S1 阶段取消，无任务启动。

## S0 — Demand 创建（已完成）

| ID | 任务 | 状态 | 日期 |
|----|------|------|------|
| D01 | 创建 demand.md | ✅ DONE | 2026-06-12 |
| D02 | 创建 proposal.md 草案 | ✅ DONE (cancelled) | 2026-06-12 |

## 未启动任务（明确取消）

以下任务**不实施**，归档时标注为 CANCELLED：

### iteration span

| ID | 任务 | 状态 |
|----|------|------|
| T01 | `runViaQueryLoop` 添加 iteration span 包裹 | ❌ CANCELLED |
| T02 | iteration span attributes 填充（index/has_tool_call） | ❌ CANCELLED |

### llm_call span

| ID | 任务 | 状态 |
|----|------|------|
| T03 | `iterate` 内 LLM 调用添加 llm_call span | ❌ CANCELLED |
| T04 | llm_call span attributes 填充（model/tokens） | ❌ CANCELLED |

### 测试与验证

| ID | 任务 | 状态 |
|----|------|------|
| T05 | 单元测试：span 嵌套结构正确 | ❌ CANCELLED |
| T06 | 集成测试：Jaeger trace 验证 | ❌ CANCELLED |
| T07 | 端到端测试：多迭代链路追踪 | ❌ CANCELLED |

## 取消原因

1. 6 天（2026-06-12 → 2026-06-18）未推进
2. 实际痛点不明确（iteration/llm_call 延迟可从 LLM 日志推断）
3. 资源优先级 → 让位给 devrix-tracing / devrix-eval

## 归档

**Status:** S7_Archived (2026-06-18)
**Verdict:** S1_Cancelled → Archived；7 个 T 点全部 CANCELLED。