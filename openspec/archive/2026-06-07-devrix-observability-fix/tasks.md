# Tasks: devrix-observability-fix

**Change ID:** devrix-observability-fix
**Parent:** devrix-observability
**Status:** S7 Archived
**Based on:** design.md, `openspec/changes/devrix-observability-fix/specs/observability/spec.md`

---

## Milestone 1: Critical Fix（P0 阻断）

### Definition of Done
- [x] Gauge 操作数值正确
- [x] AsyncGauge / AsyncCounter 回调并发安全
- [x] Histogram Prometheus 输出正确
- [x] Shutdown 刷写所有 pending spans

### Tasks

- [x] **T1**: 修复 `metrics/gauge.go` float64 原子转换
  - 使用 `math.Float64bits` / `math.Float64frombits`
  - `Add`/`Sub` 改为 `sync.Mutex` 保护
  - L5: L5-OBS-FIX-01
  - Estimate: 2h
  - Dependencies: None

- [x] **T2**: 修复 `metrics/histogram.go` Observe 逻辑
  - `Observe` 中 `break` 只递增第一个匹配桶
  - `prometheus.go` 中 `+Inf` 桶单独使用 `h.Count()`
  - 新增 golden 样本测试
  - L5: L5-OBS-FIX-02
  - Estimate: 3h
  - Dependencies: None

- [x] **T3**: 修复 `tracer/tracer.go` Shutdown 刷写
  - `TracerProvider` 持有 `[]*Tracer` 引用
  - `Shutdown` 遍历 activeSpans → End → exporter.Shutdown
  - L5: L5-OBS-FIX-03
  - Estimate: 3h
  - Dependencies: None

- [x] **T1b**: 修复 `metrics/async_gauge.go` + `async_counter.go` 回调竞态
  - 为 AsyncGauge 和 AsyncCounter 添加 `sync.Mutex` 保护回调执行
  - Register/Unregister/Observe 全部串行化
  - L5: L5-OBS-CONCUR-01
  - Estimate: 1.5h
  - Dependencies: T1 (Mutex 模式先行)

---

## Milestone 2: High Fix（P1 重要）

### Definition of Done
- [x] Int64UpDownCounter 返回 Gauge
- [x] Shutdown 覆盖全组件
- [x] 死代码清理
- [x] 日志采样生效
- [x] Registry.Reset 覆盖全类型
- [x] Handler.Close 接口完整

### Tasks

- [x] **T4**: 修复 `metrics/meter.go` Int64UpDownCounter
  - 改为创建 Gauge 而非 Counter
  - Registry.RegisterGauge 调用
  - `bridge.go` SessionBridge.ActiveSessions 返回 Gauge
  - L5: L5-OBS-FIX-05
  - Estimate: 2h
  - Dependencies: T1 (Gauge 修复先行)

- [x] **T5**: 修复 `observability.go` Shutdown 全组件
  - 增加 Logger 关闭逻辑
  - `logger/logger.go` 增加 `Close()` 方法
  - L5: L5-OBS-FIX-04
  - Estimate: 2h
  - Dependencies: T3

- [x] **T6**: 删除 `metrics/counter.go` 中未使用的 `sync.Mutex`
  - 仅删除字段，不改变任何逻辑
  - Estimate: 0.5h
  - Dependencies: None

- [x] **T7**: 实现 `logger/logger.go` 日志采样
  - 新增 `spanLogTracker` 内部结构
  - `buildEntry` 集成采样检查
  - 首条丢弃日志发出 WARN
  - L5: L5-OBS-FIX-07
  - Estimate: 3h
  - Dependencies: None

- [x] **T8**: 删除 `exporter/null.go` ExportBatch 死代码
  - 删除不在 SpanExporter 接口中的方法
  - Estimate: 0.5h
  - Dependencies: None

- [x] **T6b**: 修复 `metrics/registry.go` Reset 全类型
  - `Reset()` 当前仅重置 Counter，增加 Gauge.Set(0) 和 Histogram.Reset()
  - Gauge / Histogram 接口新增 `Reset()` 方法
  - L5: L5-OBS-FIX-09
  - Estimate: 1.5h
  - Dependencies: T1 (Gauge 接口)

- [x] **T6c**: 扩展 `logger/logger.go` Handler 接口 `Close()`
  - Handler 接口新增 `Close() error`
  - JSONHandler / TextHandler 实现为 `return nil`
  - `StructuredLogger.Close()` 委托给 Handler
  - L5: L5-OBS-FIX-10
  - Estimate: 1h
  - Dependencies: T5 (Shutdown 全组件)

