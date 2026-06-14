---
demand-id: DM-20260614-016
title: D3 LLM Gateway S/A 重切 — 从「技术模块型 S」到「价值流型 S」
source: 架构审计（D3 S 切法为技术角色词、违反 code-layout.md §2；spec/layering 与 a-registry 失同步）
priority: P0
status: S7_Archived
dsaft_domain: D3
created: 2026-06-14
last-updated: 2026-06-14
review-round: R1+R2+R3
parent: dsaft-refactoring-playbook
---

# D3 LLM Gateway S/A 重切 — 从「技术模块型 S」到「价值流型 S」

> **S3-Gate 状态（2026-06-14）**：R1 (用户 Owner) + R2 (Claude 结构层) + R3 (Claude 运行层自裁决) 全部完成；R2 §6 接力接口 12 项 + R3 §1~4 命题 4 个 + R3 §5 NQ 6 个全部闭合；S3-Gate 全部接力接口就位。**下一步：进入 S3 阶段产物（spec.md v3.0.0 / design.md v3.0.0 / 4 注册表 v3.0.0），按 tasks.md Phase B 启动。**

# D3 LLM Gateway S/A 重切 — 从「技术模块型 S」到「价值流型 S」

## 0. Review R1 决议（2026-06-14）

> 用户评审确认三项 Decision 全部接受。本需求从 S1_Open 推进到 S2_Clarified，启动 `proposal.md` 撰写（S2 阶段产物：D + S 切法，不含 A/F/T 编排）。

| Decision | 用户结论 | 影响范围 |
|---------|---------|---------|
| **D1** D3 价值流 S 切法 = A 方案 | ✅ 接受 | 5+1 S = RouteModel / StreamChat / ProtectCall / BudgetTokens / GuardContent / ConfigureGateway；scenario-slug 语义化（`route/` `stream/` `protect/` `budget/` `guard/` `configure/`） |
| **D2** Bridge 跨域归位 = D2-1 | ✅ 接受 | 留 `internal/bridges/llm/`；D3 内部 f-registry 移除 F04/F05；CROSS 段在 layering.md 新增 |
| **D3** Safety 归属 = D3-1（留 D3） | ✅ 接受 | D3-S7 Safety → D3-S5 GuardContent；V2.1 3 条 T 全绿改名；D2-S18 边界写入 cross-domain-boundaries.md |

**R1 关键澄清（与 D7 demand.md R1 同型）：**

| # | 议题 | 决议 |
|---|------|------|
| Q1 | D3 是公共域还是核心域？ | **公共域**（横向能力，D1-D6 任意域可消费；当前主消费者 = D2/D4） |
| Q2 | D3 是否允许"无场景归属"的 kernel（类似 D1 kernel/）？ | v1.0 暂不引入；`Request/Chunk/CircuitState/TokenUsage/ToolSchema/ToolCall` 等核心类型在根 `contracts.go` 已是事实 kernel，**v1.0 不动** |
| Q3 | 26 条 T ID 改名时，metric / span 名是否同步改？ | **否**。运行时字符串（metric 名 `llm_requests_total`、span 名 `llm.stream`）保持不变（playbook 原则 3 + layering.md §命名规约例外）；只改 `T:` 注释与注册表 ID |
| Q4 | Legacy alias 写入哪里？ | `t-registry.md` §Legacy Archive（D2/D7 同型）+ `demand-archive-index.md` 末尾 |
| Q5 | v1.0 改名与 v2.0 物理迁移的发布窗口？ | v1.0 与 v1.1 合并发布（避免「注册表已价值流化但代码目录仍叫 adapter/」的中间态）；v2.0 物理迁移作为下一个 release |
| Q6 | D3 韧性状态 emit 到 D5/D7 的耦合度？ | v1.1 阶段：D3 → D5 Observability 写 metric（`llm_breaker_state{provider,state}`）；D3 → D7 通过现有 EngineEvent 复用，**不新增 D3→D7 直接契约** |
| Q7 | D6 评测点暴露在 v1.1 的范围？ | 3 个 probe：Tier 解析正确性、Breaker 状态切换次数、Token 预算触发率；写入 `openspec/specs/d6-evolution/d6-domain.md` 补丁 |

> **与 D7 demand.md R1 同型对照**：D7 R1 解决了"三模型 Task 职责分离 + 编排路由矩阵 + 迁移共存契约"；本 R1 解决了"价值流 S 切法 + Bridge 跨域归位 + Safety 归属论证"。

