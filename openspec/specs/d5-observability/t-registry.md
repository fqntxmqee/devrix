# D5 Observability Domain — T 层测试点注册表

**Status:** Active
**Version:** 3.5.0
**Last Updated:** 2026-07-02 (devrix-d2-tool-input-aware-concurrency-and-classifier DM-20260702-009: 1 T IMPLEMENTED 45→46, P0 30→31 — D5-S25-A04-T01 GrowthBook override 1 flag (bash 30K→50K, Production-Safety 单测 PASS))
**Parent:** `openspec/specs/architecture/layering.md`
**Change:** devrix-d5-sa-refine（DM-20260615-001 / v1.0 Canonical 重排；增 canonical_s 列 + Legacy 双轨）+ devrix-diagnostic-tools-parity (DM-20260616-003) + devrix-diagnostic-tools-wiring (DM-20260617-002) + devrix-tools-terminal-architecture (DM-20260618-007) + **devrix-d5-v2-terminal (DM-20260619-006 / v2.1 Terminal；增 canonical_a 列；canonical_s 校正 A08→S21、A06→S0；canonical_a 校正 Doctor T→A10；3 PLANNED 闭合)** 

> **代码锚点：** 本文件 canonical_s/canonical_a 列校正，是 Phase A 的 ≥1 个代码锚点之一（AC-A8）。

---

## D5-S21: Instrument（遥测生成）

| T ID | 描述 | canonical_s | canonical_a | Priority | Test 位置 | Status | Legacy T ID |
|------|------|-------------|-------------|----------|-----------|--------|-------------|
| D5-S21-A01-T01 | Shutdown 刷写所有 pending spans | S21 | A01 | P0 | `internal/layers/observability/instrument/tracer/tracer_test.go` | IMPLEMENTED | D5-S1-A01-T01 |
| D5-S21-A01-T02 | ConsoleExporter 可直接作为 SpanExporter | S21 | A01 | P2 | `internal/layers/observability/export/console_test.go` | IMPLEMENTED | D5-S1-A01-T02 |
| D5-S21-A01-T04 | TraceID/SpanID 生成符合 W3C 格式 | S21 | A01 | P1 | `internal/layers/observability/instrument/tracer/tracer_test.go` | IMPLEMENTED | D5-S1-A01-T04 |
| D5-S21-A03-T01 | Baggage set/get/list 与 header 往返 | S21 | A03 | P2 | `internal/layers/observability/instrument/tracer/baggage_test.go` | IMPLEMENTED | D5-S1-A03-T01 |
| D5-S21-A03-T02 | Propagator inject/extract traceparent | S21 | A03 | P1 | `internal/layers/observability/instrument/tracer/propagation_test.go` | IMPLEMENTED | D5-S1-A03-T02 |
| D5-S21-A03-T03 | CLI 子进程环境含 TRACEPARENT + BAGGAGE | S21 | A03 | P2 | `internal/layers/observability/instrument/tracer/propagation_env_test.go` | IMPLEMENTED | D5-S1-A03-T03 |
| D5-S21-A05-T01 | Tracing Span 创建与传播（propagation 集成测试） | S21 | A05 | P1 | `tests/integration/obs_trace_propagation_test.go` | IMPLEMENTED | D5-S2-A01-T01 |
| D5-S21-A05-T02 | Metrics Counter 计数（Counter 单元测试） | S21 | A05 | P1 | `internal/layers/observability/instrument/metrics/counter_test.go` | IMPLEMENTED | D5-S2-A01-T02 |
| D5-S21-A05-T03 | Gauge Set/Inc/Dec/Add/Sub 数值正确 | S21 | A05 | P0 | `internal/layers/observability/instrument/metrics/gauge_test.go` | IMPLEMENTED | D5-S2-A01-T03 |
| D5-S21-A05-T04 | Histogram Prometheus 输出与 golden 一致 | S21 | A05 | P0 | `internal/layers/observability/instrument/metrics/histogram_test.go` | IMPLEMENTED | D5-S2-A01-T04 |
| D5-S21-A05-T05 | Int64UpDownCounter 返回 Gauge | S21 | A05 | P0 | `internal/layers/observability/instrument/metrics/meter_test.go` | IMPLEMENTED | D5-S2-A01-T05 |
| D5-S21-A05-T06 | tool_latency histogram 注册与 observe | S21 | A05 | P1 | `internal/layers/observability/bridge_tool_latency_test.go` | IMPLEMENTED | D5-S2-A01-T09 |
| D5-S21-A07-T01 | gen_ai.client.token.usage 含 input/output/cache_read/reasoning | S21 | A07 | P2 | `internal/layers/observability/genai_tokens_test.go` | IMPLEMENTED | D5-S2-A01-T08 |
| D5-S21-A08-T01 | 日志级别过滤 | S21 | A08 | P1 | `internal/layers/observability/instrument/logger/logger_test.go` | IMPLEMENTED | D5-S3-A01-T01 |
| D5-S21-A08-T02 | Shutdown 覆盖 Tracer + Logger | S21 | A08 | P0 | `internal/layers/observability/instrument/logger/logger_test.go` | IMPLEMENTED | D5-S3-A01-T02 |
| D5-S21-A08-T03 | Error 日志包含 stacktrace 字段 | S21 | A08 | P1 | `internal/layers/observability/instrument/logger/logger_test.go` | IMPLEMENTED | D5-S3-A01-T03 |
| D5-S21-A08-T04 | 日志采样 max_entries_per_span 生效 | S21 | A08 | P1 | `internal/layers/observability/instrument/logger/sampling_test.go` | IMPLEMENTED | D5-S3-A01-T04 |
| D5-S21-A08-T05 | 敏感字段脱敏 [REDACTED] | S21 | A08 | P1 | `internal/layers/observability/instrument/logger/logger_test.go` | IMPLEMENTED | D5-S3-A01-T05 |
| D5-S21-A09-T01 | slog 从 context 注入 traceId/spanId | S21 | A09 | P0 | `internal/layers/observability/instrument/logger/slog_bridge_test.go` | IMPLEMENTED | D5-S3-A02-T01 |
| D5-S21-A11-T01 | LayerAndComponent 映射 gateway operation | S21 | A11 | P0 | `internal/layers/observability/instrument/telemetry/names_test.go` | IMPLEMENTED | D5-S6-A01-T01 |
| D5-S21-A12-T01 | SpanAttrs 含 devrix.layer/component | S21 | A12 | P0 | `internal/layers/observability/instrument/telemetry/names_test.go` | IMPLEMENTED | D5-S6-A01-T02 |
| D5-S21-A13-T01 | GenAIUsageAttrs 含 OTel 语义属性 | S21 | A13 | P1 | `internal/layers/observability/instrument/telemetry/names_test.go` | IMPLEMENTED | D5-S6-A03-T01 |
| D5-S21-A13-T02 | GenAIUsageAttrs 含 cache_read/reasoning 细分 | S21 | A13 | P2 | `internal/layers/observability/instrument/telemetry/names_test.go` | IMPLEMENTED | D5-S6-A03-T02 |

