# 代码染色 (Code Coverage)

实时跟踪运行时 Span 调用，识别从未被触发过的 Operation（潜在死代码或未启用功能）。

**Registry SoT:** `internal/layers/observability/diagnose/coverage/registry.go`（当前 **56** 条 Operation）
**常量 SoT:** `internal/layers/observability/instrument/telemetry/names.go`

> **2026-06-18 变更:** `query.loop.*` 族已退役（DM-20260618-010），主路径为 D7 Turn（`orchestration.turn.*`）。
> **2026-06-13 变更:** `context.pev.*` 族已移除；新增 `query.loop.*`、`tool.execute.*`、`task.*`、`orchestration.*`。

---

## 工作原理

```
Tracer.Start(operation)
  → coverage.RecordHit(operation)   # 无条件，不受采样影响
  → coverage.IsKnown(operation)     # 未知则 WARN
  → 原子计数累积到进程内 Counter
  → CoverageReporter 定期持久化到 ~/.devrix/coverage/
  → HealthCheck() 暴露 coverage 摘要
```

---

## 操作注册表

所有 Span 操作 MUST 在 `diagnose/coverage/registry.go` 的 `AllOperations()` 中注册，并与 `instrument/telemetry/names.go` `Op*` 常量全集一致（`registry_test.go` 强制对账）。

### 层级结构

| Layer | Components | 示例 Operation |
|-------|------------|----------------|
| `communication` | `gateway`, `adapter` | `gateway.message.receive`, `adapter.feishu.outbound` |
| `context` | `context_engine`, `harness`, `tool_runner`, `plan_agent`, `plan_mode`, `task_manager` | `context.process` |
| `llm` | `llm_gateway`, `llm_adapter` | `llm.stream`, `llm.adapter.stream` |
| `agent` | `agent_tool` | `agent.run`, `agent.tool.call` |
| `orchestration` | `orchestrator` | `orchestration.turn.run` |

### 命名规范

```
{layer}.{module}.{action}
```

| Operation | 说明 |
|-----------|------|
| `orchestration.turn.run` | D7 Turn 主路径入口（现行主路径） |
| `orchestration.turn.iteration` | D7 Turn 迭代 |
| `orchestration.llm.invoke` | D7 LLM 调用 |
| `context.process` | 上下文处理主入口（caller=d7） |
| `tool.execute.single` | 单工具执行 |
| `llm.stream` | LLM 流式调用 |

### 已退役 Operation（不再注册）

- `context.pev.run`, `context.pev.iteration`, `context.pev.llm_call`, `context.pev.tool_execute`, `context.pev.verify`
- `context.plan.generate`, `context.milestone.run`（旧 PEV plan，由 `task.plan.*` 替代）
- `query.loop.run`, `query.loop.turn`, `query.loop.llm.call`（旧 QueryLoop，由 `orchestration.turn.*` 替代，DM-20260618-010）

---

## 新增 Operation 流程

1. 在 `instrument/telemetry/names.go` 添加 `Op*` 常量
2. 在 `diagnose/coverage/registry.go` `AllOperations()` 添加 `OperationMeta`
3. 在 `diagnose/coverage/registry_test.go` expected 列表追加
4. 在业务代码 `tracer.Start(ctx, telemetry.OpXxx, ...)` 创建 span
5. 运行 `go test ./internal/layers/observability/diagnose/coverage/...`

```go
{Name: "context.my_operation", Layer: "context", Component: "context_engine", SinceVersion: "2.2.0", Instrumented: true}
```

---

## 命令行工具

```bash
# 构建
go build -o bin/devrix-coverage ./cmd/coverage

# 列出报表
devrix-coverage --list

# 查看指定日期
devrix-coverage --date 2026-06-14 --summary

# 7 天趋势
devrix-coverage --trend 7
```

---

## 输出示例

### 简洁统计

```
========== Coverage Summary: 2026-06-14 ==========
Total: 28.6% (16/56 operations)

context         |██████░░░░░░░░░░░░░░|  32.1% (9/28)
communication   |████░░░░░░░░░░░░░░░░|  21.4% (3/14)
llm             |███░░░░░░░░░░░░░░░░░|  20.0% (1/5)
agent           |░░░░░░░░░░░░░░░░░░░░|   0.0% (0/6)
orchestration   |░░░░░░░░░░░░░░░░░░░░|   0.0% (0/3)
```

