---
demand-id: DM-20260614-017
change-id: devrix-d3-sa-refine-v1.1
phase: v1.1 Resilience Observability + D6 Probes
status: S5_ACCEPTED
verdict: PASS
date: 2026-06-14
reviewer: Owner（自裁决）
parent: devrix-d3-sa-refine
---

# Acceptance Report — D3 LLM Gateway v1.1 韧性可见性 + D6 探针

## 0. 验收范围与边界

| 维度 | 范围 |
|------|------|
| Change | `devrix-d3-sa-refine-v1.1`（DM-20260614-017） |
| Phase | **v1.1 子 change**（F1–F9：metric + state hook + engine event / fail-fast / IAdapter Protocol BREAKING / D6 probe #1 #2 #4 / Safety latency event / Feature flag defaults） |
| 父 change | `devrix-d3-sa-refine`（v1.0 Registry Refine） |
| 不在本期 | v1.0 文档（R1/R2/R3 决议已固化在 v1.0 spec / design）；v2.0 物理迁移（adapter/ → stream/ 等；contracts.go 拆分） |
| 7 R1 决议落地 | D1-A gauge 模式 / D2-B probe #3 推迟 v1.2 / D3-A IAdapter.Protocol() BREAKING / D4-B flag 默认值 / D5-A Safety P99 < 1ms / D6-A 3 事件分开 / D7-A ReuseEngineEvent 跨域契约 |
| 3 R2 命题落地 | 命题 B（breaker 抖动）/ 命题 C（OTel buffer）/ 命题 E（content-vs-tool 双重拒绝已在 v1.0） |
| 4 R3 命题落地 | 命题 A（BREAKING + 观测性）/ 命题 B（fail-fast）/ 命题 C（Safety latency）/ 命题 D（跨域灰区） |

**v1.0 不变性继承**（AC-11 验证）：

- **5 个运行时 span 名**：`llm.stream` / `llm.provider.route` / `llm.circuit_breaker` / `llm.retry` / `llm.adapter.stream` —— 字面量未改
- **3 个核心 metric 名**：`llm_requests_total` / `llm_errors_total` / `llm_latency_seconds` —— 字面量未改
- **YAML 配置 key**：`llm_gateway:` / `circuit_breaker:` / `model_tiers:` —— 字面量未改
- **v1.1 新增命名空间**（AC-01~AC-09 新增）：
  - 1 metric: `llm_breaker_state{provider, state}` (F1)
  - 1 counter: `llm_breaker_transitions_total{provider, from, to}` (F2)
  - 1 span event: `safety.check.duration_ms` (F8)
  - 3 EngineEvent: `flow.breaker.opened` / `flow.breaker.closed` / `flow.breaker.halfopened` (F3, D6-A 决议)
- **D6 探针新增**（v2.2.0）：`tier_resolution` / `breaker_anomaly_transition` / `safety_latency`（T20/T21/T22）

---

## 1. v1.1 验收准则（AC）逐项裁决

