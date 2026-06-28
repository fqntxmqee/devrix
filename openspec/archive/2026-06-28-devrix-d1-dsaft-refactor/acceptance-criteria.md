# D1 DSAFT Refactor — Acceptance Criteria

**Change ID:** `devrix-d1-dsaft-refactor`  
**Demand ID:** DM-20260628-003  
**North Star:** Trusted Intermediary — 四条流出站 + 命令通道，编排边界仅 D7

---

## 1. AC 分层

| 层 | 前缀 | 含义 |
|----|------|------|
| **LC** | LC-* | Demand 级可验证承诺（用户动线） |
| **AC-S** | AC-S13…S18 | 按 S 场景端到端 |
| **AC-A** | AC-A01… | 按 A 活动链路节点 |
| **AC-R** | AC-R* | 本 refactor 边界回归（Phase 1） |

---

## 2. Demand 级 LC（与目标对齐）

| ID | 承诺 | 验证方式 | 状态 |
|----|------|----------|------|
| **LC-1** | 指令不丢、可追、可续聊 | S13 入站 persist + turn_id 锚点 | ✅ `D1-S13-A02-T01`, signal journey |
| **LC-2** | 思考/任务/汇总三类出站可见 | S14/S15/S16 SignalKind 链 | ✅ `d1_signal_journey_test` |
| **LC-3** | 入站仅 dispatch D7 | 无 D2.Process / 无 Agent.Run 劫持 | ✅ `D7-D1-T01..T03`, **AC-R2** |
| **LC-4** | Critical 结论/错误必达 | complete/error 不被 Drain | ✅ `D1-S18-A01-F02/F03-T01` |
| **LC-5** | D1 capture 不 import D4/D7 实现包 | 静态 import 门禁 | 🆕 **AC-R1** |
| **LC-6** | Session leader 在 bootstrap，不在 D1 | beforeDispatch hook | 🆕 **AC-R2..R4** |

---

## 3. S 场景 AC（链路覆盖）

### AC-S13 — 指令流（CaptureUserIntent）

| ID | 链路 | A 覆盖 | 现有 T | 缺口 |
|----|------|--------|--------|------|
| AC-S13-1 | Parse → Accept → Persist → Dispatch(D7) | A01→A02→A03 | S13-A02-T01, S13-A03-T01/T02, D7-D1-T01 | — |
| AC-S13-2 | beforeDispatch → EnsureSessionLeader（bootstrap） | A03 前置 hook | D7-D1-T02 | **AC-R2** 显式登记 |
| AC-S13-3 | 权限门控 CRITICAL 不自动批 | A04 | S13-A04-T01 | — |
| AC-S13-4 | 结论 feedback 不触发 dispatch | A02 分支 | L5 feedback test | — |
| AC-S13-5 | 会话命令 /new /stop /help | A05 | comm_commands_test | E2E stop 流：stop_session_flow_test ✅ |

### AC-S14 — 思考流

| ID | 链路 | A 覆盖 | 现有 T | 缺口 |
|----|------|--------|--------|------|
| AC-S14-1 | EngineEvent.thinking → Signal(Thinking) → Encode | A01 | S14-A01-F01-T01 | S16-A01 text 同文件已覆盖 thinking |

### AC-S15 — 任务流

| ID | 链路 | A 覆盖 | 现有 T | 缺口 |
|----|------|--------|--------|------|
| AC-S15-1 | tool_call/result → Task 信号 | A01 | S15-A01-F01-T01 | — |
| AC-S15-2 | worker_progress → Worker 双卡 | A02 + S17-F02 | S15-A02-F01-T01 | Phase 3 DTO |
| AC-S15-3 | milestone_progress → Task 卡 | A01-F03 | legacy S5 tests (D7) | D1 presenter 单测可补 |

### AC-S16 — 汇总信息流

| ID | 链路 | A 覆盖 | 现有 T | 缺口 |
|----|------|--------|--------|------|
| AC-S16-1 | text delta → Conclusion 非终态 | A01 | signal journey (text 事件) | 可拆独立 T |
| AC-S16-2 | complete → Critical Conclusion | A02 | S16-A02-T01 | — |
| AC-S16-3 | error → Critical Conclusion | A02 | S16-A02-T02 | — |

