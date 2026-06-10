# Acceptance Report: Devrix 可观察层增强

**Change ID:** devrix-observability-enhancement
**Demand ID:** DM-20260610-001
**Status:** Draft
**Date:** TBD

---

## 验收标准

### S0 需求澄清

- [ ] proposal.md 已评审
- [ ] design.md 已评审
- [ ] tasks.md 已评审

### S1 开发完成

| ID | Criteria | Evidence | Status |
|----|-----------|-----------|---------|
| S1.1 | LLM 日志接入完成 | `AddLLMRequestEvent` 在 PEV Engine 中被调用 | TODO |
| S1.2 | PEV 迭代 Span 完成 | `context.pev.iteration` span 被创建 | TODO |
| S1.3 | 9 个新 Operation 注册完成 | `registry.go` 包含新 operation | TODO |
| S1.4 | 3 个新 Metrics 注册完成 | metrics registry 包含新指标 | TODO |
| S1.5 | Baggage 增强完成 | `session_id` 在 span attributes 中 | TODO |

### S2 测试完成

| ID | Criteria | Evidence | Status |
|----|-----------|-----------|---------|
| S2.1 | 单元测试通过 | `go test ./internal/layers/observability/...` 通过 | TODO |
| S2.2 | LLM 日志测试通过 | `TestLLMLogger_*` 通过 | TODO |
| S2.3 | Metrics 测试通过 | `TestMetrics_*` 通过 | TODO |
| S2.4 | Baggage 测试通过 | `TestBaggage_*` 通过 | TODO |

### S3 质量门禁

| ID | Criteria | Evidence | Status |
|----|-----------|-----------|---------|
| S3.1 | 覆盖率 ≥80% | `devrix-coverage --trend 7` 输出 | TODO |
| S3.2 | 编译通过 | `go build ./...` 无错误 | TODO |
| S3.3 | lint 通过 | `golangci-lint run` 无 error | TODO |
| S3.4 | 文档已更新 | `docs/coverage.md` 已更新 | TODO |

### S4 交付确认

| ID | Criteria | Evidence | Status |
|----|-----------|-----------|---------|
| S4.1 | LLM 请求可见 | Jaeger 中可见 `llm.request` event | TODO |
| S4.2 | LLM 响应可见 | Jaeger 中可见 `llm.response` event | TODO |
| S4.3 | 迭代次数可分析 | `context.pev.iteration` span 可统计 | TODO |
| S4.4 | 工具延迟可分析 | Prometheus 中可见 `tool_latency` histogram | TODO |

---

## 测试用例

### T1: LLM 日志完整性

```gherkin
Scenario: LLM 请求和响应应该在 Jaeger 中可见
  Given PEV Engine 正在处理用户消息
  When LLM 被调用
  Then span 中应包含 `llm.request` event
  And span 中应包含 `llm.response` event
  And `llm.request_json` 包含完整 messages
```

### T2: PEV 迭代可见性

```gherkin
Scenario: PEV 每次迭代应该有独立 span
  Given PEV Engine 配置 maxIterations=3
  When 用户消息触发 PEV 循环
  Then 应创建 3 个 `context.pev.iteration` span
  And 每个 span 包含 iteration 属性
```

### T3: 工具延迟可度量

```gherkin
Scenario: 工具执行延迟应该被记录
  Given 用户消息触发工具调用
  When 工具执行完成
  Then `tool_latency` histogram 应记录延迟
  And labels 应包含 tool_name 和 risk_level
```

### T4: 覆盖率统计

```gherkin
Scenario: 埋点覆盖率应 ≥80%
  Given Devrix 运行一段时间
  When 查看 coverage 报告
  Then hit operations / total operations ≥ 0.8
```

---

## 度量数据

| Metric | Target | Actual | Status |
|--------|--------|---------|--------|
| 埋点覆盖率 | ≥80% | TBD | TODO |
| 单元测试覆盖率 | ≥70% | TBD | TODO |
| LLM 日志完整率 | 100% | TBD | TODO |
| Metrics 注册率 | 100% | TBD | TODO |

---

## 遗留问题

| ID | Issue | Severity | Owner | Target |
|----|-------|----------|--------|--------|
| - | - | - | - | - |

---

## 签收

| Role | Name | Date | Signature |
|------|------|------|-----------|
| Tech Lead | | | |
| QA | | | |
| PM | | | |