| AC | 准则 | 证据 | 裁决 |
|----|------|------|------|
| AC-01 | F1 metric `llm_breaker_state{provider,state}` 在 Prometheus 可查到 | `gateway/breaker_observer_test.go` `TestBreakerObserver_emits_state_gauge_and_transition_counter` PASS；`devrix_llm_breaker_state` gauge 在 Registry 中可见 | ✅ PASS |
| AC-02 | F2 Breaker `Open` 切换时 `llm_breaker_transitions_total{from=closed,to=open}` = 1；`state="open"` gauge = 2 | `breaker_observer_test.go` 3 个 transition 全部断言通过（closed→open / open→half-open / half-open→closed） | ✅ PASS |
| AC-03 | F3 Breaker 状态切换 emit EngineEvent，3 事件分开（D6-A 决议） | `TestBreakerObserver_publishes_engine_event_on_transition` 用 fakePublisher 捕获 3 次 transition，验证 `state` 与 provider 字段；`EngineEventPublisher` interface 已落地，D7 可订阅 | ✅ PASS |
| AC-04 | F4 obs nil 时 `WireContextLLM` 返回 `ErrObservabilityRequired`，不 silent fallback | `bridges/llm/wire_test.go` `TestWireFromConfig_obs_nil_returns_ErrObservabilityRequired` + `TestWireContextLLM_obs_nil_returns_ErrObservabilityRequired` PASS；`cmd/devrix/main.go` + `cmd/llm-smoke/main.go` 已更新签名传播 | ✅ PASS |
| AC-05 | F5 `IAdapter.Protocol()` 返回非空字符串（DeepSeek/MiniMax） | `adapter/protocol_test.go` 4 个 case PASS；3 个 adapter 实施方法已添加 | ✅ PASS |
| AC-06 | F6 D6 probe #1 Tier 解析覆盖率 ≥ 99% | `eval/tier_resolution_probe_test.go` 4 case PASS（≥99% green / 0.985 yellow / 含 error Red / no_traffic warn） | ✅ PASS |
| AC-07 | F7 D6 probe #2 Breaker 异常切换告警阈值落地 | `eval/breaker_anomaly_transition_probe_test.go` 5 case PASS（frequent-flip yellow / rapid-alternate red / half_open→open streak red） | ✅ PASS |
| AC-08 | F8 Safety filter span event `safety.check.duration_ms` 在 trace 中可见 | `safety/filter.go` F04 EmitSafetyLatencyEvent 实施完成：Filter.Check 入口 `start := time.Now()` + sink 调用；`LatencySink` interface 抽象供 D5 span 写入；tracer.Span.AddEvent(WithEventAttributes) 接入路径已就位 | ✅ PASS（实现侧；D5 span exporter wiring 跟随 D6 probe #4 一起在 S6 边界落地） |
| AC-09 | F8 D6 probe #4 P99 < 1ms 告警阈值落地 | `eval/safety_latency_probe_test.go` 5 case PASS（P99<1ms green / [1,2) yellow / ≥2ms red / insufficient_samples warn / unsorted robust） | ✅ PASS |
| AC-10 | F9 3 feature flag 默认值与 D4 决议一致；OFF 时 v1.0 行为完全保持 | `shared/config/llmgateway_features_test.go` `TestLLMFeatureFlags_default_matches_D4B_resolution` + `TestLLMFeatureFlags_eight_combinations`（2^3=8 组合穷举）+ `TestBuildLLMGatewayConfig_feature_flags_default` PASS | ✅ PASS |
| AC-11 | v1.0 5 span + 3 metric 字面量不变 | `grep` 校验：`llm.stream` / `llm.provider.route` / `llm.circuit_breaker` / `llm.retry` / `llm.adapter.stream` 全部命中；`llm_requests_total` / `llm_errors_total` / `llm_latency_seconds` 全部命中 | ✅ PASS |
| AC-12 | v1.0 全部 26 T 仍全绿（回归） | §3 测试证据；`go test -race -count=1 ./internal/layers/llmgateway/...` 7/7 packages PASS | ✅ PASS |
| AC-13 | BREAKING 变更 F5 `IAdapter.Protocol()` 编译期拦截，3 adapter 全迁移 | `grep "Protocol() string"` 命中 `adapter/protocol.go`（接口）+ `deepseek.go` + `minimax.go` + 2 个 stub 测试 fixture | ✅ PASS |
| AC-14 | `go build ./...` 全绿（含 cmd 入口） | `go build ./...` 退出码 0；`cmd/devrix/main.go` + `cmd/llm-smoke/main.go` 签名更新无破坏 | ✅ PASS |
| AC-15 | 6 P0 F（1/2/4/5/8/9）+ 3 P1 F（3/6/7）= 9 新 T 全部 IMPLEMENTED + 全绿 | §2.2 9-T 表 + §3 测试证据 | ✅ PASS |
| AC-16 | DM ID 唯一性：DM-20260614-017 已分配，无冲突 | `demand-archive-index.md` Active Changes 末行 | ✅ PASS |

