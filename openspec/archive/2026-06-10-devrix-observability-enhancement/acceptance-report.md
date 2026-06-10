# Acceptance Report: Devrix 可观察层增强 — AI 排查就绪

**Change ID:** devrix-observability-enhancement
**Demand ID:** DM-20260610-001
**Status:** S7 Archived — P0 ACCEPTED (2026-06-10); P1 deferred
**Review Round:** 1（Cursor Agent）→ 待其他模型 Round 2

---

## 验收哲学（修订）

> 原验收标准以「埋点存在 + 覆盖率 ≥80%」为主。修订后以 **AI 排查就绪** 为主：
> 1. Span **层级契约**（因果链可推理）
> 2. 多源信号 **可关联**（log / trace / LLM JSONL）
> 3. **机器可读** incident export

---

## S0 需求澄清

- [x] `demand.md` 已创建（2026-06-10）
- [ ] proposal.md 二次 review
- [ ] design.md 二次 review
- [ ] tasks.md 二次 review

---

## S1 开发完成

### 1.1 代码基线（已完成，回归验证）

| ID | Criteria | Evidence | Status |
|----|-----------|-----------|---------|
| S1.0.1 | LLM span events 已接入 | `pev_engine.go` AddLLMRequestEvent/ResponseEvent | **DONE** |
| S1.0.2 | PEV iteration span 已创建 | `context.pev.iteration` in code + integration test | **DONE** |
| S1.0.3 | 主链 operation 已注册 | `registry.go` 44 operations | **DONE** |
| S1.0.4 | llm.adapter.stream 已埋点 | `gateway.go:194` | **DONE** |

### 1.2 新增工作（待开发）

| ID | Criteria | Evidence | Status |
|----|-----------|-----------|---------|
| S1.1 | Span ctx 传播修复 | PEV 集成测试 R1-R2 全绿 | **Pass** |
| S1.2 | slog trace_id 注入 | `slog_bridge_test.go` | **Pass** |
| S1.3 | LLM JSONL 含 trace_id | `llm_log_test.go` | **Pass** |
| S1.4 | tool_latency histogram | — | **Deferred (P1)** |
| S1.5 | compression_ratio histogram | — | **Deferred (P1)** |
| S1.6 | verify.failure_reason 属性 | `pev_engine.go` | **Pass** |
| S1.7 | incident export CLI | — | **Deferred (P1)** |

---

## S2 测试完成

| ID | Criteria | Evidence | Status |
|----|-----------|-----------|---------|
| S2.1 | Span 层级集成测试 | `TestIntegration_PEVSpanHierarchy` | **Pass** |
| S2.2 | 多轮 PEV 层级测试 | `TestIntegration_D2MultiRoundPEV` | **Pass** |
| S2.3 | Log-Trace 关联测试 | `slog_bridge_test.go` | **Pass** |
| S2.4 | Metrics 测试 | — | **Deferred (P1)** |
| S2.5 | Incident export 测试 | — | **Deferred (P1)** |
| S2.6 | 单元测试无回归 | CI unit + integration | **Pass** |

---

## S3 质量门禁

| ID | Criteria | Evidence | Status |
|----|-----------|-----------|---------|
| S3.1 | Hierarchy coverage R1-R2 | integration test | **Pass** |
| S3.2 | 编译通过 | CI build | **Pass** |
| S3.3 | lint 通过 | CI vet | **Pass** |
| S3.4 | 文档已更新 | spec v1.5 + l5-registry | **Pass** |
| ~~S3.5~~ | ~~Runtime coverage ≥80%~~ | 降为 P2 分 Layer 统计 | N/A |

---

## S4 交付确认（AI 排查就绪）

| ID | Criteria | Evidence | Status |
|----|-----------|-----------|---------|
| S4.1 | L1 辅助：Jaeger 可按轮次展开 PEV | R1-R2 hierarchy test | **Pass** |
| S4.2 | L2 基本：session export 含完整 trace 树 | — | **Deferred (P1)** |
| S4.3 | LLM 请求可通过 trace_id 关联 JSONL | llm_log_test | **Pass** |
| S4.4 | verify 失败可回答「为什么」 | verify.failure_reason | **Pass** |
| S4.5 | tool P99 可从 metrics 查询 | — | **Deferred (P1)** |

---

## L5 测试用例（Gherkin）

### T1: Span 层级契约 — L5-OBS-TRACE-04

