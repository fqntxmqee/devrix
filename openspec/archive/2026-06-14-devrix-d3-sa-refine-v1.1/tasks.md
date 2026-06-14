# Tasks: D3 LLM Gateway v1.1 — 韧性可见性 + 评测探针 + 适配扩展

**Change ID:** devrix-d3-sa-refine-v1.1
**Demand ID:** DM-20260614-017
**Status:** S2_Clarified（S2 阶段产物完成，S3 待启动）
**Phases:** v1.1 Traceability（Phase S2→S6 OpenSpec 流程）

> **不估时**（playbook 原则 + OpenSpec S2 阶段约束）。任务按 Phase 排列；同一 Phase 内任务可并行。Phase 间有显式依赖。
>
> **F1-F9 编号说明**：v1.1 引入 9 个 F（F1-F9 横向编号对应需求项，便于跨域协调追踪）；F ID 形式为 `D3-S{N}-A{XX}-F{NN}` / `D3-X-A{XX}-F{NN}`，挂载到 v1.0 已有的 A 容器内。

---

## Phase S2 — 提案固化（已完成 ✅）

| ID | Task | 依赖 | 状态 |
|----|------|------|------|
| S2-1 | 创建 `openspec/changes/devrix-d3-sa-refine-v1.1/` 目录 | — | ✅ |
| S2-2 | 写 `demand.md` v0.1（S1 阶段产物 + 7 R1 议题清单） | — | ✅ |
| S2-3 | 用户评审 7 R1 议题（D1-D7） | S2-2 | ✅ APPROVED |
| S2-4 | 更新 `demand.md` v0.2 状态到 S2_Clarified + 写入 R1 决议 | S2-3 | ✅ |
| S2-5 | 写 `proposal.md` v0.1（F1-F9 拆解 + 跨域责任分配） | S2-4 | ✅ |
| S2-6 | 写 `tasks.md` v0.1（本文件，任务分解骨架） | S2-5 | ✅ |
| S2-7 | `demand-archive-index.md` 末尾追加 v1.1 入口 | S2-4 | ✅ |

> Phase S2 产物：`demand.md` v0.2 + `proposal.md` v0.1 + `tasks.md` v0.1（本文件）

---

## Phase S3 — 设计

| ID | Task | 依赖 | 产出 |
|----|------|------|------|
| S3-1 | `openspec/specs/d3-llm-gateway/spec.md` v3.1.0：新增 §6 v1.1 Requirements（FR-1 ~ FR-9）+ §7 Feature Flag 矩阵 | S2-6 | spec.md v3.1.0 |
| S3-2 | `openspec/specs/d3-llm-gateway/design.md` v3.1.0：§3 时序图新增 F1-F9 子图；§6 Feature flag 配置矩阵；§7 错误码签名表 | S2-6 | design.md v3.1.0 |
| S3-3 | `openspec/specs/d3-llm-gateway/f-registry.md` v3.1.0：D3-S3-A01 新增 F02b EmitBreakerStateMetric / F02c OnStateTransitionEmit / F02d ReuseEngineEvent；D3-S2-A01 新增 F04 AdapterProtocolMethod；D3-S5-A01 新增 F04 EmitSafetyLatencyEvent；D3-X-A02 新增 F02 FailFastOnObsNil；D3-S6-A01 新增 F05 FeatureFlagDefaults | S3-1 | f-registry.md v3.1.0 |
| S3-4 | `openspec/specs/d3-llm-gateway/t-registry.md` v3.1.0：新增 9 条 T（F1-F9 各 1 条 P0/P1，T ID 形式 `D3-S{N}-A{XX}-T{XX}` 沿用 v1.0 编号池）；§Legacy Archive 100% 继承校验 | S3-3 | t-registry.md v3.1.0 |
| S3-5 | `openspec/specs/d3-llm-gateway/span-registry.md` v3.1.0：§Metrics 新增 `llm_breaker_state{provider,state}` Gauge + `llm_tier_resolve_total` Counter + `llm_breaker_transitions_total{provider,from,to}` Counter；§Span events 新增 `safety.check.duration_ms`；§Events 新增 `flow.breaker.opened` / `closed` / `halfopened` | S3-1 | span-registry.md v3.1.0 |
| S3-6 | `openspec/specs/architecture/cross-domain-boundaries.md` v1.1.0：§2.4.3 Breaker 事件命名 D6-A 决议固化（`flow.breaker.opened` / `closed` / `halfopened` 三事件分开）；§2.4.4 补 D3 → D5 metric 命名边界 | S3-1 | cross-domain-boundaries.md v1.1.0 |
| S3-7 | `openspec/specs/d6-evolution/spec.md` 补丁：probe #1（Tier 解析正确性 ≥ 99%）+ probe #2（Breaker 异常切换告警）+ probe #4（Safety P99 < 1ms 告警）实施细节 + 阈值 | S3-1 | d6-evolution spec 补丁 |
| S3-8 | `openspec/specs/d3-llm-gateway/spec.md` §0 变更摘要表更新 v3.0.0 → v3.1.0 维度（6 S 不变 + 5+1 F 新增/调整 + 9 T 新增 + 3 spec 命名空间新增） | S3-1 + S3-3 | spec.md §0 |
| S3-9 | `openspec/specs/d3-llm-gateway/f-registry.md` §0 变更摘要表更新 + Revision History v3.1.0 | S3-3 | f-registry.md §0 + §末 |

