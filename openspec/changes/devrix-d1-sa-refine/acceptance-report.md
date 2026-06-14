---
acceptance-id: devrix-d1-sa-refine
phase: S5_Acceptance
demand-id: DM-20260614-006
status: ACCEPTED
created: 2026-06-14
---

# Acceptance Report — devrix-d1-sa-refine

**Change:** D1 Communication — 切法 A 信号分层（S13–S18）  
**Demand:** DM-20260614-006  
**范围:** v1.0 registry + v1.1 契约/span/acceptance（Phase 3 结构拆包 Out of Scope）

---

## 1. AC 验收清单

| AC | 标准 | 优先级 | 证据 | 结论 |
|----|------|--------|------|------|
| AC1 | S13–S18 六场景 + Legacy S1–S12 冻结 | P0 | `openspec/specs/architecture/layering.md` v3.4；`d1-communication/spec.md` v3.0 | ✅ |
| AC2 | Thinking/Task/Conclusion 各有 S + A + Gherkin | P0 | `spec.md` §Requirements Canonical Gherkin；`a-registry.md` S14–S16 | ✅ |
| AC3 | S13 Persist + Dispatch 单 A，分支为 F | P0 | `design.md` §3.1；`f-registry.md` S13-A03-F01/F02/F03 | ✅ |
| AC4 | `IMOutboundSignal` 三 Kind 落地 contracts | P1 | `internal/shared/contracts/im_outbound_signal.go` | ✅ |
| AC5 | 44 Legacy T + Canonical 列，v1.0 不改注释 | P0 | `t-registry.md` §Legacy 44 行 | ✅ |
| AC6 | Conclusion Critical ↔ S18 + span | P0 | `eventbus/` Drain/Critical 测试全绿；`span-registry.md` `eventbus.publish_critical`；complete 经 journey 必达 | ✅ |
| AC7 | S3-Gate Approved；v1.0 阶段无 Go（已满足） | P0 | `design.md` Grill Review；Phase 1 已 merge | ✅ |
| AC8 | Trusted Intermediary 完备性边界 | P0 | `demand.md` §1.3；`design.md` Decision；`gaming-analysis.md` §最终三方共识 | ✅ |
| AC9 | 客观锚点；禁止 Agent 自填 Confidence | P0 | 契约字段 + journey 断言 `source_event_id` / `inbound_turn_id` | ✅ |
| AC10 | `d1.signal.chain_integrity` + E2E journey | P0 | `gateway/signal_hooks.go`；`d1_signal_journey_test.go` PASS | ✅ |
| AC11 | Task work_proof span 登记 + tool 链 | P1 | `span-registry.md` `d1.signal.task.work_proof`；journey 含 tool_call | ✅ |

**P0：11/11 通过。P1：2/2 通过。**

---

## 2. 测试执行证据

### 2.1 D1 通信域单元测试

```
$ go test -count=1 ./internal/layers/communication/...
ok  .../gateway      3.254s
ok  .../eventbus     3.587s
ok  .../signal       3.940s
ok  .../adapters     1.969s
(... 全包 PASS)
```

### 2.2 契约与信号映射

```
$ go test -count=1 ./internal/shared/contracts/ -run 'MapEngine|Feedback' -v
--- PASS: TestMapEngineEventToSignal_kinds
--- PASS: TestParseConclusionFeedback
ok  github.com/devrix/devrix/internal/shared/contracts
```

```
$ go test -count=1 ./internal/layers/communication/signal/ -v
--- PASS: TestTurnTracker_Next_chainIntegrity
--- PASS: TestTurnTracker_regKindBreak
ok  github.com/devrix/devrix/internal/layers/communication/signal
```

### 2.3 S18 Critical / Drain（AC6）

```
$ go test -count=1 ./internal/layers/communication/eventbus/ -run 'Critical|Drain'
--- PASS: TestCompactSkipsCritical
--- PASS: TestDrainPreservesCritical
--- PASS: TestReconnectPreservesCritical
ok  .../eventbus  0.363s
```

### 2.4 Acceptance — 信号全链（AC9/AC10）

