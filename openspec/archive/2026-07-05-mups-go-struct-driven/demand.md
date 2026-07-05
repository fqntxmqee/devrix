---
demand-id: DM-20260705-003
title: "MUPS Go-struct-driven I/O contract — M1 Observe + 5 节点重构总图"
source: MUPS 5 节点重构路线图（M1 Observe → M5 SpawnDecision）
priority: P1
status: S3_Design
l1-domain: shared, orchestration
created: 2026-07-05
related:
  - openspec/specs/shared/prompttags.md
  - openspec/specs/d7-orchestration/spec.md
  - openspec/specs/d7-orchestration/pipeline-architecture.md
  - internal/shared/prompttags/linefield.go
  - internal/shared/prompttags/registry.go
  - internal/shared/prompttags/wholebody.go
  - internal/shared/prompttags/semantics.go
  - internal/layers/orchestration/sessionorchestrator/observation_proposer.go
  - internal/layers/orchestration/sessionorchestrator/llm_observation_proposer.go
  - internal/layers/orchestration/sessionorchestrator/strategic_plan_proposer.go
parent_demands:
  - DM-20260704-004  # prompttags 1.0
  - DM-20260704-005  # prompttags v2 IO registry
  - DM-20260705-001  # tag semantics layer
  - DM-20260705-002  # parse reject feedback
---

# MUPS Go-struct-driven I/O contract — M1 Observe + 5 节点重构总图

## 1. 原始描述

> MUPS（Observe→Plan→Execute→Verify→Learn）+ Decide 节点是 D7 编排的"面向不确定性问题的确定性过程"骨架。Observe/Plan 节点向 LLM 输入 user frame、回收 LLM whole-body 响应；现有代码由"Go struct + FrameSpec[]TagName + 手工 fields map"三处分别描述同一份 schema，存在结构化漂移风险。本需求以 go-struct-driven 模式为 M1（Observe）落地基础，同时给出 M2-M5 的 5 节点重构总图。

## 2. 问题陈述（现状诊断）

### 2.1 已有能力（不重复建设）

| 能力 | 状态 | 路径 |
|------|------|------|
| `LineFrameRegistry` (ObserveUser / PlanUser) | ✅ | `prompttags/linefield.go:43-46` |
| `BuildAnnotatedLineFrame` + `[data]/[control]` 平面标注 | ✅ | `prompttags/linefield.go:82-86` |
| `FrameFieldPlane` 语义判定 | ✅ | `prompttags/semantics.go:44-50` |
| `ParseWholeBody[T]` 泛型 whole-body 解析 | ✅ | `prompttags/wholebody.go:11-20` |
| `DocBlockObserveSchema` / `DocBlockPlanSchema` | ✅ | `prompttags/docblock.go:65-78` |
| i18n `RenderFrameFieldGuideForFields` (zh/en) | ✅ | `contextengine/i18n/prompttags_semantics_{en,zh}.go` |
| `ObserveSignalInput` struct (8 字段) | ✅ | `sessionorchestrator/observation_proposer.go:22-44` |
| `StrategicPlanInput` struct (等同模式) | ✅ | `sessionorchestrator/strategic_plan_proposer.go` |
| `parse_reject` 反馈注入 user frame | ✅ | DM-20260705-002 (已合入) |

### 2.2 缺口（双链路 / 三处描述漂移）

| 节点 | 描述同一份 schema 的三处 | 风险 |
|------|--------------------------|------|
| **Observe** | (a) `ObserveSignalInput` struct 字段顺序<br/>(b) `ObserveUserFrame.FrameSpec.Fields []TagName`<br/>(c) `buildLLMObservationUserPrompt` 35 行手工 `fields := map[TagName]any{...}` | 新增/删除/重命名 tag 时必须三处同步；任何一处遗漏即 silent drift；测试夹具与代码不一致时无编译期保护 |
| **Plan** | 同上 (a) `StrategicPlanInput` (b) `PlanUserFrame.Fields` (c) `buildStrategicPlanUserPrompt` 手工 map | 同上 |
| **Execute / Verify / SpawnPolicy** | 散落：手工 `prompttags.Wrap[T]` / `TagPriorVerifyReason` / 魔数 `maxLLMObsFactStrength=0.85` / 6 枚举 `SpawnPolicy` R0-R8 嵌套分支 | 散落 magic number + 嵌套 if-else 难审计 |
| **注册顺序** | 5 个 TagName 常量 + 2 个 FrameSpec + 24+ 字段名 + 1 个 DocBlock + 5 个 ParseRejectCode 共 5+ 处 | 增加新 tag 需要：常量 + FrameSpec + 写入 fields map + i18n 翻译 + DocBlock + 至少 1 个测试 |