## D5-S25: Termination (LTL-Lite L4–L6, DM-20260701-007)

> **devrix-mups-tool-classification-and-channel-autonomy (DM-20260701-007) — Phase B LTL-Lite 落地。**
> Execute 节点 4 ToolChannel (Fact/Action/Probe/Experiment) 各挂 1 个 LTL-Lite L4–L6 termination invariant:
> - **L4 Bounded**: ProbeToolChannel 用 Bounded(n) iteration bound + 3-stage PromptPressure
> - **L5 Quotient**: ExperimentToolChannel 用 convergence/quality quotient threshold
> - **L6 Synthesize**: Action/Fact 都用 min-deliverable-chars 触发 synthesize-now 信号
>
> 与现有 L0–L3 safety invariants (ReadOnly/Destructive/OpenWorld/ConcurrencySafe) 通过 ≥3 条 cross-check 兼容 (P1-AC-2).
>
> 实现位置: `internal/layers/observability/instrument/ltl/invariants/termination/` (新建包, 4 文件 + 1 test 文件).

| T ID | 描述 | canonical_s | canonical_a | Priority | Test 位置 | Status | Legacy T ID |
|------|------|-------------|-------------|----------|-----------|--------|-------------|
| D5-S25-A01-T01 | L4 BoundedInvariant: Check(state) 在 iter ≥ MaxN 时返 (false, "Bounded exceeded iter N/MaxN, inject synthesize-now"); NewBoundedInvariant(MaxN) fail-fast 校验 > 0; Name() 返 "L4-Bounded" | S25 | A01 | P0 | `internal/layers/observability/instrument/ltl/invariants/termination/bounded_test.go::TestBoundedInvariant_{FiresAtBound, RejectsZeroMax, Name, DoesNotBypassPermissionGuards}` (4 tests) | **IMPLEMENTED (DM-20260701-007)** | — |
| D5-S25-A02-T01 | L5 QuotientInvariant: Check(state) 在 metric(state) < Threshold 时返 (false, "Quotient below threshold X/Y, inject synthesize-now"); NewQuotientInvariant(Threshold, Metric) 校验 0 ≤ T ≤ 1; Name() 返 "L5-Quotient" | S25 | A02 | P0 | `quotient_test.go::TestQuotientInvariant_{FiresAtThreshold, BelowThreshold, CustomMetric}` (3 tests) | **IMPLEMENTED (DM-20260701-007)** | — |
| D5-S25-A03-T01 | L6 SynthesizeInvariant: Check(state) 在 len(text) < MinChars 时返 (false, "Synthesize too short X < MinChars, inject synthesize-now"); NewSynthesizeInvariant(MinChars) fail-fast; Name() 返 "L6-Synthesize" | S25 | A03 | P0 | `synthesize_test.go::TestSynthesizeInvariant_{NeverFiresFromCheck, BelowMinChars, ExactlyMinChars}` (3 tests) | **IMPLEMENTED (DM-20260701-007)** | — |
| D5-S25-A04-T01 | GrowthBook override 1 flag (bash 30K→50K, Production-Safety 单测 PASS): `Override.BashReadOnlyThresholdBytes(defaultBytes int) int`; default 30000 → override 50000; seedFeatureFlags 空 map 时返 default (0 行为变化); 运行时变更通过 GrowthBook SDK 推送不需重启; 2 deferred flags (bash readonly canary 5%→50% / classifier 5% canary) 推迟等 P1 实施时再立 | S25 | A04 | P0 | `internal/layers/observability/instrument/growthbook/registry.go::NewGrowthBookOverride + concurrency_override.go::BashReadOnlyThresholdBytes + growthbook_override_test.go (Production-Safety 单测)` | **IMPLEMENTED (DM-20260702-009)** | — |

