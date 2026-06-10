# Tasks: Devrix 可观察层增强

**Change ID:** devrix-observability-enhancement
**Demand ID:** DM-20260610-001
**Status:** Draft

---

## Task 1: LLM 日志完整接入

**Change:** 在 PEV Engine 中调用 `AddLLMRequestEvent/ResponseEvent`
**Files:** `contextengine/pev_engine.go`
**Effort:** 2h

| ID | Task | Status | Notes |
|----|------|--------|-------|
| T1.1 | 在 LLM 调用 span 创建后调用 `AddLLMRequestEvent` | TODO | |
| T1.2 | 在 LLM 调用完成后调用 `AddLLMResponseEvent` | TODO | |
| T1.3 | 添加单元测试验证 LLM 日志输出 | TODO | |
| T1.4 | 验证 Jaeger 中可见完整 LLM 请求/响应 | TODO | 手动测试 |

---

## Task 2: PEV 迭代 Span

**Change:** 添加 `context.pev.iteration` 独立 span
**Files:** `contextengine/pev_engine.go`
**Effort:** 1h

| ID | Task | Status | Notes |
|----|------|--------|-------|
| T2.1 | 在 PEV 循环迭代开始处创建独立 span | TODO | |
| T2.2 | 添加 `pev.iteration` 和 `pev.max_iterations` 属性 | TODO | |
| T2.3 | 单元测试验证迭代次数分布 | TODO | |

---

## Task 3: 缺失 Span Operation 补充

**Change:** 补充 9 个缺失的 operation 注册
**Files:** `coverage/registry.go`
**Effort:** 3h

| ID | Task | Status | Notes |
|----|------|--------|-------|
| T3.1 | 新增 `context.compression.run` | TODO | engine.go |
| T3.2 | 新增 `context.longterm.recall` | TODO | engine.go |
| T3.3 | 新增 `context.longterm.store` | TODO | engine.go |
| T3.4 | 新增 `context.plan.generate` | TODO | pev_engine.go |
| T3.5 | 新增 `context.pev.iteration` | TODO | pev_engine.go |
| T3.6 | 新增 `context.pev.synthesis` | TODO | pev_engine.go |
| T3.7 | 新增 `context.milestone.run` | TODO | pev_engine.go |
| T3.8 | 新增 `llm.adapter.stream` | TODO | llmgateway |
| T3.9 | 新增 `adapter.feishu.outbound` | TODO | adapters |
| T3.10 | 单元测试验证所有新 operation | TODO | |

---

## Task 4: 新增 Metrics

**Change:** 注册 3 个新 metrics
**Files:** `observability/bridge.go`, `contextengine/pev_engine.go`
**Effort:** 2h

| ID | Task | Status | Notes |
|----|------|--------|-------|
| T4.1 | 新增 `tool_latency` Histogram | TODO | labels: tool, risk_level |
| T4.2 | 新增 `compression_ratio` Histogram | TODO | |
| T4.3 | 新增 `session_active_count` Gauge | TODO | labels: adapter |
| T4.4 | 在工具执行处注册并使用 metrics | TODO | pev_engine.go |
| T4.5 | 在压缩处注册并使用 metrics | TODO | engine.go |
| T4.6 | 单元测试验证 metrics 输出 | TODO | |

---

## Task 5: Baggage 增强

**Change:** 在 ContextEngine 中传递关键业务上下文
**Files:** `contextengine/engine.go`
**Effort:** 1h

| ID | Task | Status | Notes |
|----|------|--------|-------|
| T5.1 | 在 Process 开始时设置 session_id baggage | TODO | |
| T5.2 | 在 Process 开始时设置 model baggage | TODO | |
| T5.3 | 在 LLM span 中记录 baggage 属性 | TODO | |
| T5.4 | 单元测试验证 baggage 传递 | TODO | |

---

## Task 6: 覆盖率验证

**Change:** 验证埋点覆盖率 ≥80%
**Effort:** 1h

| ID | Task | Status | Notes |
|----|------|--------|-------|
| T6.1 | 运行 coverage 测试统计当前覆盖率 | TODO | |
| T6.2 | 对比目标与现状差距 | TODO | |
| T6.3 | 确认覆盖率 ≥80% | TODO | |

---

## Task 7: 文档更新

**Change:** 更新设计文档
**Files:** `docs/observability-design.md`
**Effort:** 1h

| ID | Task | Status | Notes |
|----|------|--------|-------|
| T7.1 | 更新架构图 | TODO | |
| T7.2 | 更新覆盖清单 | TODO | |
| T7.3 | 更新使用手册 `docs/coverage.md` | TODO | |

---

## Task Summary

| Priority | Tasks | Effort |
|----------|-------|---------|
| P0 | T1, T2, T3 | 6h |
| P1 | T4, T5 | 3h |
| P2 | T6, T7 | 2h |
| **Total** | | **11h** |