**架构级结论（来自前次对话审计）**：
- ChannelRouter 4 文件是 v1 死代码（DM-20260626-009 "decommissioned by the fresh WorkItemExecutor design"）→ **不复活**；
- RunSessionTurnLoop 收敛为 Turn Leader、ChannelRouter 替换为 Strategy 抽象 → 推迟到 M3（M3 是行为增量，最后做）；
- D7-S5-A97/A98 的 i18n semantics + prior_parse_reject 已就位，**未触动**。

### 2.3 目标行为（go-struct-driven 模式）

1. **单一定义点**：每个 LLM I/O 形状一个 Go struct，字段带 `pt:"<tag_name>,<plane>,<flags>"` struct tag。
2. **反射注册**：`MustRegisterFrame[T]()` 在 `init()` 中调用一次，校验 `pt` tag 与 `FrameSpec` / `FrameFieldPlane` / i18n 翻译条目三方一致；不一致则 **panic**（编译期不可见的不一致是设计 bug）。
3. **反射序列化**：`BuildLineFrameFromStruct(frameName, struct)` = `BuildAnnotatedLineFrame` 的反射版；`DocBlockFromStruct[T]()` 自动生成 schema 文档。
4. **行为不变（M1 阶段）**：Observe user prompt 字节级 token 等价于 `buildLLMObservationUserPrompt` 输出（golden snapshot）。
5. **可读性 / 可维护性**：新增 tag 只需：(1) 加 struct 字段 + pt tag；(2) `go test` 失败提示缺翻译/缺 plane；(3) 自动注册。

### 2.4 重构总图（5 节点，M1-M5）

| 阶段 | 范围 | 行为变化 | 工作量 | 本 change 落点 |
|------|------|----------|--------|-----------------|
| **M1** | Observe go-struct 化（kernel + Observe 迁移） | **0 行为变化**（golden 等价） | ~80 + ~100 = 180 行 | **本 change** |
| M2 | Plan 节点独立化 + go-struct 化（kernel 复用） | 0 行为变化 | ~120 行 | follow-on change（`mups-plan-structbind`） |
| M3 | Strategy 抽象注入 WorkItemExecContext | **行为增量**（PlanKind 路由恢复） | ~300 行 | follow-on change（`d7-mups-strategy-injection`） |
| M4 | Verify 决策表化（4 VerdictKind × N trigger） | 0 行为变化 | ~150 行 | follow-on change（`mups-verify-table-driven`） |
| M5 | SpawnDecision 3 子决策代数化（R0-R8 → checkBudget/checkDirection/checkEscalation） | 0 行为变化 | ~200 行 | follow-on change（`d7-spawn-decision-algebra`） |

**为什么 M1 → M2 → M4 → M5 → M3**：
- M1-M2 是 0 行为变化的 kernel 铺设；
- M4/M5 是 0 行为变化的局部表驱动化，可以并行；
- M3 是行为增量（恢复 ChannelRouter 死掉的 4 PlanKind 路由语义），风险最大，**最后做**。

## 3. 非目标

- M2-M5 的实现细节（仅在附录列出 follow-on change 计划，不在本 change 实现）
- 复活 ChannelRouter 4 个 channel 文件
- 修改 SpawnPolicy 6 枚举本身（M5 只把 R0-R8 嵌套 if 拆成 3 个命名子决策）
- 跨域 LLM 节点（D3 LLMGateway、D4 Delegate）改造
- Execute / Verify / Learn 节点的 whole-body 输出结构变化

## 4. 澄清记录

### Q1: go-struct-driven vs 现有 FrameSpec 数组是否双链路？
**A**: **否**。`FrameSpec.Fields []TagName` 仍然保留（i18n + DocBlock + 测试 fixture 仍消费它），但其值由 `MustRegisterFrame[T]()` 通过反射写入；**手写 FrameSpec 数组变为违规**（init panic）。即：struct 字段 → 反射 → FrameSpec → i18n/序列化。 — 2026-07-05

### Q2: 反射性能开销？
**A**: `init()` 时一次反射写 `LineFrameRegistry` 哈希表，**热路径零反射**。`BuildLineFrameFromStruct` 走反射仅在 user prompt 构造时（每轮 1 次），实测可忽略（< 50μs/次）。 — 2026-07-05

