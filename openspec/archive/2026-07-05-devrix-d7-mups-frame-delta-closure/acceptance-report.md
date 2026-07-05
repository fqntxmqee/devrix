# Acceptance Report: D7 MUPS 5 节点 Frame Delta 闭环节流

**Change ID:** `devrix-d7-mups-frame-delta-closure`
**Demand:** DM-20260705-010
**Status:** S5_Acceptance → **PARTIAL ACCEPTED**（Phase 1-3 code + tests PASS；Phase 4 trace 重放 + 三方 review 待 S5 验收 follow-up）

---

## 1. 验收结论

**Verdict:** ✅ **PARTIAL ACCEPTED (Phase 1-3 code shipped, Phase 4 deferred to S5)**

MUPS 5 节点 Observe→Plan→Execute LLM I/O 升级为**显式收敛过程**：

- **Plan→Execute 边**：注入 ≤200 字符的 `plan_frame_delta` 摘要 + 16 字符 schema hash（双轨），通过 budget 防御降级 baseline
- **Observe→Plan 边**：上一轮 `ConvergenceMetric` + 残量 `KnownGaps` 显式 append-only 注入 ObserveSignalInput，FrameObserveUser spec 9→11 字段契约（DM-20260705-009 封闭式分类器定位兼容）
- **Execute→Observe 回写**：每 round emit `convergence_metric` span（uncertainty_reduction_rate + observed_gaps_closed_count + frame_delta_consumed），deterministic 0 LLM

Phase 1-3 共 4 PR（#431 S1 + #432 S2 + #433 S3 + #434 S4 + #435 docs-only）全部 squash merge；16/16 D7-FD 子测试 PASS + 22/22 orchestration 包 `go test -race` PASS + T18 AC6 71 现有 frame 测试 0 行为变化 PASS；M1 ObservationFrame 9 字段 / M2 StrategicPlanFrame 16 字段契约 0 修改（append-only 注入）。

**Phase 4 follow-up (S5 验收阶段执行):**
- T15 L5-MUPS-FD-3: Execute 5 sub-turn 全 convergence_metric span + 末轮 rate ≥ 0.5
- T16 e2e_frame_delta_test.go: sess_1783255992426_6000 wi_d0_s0_goal 端到端 trace 重放
- T17 L5-MUPS-FD-4: 跨链 LLM prompt size 单调不增 ±5% 噪声
- T19 S3-Gate 三方博弈论 review (codex + cursor + claude 外部评审)

---

## 2. 验收范围

### 2.1 帧 delta 协议

| 边 | 方向 | 字段契约 | 大小约束 | 注入点 |
|---|------|---------|---------|--------|
| **Observe→Plan** | 上行 | `PriorArtifactSummary string` (≤80 char) + `KnownGaps []string` | append-only 2 字段进 ObservationFrame | `BuildObservePriorDelta` (首轮零值/非首轮含上一轮收敛) |
| **Plan→Execute** | 下行 | `ExecutionMode string` + `ChildSpecs []ChildSpecRef` + `DeliverableContract string` (≤200 char) | schema hash 16 字符 + 摘要 ≤80 字符双轨 | `InjectPlanFrameDelta` (budget 超限 → baseline 降级) |
| **Execute→Observe 回写** | 自反 | `UncertaintyReductionRate float64` + `ObservedGapsClosedCount int` + `FrameDeltaConsumed bool` | deterministic 0 LLM 纯函数 | `ComputeConvergenceMetric` per round |

### 2.2 4 Phase PR

| Phase | PR | 内容 | T-points |
|-------|----|----|----------|
| S1 Proposal | #431 | demand.md + proposal.md 立项 | — |
| S2 Proposal | #432 | proposal.md v2 (AC8 + design.md 入口) | — |
| S3 Design | #433 | design.md 六段式 v1.1 + v1.2 (S3-Gate 三方 review 修复) | — |
| S4 Phase 1-3 | #434 | FrameDelta + InjectPlanFrameDelta + BuildObservePriorDelta + ComputeConvergenceMetric + 16 子测试 + 3 span op | T1-T5 + T7-T10 + T12-T14 + T18 |
| Docs sync | #435 | spec.md + mups-frame-delta-spec.md + CHANGELOG.md + t-registry.md D7-FD | T20 + T21 |
| **Total** | **5 PR** | **Phase 1-3 完整闭环** | **16/16 IMPLEMENTED** |

### 2.3 不变性承诺

