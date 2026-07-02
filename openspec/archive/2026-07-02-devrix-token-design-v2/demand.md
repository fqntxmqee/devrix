# Demand: DM-20260702-008 — Token Design 2.0

**Demand ID:** DM-20260702-008
**Created:** 2026-07-02
**Priority:** P0
**Source:** 复盘 DM-20260701-007 (PR-B/C/D 8K token 治本) + clawcode 源码深读

---

## 1. 问题陈述 (复盘发现)

DM-20260701-007 (PR #374 + PR #375, S7_archived) 通过 4-PR 联动实现 "8K token 自我循环治本", 33/33 T 全部 IMPLEMENTED。但 2026-07-02 复盘发现 8K token 截断 + Bounded(15) hard reject 部分**治标不治本**, 跟 PR #373 红卡本质一样, 只是失败点从 D1 表现层挪到 D7 channel 层。

### 1.1 3 个根因 (复盘)

**RC-1: 信息物理丢失**
- 现象: devrix 8K chars 截断 (`compression_steps.go:14-19`) → 物理消失
- 后果: LLM 看到 marker 后只能 REREAD, 但 REREAD 也截断 → 死循环 → Bounded(15) hard reject → 任务失败
- vs clawcode: 持久化到 `<session>/tool-results/<toolUseId>.txt` + 2KB preview + 路径引用, 信息永远可达

**RC-2: 阈值偏低 + 缺差异化**
- 现象: devrix 19 工具 MaxResultSizeChars 全部 8K chars uniform
- 后果: bash 50K 输出截 8K 不够, edit/write 100K 截 8K 不够, webfetch 100K markdown 截 4K-8K 不够
- vs clawcode: per-tool 差异化 (Read=Infinity, Bash=30K, Grep=20K, Edit/Write=100K, Web*=100K) + 全局 50K + per-message 200K + token 100K

**RC-3: 强 reject = 治标**
- 现象: `probe.go:78-82` 返 `ErrProbeToolChannelBoundExceeded` 触发任务失败
- 后果: 任务失败 → 走 D7 Verify → 标 task_incomplete → D1 红卡 → 用户重发
- vs PR #373: 失败模式本质一样, 失败点从 D1 表现层挪到 D7 channel 层
- vs clawcode: 无 iteration bound, LLM 自由探索, Read 自带 offset/limit 自治

### 1.2 借鉴关系

| 项 | devrix 现状 | clawcode 真实做法 | 差距 |
|----|------------|------------------|------|
| 截断单位 | 800 tokens (config) + 8K chars (metadata) | chars (统一) | 单位不统一 |
| 默认阈值 | 800 / 8K uniform | 50K 全局 + per-tool 差异化 | 阈值偏低 |
| Read 工具 | 8K chars 强截 | Infinity (自带 offset/limit) | devrix 缺 offset/limit |
| 截断产物 | 物理消失 + in-content marker | 持久化 + 引用 + preview | 信息丢失 |
| Iteration bound | Bounded(15) hard reject | 无 (LLM 自治) | 强 reject 治标 |
| 并发/串行 | 无分桶 (turn-level 串行) | isConcurrencySafe 分桶 | 浪费并发 |
| 决策稳定性 | 无 decision freeze | ContentReplacementState | 重放可能不同 |
| 运行时调参 | 改 config 需重启 | GrowthBook feature flag | 调参慢 |
| 第二道安全 | 无 (Verify 是事后) | toAutoClassifierInput + classifier | 缺中间层 |
| Image block | 一律 tiktoken 切 | hasImageBlock 跳过 | image 损坏 |

### 1.3 保留 devrix 创新 (clawcode 缺)

- **EmissionClass 4 类 (Fact/Action/Probe/Experiment)** — 架构性创新
- **task_kind 推 Filter v2** — 创新
- **VerifyContract 4 元组 (事后治本)** — 创新
- **MUPS 5 节点 × 4 类正交分解** — 架构性创新
- **Learn FeedbackMemory (H7 reputation)** — 创新
- **LTL-Lite L4-L6 (改 advisory)** — 创新
- **InterruptBehavior / RiskLevel / ShouldDeferByDefault** — 已实现

---

## 2. 目标

### 2.1 治本目标

| 目标 | 衡量 | 现状 | 目标 |
|------|------|------|------|
| 信息不丢失 | 8K 截断触发 REREAD 次数 | 9+ 死循环 | 0 (persist + offset/limit) |
| 任务成功率 | PR #373 case 100 次跑 | 0/100 失败 | 100/100 成功 |
| 阈值合理 | 19 工具 MaxResultSizeChars | 全部 8K uniform | per-tool 差异化 |
| N parallel 累加 | per-message 200K aggregate | 无 (潜在风险) | 有 |

### 2.2 保留目标

- VerifyContract 4 元组 (沿用, 不改)
- EmissionClass 4 类路由 (沿用, 不改)
- task_kind 推 Filter v2 (沿用, Bounded 改 advisory)
- Learn FeedbackMemory (沿用, 不改)
- MUPS 5 节点 × 4 类正交分解 (沿用, 不改)
- InterruptBehavior / RiskLevel / ShouldDeferByDefault (沿用, 不改)

### 2.3 不在本次目标 (走下个 change)

- isConcurrencySafe: P1, DM-20260702-009
- toAutoClassifierInput: P1, DM-20260702-009
- GrowthBook runtime override: P2
- decision freeze: P2 (跟 isConcurrencySafe 一起)

---

## 3. 关联需求

### 3.1 Supersede (narrow)

- DM-20260701-007 partial: 5 T 点 (TruncateWithMarker / Bounded(15) / 19 工具 8K / LTL-Lite L4-L6 / D2 物理 truncate 路径)
- 详见 archive/2026-07-02-devrix-mups-tool-classification-and-channel-autonomy/SUPERSEDE-NOTICE.md

### 3.2 Related (上游)

- DM-20260701-007 (MUPS 5 节点 × Tool 元数据 Control Plane + ToolChannel 自治) — 4-PR 治本基础
- DM-20260701-005 (D7 Verify synthesize enforce) — 治本缺失的现状 PR
- DM-20260701-006 (D2 tool_result budget profile) — task_kind 路由
- DM-20260630-012 (D7 deliverable convergence) — task_incomplete 标识修

### 3.3 Related (前置)

- DM-20260629-001 (D7 DSAFT restructuring) — Span Evidence 100%
- DM-20260618-001 (Tool Spec v2 + CheckPermission + DeferLoading) — 现有 9 字段基线
- DM-20260617-008 (Tool Surface Phase 2 full) — 12→0 global loop
- DM-20260617-007 (D7 Tool Surface Contract S1-S3) — 7 surface + 3 filter
- DM-20260625-019 (D7 5-node coverage) — MUPS Phase 3 PR-C1 跨域类型
- DM-20260626-005 (D7 6S Verify promotion) — executionflow/verify/ 物理 promote
- DM-20260625-011 (D7 5-Node Mups PHASE-1) — 5 节点 routing 基础