> Phase S3 产物：`spec.md` v3.1.0 + `design.md` v3.1.0 + 4 注册表 v3.1.0 + `cross-domain-boundaries.md` v1.1.0 + `d6-evolution/spec.md` 补丁

### F ↔ T 编号预分配（Phase S3-4 实施时固化）

| F | 挂载 A | 计划 T ID | 优先级 |
|---|--------|----------|--------|
| F1 EmitBreakerStateMetric | D3-S3-A01 ShieldAndRetry | D3-S3-A01-T09 | P0 |
| F2 OnStateTransitionEmit | D3-S3-A01 ShieldAndRetry | D3-S3-A01-T10 | P0 |
| F3 ReuseEngineEvent | D3-S3-A01 ShieldAndRetry | D3-S3-A01-T11 | P1 |
| F4 FailFastOnObsNil | D3-X-A02 WireLLMStack | D3-X-A02-T01 | P0 |
| F5 AdapterProtocolMethod | D3-S2-A01 StreamChatCompletion | D3-S2-A01-T04 | P0 |
| F6 ProbeTierResolution | D3-S1-A01 ResolveModelRoute | D3-S1-A01-T04 | P1 |
| F7 ProbeBreakerTransitions | D3-S3-A01 ShieldAndRetry | D3-S3-A01-T12 | P1 |
| F8 EmitSafetyLatencyEvent | D3-S5-A01 FilterAndMatchContent | D3-S5-A01-T03 | P0 |
| F9 FeatureFlagDefaults | D3-S6-A01 LoadAndValidateLLMConfig | D3-S6-A01-T02 | P0 |

> **P0 合计 6 条**（F1/F2/F4/F5/F8/F9） + **P1 合计 3 条**（F3/F6/F7）= **9 新 T**
> v1.0 26 T 全量继承（不增不删）+ 9 新 T = **v1.1 目标 35 T**

---

## Phase S3-Gate — 设计评审

| ID | Task | 依赖 | 产出 |
|----|------|------|------|
| SG-1 | 写 `review-r1.md`（Owner 评审） | S3-1 ~ S3-9 | review-r1.md |
| SG-2 | 写 `review-r2.md`（Claude 结构层） | S3-1 ~ S3-9 | review-r2.md |
| SG-3 | 写 `review-r3.md`（Claude 运行层） | S3-1 ~ S3-9 | review-r3.md |
| SG-4 | 决议落定 → `demand.md` v0.3 状态 `S3_Design_Gate_Cleared` | SG-1 + SG-2 + SG-3 | demand.md 状态推进 |

> Phase S3-Gate 产物：3 review 文件 + `demand.md` 状态推进

---

## Phase S4 — 实现

### S4-1 — 准备