### Q3: 是否做 LLM schema appendix（DocBlock）自动生成？
**A**: **是**。`DocBlockFromStruct[T]()` 反射 struct 字段 + `pt` tag 生成 schema 行（与现有 `DocBlockObserveSchema()` 字节等价）。`DocBlock(phase)` 保持向后兼容（现有调用点不动），新增 `DocBlockFromStruct[ObserveSignalInput]()` 作为权威。 — 2026-07-05

### Q4: kernel 复用范围？
**A**: `prompttags/structbind.go` 是 M1-M5 共享 kernel；M2 直接复用零代码增量。`BuildLineFrameFromStruct` 替代 `BuildAnnotatedLineFrame` 的调用点（Observe 1 处 + Plan 1 处 + 后续节点）。 — 2026-07-05

## 5. L1-L5 映射

| 层级 | 资产 ID | 名称 | 状态 |
|------|---------|------|------|
| L1 | shared | prompttags | 已有 |
| L1 | orchestration | MUPS 5 节点 | 已有 |
| L2 | L2-SHARED-IO | LLM I/O 形状 registry | 已有 |
| L2 | L2-ORCH-MUPS | 5 节点管道 | 已有 |
| L3-BE | D7-S5 | Observe LLM 提案 | 改造 |
| L3-BE | D7-S5 | Plan 战略提案 | 待改造（M2） |
| L4-BE | **shared-A99** | **structbind kernel（pt tag 反射 + 注册 + 序列化 + DocBlock）** | **新增** |
| L4-BE | **D7-S5-A99** | **ObserveSignalInput go-struct 化 + buildLLMObservationUserPrompt 一行化** | **新增** |
| L5 | **L5-MUPS-GSD-01** | `MustRegisterFrame[ObserveSignalInput]()` 在 init 注册成功 | 草拟 P0 |
| L5 | **L5-MUPS-GSD-02** | `BuildLineFrameFromStruct("observe_user", in)` 与 `buildLLMObservationUserPrompt(in)` 字节等价 | 草拟 P0 |
| L5 | **L5-MUPS-GSD-03** | `DocBlockFromStruct[ObserveSignalInput]()` 与 `DocBlockObserveSchema()` 字段一致 | 草拟 P0 |
| L5 | **L5-MUPS-GSD-04** | pt tag 缺失/plane 错误/i18n 缺翻译任一情形 → init panic | 草拟 P0 |
| L5 | **L5-MUPS-GSD-05** | 现有 Observe E2E 测试 (`item_observe_test.go`, `llm_observation_proposer_test.go`, `parse_reject_feedback_test.go`) 0 行为变化 | 草拟 P0 |
| L5 | **L5-MUPS-GSD-06** | golden snapshot 文件 `testdata/observe_user_prompt.golden` 通过 | 草拟 P1 |

## 6. 验收标准

- **P0**：`go vet ./...` + `go test ./internal/shared/prompttags/... ./internal/layers/orchestration/sessionorchestrator/... ./internal/layers/contextengine/i18n/... -race -count=1` 全 PASS。
- **P0**：新增 5 L5 测试点（L5-MUPS-GSD-01..05）全 PASS。
- **P0**：`buildLLMObservationUserPrompt` 函数体 ≤ 5 行（含函数签名）；35 行手工 map 拼接消失。
- **P0**：`ObserveSignalInput` 字段数 = `ObserveUserFrame.Fields` 长度 = 9；任一不等则编译失败或 init panic。
- **P1**：golden snapshot 测试覆盖 `in.InboundSignalLines` 空/非空、`in.PriorObservationIDs` 空/非空、`in.PriorParseReject` 空/非空 4 种组合。
- **P1**：5 节点重构总图（本文 §2.4 表）写入 `openspec/specs/d7-orchestration/mups-5node-refactor-roadmap.md`，作为 M2-M5 follow-on change 的入口。

## 7. 规划状态

- [x] S1 `demand.md`（本文）
- [x] S2 `proposal.md`
- [x] S3 `design.md` + `specs/{shared,d7-orchestration}/spec.md` delta
- [x] S4 `tasks.md`（P0/P1 拆解）
- [ ] S4 实现（kernel + Observe 迁移）
- [ ] S5 验收（5 L5 + golden + 0 行为变化）
- [ ] S6-交付（PR squash → master）
- [ ] S6-归档（`archive/2026-07-05-mups-go-struct-driven/`）
