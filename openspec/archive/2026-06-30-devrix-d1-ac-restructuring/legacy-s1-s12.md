# D1 Legacy Scenarios — D1-S1 ~ D1-S12（冻结追溯）

**Demand ID:** DM-20260629-005 (PR-4 #2 registry-sync)
**Original Change ID:** devrix-d1-sa-refine (DM-20260614-006)
**Archived:** 2026-06-30
**Status:** Frozen (read-only)
**Reason:** D1 域 v2.0 价值流切法 A 双轨完成（S13–S18 取代 S1–S12），S1–S12 详细 A/F 表已沉到本 archive，仅保留追溯。

---

## §1 冻结索引

| Legacy S | 主题 | Canonical 归属 | 迁移路径 |
|----------|------|---------------|----------|
| D1-S1 | Gateway | D1-S13 CaptureUserIntent | `gateway/` → `capture/` |
| D1-S2 | Adapters | D1-S17 ConnectChannel | `adapters/` → `channel/adapters/` |
| D1-S3 | Commands | D1-S13-A05 ParseCommand | `commands/` → `channel/adapters/cli/` 内嵌（CLI 即 IM 入口） |
| D1-S4 | Worker (legacy signal) | D1-S15-A02 WorkerProgress | `worker/` → `present/` + `channel/adapters/feishu_worker_card.go` |
| D1-S5 | Milestone | D1-S15-A01 EmitMilestoneProgress (部分迁 D7) | `milestone/` → `orchestration/milestone/` (D7 owns) |
| D1-S6 | RateLimit | D1-S17-A06 CheckRateLimit | `ratelimit/` → `channel/ratelimit/` |
| D1-S7 | SessionStore | D1-S13-A02-F01 CreateSession | `session/` → `capture/session_store.go` |
| D1-S8 | Renderers | D1-S17 Encode F | `renderers/` → `channel/renderers/` |
| D1-S9 | EventBus | D1-S18 GuaranteeDelivery | `eventbus/` → `delivery/eventbus/` |
| D1-S10 | Connection | D1-S17-A04 ManageConnection | `connection/` → `channel/connection/` |
| D1-S11 | Core (Card/Session model) | 横切 kernel | `core/` → `kernel/` |
| D1-S12 | Instance Registry | D1-S17-A05 RegisterInstance | `instance/` → `channel/instance/` |

> 完整 frozen A/F 表见 `openspec/archive/2026-06-14-devrix-d1-sa-refine/`（S1–S6 frozen A/F）+ `openspec/archive/2026-06-28-devrix-d1-dsaft-refactor/`（S7–S12 frozen A/F + Gateway 拆分 + contracts DTO + import boundary lint）。

---

## §2 Legacy T 层（冻结追溯）

> 来源：`openspec/specs/d1-communication/t-registry.md` v3.3.0 §Legacy T（44 条 IMPLEMENTED）。每条 T 行已含 Span Evidence 列（PR-4 #2 落地后），其中 Legacy 默认 `—` 表示 legacy span 已退役，登记至 `span-registry.md` §Legacy。

### 2.1 D1-S1 Gateway Module (9 T)

- D1-S1-A01-T01..T05: Session 生命周期 + Inbound/Outbound 路由
- D1-S1-A02-T01: Inbound/Outbound 全链路
- D1-S1-A03-T01..T02: Permission YOLO + Lifecycle
- D1-S1-A04-T01: AgentFactory 注入（SUPERSEDED → D1-RF-T02）

### 2.2 D1-S5 Milestone Module (3 T)

- D1-S5-A01-T01..T03: Milestone 环检测 + TaskFlow + CRUD

### 2.3 D1-S3 Commands Module (3 T)

- D1-S3-A01-T01..T03: `/new` `/help` `/stop` 命令解析

### 2.4 D1-S8 Renderers Module (3 T)

- D1-S8-A01-T01..T03: ShortId / ProgressBar+StatusBadge / CLIRenderer

### 2.5 D1-S2 Adapters Module (14 T)

- D1-S2-A01-T01..T02: 飞书 / 钉钉 Parse
- D1-S2-A02-T03..T09: 钉钉 milestone + CardKit 流式 + /stop 清理
- D1-S2-A03-T01..T02: WorkerCard 双块
- D1-S2-A04-T01..T02: Cardkit CreateCard/Stream
- D1-S2-A05-T01: Session 内存/磁盘/新建三级回退

### 2.6 D1-S9 EventBus Module (7 T)

- D1-S9-A01-T01: BackpressureEventBus 正常流
- D1-S9-A02-T02..T04: Drain / Compact / Reconnect
- D1-S9-A01-T05..T07: Critical complete/error 必达 + Publish 背压

### 2.7 D1-S10 Connection Module (2 T)

- D1-S10-A01-T01..T02: 心跳指数退避 + Register/Unregister 生命周期

### 2.8 D1-S6 RateLimit Module (3 T)

- D1-S6-A01-T01..T03: 令牌桶 + adapter 隔离 + HTTP 中间件

---

## §3 重新评估触发条件（DM-20260629-005 PR-4 同步 D4 模板）

| 触发 | 行动 |
|------|------|
| 任何 S1–S12 代码被生产代码引用 | 立即迁移到 Canonical S13–S18 + archive 索引 |
| 任何 Legacy T 引用被新测试覆盖 | 在 `t-registry.md` 加 canonical 等价 + `supersedes` link |
| 任何 D1 跨域 boundary 变化（如新增 D7/D4 接线） | 更新 `openspec/specs/architecture/d1-flow-architecture.md` §跨域接线 |
| v2.0+ 进入 v3.0（federation / multi-tenant） | 重启切法 B 评估：`d1-domain.md` §North Star 重写 + Legacy S 沉更深 archive |

---

**END of D1 Legacy Archive (D1-S1 ~ D1-S12)**