## 1. 原始描述

# D3 LLM Gateway S/A 重切 — 从「技术模块型 S」到「价值流型 S」

## 1. 原始描述

D3 LLM Gateway 当前 7 个 Scenario（Adapter / Gateway / Breaker / Retry / Token / Config / Safety）**全部为技术角色词**，目录亦同步（`adapter/`、`gateway/`、`breaker/`、`retry/`、`token/`、`config/`、`safety/`）。对照 `openspec/specs/architecture/code-layout.md` §2 明确**禁止** `gateway/adapters/...` 作为 L2 scenario-slug，以及 `dsaft-refactoring-playbook.md` §1 适用场景（S 层被技术模块绑架），D3 是当前唯一**未做价值流切法**的核心 / 公共域。

同时 D3 注册表内部存在**两处不同步**：

| # | 失同步项 | 影响 |
|---|---------|------|
| S1 | `spec.md` / `layering.md` 列 D3-S1..S6；`a-registry.md` 实际有 D3-S7 Safety（V2.1 后补） | 注册表不一致，评审时易漏查 |
| S2 | `f-registry.md` 把 `AdaptToContextEngine`（bridge.go）与 `WireLLMStack`（context_wiring.go）挂到 D3-S2-A01-F04/F05 | Bridge 与 Bootstrap 误归「路由」A，违反 playbook 原则 4「跨域问题在 D 边界决策」 |

## 2. 问题陈述（4 轴 Review）

### 轴 ① DSAFT 分层合规

| DSAFT 原则 | 体检结果 | 证据 |
|----------|---------|------|
| S = 价值流，非技术模块 | ❌ 7/7 S 为技术角色词 | spec.md §DSAFT 结构；layering.md §D3 |
| 目录 scenario-slug 须语义化 | ❌ 7/7 子目录为 `adapter/`、`gateway/` 等技术词 | `internal/layers/llmgateway/` |
| A-F 编排一致 | ⚠️ D3-S2-A01 RouteLLMCall 挂载了 bridge（`bridges/llm/bridge.go`）和 bootstrap（`bridges/llm/context_wiring.go`）两类异质 F | f-registry.md |
| T 与 Span 一致 | ✅ 26 条 T（25 IMPLEMENTED + 1 PLANNED）有对应 span 锚点 | span-registry.md |
| 跨域边界 | ⚠️ Safety filter 与 D2-S18 PermissionMode 职责需澄清 | safety/patterns.go vs contextengine/policy |
| 注册表内部一致 | ❌ layering.md/spec.md 与 a-registry.md 失同步（S7 漏登） | layering.md §D3 |

**根因（非单点 bug）：**

1. **D3 是「实现先于规格」**：V1 一次性把 6 个 S 写出（S1-S6），由目录结构倒推 S 编号；V2.1 加 Safety 时又新造一个 S7。S 编号复用旧习，无价值流约束。
2. **D3 公共域身份未贯彻**：D3 是公共域（横向能力），**天然应被其他域消费**；但当前 D3 的 A 层没有清晰的「对其他域暴露的契约」分组（route/bridge/bootstrap 混在一起）。
3. **Safety 是 D3 的"补丁场景"**：V2.1 把 Safety 加为 S7 仅为放置新代码，未做「为什么属于 D3 不属于 D2 Policy」的边界论证。

### 轴 ② 用户动线

D3 作为公共域，"用户"实际是 **D2 Context Engine** 与 **D4 Multi-Agent**（及未来其他 LLM 消费者）。D3 必须对"用户"提供可验证承诺：

| 承诺 | 现状可验证性 | 现有 S 切法问题 |
|------|------------|----------------|
| "我调一个模型名（或 tier），能拿到流式结果" | ✅ Gateway.Stream + Router.ResolveTier 已有 T | OK，但 S2 名字"Gateway" 含义模糊 |
| "Provider 故障不能阻塞我" | ✅ Breaker + Retry + Fallback 已有 T（P0 D3-S3-T01/02/03/04、D3-S4-T01、T03/04） | **S3+S4 是同一承诺的两个机制，缺统一动线** |
| "超长上下文不能阻塞我" | ✅ Counter.CheckBudget + Truncate 已有 T | OK |
| "恶意 prompt 不能绕过安全检查" | ✅ Filter.Check 已有 T | S7 是新加，"为什么属 D3" 需论证 |
| "我换 Provider 不需要改业务代码" | ✅ IAdapter + OpenAIStreamClient 已有 T | OK，但 S1 名字"Adapter" 含义偏窄（实际包含 Routing/Format/Stream） |
| "我能查到我的调用花了多少 token" | ✅ observability metrics + GenAI span | 横切到 D5，非 D3 主价值流 |

