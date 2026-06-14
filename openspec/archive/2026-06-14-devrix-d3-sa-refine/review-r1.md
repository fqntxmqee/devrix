# Review R1 — D3 S/A 重切提案评审

**Change ID:** devrix-d3-sa-refine
**Demand ID:** DM-20260614-016
**Review Date:** 2026-06-14
**Reviewer:** 用户（架构 Owner）
**Reviewed Documents:** `demand.md v0.1`、`proposal.md v0.1`
**Verdict:** ✅ **APPROVED**（R1 全部 3 个 Decision 接受，7 项澄清写入 demand.md §0）

---

## 1. 评审范围

| 评审对象 | 内容 |
|---------|------|
| D 切法 | D3 公共域身份是否成立 |
| S 切法 | 5+1 价值流 S 是否与 North Star 5 承诺 1:1 对应 |
| scenario-slug | 全部语义化、无技术角色词、Go 合法目录名 |
| Legacy 双轨 | 26 条 T alias 100% 覆盖策略 |
| 跨域边界 | Safety 归属论证 + Bridge 跨域归位 |
| 三段终态 | v1.0 / v1.1 / v2.0 风险梯度与发布窗口 |
| Out of Scope | 8 项边界是否清晰 |

---

## 2. Decision 评审

### D1: D3 价值流 S 切法

| 方案 | 评审结论 |
|------|---------|
| **A 价值流承诺型** | ✅ **接受**（5+1 S = RouteModel / StreamChat / ProtectCall / BudgetTokens / GuardContent / ConfigureGateway） |
| B 模块+价值流描述 | ❌ 违反 code-layout §2；价值流承诺无法对应到 S |
| C 激进 3 S | ❌ Safety 与 Breaker 同 S 弱化承诺；Token 与 Config 同 S 失焦 |
| D Safety 外移 D2 | ❌ D2-S18 失焦；需另立 D2 change；V2.1 T 失效 |

**关键论据：**
- A 方案的 5+1 S 与 North Star 5 承诺 1:1 对应（`proposal.md §3.2` 表格）
- scenario-slug 全部语义化（`route/` `stream/` `protect/` `budget/` `guard/` `configure/`），满足 `code-layout.md §2`
- D1/D2/D7 已通过 v2.0 重切完成价值流化，D3 是空白，结构债务最显眼
- 物理路径迁移可分阶段（v2.0）执行，不阻塞 v1.0 注册表共识
- D2/D7 的 Legacy 双轨已经成功（D7 demand.md §Q6 迁移共存契约可复用）

### D2: Bridge 跨域归位

| 方案 | 评审结论 |
|------|---------|
| **D2-1 留 `internal/bridges/llm/`** | ✅ **接受** |
| D2-2 挂 D3 新增 S0 跨域桥接 | ❌ S0 是技术词，违反价值流 |
| D2-3 拆给 D2 + D7 | ❌ 强拆破坏内聚；增加跨包依赖 |

**关键论据：**
- Bridge 是 D3 对 D2 的**契约实现**，不是 D3 内部 A（playbook 原则 4「跨域问题在 D 边界决策」）
- `internal/bridges/llm/` 已存在且职责明确（`ChatStream` + `WireContextLLM`），无需新增 D3 子包
- `layering.md §Naming Policy` 已把 `Bridge` 列为语义角色，D2-1 直接受益
- 与 D7-S2/D4-S10 bridge 风格一致

### D3: Safety Filter D3 vs D2 归属

| 方案 | 评审结论 |
|------|---------|
| **D3-1 留 D3** | ✅ **接受**（D3-S7 Safety → D3-S5 GuardContent） |
| D3-2 给 D2-S18 | ❌ D2-S18 失焦；需另立 D2 change |
| D3-3 升 D5 / D6 | ❌ 跨域延迟↑；V2.1 T 失效 |