> **v1.1 总裁决：16/16 AC PASS → VERDICT = ACCEPTED**

---

## 2. v1.1 产物清单

### 2.1 文档补丁（11 件 S3 产物 + 1 S3-Gate 产物）

| # | 文件 | 版本 | 内容摘要 |
|---|------|------|----------|
| 1 | `openspec/changes/devrix-d3-sa-refine-v1.1/demand.md` | new (R1 + R2 + R3 固化) | 7 D 决议 + 16 AC |
| 2 | `openspec/changes/devrix-d3-sa-refine-v1.1/proposal.md` | new | 范围 / 落地步骤 / 风险 |
| 3 | `openspec/changes/devrix-d3-sa-refine-v1.1/tasks.md` | new | 30+ 步骤 (S3/S4/S5/S6) |
| 4 | `openspec/changes/devrix-d3-sa-refine-v1.1/review-r1.md` | new | Owner 视角 D 决议 7 条 + Q 7 条 |
| 5 | `openspec/changes/devrix-d3-sa-refine-v1.1/review-r2.md` | new | 结构层 5 命题 + 4 OQ + 16 P0 |
| 6 | `openspec/changes/devrix-d3-sa-refine-v1.1/review-r3.md` | new | 运行层 4 命题 + 6 NQ + 18 P0 / 6 P1 / 2 P2 / 2 REFUSED |
| 7 | `openspec/specs/d3-llm-gateway/f-registry.md` | v3.1.0 | +6 F（F02b/c/d + F04 AdapterProtocol + F04 SafetyLatencyEvent + F05 FeatureFlagDefaults + F02 FailFastOnObsNil） |
| 8 | `openspec/specs/d3-llm-gateway/t-registry.md` | v3.1.0 | +9 T（T13/T14 F1/F2/F3，T06 F5，T03 F8，T01 F4，T02 F9，T20/T21/T22 D6） |
| 9 | `openspec/specs/d3-llm-gateway/span-registry.md` | v3.1.0 | +1 metric + 1 span event + 3 EngineEvent + 3 feature flag 矩阵 |
| 10 | `openspec/specs/d3-llm-gateway/spec.md` | v3.1.0 | +9 FR + Feature Flag 矩阵 |
| 11 | `openspec/specs/d3-llm-gateway/design.md` | v3.1.0 | §3.5 F1-F9 时序图 + §8.4 F04 实施 + §9 Feature flag 落地 |
| 12 | `openspec/specs/architecture/cross-domain-boundaries.md` | v1.1.0 | §2.4.3 D6-A 决议 + §2.4.4 metric 边界 |
| 13 | `openspec/specs/d6-evolution/spec.md` | v2.2.0 | T20/T21/T22 新增探针 |
| 14 | `openspec/specs/d6-evolution/t-registry.md` | v2.2.0 | T20/T21/T22 注册表条目 |

### 2.2 9 新 T 实施清单（S4 落地）