### 轴 ③ 博弈论 / 机制设计

| 参与者 | 目标函数 | 当前博弈失衡点 |
|--------|---------|---------------|
| **D3 Adapter 作者**（本地最优：加新 Provider 越快越好） | 改一个 `*.go` 文件 + 注册 | S1=Adapter 让实现者按"包"思考，但 S 实际承载"流式 + 协议适配 + SSE 解析"三种异质能力 |
| **D3 SRE/可观测性**（全局最优：故障能定界） | P0 失败可定位到具体机制 | Breaker 与 Retry 跨 S 编排，错误归因常被「Retry 里挂了」与「Breaker 拒了」混淆 |
| **D2 Context Engine 消费者**（全局最优：契约稳定） | IGateway 接口不变 + Tier 解析正确 | `IGateway.ResolveTier` 与 `ILLMGateway.ChatStream` 与 `ITierResolver.ResolveTier` 三者签名不一致，违反接口隔离 |
| **D7 Orchestrator 编排者**（跨域价值流：可调度性） | 编排决定"用哪条 LLM 路径" | D7 当前不感知 D3 的韧性状态（Breaker 状态未 emit 到 D5/D7） |
| **D6 Evolution 评测**（全局最优：可量化质量） | Probe 覆盖关键决策点 | D3-S5（Token）P2 CJK 补偿是 F 级实现细节，未暴露 D6 可评测的 WHAT |

**核心错配（DSAFT 分层任务 = 让局部最优指向全局最优）：**

> D3 当前「实现 = 目录 = S 编号」三位一体，导致：
> - 新加 Provider 的人只需碰 `adapter/`（局部最优 ✅）
> - 但 Provider 失败时 SRE 不知道该看 Breaker 还是 Retry（全局最优 ❌）
> - 因为 S 切法按机制切（Breaker / Retry / Safety / Token），不是按"对消费者的承诺"切

### 轴 ④ OpenSpec 交付

- 当前 D3 spec.md / design.md / layer-delta.md / a/f/t-registry.md / span-registry.md **齐备**（V2.1）；
- 但**注册表内部失同步**（spec/layering vs a-registry）违反 `archiving.md` §6 同步约束；
- 缺乏 `demand-archive-index.md` 追溯（V1/V2 归档但 index 缺 D3 入口）；
- `code-layout.md` §4 缺 D3 scenario-slug 注册表（D1/D2/D7 都有，唯独 D3 漏登）。

## 3. North Star

D3 LLM Gateway 是**公共域**（横向能力），向所有 LLM 消费者（D2/D4 及未来）提供 5 类**可验证承诺**：

1. **路由承诺**：给我一个模型名（或 tier 别名），我返回正确 Provider + 模型。
2. **流式承诺**：给我一个 chat 请求，我流式返回 chunk，含 SSE 解析与中止控制。
3. **韧性承诺**：Provider 故障不阻塞我（Breaker 拒 / Retry 退避 / Fallback 切换 / 错误归因可定位）。
4. **预算承诺**：上下文不超 token 预算；超限截断或报错。
5. **安全承诺**：恶意 / 越权内容不能穿过 gateway。

> 这 5 类承诺对应 D3 的 5 个**价值流 S**（见 §5 切法 A）。**ConfigureGateway**（配置加载）作为 S6 横切支撑，不直接暴露给消费者。

## 4. 目标（L5 锚点 / 验收）

### 4.1 必达