---

## Milestone 3: Medium Fix（P2 改进）

### Definition of Done
- [x] 错误日志包含堆栈
- [x] Registry 使用 sort.Strings
- [x] ConsoleExporter 直接实现接口

### Tasks

- [x] **T9**: 错误日志增加堆栈跟踪
  - `buildEntry` 中 error value 追加 `debug.Stack()`
  - L5: L5-OBS-FIX-06
  - Estimate: 1h
  - Dependencies: None

- [x] **T10**: `metrics/registry.go` sortedKeys 替换为 `sort.Strings`
  - 删除冒泡排序，import "sort"
  - Estimate: 0.5h
  - Dependencies: None

- [x] **T11**: `exporter/console.go` Export 签名匹配 SpanExporter
  - `Export(s)` → `Export(ctx, s)`
  - 删除 `factory.go` 中 `consoleExporterAdapter`
  - L5: L5-OBS-FIX-08
  - Estimate: 1h
  - Dependencies: None

- [x] **T12**: Logger 解耦 tracer 包依赖
  - `WithTrace(sc tracer.SpanContext)` → `WithTrace(traceID, spanID string)`
  - 移除 `logger/logger.go` 中 `import tracer`
  - Estimate: 1h
  - Dependencies: None

---

## Milestone 4: Test Enhancement

### Definition of Done
- [x] 8 个新 L5 测试点全部 IMPLEMENTED
- [x] 现有测试断言增强
- [x] Benchmark 补充

### Tasks

- [x] **T13**: 编写 Critical Fix 单元测试
  - `gauge_test.go`: Set/Add/Sub/Inc/Dec 精确验证
  - `async_gauge_test.go`: 回调并发安全验证
  - `histogram_test.go`: golden 样本对比 + Buckets() +Inf 不变式
  - `tracer_test.go`: Shutdown 刷写 + double-check 竞态模拟
  - L5: L5-OBS-FIX-01, -02, -03, L5-OBS-CONCUR-01
  - Estimate: 4h
  - Dependencies: T1, T1b, T2, T3

- [x] **T14**: 编写 High/Medium Fix 测试
  - `meter_test.go`: Int64UpDownCounter 返回 Gauge
  - `logger_test.go`: 堆栈跟踪 + 采样 + Close + capacity eviction
  - `console_test.go`: 接口匹配
  - `registry_test.go`: Reset 全类型
  - L5: L5-OBS-FIX-04, -05, -06, -07, -08, -09, -10
  - Estimate: 3h
  - Dependencies: T4-T12, T6b, T6c

- [x] **T15**: 强化现有测试断言
  - `logger/logger_test.go`: 增加实际输出验证
  - `exporter/exporter_test.go`: 增加内容验证
  - `bench_test.go`: 增加 Gauge benchmark
  - Estimate: 1.5h
  - Dependencies: T13, T14

---

## 任务统计

| Milestone | 任务数 | 预估 |
|-----------|--------|------|
| M1 Critical Fix | 4 | 9.5h |
| M2 High Fix | 7 | 10.5h |
| M3 Medium Fix | 4 | 3.5h |
| M4 Test Enhancement | 3 | 8.5h |
| **合计** | **18** | **~32h** |

---

## 依赖关系图

```
T1 ────── T1b ───── T4 ──────┐
                              ├── T13 ── T15
T2 ──────────────────────────┤
                              │
T3 ── T5 ── T6c ─────────────┤
                              │
T7 ──────────────────────────┤
                              │
T9 ──────────────────────────┤
                              │
T10 ─────────────────────────┼── T14 ── T15
T11 ─────────────────────────┤
T12 ─────────────────────────┤
T1 ────── T6b ───────────────┘

T6, T8 (独立，无依赖)
```

## 执行顺序建议

1. **并行**: T1, T2, T3, T6, T7, T8, T9, T10, T11, T12 （Critical/High/Medium 无代码依赖的项）
2. **串行**: T1b → (等待 T1 完成，Gauge Mutex 模式)
3. **串行**: T4 → (等待 T1 完成)
4. **串形**: T5 → T6c → (等待 T3 完成)
5. **串行**: T6b → (等待 T1 完成)
6. **验证**: T13, T14 → (等待所有修复完成)
7. **最终**: T15 → (等待 T13, T14)