### 符号说明

| 符号 | 含义 |
|------|------|
| `●` | 已命中 |
| `○` | 零命中（可能是条件分支或未启用功能） |

---

## 配置文件

```yaml
observability:
  coverage:
    enabled: true
    dir: "~/.devrix/coverage"
    interval: 1h
```

报表存储：`~/.devrix/coverage/coverage_YYYY-MM-DD.json`

```json
{
  "date": "2026-06-14",
  "operations_total": 56,
  "operations_hit": 16,
  "operations_zero": 40,
  "coverage_ratio": 0.286,
  "zero_hit_operations": [
    {"operation": "orchestration.wave.schedule", "layer": "orchestration", "component": "orchestrator", "since_version": "2.1.0"}
  ],
  "hits": {
    "context.process": 42,
    "orchestration.turn.run": 38,
    "orchestration.llm.invoke": 35,
    "llm.stream": 35
  }
}
```

---

## 按 Layer 验收

| Layer | 典型 integration 测试 | build tag |
|-------|------------------------|-----------|
| `communication` | `obs_trace_propagation_test.go` | `integration && d5` |
| `context` | `context_harness_obs_test.go` | `integration && d2` |
| `llm` | D3 integration | `integration && d3` |
| `agent` | D4 integration | `integration && d4` |
| `orchestration` | D7 Turn span tests | `integration && d7` |
| cross | trace 层级 + SpanKind | `integration && cross` |

**注意:** 条件 Operation 在未触发时 zero-hit 是正常现象，不代表死代码。常见条件触发：

| Operation | 触发条件 |
|-----------|----------|
| `context.compression.run` | token 超 CompressionTarget |
| `context.harness.*` | `harness.enabled=true`（legacy 路径） |
| `context.longterm.*` | `longterm.enabled` |
| `task.plan_mode.*` | plan mode 激活 |
| `orchestration.wave.*` | Wave Scheduler 启用 |
| `agent.*` | Multi-Agent 路径 |

---

## 多维指标（Coverage 成功指标双轨）

> **v2.1 Terminal：** D5 成功指标为双轨制 — 过程指标 + 验证指标。防止 Goodhart's Law（单一覆盖率 KPI 被博弈）。

| 维度 | 指标 | 说明 |
|------|------|------|
| `ratio` | 命中率 | hit/total（传统覆盖率） |
| `completeness` | 完整性 | Registry 与 names.go 是否一致 |
| `link_integrity` | 链路完整性 | Trace 树中 parent-child 关系是否完整 |
| `recency` | 时效性 | 最近一次覆盖率报告的新鲜度 |

---

## 与 T 层测试点

| T 层 | 验证方式 |
|------|----------|
| D5-S23-A01-T01 | `registry_test.go` — Registry ≡ names.go |
| D5-S23-A01-T02~T04 | `coverage_test.go` — zero_hit / 并发 / 采样独立 |
| D5-S23-A01-T05 | `context_harness_obs_test.go` — harness span 树 |
| D5-S22-A01-T03 | `obs_trace_propagation_test.go` — trace_id 继承 |

---

## API 接口

### HealthCheck

```bash
curl http://localhost:8080/health
```

```json
{
  "status": "healthy",
  "coverage": {
    "operations_total": 56,
    "operations_hit": 16,
    "coverage_ratio": 0.286,
    "zero_hit_count": 40
  }
}
```

### 编程接口

```go
report := obs.CoverageReport(true)
daily, err := obs.GenerateCoverageReport()
trend, err := obs.CoverageReporter().GetTrend(7)
```

---

## 最佳实践

1. **新增 Span 时同步注册** — 三处：`names.go` + `registry.go` + `registry_test.go`
2. **定期审查 zero-hit** — 区分「条件未触发」vs「死代码」
3. **版本标记** — `since_version` 记录引入版本
4. **不追求 100% hit** — Coverage 用于发现「完全未触达的层」，非行覆盖率

## 故障排查

| 症状 | 原因 | 解决 |
|------|------|------|
| 报表为空 | `observability.enabled=false` | 检查配置 |
| `unknown operation` WARN | Registry 未登记 | 添加 `OperationMeta` |
| 每次启动计数归零 | 进程内 Counter 设计 | 查看 `~/.devrix/coverage/` 持久化报表 |