| ID | L5 锚点 | 关联 T（待映射） |
|----|---------|----------------|
| G1 | **注册表一致**：`layering.md` / `spec.md` / `a-registry.md` / `f-registry.md` 在 v1.0 后全表无失同步 | — |
| G2 | **S 切法合规**：D3 全部 S 为价值流名，scenario-slug 语义化、无技术角色词 | — |
| G3 | **P0 测试全绿**：D3 现有 11 条 P0（T01/02/03/04×Breaker + T01×Retry + T01×Counter + T01×Loader + T01×Filter + T01×Safety-critical + T04/T05×Gateway）在 v1.0 改名 / v2.0 物理迁移后均 IMPLEMENTED 且绿 | t-registry.md 现状 |
| G4 | **T 映射不丢**：v1.0 registry 重切后，所有现有 26 条 T 都有 `<!-- T: -->` 注释或 alias 指向新 S/A ID | t-registry.md |
| G5 | **Bridge 跨域归位**：`AdaptToContextEngine`（bridge.go）+ `WireLLMStack`（context_wiring.go）从 D3 内部 A 迁出，挂到 D3-S0「跨域桥接」/ 或归入 `internal/bridges/llm/` 名义下的 CROSS 段 | f-registry.md §Bridge |

### 4.2 范围

#### In Scope（v1.0 Registry）
- D3 6/7 个 S 重切为 5+1 个价值流 S
- `a-registry.md` / `f-registry.md` / `t-registry.md` / `span-registry.md` 全部按新 S 重编号
- 注册表一致性修复（layering.md / spec.md 同步）
- `code-layout.md` §4 增加 D3 scenario-slug 注册表
- Legacy 双轨：旧 S ID 冻结追溯（写入 `t-registry.md` §Legacy Archive）

#### In Scope（v1.1 Traceability）
- Bridge 跨域归位（详见 Decision D2）
- Span / T / 契约三向追溯表
- D7-S1 PlanTask 通过 D3 韧性状态的可见性（emit 到 D5/D7）
- D6 评测点暴露：Tier 解析、Breaker 状态切换、Token 预算截断

#### In Scope（v2.0 Structure）
- 物理路径迁移：`adapter/` `gateway/` `breaker/` `retry/` `token/` `config/` `safety/` → 价值流 scenario-slug 目录
- contracts.go 按价值流拆分到各子包；`internal/layers/llmgateway/contracts.go` 保留 re-export 一个发布周期

#### Out of Scope
- 重写 Provider 适配器（DeepSeekAdapter / MiniMaxAdapter 行为不变）
- D2-S4 Token（Context Engine 的 Token 计数）vs D3 Token 的合并（属 D2 重构范畴，DM-20260614-009 已规划）
- Safety filter 与 D2-S18 PermissionMode 的合并（需另立 change 包，本 change 仅做归属论证，不动实现）
- V3 计划：Anthropic/OpenAI Adapter、Rate Limiter、负载均衡（属未来 change）

## 5. S 切法候选（Decision 表）

### Decision: D3 价值流 S 切法

| 方案 | S 切法 | 优点 | 缺点 |
|------|--------|------|------|
| **A: 价值流承诺型**（推荐） | S1 RouteModel, S2 StreamChat, S3 ProtectCall, S4 BudgetTokens, S5 GuardContent, S6 ConfigureGateway | ① S 与用户可验证承诺 1:1 对应 ② playbook 原则 1/2 满足 ③ scenario-slug 可语义化（`route/` `stream/` `protect/` `budget/` `guard/` `configure/`） ④ Resilience 合并 S3 减少跨 S 编排 | ① 旧 T ID 全部需 alias ② 物理路径全变 |
| B: 模块+价值流描述 | 沿用 S1-S7 技术名，补「用户目标」列 | ① T ID 不变 ② 工作量小 | ① 违反 code-layout.md §2（scenario-slug 仍为技术词） ② 价值流承诺无法对应到 S ③ 反复被「S 看起来像模块」质疑 |
| C: 激进 3 S | S1 Call（route+adapter+stream）, S2 Protect（breaker+retry+safety）, S3 Plan（config+token） | ① 极简 ② 物理路径少 | ① Token 预算 ≠ Gateway 配置 ② Safety 与 Breaker 同 S 弱化承诺语义 ③ Bridge / Bootstrap 无处安放 |
| D: Safety 外移 | 把 D3-S7 Safety 移给 D2-S18 Policy | ① 公共域更"纯流式" ② 减少 S 数量 | ① D2-S18 Policy 已承载 PermissionMode，再加 SafetyFilter 致 PermissionMode 失焦 ② D2 与 D3 在 Safety 上需新增契约，跨域耦合↑ ③ 需另立 D2 change 包，与本次范围冲突 |