```
$ go test -tags='acceptance d1' ./tests/acceptance/p0/ -run 'D1_Signal|D1_Conclusion' -v
--- PASS: TestL5_D1_SignalJourney_CaptureToConclusion
--- PASS: TestL5_D1_ConclusionFeedbackCapture
ok  github.com/devrix/devrix/tests/acceptance/p0  0.707s
```

**Journey 断言：** thinking / task / complete 三类 `signal_kind`；每条 outbound 含 `source_event_id`、`inbound_turn_id=turn-1`。

### 2.5 观测 registry

```
$ go test -count=1 ./internal/layers/observability/coverage/...
ok  .../coverage  (64 ops incl. d1.* + user.feedback.*)
```

---

## 3. Canonical T 状态更新（v1.1）

| T ID | 状态 | Test 位置 |
|------|------|-----------|
| D1-S13-A02-T01 | **IMPLEMENTED** | `tests/acceptance/p0/d1_signal_journey_test.go` |
| D1-S14-A01-F01-T01 | **IMPLEMENTED** | 同上（Kind=thinking；P99 延迟指标待 D5 压测） |
| D1-S15-A01-F01-T01 | **IMPLEMENTED** | 同上（tool_call → task） |
| D1-S16-A02-T01 | **IMPLEMENTED** | 同上（complete → conclusion） |
| D1-S18-A01-F02-T01 | **IMPLEMENTED** | `eventbus/bus_test.go`（Legacy T 覆盖，canonical 追溯） |
| D1-S18-A01-F03-T01 | **IMPLEMENTED** | `eventbus/drain_test.go` |
| D1-S13-A03-T01/T02 | PLANNED | 已有 `coordinator_matrix_test.go`，待 canonical `// T:` |
| D1-S13-A04-T01 | PLANNED | `permission_test.go`（Legacy T01 绿，canonical 注释待 2.6） |
| D1-S15-A02-F01-T01 | PLANNED | `feishu_worker_card_test.go`（Legacy T） |
| D1-S16-A02-T02 | PLANNED | error journey 待补 |
| D1-S17-A01-T01 | PLANNED | 飞书 Parse + 信号链 E2E 待补 |

---

## 4. 交付物清单

| 产物 | 路径 | 状态 |
|------|------|------|
| demand.md | change 包 | ✅ S3_Approved |
| proposal.md | change 包 | ✅ |
| design.md | change 包 | ✅ |
| tasks.md | change 包 | ✅ |
| gaming-analysis.md | change 包 | ✅ |
| layering v3.4 | `openspec/specs/architecture/` | ✅ merged |
| d1 registries v3.0 | `openspec/specs/d1-communication/` | ✅ merged |
| IMOutboundSignal | `internal/shared/contracts/` | ✅ |
| signal turn tracker | `internal/layers/communication/signal/` | ✅ |
| gateway 集成 | `gateway/signal_hooks.go` 等 | ✅ |
| acceptance | `tests/acceptance/p0/d1_signal_journey_test.go` | ✅ |

---

## 5. 影响评估

| 维度 | 影响 |
|------|------|
| Legacy 44 T | 无注释变更；行为兼容 |
| Outbound Metadata | 新增 `signal_*` / `source_event_id` / `elapsed_ms` 字段（additive） |
| `/feedback ` 入站 | 新命令：捕获反馈，不 Dispatch（breaking 仅对该前缀） |
| 性能 | turn tracker O(1)/event；span 仅在 obsBridge 非 nil 时创建 |
| Phase 3 | 未启动（outbound 拆包 / Milestone 迁 D7） |

---

## 6. 例外与后续

| 项 | 说明 | 跟踪 |
|----|------|------|
| S14 P99 < 800ms | journey 未压测延迟 | D5 指标 + 压测 follow-up |
| Canonical `// T:` 迁移 | tasks 2.6 未完成 | 可选 PR |
| 信誉/评级/惩罚 | Out of Scope | DM-20260614-007 |
| IM 注意力 UX | P2 | 产品迭代 |

---

## 7. 决议

**ACCEPTED** — P0 全绿，P1 全绿。允许进入 **S6 交付 / S7 归档**（合 main 后归档 change 包至 `openspec/archive/`）。