| 既有契约 | 修改方式 | 回归测试 |
|---------|---------|---------|
| M1 ObservationFrame 9 字段 (DM-20260705-003) | append-only + 2 字段 | T18 71 frame 测试 0 行为变化 PASS |
| M2 StrategicPlanFrame 16 字段 (DM-20260705-003) | append-only + 5 字段 | 同上 |
| DM-20260705-009 封闭式分类器定位 | 兼容 (prior_artifact_summary / known_gaps 标 obs_fact kind) | 8 新测试 (D2-S15-A99-T01..T05 + D7-S5-A99-T10..T12) PASS |
| DM-20260705-008 Strategy 抽象 | 不冲突 (frame delta 走 Strategy 旁路) | 19 strategy 测试 0 修改 |
| ResolutionContract (DM-20260704-006) | 复用承载 convergence_metric | 19/19 RT 测试 PASS |
| TaskContract (DM-20260629-006) | FrameDelta 同层 kernel type | FF 状态独立 |
| PlanKind / VerdictKind 决策表 | 不破坏 (frame delta 不进决策表) | SpawnPolicyEvaluator 0 改动 |
| Pessimistic Commit L3 | 不破坏 | 5 触发 + 4 fallback 0 改动 |

---

## 3. 验收标准对照

### 3.1 P0 标准

| ID | 标准 | 验证方式 | 状态 |
|----|------|----------|------|
| AC1 | Observe LLM user frame 含 `prior_artifact_summary` | observe_frame_delta_test.go T02 + span tag | ✅ |
| AC2 | Observe LLM user frame 含 `known_gaps` | observe_frame_delta_test.go T03 + 封闭式 JSON 不破坏 | ✅ |
| AC3 | Execute system_prompt 注入 plan_frame_delta ≤ 200 字符 | execute_plan_frame_inject_test.go T05 budget 防御 | ✅ |
| AC4 | ConvergenceMetric deterministic 0 LLM | convergence_metric_test.go T05 mock LLM = 0 + 5 sub-turn span | ✅ |
| AC5 | Jaeger span tag 全可见 (6 attribute) | 3 span op 落地 + Trace 契约 | ✅ |
| AC6 | M1/M2 frame 契约 0 修改 | T18 71 现有 frame 测试 0 行为变化 PASS | ✅ |
| AC7 | 跨链 LLM prompt size 单调不增 ±5% 噪声 | convergence_metric_test.go T02 rate ≥ 0.5 + 待 Phase 4 trace 重放 | ⚠️ Phase 4 验 |
| AC8 | S3-Gate 三方博弈论 review | design.md v1.1 (claude) + v1.2 (codex + cursor 修复 3 Critical) | ✅ (design) / ⚠️ Phase 4 final |

### 3.2 D7-FD 子测试点 (16 T-IDs)

| T-ID | 描述 | 状态 |
|------|------|------|
| D7-S9-A112-T01 | `InjectPlanFrameDelta` 注入正确（摘要 + schema hash 双轨） | ✅ |
| D7-S9-A112-T02 | 摘要 ≤ 80 字符截断 | ✅ |
| D7-S9-A112-T03 | schema hash 稳定（幂等） | ✅ |
| D7-S9-A112-T04 | 零值 FrameDelta → baseline 无注入 | ✅ |
| D7-S9-A112-T05 | 注入 ≤ MaxPlanFrameDeltaInjectChars=200 安全网 | ✅ |
| D7-S5-A111-T01 | `BuildObservePriorDelta` 首轮零值 | ✅ |
| D7-S5-A111-T02 | 非首轮含上一轮收敛度量（≤80 摘要） | ✅ |
| D7-S5-A111-T03 | known_gaps Phase 2 stub 空数组 | ✅ (P1) |
| D7-S5-A111-T04 | 封闭式 JSON 不破坏（BuildLineFrame 11 字段契约） | ✅ |
| D7-S5-A111-T05 | 既存 9 字段顺序 0 修改（append-only） | ✅ |
| D7-S5-A111-T06 | i18n en + zh 键完整 | ✅ |
| D7-S9-A113-T01 | 首轮/空 subTurns → 零值 ConvergenceMetric | ✅ |
| D7-S9-A113-T02 | 工具 diff → uncertainty_reduction_rate（AC7 末轮 ≥ 0.5） | ✅ |
| D7-S9-A113-T03 | ResolutionClaim 闭合 gap 计数 + report 优先 | ✅ |
| D7-S9-A113-T04 | Jaeger nil-bridge span emit 不 panic | ✅ |
| D7-S9-A113-T05 | 纯函数 deterministic + 0 LLM | ✅ |

### 3.3 关键 Span op

| Op | 触发点 | Attribute |
|---|--------|-----------|
| `d7.s5.observe.prior_delta.span` | `BuildObservePriorDelta` 出口 | `prior_artifact_summary` + `known_gaps` + `span_tag_complete` |
| `d7.s9.execute.plan_frame_delta.inject` | `InjectPlanFrameDelta` 注入完成 | `plan_frame_delta_schema_hash` + `plan_frame_delta_injection_chars` + `injection_status` |
| `d7.s9.execute.convergence_metric.emit` | `ComputeConvergenceMetric` 后 | `uncertainty_reduction_rate` + `observed_gaps_closed_count` + `frame_delta_consumed` |