**选择:** A
**理由:** ① D3 当前 7 S 全部为技术词，是 D1/D2/D7 之外**唯一未做价值流切法**的核心/公共域，已成架构债务；② A 方案的 5+1 S 与 §3 North Star 的 5 类承诺一一对应，符合 playbook 原则 1「先问用户可验证承诺」；③ scenario-slug 可全部语义化（route/stream/protect/budget/guard/configure），满足 code-layout.md §2；④ 物理路径迁移可分阶段（v2.0）执行，不阻塞 v1.0 注册表共识；⑤ D2/D7 的 Legacy 双轨已经成功（D7 demand.md §Q6 迁移共存契约可复用）。
**影响:** ① 26 条 T 全部需在新注册表中保留 `<!-- T: -->` 注释 + Legacy ID alias；② a-registry/f-registry/t-registry 三个注册表重排；③ v2.0 物理迁移需 re-export 桥接包 + 一次发布周期兼容窗口。

### Decision: Bridge 跨域归位

| 方案 | Bridge 归属 | 优点 | 缺点 |
|------|------------|------|------|
| **D2-1: 留 `internal/bridges/llm/`**（推荐） | D3 内部不再挂 F04/F05；bridge.go 与 context_wiring.go 属 CROSS 段，由 `bridges/llm/` 名义下独立注册 | ① 物理路径不动 ② contracts 边界清晰 ③ 与 D7-S2/D4-S10 bridge 风格一致 | ① 需在 layering.md 加 CROSS 段说明 |
| D2-2: 挂 D3 新增 S0 跨域桥接 | D3-S0 = Bridge S（含 5 个 F） | ① 全部在 D3 注册表内 | ① S0 是技术词，违反价值流 ② 与"无 bridge 也可调"的设计不符 |
| D2-3: 拆给 D2 + D7 | `ChatStream` 给 D2-ILLMGateway，`WireLLMStack` 给 D7-bootstrap | ① 严格按调用方归属 | ① Bridge 文件物理上是一对，强拆破坏内聚 ② 增加跨包依赖 |

**选择:** D2-1
**理由:** ① `internal/bridges/llm/` 已存在且职责明确（`ChatStream` + `WireContextLLM`），无需新增 D3 子包；② Bridge 是 D3 对 D2 的**契约实现**，不是 D3 的内部 A，与 playbook 原则 4「跨域问题在 D 边界决策」一致；③ layering.md §Naming Policy 已把 `Bridge` 列为语义角色，方案 D2-1 直接受益。
**影响:** ① f-registry.md 删去 D3-S2-A01-F04/F05；② 新增 CROSS 段：`D3-S2-A01-F04 AdaptToContextEngine` (alias) + `D3-S2-A01-F05 WireLLMStack` (alias)；③ bridge.go 与 context_wiring.go 注释需更新指向新位置。

### Decision: Safety Filter D3 vs D2 归属

| 方案 | 归属 | 优点 | 缺点 |
|------|------|------|------|
| **D3-1: 留 D3**（推荐 v1.0） | 留在 D3 新 S5 GuardContent | ① 价值流承诺「恶意内容不能穿过 gateway」 属 D3 边界（gateway 自身可拒绝） ② 与 Breaker/Retry 同样属"网关质量门" ③ 不破坏 v2.1 已 IMPLEMENTED 的 3 条 T | ① 需论证 D2 Policy 不重复实现 |
| D3-2: 给 D2-S18 | 移到 D2-S18 PermissionMode | ① D3 纯流式 ② 内容策略与权限模式同域 | ① D2-S18 已有 PermissionMode 失焦 ② D2 与 D3 需新增 Safety 契约 ③ 需另立 D2 change |
| D3-3: 升 D5 / D6 | 内容安全 = 横向可观测/进化 | ① 客观锚点 ② 与未来 LLM 安全策略迭代对齐 | ① 跨域延迟↑ ② D3 失去"网关拒绝"能力 ③ V2.1 IMPLEMENTED T 失效 |

**选择:** D3-1
**理由:** ① 当前 Safety 处于"流式调用前最后一道门"位置（Filter.Check → IGateway.Stream），属 D3 价值流承诺范畴；② D2-S18 已承载 PermissionMode（执行权限沙箱），职责是"允许哪些工具调用"，与"过滤哪些 prompt 内容"语义不同；③ V2.1 已有 3 条 P0/P1 T 全绿（critical reject / medium warn / custom patterns），v1.0 改名即可，不丢测试点；④ 跨域风险：D2-S18 可在 v1.1 通过 D2 → D3 SafetyCheckHook 复用（不在本 change 范围）。
**影响:** ① a-registry.md 中 D3-S7 → D3-S5 GuardContent；② 与 D2-S18 的边界声明写入 `cross-domain-boundaries.md`（D3 负责 prompt 内容过滤；D2-S18 负责 tool execution policy；D5 负责 audit log；D6 负责 Safety 评测）。

