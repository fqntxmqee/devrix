# Review R1 — D3 LLM Gateway v1.1 子 change（韧性可见性 + 评测探针 + 适配扩展）

**Change ID:** devrix-d3-sa-refine-v1.1
**Demand ID:** DM-20260614-017
**Review Date:** 2026-06-14
**Reviewer:** 用户（架构 Owner）
**Reviewed Documents:** `demand.md v0.1`、`proposal.md v0.1`、`tasks.md v0.1`
**Verdict:** ✅ **APPROVED**（v1.0 7 R 决议继承 + v1.1 7 D 决议已闭合）

---

## 1. 评审范围

| 评审对象 | 内容 |
|---------|------|
| F 拆解完整性 | F1-F9 横向编号 ↔ D3-S{N}-A{XX} 挂载正确性 |
| 决议追溯 | v1.0 R1/R2/R3 17 项决议 → v1.1 F 实施映射 |
| 跨域责任 | D3→D5 metric、D3→D7 EngineEvent、D3→D6 探针 三条契约边界 |
| Feature Flag | `d3_resilience_emit_enabled` ON / `d3_safety_latency_event_enabled` ON / `d3_metric_emit_warn` OFF 默认值 |
| BREAKING 范围 | `IAdapter.Protocol()` 新增方法的影响（3 implementations） |
| 运行时稳定性 | 5 span 字面量保持 + 3 metric 字面量保持 + 新增 3 metric + 1 event + 3 events |
| 跨域边界一致性 | `cross-domain-boundaries.md v1.1.0` §2.4.3 / §2.2.4 / §2.3.2 决议固化 |
| D6 探针接入 | probe #1 / #2 / #4 接入 D3 emit 数据源（probe #3 推迟 v1.2） |

---

## 2. Decision 评审（v1.1 7 个 D 决议）

### D1-A：Breaker State Metric Emit（`llm_breaker_state`）

| 方案 | 评审结论 |
|------|---------|
| **A1 Gauge + 2 provider × 3 state = 6 series** | ✅ **接受**（Cardinality 受控；F1 EmitBreakerStateMetric） |
| A2 Counter 计数状态切换 | ❌ 失去"当前状态"语义；与 R2 命题 B 衍生结论冲突 |
| A3 Histogram 状态停留时长 | ❌ Cardinality 爆炸（provider × state × duration）；非可观测语义 |

**关键论据：**
- Gauge 表达"当前状态"是 SRE 视角最直接（`{provider="deepseek", state="open"} == 2` 即故障）
- 2 provider × 3 state = 6 series 在 Prometheus 最佳实践内（< 20/cardinality）
- 与 `design.md §4.2 熔断器状态机`语义对齐（Closed / Open / HalfOpen 三态）
- D5 dashboard `d3_breaker_state` 可直接消费

### D2-B：D6 Probe #3 Token 预算触发率 推迟至 v1.2

| 方案 | 评审结论 |
|------|---------|
| **B1 推迟至 v1.2 + 落地 probe #1 / #2 / #4** | ✅ **接受**（D3-S4 BudgetTokens span event `budget.check.exceeded` 未实施；先期落地） |
| B2 强行落地 #3 + 补 span event | ❌ span event 设计属 D3-S4 注入模式范畴；需先期独立 change |
| B3 删除 #3（不做） | ❌ 与 R1 Q7 决议冲突；v1.2 仍要做 |

**关键论据：**
- BudgetTokens 当前是「注入模式」（span attribute `safety.checked` / `budget.checked`），不直接 emit span
- probe #3 需依赖 D3-S4-A01 emit `budget.check.exceeded` span event，属独立 F 设计
- v1.2 单独 change 处理 span event + probe #3，命名与时机一致

### D3-A：`IAdapter.Protocol()` 新增方法（BREAKING）

| 方案 | 评审结论 |
|------|---------|
| **A1 `Protocol() string` 返回 `"openai-compatible"` / 未来 `"anthropic-native"`** | ✅ **接受**（BREAKING 风险受控：3 implementations） |
| A2 抽 `AdapterMeta` 结构体 | ❌ 过设计；当前 V3 路线图只需 1 个 string 字段 |
| A3 维持现状 | ❌ 与 R3 P1 #15 决议冲突；V3 Anthropic 接入时必重构 |