**D5-S25 (DM-20260701-007) Total: 4 T 全部 P0 IMPLEMENTED**（3 T 既有 + 1 T 新增 A04-T01）。3 invariant 共 10 unit tests, 0 race warnings; coverage 70.2% (新包); 配套 cross-check 测试 `TestBoundedInvariant_DoesNotBypassPermissionGuards` 验证 L4 不得 override L0–L3 readonly/permission guards。

## D5-S22: Export（遥测导出）

| T ID | 描述 | canonical_s | canonical_a | Priority | Test 位置 | Status | Legacy T ID |
|------|------|-------------|-------------|----------|-----------|--------|-------------|
| D5-S22-A01-T01 | OTLP span 事件属性序列化 | S22 | A01 | P1 | `internal/layers/observability/export/otlp_event_test.go` | IMPLEMENTED | D5-S4-A01-T01 |
| D5-S22-A01-T02 | D7 Turn 产生 canonical Operation span | S22 | A01 | P0 | `internal/layers/orchestration/turn/orchestrator_test.go` | IMPLEMENTED | D5-S4-A01-T02 |
| D5-S22-A01-T03 | Adapter→Gateway trace_id 继承 | S22 | A01 | P0 | `tests/integration/obs_trace_propagation_test.go` | IMPLEMENTED | D5-S4-A01-T03 |

## D5-S23: Diagnose（诊断辅助）