## 6. 范围（详细）

### 6.1 In Scope（v1.0 Registry）

- 重切 5+1 S，注册表 4 文件全表更新
- Legacy S ID alias 表（写入 `t-registry.md` §Legacy Archive）
- `code-layout.md` §4 新增 D3 scenario-slug 注册表
- `layering.md` §D3 与 `spec.md` 同步
- 增补新 S 的 A/F/T（保留 P0 全绿）
- 跨域边界声明（Safety vs D2-S18）

### 6.2 In Scope（v1.1 Traceability）

- Bridge 跨域归位（Decision D2-1）
- D3 → D5/D7 emit Breaker 状态（韧性可见性）
- D6 评测点暴露：Tier 解析正确性 / Breaker 状态切换次数 / Token 预算触发率
- 4 表追溯：S → A → F → T → Span 完整 chain
- 4 组合回归矩阵（model tier × adapter 切换）

### 6.3 In Scope（v2.0 Structure）

- 物理路径迁移到 scenario-slug 目录
- contracts.go 拆分到各子包
- re-export 桥接包（1 个发布周期）
- layering.md §Domain Layout 更新

### 6.4 Out of Scope

- Provider 适配器重写（行为不变）
- D2-S4 Token vs D3-S4 合并（属 D2 change 范畴）
- Safety 与 D2-S18 合并（需另立 D2 change）
- V3 计划（Anthropic Adapter / Rate Limiter / 负载均衡）
- D3 公共域对**新消费者**（非 D2/D4）的开放（属未来 change）

## 7. 验收标准（P0 摘要）

| ID | 标准 |
|----|------|
| AC1 | v1.0 注册表 merge 后，`grep -r "D3-S[1-7]" openspec/specs/` 与 `a-registry.md` / `f-registry.md` / `t-registry.md` 无失同步 |
| AC2 | v1.0 完成后，所有 26 条 T（含 11 P0）保持 IMPLEMENTED 状态；T ID 改名的须有 Legacy alias |
| AC3 | v1.1 完成后，Bridge 跨域归位；f-registry.md 中 D3 不再挂 bridge / bootstrap F；CROSS 段新增对应 alias |
| AC4 | v2.0 物理路径迁移后，`go build ./...` 与 `go test ./internal/layers/llmgateway/... ./internal/bridges/llm/...` 全绿；`go vet` 无新增警告 |
| AC5 | 价值流承诺 §3 全部 5 条有 Span 证据（D5 全链路） |
| AC6 | D3 对 D2 暴露的契约（`ILLMGateway` + `ITierResolver`）签名在 v1.0/v1.1/v2.0 全程不变 |
| AC7 | D2-S18 与 D3-S5（GuardContent）边界声明写入 `openspec/specs/architecture/cross-domain-boundaries.md`（新文件） |

## 8. 实施阶段（v1.0 → v1.1 → v2.0 三段终态）

| Phase | 阶段 | 内容 | 交付物 | 风险 |
|-------|------|------|--------|------|
| **A** | 文档澄清（v1.0 启动） | demand.md（本文件）、tasks.md、spec.md 重切草案 | OpenSpec S1→S2 | 低（纯文档） |
| **B** | v1.0 Registry | a/f/t/span-registry.md 重排 + Legacy alias + layering.md 同步 + code-layout.md §4 注册表 | 4 注册表 + cross-domain-boundaries.md | 低（注册表级别） |
| **C** | v1.0 验证 | 所有 P0 T 重命名后仍绿；4 注册表 grep 一致性 | acceptance-report（v1.0） | 中（alias 表必须 100% 覆盖） |
| **D** | v1.1 Bridge 归位 | D3-S2-A01 F04/F05 迁出；CROSS 段注册 | f-registry.md §CROSS + bridge.go 注释 | 中（需调 D2 bootstrap 调用方） |
| **E** | v1.1 可观测 | D3 韧性状态 emit 到 D5；D6 评测点暴露 | span-registry.md + d6-evolution spec | 中（新增 span / metric 名） |
| **F** | v1.1 验证 | Bridge 跨域测试 + Span 完整性 | acceptance-report（v1.1） | 中 |
| **G** | v2.0 物理迁移 | 子目录改名 → 价值流 scenario-slug；contracts.go 拆分；re-export 桥接 | 物理目录 + contracts.go re-export | **高**（需 T 全绿 + 一发布周期兼容） |
| **H** | v2.0 验证 | 完整 build + 跨域回归 + 旧路径 dead code 清理 | acceptance-report（v2.0）+ archive | 高 |