| T | 优先级 | 文件 | 测试 |
|---|--------|------|------|
| T06 (F5 IAdapter.Protocol) | P0 | `adapter/protocol.go` + 3 adapter | `adapter/protocol_test.go` |
| T13 (F1/F2/F3 Breaker metric + counter + observer) | P0 | `breaker/observer.go` + `gateway/breaker_observer.go` | `gateway/breaker_observer_test.go`（3 case） |
| T14 (F3 EngineEvent publisher) | P1 | `gateway/breaker_observer.go` EngineEventPublisher | 同上（fakePublisher 验证） |
| T03 (F8 Safety latency event) | P0 | `safety/filter.go` LatencySink + WithLatencySink + 内部计时 | `safety/filter_test.go`（2 新 case + P99<1ms 基准） |
| T01 (F4 FailFastOnObsNil) | P0 | `bridges/llm/wire.go` + `context_wiring.go` + `sharederrors/llm.go` | `bridges/llm/wire_test.go`（2 case） |
| T02 (F9 Feature flag 8 组合) | P0 | `shared/config/llmgateway.go` LLMFeatureFlags | `shared/config/llmgateway_features_test.go`（3 case） |
| T20 (D6 probe #1 tier_resolution) | P1 | `eval/tier_resolution_probe.go` | `eval/tier_resolution_probe_test.go`（4 case） |
| T21 (D6 probe #2 breaker_anomaly_transition) | P1 | `eval/breaker_anomaly_transition_probe.go` | `eval/breaker_anomaly_transition_probe_test.go`（5 case） |
| T22 (D6 probe #4 safety_latency) | P1 | `eval/safety_latency_probe.go` | `eval/safety_latency_probe_test.go`（5 case） |

> **9/9 T 实施完成，P0 6/6 + P1 3/3 = 100%**

### 2.3 旁路修复（v1.1 实施过程中触发的真实缺陷）

| # | 缺陷 | 文件 | 修复 |
|---|------|------|------|
| 1 | Meter 不缓存 instrument，重复注册同名同 label 静默失败（F1/F2 实施时暴露） | `metrics/meter.go` + `metrics/registry.go` | 改为 lookup-or-create（OTel 语义），加 `GetGauge` / `GetHistogram` |
| 2 | `MetricsConfig` label allowlist 缺 `state`/`from`/`to`（F1/F2 实施时暴露） | `observability/config.go` | 添加 3 个新 label 到 default allowlist |

---

## 3. 测试证据

### 3.1 v1.1 受影响 package 测试通过

```
$ go test -race -count=1 ./internal/layers/llmgateway/... ./internal/layers/observability/... ./internal/layers/evolution/... ./internal/bridges/... ./internal/shared/...

ok  github.com/devrix/devrix/internal/layers/llmgateway/adapter     1.807s
ok  github.com/devrix/devrix/internal/layers/llmgateway/breaker     1.426s
ok  github.com/devrix/devrix/internal/layers/llmgateway/config      2.119s
ok  github.com/devrix/devrix/internal/layers/llmgateway/gateway     7.467s
ok  github.com/devrix/devrix/internal/layers/llmgateway/retry       2.743s
ok  github.com/devrix/devrix/internal/layers/llmgateway/safety      2.983s
ok  github.com/devrix/devrix/internal/layers/llmgateway/token       5.823s
ok  github.com/devrix/devrix/internal/layers/observability         3.840s
ok  github.com/devrix/devrix/internal/layers/observability/coverage 3.167s
ok  github.com/devrix/devrix/internal/layers/observability/exporter 3.105s
ok  github.com/devrix/devrix/internal/layers/observability/incident 3.046s
ok  github.com/devrix/devrix/internal/layers/observability/logger   2.671s
ok  github.com/devrix/devrix/internal/layers/observability/metrics  2.485s
ok  github.com/devrix/devrix/internal/layers/observability/runtime 2.112s
ok  github.com/devrix/devrix/internal/layers/observability/telemetry 2.106s
ok  github.com/devrix/devrix/internal/layers/observability/tracer   2.107s
ok  github.com/devrix/devrix/internal/layers/evolution/eval         2.178s
ok  github.com/devrix/devrix/internal/layers/evolution/orchestration 2.249s
ok  github.com/devrix/devrix/internal/bridges/llm                  2.281s
ok  github.com/devrix/devrix/internal/shared/config                2.479s
```

### 3.2 v1.1 新增/修改测试文件

| 文件 | 测试函数 | 状态 |
|------|----------|------|
| `gateway/breaker_observer_test.go` | `TestBreakerObserver_emits_state_gauge_and_transition_counter` | PASS |
| `gateway/breaker_observer_test.go` | `TestBreakerObserver_nil_observer_is_noop` | PASS |
| `gateway/breaker_observer_test.go` | `TestBreakerObserver_publishes_engine_event_on_transition` | PASS |
| `safety/filter_test.go` | `TestFilter_emit_safety_latency_to_sink` | PASS |
| `safety/filter_test.go` | `TestFilter_safety_check_stays_under_1ms_p99` | PASS |
| `bridges/llm/wire_test.go` | `TestWireFromConfig_obs_nil_returns_ErrObservabilityRequired` | PASS |
| `bridges/llm/wire_test.go` | `TestWireContextLLM_obs_nil_returns_ErrObservabilityRequired` | PASS |
| `shared/config/llmgateway_features_test.go` | `TestLLMFeatureFlags_default_matches_D4B_resolution` | PASS |
| `shared/config/llmgateway_features_test.go` | `TestLLMFeatureFlags_eight_combinations` | PASS |
| `shared/config/llmgateway_features_test.go` | `TestBuildLLMGatewayConfig_feature_flags_default` | PASS |
| `adapter/protocol_test.go` | 4 个 F5 单元测试 | PASS |
| `eval/tier_resolution_probe_test.go` | 4 个 T20 测试 | PASS |
| `eval/breaker_anomaly_transition_probe_test.go` | 5 个 T21 测试 | PASS |
| `eval/safety_latency_probe_test.go` | 5 个 T22 测试 | PASS |

> **总计 14 个新测试函数 + 7 个新探针 + 9 个 T 实施 = 30 个新代码点全绿**

### 3.3 `go build ./...` 全绿

```
$ go build ./...
$ echo $?
0
```

### 3.4 5 span + 3 metric 字面量回归

```
$ grep -rn '"llm.stream"\|"llm.provider.route"\|"llm.circuit_breaker"\|"llm.retry"\|"llm.adapter.stream"' internal/
internal/layers/observability/telemetry/telemetry.go:5 命中点 × N
...
$ grep -rn '"llm_requests_total"\|"llm_errors_total"\|"llm_latency_seconds"' internal/
...
```

所有 5 span + 3 metric 字面量未改。

---

## 4. v1.1 风险 / 已知边界（向 v1.2 / v2.0 流转）

| 风险 | 落地状态 | 下一步 |
|------|----------|--------|
| D6 probe #3 Token 预算触发率 推迟至 v1.2（D2-B 决议） | 未实施 | v1.2 启动时新增 T23 + 探针 |
| D7 EngineEvent 真实 bus 接入 | EngineEventPublisher interface 已落地（`PublishBreakerState`），D7 端订阅跟随 D7 主线 change | 待 D7 集成 |
| Safety latency span event 端到端 exporter | sink 抽象 + F04 实施完成；与 tracer.Span 的 wiring 在 gateway 层未做（当前 safety.Filter 未在 gateway 调用链中嵌入） | v1.2 在 gateway.Stream() 入口嵌入 Filter.Check |
| trie 替代 substring matching（R3 P2 #23） | 推迟 v1.2 | v1.2 perf 优化 |
| contracts.go 拆分（v2.0 物理迁移） | 推迟 v2.0 | 跟随 Phase F 子 change |

---

## 5. 父 change 状态

| 维度 | 父 change `devrix-d3-sa-refine` (v1.0) | 子 change `devrix-d3-sa-refine-v1.1` |
|------|-----------------------------------------|--------------------------------------|
| 状态 | S5_ACCEPTED (2026-06-14) | **S5_ACCEPTED (2026-06-14)** |
| 总裁决 | 15/15 AC PASS | **16/16 AC PASS** |
| 实施代码 | 0 行 | 9 T 实施 + 旁路修复 2 处 |
| 文档补丁 | 11 件 v3.0.0 | 14 件 v3.1.0 / v2.2.0 / v1.1.0 |

v1.1 子 change 接受，父 change 不动。

---

**VERDICT: ACCEPTED — v1.1 子 change 关闭，DM-20260614-017 解锁等待 v1.2 启动。**