| T ID | 描述 | canonical_s | canonical_a | Priority | Test 位置 | Status | Legacy T ID |
|------|------|-------------|-------------|----------|-----------|--------|-------------|
| D5-S23-A01-T01 | Operation Registry 与 names.go 全集一致（56 条） | S23 | A01 | P0 | `internal/layers/observability/diagnose/coverage/registry_test.go` | IMPLEMENTED | D5-S5-A01-T01 |
| D5-S23-A01-T02 | Coverage 报告正确列出 zero_hit operations | S23 | A01 | P0 | `internal/layers/observability/diagnose/coverage/coverage_test.go` | IMPLEMENTED | D5-S5-A01-T02 |
| D5-S23-A01-T03 | 100 并发 RecordHit 计数正确 | S23 | A01 | P0 | `internal/layers/observability/diagnose/coverage/coverage_test.go` | IMPLEMENTED | D5-S5-A01-T03 |
| D5-S23-A01-T04 | 采样关闭仍 RecordHit | S23 | A01 | P0 | `internal/layers/observability/diagnose/coverage/coverage_test.go` | IMPLEMENTED | D5-S5-A01-T04 |
| D5-S23-A01-T05 | Harness enabled 产生 harness span 树 | S23 | A01 | P1 | `tests/integration/context_harness_obs_test.go` | IMPLEMENTED | D5-S5-A01-T05 |
| D5-S23-A02-T01 | 端到端染色集成 | S23 | A02 | P1 | `internal/layers/observability/coverage_integration_test.go` | IMPLEMENTED | D5-S5-A02-T01 |
| D5-S23-A03-T01 | Doctor 报告 healthy（7 项全 pass） | S23 | **A10** ⬅️ | P0 | `internal/layers/observability/diagnose/doctor/doctor_test.go` | IMPLEMENTED | — |
| D5-S23-A03-T02 | Doctor 检出 missing lsp server → StatusFail | S23 | **A10** ⬅️ | P0 | `internal/layers/observability/diagnose/doctor/doctor_test.go` | IMPLEMENTED | — |
| D5-S23-A04-T01 | Export bundle schema v1 有效 JSON | S23 | A04 | P1 | `internal/layers/observability/diagnose/incident/export_test.go` | IMPLEMENTED | D5-S8-A01-T01 |
| D5-S23-A04-T02 | CLI `devrix debug export` 行为一致 | S23 | A04 | P2 | `internal/cli/debug/export_test.go` | IMPLEMENTED | D5-S8-A01-T02 |
| D5-S23-A06-T01 | SessionBridge ActiveSessions gauge 增减 | **S0** ⬅️ | **A03** ⬅️ | P1 | `tests/integration/obs_session_bridge_test.go` | IMPLEMENTED | D5-S0-A02-T01 |
| D5-S23-A06-T02 | HealthCheck 含 coverage 摘要字段 | S23 | A06 | P1 | `internal/layers/observability/health_test.go` | IMPLEMENTED | D5-S0-A02-T02 |
| **D5-S23-A06-T03** | **Observability.Shutdown 错误聚合改用 `errors.Join` + `%w`，保留 typed chain（DM-20260620-003 PR-C M3）** | **S23** | **A06** | **P0** | **`internal/layers/observability/observability.go::Shutdown` (lines 165-184)** | **IMPLEMENTED** | **—** |
| **D5-S23-A02-F01-T01** | **Tracker diff 收集 (W6)** | **S23** | **A07** | **P0** | **`internal/layers/observability/diagnose/tracker/tracker_test.go` (TestW6_DiffCollection_T01_CrossRef)** | **IMPLEMENTED** | **—** |
| **D5-S23-A02-F02-T01** | **Tracker LRU dedup 跨文件 (W6)** | **S23** | **A07** | **P0** | **`internal/layers/observability/diagnose/tracker/tracker_test.go` (TestW6_LRUDedup_T02_CrossRef)** | **IMPLEMENTED** | **—** |
| **D5-S23-A02-F03-T01** | **Tracker 异步 Linter 集成 (.go → go vet) + WatchFile (W7)** | **S23** | **A07** | **P0** | **`internal/layers/observability/diagnose/tracker/tracker_test.go` (TestW7_LinterIntegration_T03_CrossRef) + tests/integration/tools_terminal_test.go (TestTracker_NonBlocking)** | **IMPLEMENTED** | **—** |
| **D5-S23-A02-F03-T02** | **Tracker 高频 tick 非阻塞 (W7)** | **S23** | **A07** | **P0** | **`tests/integration/tools_terminal_test.go` (TestTracker_NonBlocking)** | **IMPLEMENTED** | **—** |
| D5-S23-A07-T01 | Diagnostic Tracker 500-file LRU + Diff | S23 | A07 | P0 | `internal/layers/observability/diagnose/tracker/tracker_test.go` | IMPLEMENTED | — |
| D5-S23-A07-T02 | Tracker 内置 linter (go/tsc/shellcheck) 报告 | S23 | A07 | P0 | `internal/layers/observability/diagnose/tracker/tracker_test.go` | IMPLEMENTED | — |
| D5-S23-A08-T01 | DebugFilter 按 categories 过滤 debug 级别 | **S21** ⬅️ | **A14** ⬅️ | P0 | `internal/layers/observability/instrument/logger/debugfilter/filter_test.go` | IMPLEMENTED | — |
| D5-S23-A08-T02 | DebugFilter 非 debug 级别 passthrough | **S21** ⬅️ | **A14** ⬅️ | P0 | `internal/layers/observability/instrument/logger/debugfilter/filter_test.go` | IMPLEMENTED | — |
| D5-S23-A09-T01 | FaultInject (testbuild) env 解析 + Hook | S23 | A09 | P0 | `internal/layers/observability/diagnose/faultinject/injector_test.go` | IMPLEMENTED | — |
| D5-S23-A09-T02 | FaultInject 生产 no-op stub (无 testbuild) | S23 | A09 | P0 | `internal/layers/observability/diagnose/faultinject/injector_prod.go` | IMPLEMENTED | — |