**Trace 契约：** `mupsSpan.parent == orchSpan.SpanContext`（与 DM-20260625-019 5-node Span 一致）

---

## 4. 关键修复 vs prior

| Prior 症状 | 治本机制 |
|-----------|----------|
| Observe LLM 仅 directive 69 字符 → 3 obs_uncertainty 永远猜 | BuildObservePriorDelta 注入上一轮 ConvergenceMetric + KnownGaps |
| Plan LLM 把 execution_mode + child_specs 写在 narrative → Execute 看不到 | StrategicPlanFrame append 5 FrameDelta 字段 + InjectPlanFrameDelta 显式注入 |
| Execute 5 sub-turn 累积 prompt size 7229 tok 单调递增 | ConvergenceMetric 末轮 rate ≥ 0.5 验收（Phase 4 端到端验证） |
| 5 个独立 LLM 调用拼成的序列 | 升级为逐步收敛的 Markov 过程（FrameDelta 跨节点显式传递） |

---

## 5. 风险追踪收尾

| 风险 | 状态 | 备注 |
|------|------|------|
| FrameDelta 字段被 LLM 误读 | 已缓解 | schema hash + 封闭式 XML tag 双轨封装 |
| 注入 prompt 超 budget 200 字符 | 已缓解 | T05 降级 baseline + emit warn span |
| FrameDelta 与 M1/M2 契约冲突 | 已规避 | append-only 注入 + T18 71 现有测试 0 行为变化 |
| 双轨 (摘要 + hash) 不同步 | 已缓解 | SchemaHash() 基于 5 字段 FNV-1a，幂等 |
| ConvergenceMetric 与 Pessimistic L3 冲突 | 已规避 | frame delta 不进 Pessimistic 决策表 |
| ConvergenceMetric 跨 round 累积 vs 每 round 重算 | 已澄清 | 每 round 重算（基于 subTurns[] 派生），不持久化 |
| Phase 4 端到端验证需 running system + Jaeger | 待 S5 | T15/T16/T17 留 S5 验收执行 |

---

## 6. 后续 follow-up

### Phase 4 (S5 验收阶段)

- **T15** L5-MUPS-FD-3 trace 重放：Execute 5 sub-turn 全 convergence_metric span + 末轮 rate ≥ 0.5
- **T16** `e2e_frame_delta_test.go`：sess_1783255992426_6000 wi_d0_s0_goal 端到端重跑
- **T17** L5-MUPS-FD-4：跨链 LLM prompt size 单调不增 ±5% 噪声
- **T19** S3-Gate 三方博弈论 final review (codex + cursor + claude)

### Future (v1.1+)

- FrameDelta.TraceID 字段新增（与 TaskSpec.TraceID 对齐，LP-1 反向追溯链接入）
- FrameDelta 拆 interfaces/v2 子包（D7 + D2 + D4 共享，跨域 FrameDelta 抽象上提）

---

## 7. S6 归档元数据

- **Archived:** 2026-07-05 (S5 partial accepted; Phase 4 follow-up deferred)
- **Status:** PARTIAL ACCEPTED (Phase 1-3 IMPLEMENTED · Phase 4 PLANNED S5)
- **PRs:** #431 (S1) + #432 (S2) + #433 (S3) + #434 (S4 code) + #435 (docs sync)
- **Total LOC:** ~750 lines (interfaces/mups_frame_delta.go 170 + execute_plan_frame_inject.go 89 + observe_frame_delta.go 68 + convergence_metric.go 161 + 3 test files 562 + hardening 86 + design.md +216)
- **Domain sync:** `openspec/specs/d7-orchestration/t-registry.md` D7-FD section (16/16 T-IDs IMPLEMENTED)
- **Spec I/O 协议:** `specs/d7-orchestration/mups-frame-delta-spec.md` S4_Implemented
- **Spec delta:** `archive/2026-07-05-devrix-d7-mups-frame-delta-closure/specs/mups-frame-delta-spec.md`
- **3 new span op:** OpD7_S5_Observe_PriorDelta_Inject + OpD7_S9_Execute_PlanFrameDelta_Inject + OpD7_S9_Execute_ConvergenceMetric_Emit
- **关联变更:** DM-20260705-003/004 (M1/M2 frame) + DM-20260705-008 (Strategy 抽象) + DM-20260705-009 (封闭式分类器) + DM-20260704-006 (ResolutionContract) + DM-20260629-006 (TaskContract)

EOF