> **分阶段理由（playbook 原则 6）：** v1.0 闭合 Registry 共识 → v1.1 可追溯 → v2.0 物理结构。每阶段独立 S5/S7，可中断可回滚。

## 9. 依赖与对齐

| 关联 | 影响 |
|------|------|
| DM-20260614-009（D2 v2.0 S15–S20） | D2 S4 Token（Context Engine）vs D3 S4 BudgetTokens 边界需在 v1.0 论证，但合并不在本 change |
| DM-20260614-006（D1 v2.0 S13–S18） | Bridge 跨域归位（D2-1）需参照 D1→D7 bridge 风格 |
| DM-20260613-001（D7 升格） | D7 不感知 D3 韧性状态属 D7 改进项；本 change 仅在 v1.1 加 D3→D7 通知 hook |
| DM-20260610-012（ORCH v2） | D3 Breaker 状态历史数据可被 D7 编排参考（不在 v1.0/v1.1 范围） |
| dsaft-refactoring-playbook.md | 本 change 是该 playbook 的**首次 D 域应用**，输出可作为后续 D2/D4/D5/D6 重构样板 |

## 10. 评审入口（供 S2 Review）

| 文档 | 用途 |
|------|------|
| 本文档 `demand.md` | 需求澄清 SoT（已完成 S1） |
| `openspec/specs/d3-llm-gateway/spec.md`（待 S2 重写） | 价值流 S + A 切法 |
| `openspec/specs/d3-llm-gateway/design.md`（待 S3 重写） | F 编排 + 物理映射 |
| `openspec/specs/architecture/cross-domain-boundaries.md`（新文件，v1.0 阶段产出） | D3-S5 GuardContent vs D2-S18 边界 |
| `openspec/specs/architecture/code-layout.md` §4（v1.0 阶段补 D3） | scenario-slug 注册表 |
| `openspec/changes/devrix-d3-sa-refine/tasks.md`（待 S2 写） | 任务分解（无代码） |

## 11. 风险与缓解

| 风险 | 可能性 | 影响 | 缓解 |
|------|--------|------|------|
| v1.0 T ID 改名导致 grep 失效（外部 dashboard / 告警） | 中 | 中 | Legacy alias 写入 `t-registry.md` §Legacy Archive + metric 名保留 |
| v2.0 物理迁移破坏 v2.1 IMPLEMENTED 状态 | 中 | 高 | re-export 桥接包 + 1 发布周期兼容 + 完整 P0 回归 |
| Bridge 归位（D2-1）打破 D2 bootstrap 调用方 | 中 | 中 | Phase D 优先于 v2.0；D2 已有 WireContextLLM 调用方清单（consumer: 1 处） |
| 价值流 S 切法与 D7 编排期望冲突 | 低 | 中 | D7 编排者进 S2 Review；D3 韧性状态 emit（v1.1）作为解耦手段 |
| Safety 归属论证不充分被 D2/D6 挑战 | 中 | 中 | Decision D3-1 写入 §5；与 D2-S18 边界声明写入 cross-domain-boundaries.md |

---

**Revision History**

| 版本 | 日期 | 变更 |
|------|------|------|
| 0.1 | 2026-06-14 | 初稿：4 轴 Review + 5+1 S 切法 + Decision D1/D2/D3 |
| 0.2 | 2026-06-14 | S2_Clarified：Review R1 决议写入 + 7 项澄清（Q1-Q7） + 与 D7 R1 对齐 |
| 0.3 | 2026-06-14 | S3_Design_Gate_Cleared：R2 5 命题 + R3 4 命题 + NQ 6 项 + R2 §6 接力接口 12 项全部闭合；R3 增补 P0 #8（factory.go fail-fast）；状态推进到 S3-Gate Cleared |