### AC-S17 — 命令通道 + 编解码

| ID | 链路 | A 覆盖 | 现有 T | 缺口 |
|----|------|--------|--------|------|
| AC-S17-1 | Feishu 入站 Parse | A01 | S17-A01-T01 | — |
| AC-S17-2 | DingTalk 入站/出站 | A02 | dingtalk_test | — |
| AC-S17-3 | CLI 入站 | A03 | cli_test | — |
| AC-S17-4 | Connection 生命周期 | A04 | connection/*_test | — |
| AC-S17-5 | Instance Register | A05 | registry_test | 保持现状 ✅ |
| AC-S17-6 | RateLimit | A06 | limiter_test | — |
| AC-S17-7 | CardKit 流式 Encode | F01 | feishu_streaming_test | — |

### AC-S18 — 必达

| ID | 链路 | A 覆盖 | 现有 T | 缺口 |
|----|------|--------|--------|------|
| AC-S18-1 | Publish / Drain / Compact / Reconnect | A01-F01/F03/F04/F05 | eventbus *_test | — |
| AC-S18-2 | PublishCritical complete/error | A01-F02 | bus_test, signal journey | — |

---

## 4. Refactor 边界 AC（Phase 1 新增）

| ID | Given | When | Then | T ID |
|----|-------|------|------|------|
| **AC-R1** | `capture/*.go` 生产代码 | 静态扫描 import | 无 `multiagent`、`orchestration/` | **D1-RF-T01** |
| **AC-R2** | D7 entry + sessionagents wired | RouteInbound | ProcessMessage 调用 1 次；leader Created 未 Run | **D1-RF-T02** (= D7-D1-T02 迁登记) |
| **AC-R3** | Agent permission_required | RoutePermission → Resolve | Agent.ResolvePermission 收到结果 | **D1-RF-T03** |
| **AC-R4** | 无 active process + orphan EngineEvent | Agent sink | DeliverOrphanEngineEvent → 出站 | **D1-RF-T04** |
| **AC-R5** | orchestrationEntry nil | RouteInbound | 错误返回，beforeDispatch 不 bypass | **D1-RF-T05** (= D7-D1-T03) |

---

## 5. 覆盖缺口汇总（待 Phase 1.5 / 2 关闭）

| 优先级 | 缺口 | 计划 |
|--------|------|------|
| P0 | AC-R1..R5 登记到 t-registry | Phase 1.5 本 PR |
| P0 | terminal-state-guide F03 routeAgent 迁 bootstrap | Phase 1.5 文档 |
| P1 | S16-A01 text delta 独立 T | ✅ D1-RF-T06 |
| P1 | milestone_progress D1 presenter 单测 | ✅ D1-RF-T07 |
| P1 | Gateway 拆分后 A03 单测锚点迁移 | ✅ Phase 2 完成 |
| P2 | channel 零 orchestration import | ✅ D1-RF-T09 + contracts DTO |
| P2 | Legacy S1-A04-T01 routeAgent 测试更新/退役 | t-registry 标注 SUPERSEDED |

---

## 6. P0 Runbook（生产可验证）

与 `observability-guide.md` 对齐：

1. Jaeger：`D1_Capture_RouteInbound` → `D7_ProcessMessage` → `D1_Signal_*` → `D1_EventBus_PublishCritical`
2. 飞书 E2E：thinking 区 + tools 区 + complete 卡
3. `/stop` 后同 session 续聊（`stop_session_flow_test` 自动化代理）

---

## 7. AC → 测试文件映射（Canonical）

| T ID | 文件 |
|------|------|
| D1-RF-T01 | `capture/import_boundary_test.go` |
| D1-RF-T02 | `capture/coordinator_integration_test.go` + `tests/acceptance/p0/d1_dsaft_refactor_test.go` |
| D1-RF-T03 | `sessionagents/manager_test.go` |
| D1-RF-T04 | `sessionagents/manager_test.go` |
| D1-RF-T05 | `capture/coordinator_integration_test.go` |
| L5 四流 | `tests/acceptance/p0/d1_signal_journey_test.go` |
| L5 命令 | `tests/acceptance/p0/comm_commands_test.go` |