## D5-S24: Configure（配置与运行时管理）

| T ID | 描述 | canonical_s | canonical_a | Priority | Test 位置 | Status | Legacy T ID |
|------|------|-------------|-------------|----------|-----------|--------|-------------|
| D5-S24-A03-T01 | RegisterRuntimeMetric 幂等注册 path counter | S24 | A03 | P1 | `internal/layers/observability/configure/runtime/runtime_metric_test.go` | IMPLEMENTED | D5-S9-A01-T01 |
| D5-S24-A03-T02 | IncRuntimeMetric 桥接 d7_turn/legacy_harness 计数 | S24 | A03 | P1 | `internal/layers/observability/configure/runtime/runtime_metric_test.go` | IMPLEMENTED | D5-S9-A01-T02 |
| D5-S24-A03-T03 | PathResolver 并发 Record 安全 | S24 | A03 | P1 | `internal/layers/observability/configure/runtime/path_resolver_test.go` | IMPLEMENTED | D5-S9-A01-T03 |
| **D5-S24-A02-T04** | **~~legacy QueryLoop counter~~ REMOVED (DM-20260618-010)** | **S24** | **A02** | **P0** | **—** | **REMOVED** | **DM-20260617-001** |
| **D5-S24-A02-T05** | **~~Loop.Run slog.Warn~~ REMOVED (DM-20260618-010)** | **S24** | **A02** | **P0** | **—** | **REMOVED** | **DM-20260617-001** |

## CROSS: 跨域性能测试（从 D5-S2 迁出）

| T ID | 描述 | canonical_s | canonical_a | Priority | Test 位置 | Status | Legacy T ID |
|------|------|-------------|-------------|----------|-----------|--------|-------------|
| CROSS-D5-T01 | Compression P99 latency < 500ms | CROSS | — | P1 | `tests/performance/compression_test.go` | IMPLEMENTED | D5-S2-A01-T06 |
| CROSS-D5-T02 | Concurrent session memory bounded | CROSS | — | P1 | `tests/performance/memory_test.go` | IMPLEMENTED | D5-S2-A01-T07 |

---

## Legacy Module Index（旧 T 编号→新 Canonical）

| Legacy S | T 数 | Canonical S | Scenario |
|----------|------|-------------|----------|
| D5-S1 Tracer | 6 | S21 | Instrument |
| D5-S2 Metrics | 7 + 2 CROSS | S21 + CROSS | Instrument |
| D5-S3 Logger | 6 | S21 | Instrument |
| D5-S4 Exporter | 3 | S22 | Export |
| D5-S5 Coverage | 6 | S23 | Diagnose |
| D5-S6 Telemetry | 4 | S21 | Instrument |
| D5-S8 Incident | 2 | S23 | Diagnose |
| D5-S9 Runtime | 3 | S24 | Configure |
| D5-S0 Cross | 2 | S23/S0 | Diagnose/Facade |

---

## Statistics