| ID | Task | 依赖 | 产出 |
|----|------|------|------|
| S4-1-1 | 确认 `internal/shared/errors/ErrObservabilityRequired` 已存在（v1.0 R3 P0 #8 占位） | — | 错误码定位 |
| S4-1-2 | grep IAdapter 现有 3 处实现（DeepSeek / MiniMax / stubAdapter） | — | 3 处清单 |
| S4-1-3 | D6 spec 补丁就绪（S3-7 已完成；D6 探针实施准备） | S3-7 | D6 实施环境 |

### S4-2 — 代码改动（按 F 顺序）

| ID | Task | F | 依赖 | 产出 |
|----|------|---|------|------|
| S4-2-1 | `breaker/circuit_breaker.go`：状态切换处 emit `llm_breaker_state{provider,state}` Gauge | F1 | S4-1-1 | `breaker/circuit_breaker.go` |
| S4-2-2 | `breaker/state.go`：状态变化钩子（`Open` / `Closed` / `HalfOpen` 三态切换） | F2 | S4-2-1 | `breaker/state.go` |
| S4-2-3 | `events/`（或 `breaker/events.go`）：emit `flow.breaker.opened` / `closed` / `halfopened` 三事件 | F3 | S4-2-2 | events 模块 |
| S4-2-4 | `internal/bridges/llm/wire.go` + `context_wiring.go`：obs nil 检查 → `ErrObservabilityRequired` | F4 | S4-1-1 | wire.go + context_wiring.go |
| S4-2-5 | `adapter/iadapter.go`：接口新增 `Protocol() string`；3 处实现（DeepSeek / MiniMax / stubAdapter）补方法 | F5 | S4-1-2 | iadapter.go + 2 adapter impl |
| S4-2-6 | `gateway/router.go`（D3-S1-A01）：调用次数 + 错误率 emit `llm_tier_resolve_total` Counter | F6 | S4-2-1 | router.go |
| S4-2-7 | `breaker/circuit_breaker.go`：emit `llm_breaker_transitions_total{provider, from, to}` Counter | F7 | S4-2-2 | circuit_breaker.go |
| S4-2-8 | `safety/filter.go`：计时写入 span event `safety.check.duration_ms` | F8 | — | filter.go |
| S4-2-9 | `shared/config/llmgateway.go` + `wire.go`：3 feature flag schema + 默认值 + 读取 | F9 | S4-2-1, S4-2-8 | llmgateway.go + wire.go |

### S4-3 — 测试

| ID | Task | F | 依赖 | 产出 |
|----|------|---|------|------|
| S4-3-1 | T09 unit test（Breaker metric 切换值变化） | F1 | S4-2-1 | breaker_test.go |
| S4-3-2 | T10 unit test（状态机钩子触发） | F2 | S4-2-2 | state_test.go |
| S4-3-3 | T11 unit test（事件命名 + D7 订阅 mock） | F3 | S4-2-3 | events_test.go |
| S4-3-4 | T-X-A02-T01 unit test（obs nil → `ErrObservabilityRequired`；mock obs → 正常返回） | F4 | S4-2-4 | wire_test.go |
| S4-3-5 | T04 unit test（3 个 impl `Protocol()` 返回非空字符串） | F5 | S4-2-5 | iadapter_test.go + 2 adapter_test.go |
| S4-3-6 | T04 unit test（路由正确性 ≥ 99% 通过 fixture） | F6 | S4-2-6 | router_test.go |
| S4-3-7 | T12 unit test（Breaker 异常切换触发） | F7 | S4-2-7 | circuit_breaker_test.go |
| S4-3-8 | T03 unit test（计时精度 + span event 写入） | F8 | S4-2-8 | filter_test.go |
| S4-3-9 | T02 unit test（flag schema + 默认值 + OFF 行为继承 v1.0） | F9 | S4-2-9 | llmgateway_test.go + wire_test.go |
| S4-3-10 | 跑 v1.0 全部 26 T 回归 | — | S4-3-1 ~ S4-3-9 | 26/26 绿 |
| S4-3-11 | 跑 v1.1 新增 9 T | — | S4-3-1 ~ S4-3-9 | 9/9 绿 |
| S4-3-12 | integration test：F1 + F2 + F3 端到端（Breaker open → metric + 事件 + D7 订阅） | F1/F2/F3 | S4-3-1 ~ S4-3-3 | integration_test.go |
| S4-3-13 | integration test：F8 端到端（Safety check 触发 → span event → D6 探针） | F8 | S4-3-8 | safety_integration_test.go |
| S4-3-14 | D6 集成测试：probe #1/#2/#4 接入 + 阈值告警 | F6/F7/F8 | S4-3-6 ~ S4-3-8 | d6 集成测试 |