```gherkin
Scenario: PEV 单轮迭代的 span 层级正确
  Given PEV Engine 配置 maxIterations=1 且触发 1 次 LLM 调用
  When 用户消息走完 D1-D3 全链
  Then context.pev.llm_call 的 parent MUST 为 context.pev.iteration
  And llm.stream 的 parent MUST 为 context.pev.llm_call
  And context.pev.iteration 的 duration MUST 小于 context.pev.run 的 duration
```

### T2: 多轮 PEV 迭代生命周期 — L5-OBS-TRACE-02/04

```gherkin
Scenario: 3 轮 PEV 迭代 span 不重叠
  Given PEV maxIterations=3 且 verify 前 2 轮失败
  When 采集 spans
  Then 应有 3 个 context.pev.iteration span
  And 各 iteration span 的 start/end 时间不重叠
  And 每个 iteration 下各有 1 个 context.pev.llm_call 子 span
```

### T3: Log-Trace 关联 — L5-OBS-TRACE-05

```gherkin
Scenario: 业务 slog 含 trace_id
  Given observability.logging.include_trace_id=true
  When context engine 处理消息并输出 slog
  Then slog JSON 应含 traceId 字段
  And traceId 与 Jaeger trace_id 一致
```

### T4: LLM JSONL 关联 — L5-OBS-TRACE-05

```gherkin
Scenario: LLM JSONL 可关联 trace
  Given observability.llm.log_content=true
  When PEV 完成 1 轮 LLM 调用
  Then ~/.devrix/logs/llm/{session}.jsonl 记录含 trace_id
  And trace_id 与 Jaeger 一致
```

### T5: 工具延迟 Metrics — L5-OBS-METRICS-01

```gherkin
Scenario: 工具执行延迟被 histogram 记录
  Given 用户消息触发 bash 工具调用
  When 工具执行完成
  Then Prometheus 中 tool_latency{tool="bash"} 有观测值
```

### T6: Verify 决策语义 — L5-OBS-DECISION-01

```gherkin
Scenario: Verify 失败时 span 含可读原因
  Given verify.mode=commands 且 tool 输出为空
  When verify 未通过
  Then context.pev.verify span 含 verify.failure_reason 属性
  And verify.passed=false
```

### T7: Session Incident Export — L5-OBS-EXPORT-01

```gherkin
Scenario: 给定 session_id 导出 AI 可读 bundle
  Given 已完成一次完整用户对话
  When 执行 devrix debug export --session {id} --format json
  Then 输出 JSON 含 trace.spans 树形结构
  And 含 llm_rounds 数组（与 JSONL 一致）
  And schema_version=1.0
```

---

## 度量数据

| Metric | Target | Actual | Status |
|--------|--------|---------|--------|
| Hierarchy rules (R1-R2) pass rate | 100% | 100% | **Pass** |
| Log-Trace 关联率 | 100% error path | slog + JSONL tests | **Pass** |
| LLM JSONL trace_id 覆盖率 | 100% | unit test | **Pass** |
| tool_latency 注册率 | 100% builtin tools | — | **P1** |
| Incident export 可用性 | CLI 可运行 | — | **P1** |
| ~~Runtime coverage~~ | 分 Layer ≥60% | TBD | P2 |

---

## Review 结论摘要（Round 1）

**是否满足 AI 未来排查？**

| 档位 | 结论 |
|------|------|
| 现在 | **部分满足** — 有原料，因果链/关联/export 不足 |
| 本 change P0 完成后 | **L1 辅助达标** — 因果链可折叠、log/JSONL 可关联 |
| L2 基本满足 | **Deferred** — 需 P1 incident export + metrics |
| L3 自主闭环 | **不满足** — 需 Agent trace 统一 + error-biased sampling |

---

## 遗留问题

| ID | Issue | Severity | Owner | Target |
|----|-------|----------|--------|--------|
| I1 | Agent 路径 trace 与主链不一致 | P1 | TBD | 独立 change |
| I2 | Tool I/O 仅 preview 500 字符 | P2 | TBD | export 读 JSONL 缓解 |
| I3 | LLM 全量 log_content 生产安全风险 | P1 | TBD | redaction 策略 |

---

## Verdict

**P0 ACCEPTED** — PR [#11](https://github.com/fqntxmqee/devrix/pull/11) merged 2026-06-10.  
**P1 遗留**（metrics / incident export / SpanKind 全量）建议独立 change `devrix-observability-enhancement-p1` 跟进。