| Total | IMPLEMENTED | PLANNED | REMOVED |
|-------|-------------|---------|---------|
| 47 | 45 | 0 | 2 |

> **v2.1 Terminal:** 41/41 T IMPLEMENTED（3 PLANNED 全部闭合：D5-S21-A05-T01 propagation 集成测试已存在、D5-S21-A05-T02 Counter 单元测试已存在、D5-S23-A06-T02 HealthCheck coverage 字段已验证）。2 REMOVED 为 QueryLoop legacy（DM-20260618-010）。D5-S21-A01-T03 为历史缺口（原始 S1-A01 无 T03），非本 change 引入。

## P0 测试点清单

D5-S21-A01-T01, D5-S21-A05-T03, D5-S21-A05-T04, D5-S21-A05-T05, D5-S21-A08-T02, D5-S21-A09-T01, D5-S21-A11-T01, D5-S21-A12-T01, D5-S22-A01-T02, D5-S22-A01-T03, D5-S23-A01-T01, D5-S23-A01-T02, D5-S23-A01-T03, D5-S23-A01-T04, D5-S23-A03-T01, D5-S23-A03-T02, D5-S23-A06-T03, D5-S23-A07-T01, D5-S23-A07-T02, D5-S23-A08-T01, D5-S23-A08-T02, D5-S23-A09-T01, D5-S23-A09-T02, D5-S23-A02-F01-T01, D5-S23-A02-F02-T01, D5-S23-A02-F03-T01, D5-S23-A02-F03-T02, D5-S25-A01-T01, D5-S25-A02-T01, D5-S25-A03-T01

## Revision History

| 版本 | 日期 | 变更 |
|------|------|------|
| 2.0.0 | 2026-06-14 | 初版：9 技术模块 S 层，38 T |
| 3.0.0 | 2026-06-15 | SA Refine v1.0：Canonical S21–S24 重排；增 canonical_s + Legacy T ID 列；2 性能 T 迁 CROSS 段 |
| 3.1.0 | 2026-06-17 | devrix-queryloop-legacy-decommission (DM-20260617-001)：(1) D5-S24-A02-T04 legacy counter 已注册；(2) D5-S24-A02-T05 一次警告 sync.Once。IMPLEMENTED 38→40 |
| **3.2.0** | **2026-06-19** | **v2.1 Terminal（代码锚点）**：增 canonical_a 列（全量填充）；canonical_s 校正 A08→S21 (DebugFilter)、A06→S0 (SessionBridge)；canonical_a 校正 Doctor T→A10、DebugFilter T→A14、SessionBridge T→A03；3 PLANNED 全部闭合 → 41/41 IMPLEMENTED；Statistics 更新 IMPLEMENTED 40→41 |
| **3.3.0** | **2026-06-20** | **devrix-error-handling-tier1-tier2 (DM-20260620-003)**: D5-S23-A06-T03 Observability.Shutdown errors.Join typed chain (PR-C M3)。IMPLEMENTED 41→42, P0 26→27 |
| **3.4.0** | **2026-07-02** | **devrix-mups-tool-classification-and-channel-autonomy (DM-20260701-007)**: D5-S25-A01-T01 BoundedInvariant + D5-S25-A02-T01 QuotientInvariant + D5-S25-A03-T01 SynthesizeInvariant (LTL-Lite L4–L6 termination invariants for PR-B 4 ToolChannel). Total 42→45, P0 27→30. S25 新 S section (0→3 A + 3 T) [retroactive S6 archive 2026-07-02 — DM-20260702-008 devrix-token-design-v2 PR #376 (LTL-Lite Bounded advisory T25) 共用此版本条目, 详见 `openspec/archive/2026-07-02-devrix-token-design-v2/acceptance-report.md`] |
| **3.5.0** | **2026-07-02** | **devrix-d2-tool-input-aware-concurrency-and-classifier (DM-20260702-009) S5 验收 S6 归档**: D5-S25-A04-T01 GrowthBook override 1 flag (bash 30K→50K, Production-Safety 单测 PASS). Total 45→46, P0 30→31. PR-D+E `57469504` 全部合入. 详见 `openspec/archive/2026-07-02-devrix-d2-tool-input-aware-concurrency-and-classifier/acceptance-report.md` (verdict: ACCEPTED) |