**关键论据：**
- Safety 当前在「流式调用前最后一道门」位置（`Filter.Check` → `IGateway.Stream`），属 D3 边界
- D2-S18 PermissionMode 语义是「允许哪些 tool 调用」，与「过滤哪些 prompt 内容」不同
- V2.1 已有 3 条 P0/P1 T 全绿（critical reject / medium warn / custom patterns），改名即可不丢测试点
- 跨域风险：D2-S18 可在 v1.1 通过 D2 → D3 SafetyCheckHook 复用（不在本 change 范围）

---

## 3. R1 关键澄清（7 项）

> 与 D7 demand.md R1 同型（D7 解决"三模型 Task 职责分离 + 编排路由矩阵 + 迁移共存契约"；本 R1 解决"价值流 S 切法 + Bridge 跨域归位 + Safety 归属"）。

| # | 议题 | 决议 | 写入位置 |
|---|------|------|---------|
| Q1 | D3 是公共域还是核心域？ | **公共域**（横向能力，D1-D6 任意域可消费；当前主消费者 = D2/D4） | demand.md §0 + proposal.md §3.1 |
| Q2 | D3 是否允许"无场景归属"的 kernel（类似 D1 kernel/）？ | v1.0 暂不引入；根 `contracts.go` 已是事实 kernel | demand.md §0 Q2 |
| Q3 | 26 条 T ID 改名时，metric / span 名是否同步改？ | **否**。运行时字符串保持不变（playbook 原则 3 + layering.md §命名规约例外） | demand.md §0 Q3 |
| Q4 | Legacy alias 写入哪里？ | `t-registry.md §Legacy Archive` + `demand-archive-index.md` 末尾 | demand.md §0 Q4 |
| Q5 | v1.0 改名与 v2.0 物理迁移的发布窗口？ | v1.0 + v1.1 合并发布；v2.0 物理迁移作为下一个 release | demand.md §0 Q5 + proposal.md §3.6 |
| Q6 | D3 韧性状态 emit 到 D5/D7 的耦合度？ | v1.1：D3 → D5 写 metric；D3 → D7 复用 EngineEvent，**不新增 D3→D7 直接契约** | demand.md §0 Q6 |
| Q7 | D6 评测点暴露在 v1.1 的范围？ | 3 个 probe：Tier 解析正确性、Breaker 状态切换次数、Token 预算触发率；写入 d6-evolution spec 补丁 | demand.md §0 Q7 + proposal.md Phase D |

---

## 4. 进入 S3 的前置条件

| 条件 | 状态 |
|------|------|
| proposal.md 与 demand.md 一致 | ✅ |
| 5+1 S 切法 Owner 接受 | ✅ |
| Legacy alias 策略明确 | ✅ |
| 跨域边界声明已规划 | ✅（`cross-domain-boundaries.md` v1.0 阶段产出） |
| Out of Scope 明确 | ✅（8 项边界） |

**S3 阶段待产出（不属本 change）：**

- `openspec/specs/d3-llm-gateway/design.md`（A + F 编排 + 物理映射）
- `openspec/specs/d3-llm-gateway/spec.md`（重写为 5+1 S 规格）
- 4 注册表（a/f/t/span-registry.md）重排
- 完整 tasks.md（v1.0 任务分解 + v1.1 任务分解 + v2.0 任务分解）

> S3 产出需经 S3-Gate（参考 D7 重构 review-r1 → review-r2 → review-r3 三轮 review 风格）。

---

## 5. 决议

| 决议项 | 结论 |
|--------|------|
| **R1 Verdict** | ✅ **APPROVED** |
| **demand.md 状态** | S1_Open → **S2_Clarified** |
| **proposal.md 状态** | **DRAFT v0.1**（S2 产物 Owner 接受） |
| **下一步** | 进入 S3 设计阶段（产出 `spec.md` 重写 + 4 注册表重排 + `design.md`） |
| **S3 启动前需确认** | 是否需要 R2 / R3 多轮 review（D7 走 3 轮） |

---

**Revision History**

| 版本 | 日期 | 变更 |
|------|------|------|
| 0.1 | 2026-06-14 | 初稿：3 Decision 评审 + 7 项澄清 + S3 启动前置条件 |