### S4-4 — 校验

| ID | Task | 依赖 | 产出 |
|----|------|------|------|
| S4-4-1 | `go build ./...` 全绿 | S4-2 全部 | 全绿 |
| S4-4-2 | `go vet ./...` 无新增警告 | S4-2 全部 | 0 新增 |
| S4-4-3 | `go test ./internal/layers/llmgateway/... ./internal/bridges/llm/... ./internal/layers/evolution/...` 全绿 | S4-3 全部 | 35/35 T 绿 |
| S4-4-4 | grep v1.0 5 span 名 + 3 metric 名 字面量不变（AC-11） | S4-2 全部 | 校验通过 |
| S4-4-5 | grep v1.0 26 YAML key 字面量不变（AC-11） | S4-2 全部 | 校验通过 |
| S4-4-6 | grep 跨域 import 违规（D3 import D2/D4） | S4-2 全部 | 0 违规 |

> Phase S4 产物：8+ 代码文件 + 9 新 T + integration test + 全量回归

---

## Phase S4-Gate — 代码评审

| ID | Task | 依赖 | 产出 |
|----|------|------|------|
| SR-1 | `code review`（Codex + Gemini 综合） | S4-4 全部 | review 报告 |
| SR-2 | CRITICAL / HIGH 全部闭合 | SR-1 | 修复 commit |
| SR-3 | `demand.md` v0.4 状态 `S4_Implementation_Complete` | SR-2 | demand.md 状态推进 |

> Phase S4-Gate 产物：code review 报告 + 修复 + `demand.md` 状态推进

---

## Phase S5 — 验收

| ID | Task | 依赖 | 产出 |
|----|------|------|------|
| S5-1 | AC-01：`curl /metrics \| grep llm_breaker_state` 验证 | S4-Gate | 验证日志 |
| S5-2 | AC-02：F2 Breaker Open 切换时 metric 值变化 integration test | S4-Gate | 验证日志 |
| S5-3 | AC-03：F3 D7 集成测试通过（订阅 `flow.breaker.*`） | S4-Gate | 验证日志 |
| S5-4 | AC-04：F4 obs nil → `ErrObservabilityRequired` unit test | S4-Gate | 验证日志 |
| S5-5 | AC-05：F5 3 个 impl `Protocol()` 返回非空 unit test | S4-Gate | 验证日志 |
| S5-6 | AC-06：D6 probe #1 Tier 解析覆盖率 ≥ 99% D6 报告 | S4-Gate + D6 探针 | D6 报告 |
| S5-7 | AC-07：D6 probe #2 Breaker 异常切换告警阈值落地 D6 报告 | S4-Gate + D6 探针 | D6 报告 |
| S5-8 | AC-08：F8 span event `safety.check.duration_ms` 在 trace 中可见（Jaeger 查询） | S4-Gate | Jaeger 截图 |
| S5-9 | AC-09：F8 D6 probe #4 P99 < 1ms 告警阈值落地 D6 报告 | S4-Gate + D6 探针 | D6 报告 |
| S5-10 | AC-10：F9 3 feature flag 默认值与 D4-B 一致；OFF 时 v1.0 行为完全保持 | S4-Gate | 验证日志 |
| S5-11 | AC-11：v1.0 5 span 名 + 3 metric 名 + 26 YAML key 字面量不变（grep 校验） | S4-Gate | 校验日志 |
| S5-12 | AC-12：v1.0 全部 26 T 仍全绿 | S4-Gate | 26/26 绿 |
| S5-13 | 写 `acceptance-report.md（v1.1）` 状态 = **ACCEPTED** | S5-1 ~ S5-12 | acceptance-report.md |