**关键论据：**
- BREAKING 影响范围：`adapter/openai_stream.go` + `adapter/deepseek_stream.go` + `adapter/minimax_stream.go`（3 个 adapter，stubAdapter 不强制）
- 当前 3 implementations 均返回 `"openai-compatible"`（V2.1 行为零变化）
- V3 接入 AnthropicAdapter 时自然返回 `"anthropic-native"`，零额外设计
- 与 `R3 P1 #15` 完全对齐（决议已闭合）

### D4-B：Feature Flag 默认值（`d3_resilience_emit_enabled` ON / `d3_safety_latency_event_enabled` ON / `d3_metric_emit_warn` OFF）

| 方案 | 评审结论 |
|------|---------|
| **B1 emit_enabled = ON（Dogfood 路径启用 metric）+ warn = OFF（不污染日志）** | ✅ **接受**（Cardinality 受控；fail-fast 已闭合） |
| B2 emit_enabled = OFF（保守） | ❌ 与 R3 P0 #8 fail-fast 联动失效；dogfood 无 metric 看 |
| B3 warn = ON（保留 v1.0 行为） | ❌ emit 失败走 D5 健康检查，不需日志噪音 |

**关键论据：**
- `d3_resilience_emit_enabled` ON：6 series 受控；dashboard 默认可用；D6 probe #2 立即接数据
- `d3_safety_latency_event_enabled` ON：P99 < 1ms 验证所需；与 D5-A 决议对齐
- `d3_metric_emit_warn` OFF：emit 失败走 D5 健康检查；日志降噪
- OFF 行为继承（F9 FeatureFlagDefaults）：3 flag 默认值变更时单元测试验证 v1.0 行为完全保持

### D5-A：Safety Filter Latency P99 < 1ms

| 方案 | 评审结论 |
|------|---------|
| **A1 span event `safety.check.duration_ms`（µs 精度）** | ✅ **接受**（F8 EmitSafetyLatencyEvent；D6 probe #4 落地） |
| A2 Histogram metric | ❌ 增加 label cardinality；span event 已能 P99 聚合 |
| A3 log 关键字 | ❌ 非结构化；D6 probe 无法消费 |

**关键论据：**
- `design.md §1.2` 已声明目标 P99 < 1ms，v1.0 无 metric/event 验证
- span event `safety.check.duration_ms`（µs 精度）由 OTel 聚合 → D5 dashboard P99
- D6 probe #4 检测 P99 ≥ 2ms 触发 Red 回归告警
- 与 F8 / T03 一致（v1.1 实施）

### D6-A：D3→D7 复用 EngineEvent，3 事件分开（`flow.breaker.opened` / `closed` / `halfopened`）

| 方案 | 评审结论 |
|------|---------|
| **A1 3 事件分开（open/closed/halfopened），D7 订阅时按 state 路由** | ✅ **接受**（F3 ReuseEngineEvent；语义清晰） |
| A2 1 事件 + payload.state | ❌ D7 订阅路由成本高；state 在 payload 内嵌耦合 |
| A3 维持现状不发 event | ❌ 与 R1 Q6 决议冲突；D7 编排者盲区 |

**关键论据：**
- 3 事件分开与状态机语义对齐（Closed / Open / HalfOpen 三态）
- D7 编排者按 state 路由 = 显式契约，零推断
- EngineEvent 现有 `FlowStarted` / `FlowFailed` 风格延续
- 与 `cross-domain-boundaries.md v1.1.0 §2.4.3` 一致

### D7-A：D3 韧性 metric 边界（`d3_metric_emit_total{status=ok|missing}` 仅启动期）

| 方案 | 评审结论 |
|------|---------|
| **A1 `d3_metric_emit_total{status=ok\|missing}` 仅启动期一次性 + `d3_resilience_emit_enabled = true` 前 must-be-zero 5min** | ✅ **接受**（F4 FailFastOnObsNil 联动） |
| A2 持续 emit（请求期） | ❌ 高频 label cardinality；启动期检测足够 |
| A3 删 metric，靠 fail-fast 阻止 | ❌ 失去 dogfood 期间的可观测性 |

**关键论据：**
- 启动期一次性 emit 足够检测 D5 readiness
- `d3_resilience_emit_enabled = true` 前 `status=missing == 0` 持续 5min（与 R3 命题 B #3 一致）
- fail-fast（`ErrObservabilityRequired`）是 v1.0 现状 silent fallback bug 修复（v1.1 P0 #8 同步）

---

## 3. R1 关键澄清（v1.1 子 change 继承 + 新增）