> Phase S5 产物：`acceptance-report.md（v1.1）` 状态 = **ACCEPTED**

---

## Phase S6 — 归档

| ID | Task | 依赖 | 产出 |
|----|------|------|------|
| S6-1 | change 包归档到 `openspec/archive/2026-06-14-devrix-d3-sa-refine-v1.1/` | S5-13 | archive 目录 |
| S6-2 | `demand-archive-index.md` 标记 v1.1 ACCEPTED 状态 | S6-1 | index 更新 |
| S6-3 | 反向链接更新：D6 spec 引用 v1.1 决议（probe #1/#2/#4 引入） | S6-1 | d6 spec 反向链接 |
| S6-4 | v2.0 子 change 占位记录（Phase F 启动时新建） | S6-1 | v2.0 占位 |

> Phase S6 产物：archive 目录 + index 更新 + D6 spec 反向链接

---

## 任务依赖图

```
Phase S2 (已完成)
   ↓
Phase S3 (设计)
   ├─ S3-1 (spec.md) ──┐
   ├─ S3-2 (design.md) ┤
   ├─ S3-3 (f-registry) ┤
   ├─ S3-4 (t-registry) ┤
   ├─ S3-5 (span-registry) ┤
   ├─ S3-6 (cross-domain) ┤
   ├─ S3-7 (d6 spec) ──┤
   ├─ S3-8 (spec §0) ──┤
   └─ S3-9 (f-registry §0) ┤
   ↓
Phase S3-Gate (评审)
   ├─ SG-1 (review-r1)
   ├─ SG-2 (review-r2)
   ├─ SG-3 (review-r3)
   └─ SG-4 (状态推进)
   ↓
Phase S4 (实现)
   ├─ S4-1 (准备)
   ├─ S4-2 (代码, F1-F9)
   ├─ S4-3 (测试, 9 新 T)
   └─ S4-4 (校验)
   ↓
Phase S4-Gate (代码评审)
   ├─ SR-1 (code review)
   ├─ SR-2 (修复)
   └─ SR-3 (状态推进)
   ↓
Phase S5 (验收, AC-01~AC-12)
   ↓
Phase S6 (归档)
```

---

## 后续子 change 计划

| 子 change | DM ID | 范围 | 启动时机 |
|----------|-------|------|---------|
| `devrix-d3-sa-refine-v1.1`（本 change） | DM-20260614-017 | F1-F9 运行时代码 + 4 spec 补丁 + D6 3 probe | 本期 |
| `devrix-d3-sa-refine-v2.0` | DM-YYYYMMDD-NNN（待 S3 申请） | 物理路径迁移 + contracts.go 拆分 | v1.1 ACCEPTED 后 |

---

## 与 v1.0 tasks.md 的差异

| 维度 | v1.0 | v1.1 |
|------|------|------|
| Phase 命名 | Phase A-G（自定义） | Phase S2-S6（OpenSpec 标准阶段） |
| 任务粒度 | 7 阶段 ~30 任务 | 6 阶段 ~40 任务（F 粒度细分） |
| 跨域协调 | 0（仅 D3 内部） | 3 处（D5 metric / D6 probe / D7 事件） |
| 风险登记 | 7 | 10（含 F1/F5/F8/F9 等代码风险） |
| Feature flag | 0 | 3（`d3_resilience_emit_enabled` / `d3_safety_latency_event_enabled` / `d3_metric_emit_warn`） |
| 新 T 数量 | 0（沿用 V2.1 26 T） | 9（v1.1 范围） |
| 代码改动 | 0（仅文档 + 注释） | 8+ 文件 |

---

**Revision History**

| 版本 | 日期 | 变更 |
|------|------|------|
| 0.1 | 2026-06-14 | 初稿：Phase S2-S6 任务分解骨架 + 9 F ↔ 9 T 编号预分配 + 与 v1.0 tasks.md 差异对照 |