| # | 议题 | 决议 | 写入位置 |
|---|------|------|---------|
| Q1 | v1.0 R1/R2/R3 17 项决议是否全部继承？ | ✅ 全部继承；5 项保留决议（D-2/D-3/D-5/D-7/D-8）作为 v1.1 F1/F3/F4/F8/F5 落地 | `proposal.md §1 + §3.1` |
| Q2 | v1.1 F 编号横向（F1-F9）vs v1.0 纵向（D3-S{N}-A{XX}-F{NN}）是否冲突？ | 不冲突：F1-F9 横向便于跨域协调；纵向 F ID 在 `f-registry.md` 内部保持 | `proposal.md §3.1 表脚` |
| Q3 | `IAdapter.Protocol()` BREAKING 是否影响 Bridge？ | 不影响：Bridge 调 `Stream(ctx, req)` 不调 `Protocol()`；BREAKING 仅限 3 adapter 实施 | `proposal.md §3.4` |
| Q4 | probe #3 推迟到 v1.2 后，d6-evolution spec 补丁是否仍写 placeholder？ | 写「probe #3 推迟 v1.2（D2-B 决议）」+ 不写 FR 段；避免误导后续 reviewer | `d6-evolution/spec.md §新增探针` |
| Q5 | 3 flag 默认值变更（v1.0 false → v1.1 ON）是否需 changelog？ | 需：`f-registry.md §3.6 + span-registry.md §7 + design.md §9 + spec.md §13` 全部标注 D4-B 决议 | `f-registry.md + span-registry.md` |
| Q6 | v1.1 是否新增 EngineEvent schema？ | 不新增：复用 D7 现有 `FlowStarted`/`FlowFailed`；D3 emit `flow.breaker.{state}` 3 事件通过 payload.state 路由 | `cross-domain-boundaries.md §2.4.3` |
| Q7 | v1.1 F4 FailFastOnObsNil 是否需 `obs.Health()` 接口？ | 需：D5 Bridge 加 `Health() error`（v1.1 F04 配套）；fail-fast 调用 `Health()` 决定是否启动 | `proposal.md §3.4 + cross-domain-boundaries.md §2.4.4` |

---

## 4. 进入 S3 的前置条件

| 条件 | 状态 |
|------|------|
| proposal.md 与 demand.md 一致 | ✅ |
| F1-F9 拆解与 v1.0 决议追溯完整 | ✅ |
| 跨域边界（D3→D5/D7/D6）声明清晰 | ✅（`cross-domain-boundaries.md v1.1.0`） |
| Feature Flag 默认值变更影响范围明确 | ✅（3 flag + 3 注册表 + spec + design） |
| BREAKING 范围（IAdapter.Protocol）明确 | ✅（3 adapter 实施） |
| D6 probe #1/#2/#4 接入点明确 | ✅（`d6-evolution/spec.md v2.2.0`） |
| Out of Scope 明确 | ✅（probe #3 / v2.0 物理迁移 / V3 Anthropic） |

**S3 阶段待产出（已部分完成）：**

- ✅ `openspec/specs/d3-llm-gateway/spec.md` v3.1.0（9 FR + Feature Flag 矩阵）
- ✅ `openspec/specs/d3-llm-gateway/design.md` v3.1.0（F1-F9 时序 + Flag 矩阵）
- ✅ `openspec/specs/d3-llm-gateway/f-registry.md` v3.1.0（6 F 新增/调整）
- ✅ `openspec/specs/d3-llm-gateway/t-registry.md` v3.1.0（9 新 T）
- ✅ `openspec/specs/d3-llm-gateway/span-registry.md` v3.1.0（3 metric + 1 event + 3 events）
- ✅ `openspec/specs/architecture/cross-domain-boundaries.md` v1.1.0（§2.4.3 D6-A + §2.4.4 metric 边界）
- ✅ `openspec/specs/d6-evolution/spec.md` v2.2.0（probe #1/#2/#4）
- ✅ `openspec/specs/d6-evolution/t-registry.md` v2.2.0（T20/T21/T22）

---

## 5. 决议

| 决议项 | 结论 |
|--------|------|
| **R1 Verdict** | ✅ **APPROVED** |
| **demand.md 状态** | S2_Clarified → **S3_Designing** |
| **proposal.md 状态** | v0.1 → **v0.2 ACCEPTED**（v1.0 17 决议 + v1.1 7 决议继承） |
| **下一步** | 进入 S3-Gate 阶段（review-r2 结构层 + review-r3 运行层） |

---

**Revision History**

| 版本 | 日期 | 变更 |
|------|------|------|
| 0.1 | 2026-06-14 | 初稿：7 D 决议评审 + 7 Q 澄清 + S3 启动前置条件 |