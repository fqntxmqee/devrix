# Demand: MUPS 5 节点 × Tool 元数据 Control Plane + ToolChannel 自治

**Change ID:** `devrix-mups-tool-classification-and-channel-autonomy`
**Demand ID:** DM-20260701-007
**Stage:** S2_Clarified（博弈论双 review 共识已并入）
**Created:** 2026-07-01
**Game Theory Reviews:** [game-theory-review.md](./game-theory-review.md)（Codex v1, 2026-07-01）· [game-theory-review-composer.md](./game-theory-review-composer.md)（Composer v1, 2026-07-01）· [game-theory-review-codex2.md](./game-theory-review-codex2.md)（Codex v2 / 2nd pass, 2026-07-02 post-PR-A — 答 Cursor "8K token 问题是否解决"）
**Parent Demands:**

- DM-20260701-005 (D7 Verify synthesize enforce — 治标)
- DM-20260701-006 (D2 tool_result budget profile — 治标)
- DM-20260630-012 (D7 deliverable convergence — 治标)
- DM-20260629-001 (D7 DSAFT restructuring — 治本前置)
- DM-20260617-008 (Tool Surface Phase 2 full — 前置)
- DM-20260617-007 (D7 Tool Surface Contract S1-S3 — 前置)
- DM-20260625-019 (D7 5-node coverage — 前置)
- DM-20260626-005 (D7 6S Verify promotion — 前置, executionflow/verify/ 物理 promote)
- DM-20260618-001 (Tool Spec v2 + CheckPermission + DeferLoading — 前置, 9 字段基线)

---



## 1. 现象 (Symptom)

Sess `sess_1782885908460_4000`（2026-07-01）复现的 LLM 自我循环：


| 步骤  | 行为                                                                        |
| --- | ------------------------------------------------------------------------- |
| 1   | User 发起 "review d2 领域 kernel 目录下代码"                                       |
| 2   | D1 feishu → D7 ProcessMessage → IntentClassify(PLAN)                      |
| 3   | D7 RunSessionTurnLoop → 5 节点 Pipeline (Observe→Plan→Execute→Verify→Learn) |
| 4   | **Execute 节点**：LLM 反复 `read_file` × 9 次（累计 50K tokens）                    |
| 5   | 每次被 D2 `TruncateToTokens(cfg.ToolResultBudget=8K)` 截断，**无 marker 告知 LLM** |
| 6   | LLM 看不到完整内容 → 继续自我循环探索                                                    |
| 7   | **Verify 节点**：接受探索性 finalText → 失败标 `task_incomplete=true`                |
| 8   | D1 渲染红卡（PR #373 hotfix 已修）                                                |
| 9   | **LLM 实际产出 = 0 review**                                                   |


---



## 2. 根因 (Root Cause)

devrix 当前架构有 5 个根因（博弈论映射见 §3.1）：


| ID   | 根因                                                                                      | 影响                  | 博弈论本质                                 |
| ---- | --------------------------------------------------------------------------------------- | ------------------- | ------------------------------------- |
| RC-1 | ToolSpec v2 仅 9 字段（init-time 1 次查表），runtime 无 termination signal                        | LLM 自我循环无自收敛信号      | Pooling equilibrium — 所有 tool 等价可无限调用 |
| RC-2 | D2 `ToolResultBudget=8K` 截断**无 marker**，LLM 不知道截断发生                                     | LLM 反复重读同一文件        | Akerlof 信号失灵 — 完整/截断 read 不可分         |
| RC-3 | D7 Execute 节点现有 mups/execute/ 4 PlanKind Channel **不分类 per-tool termination invariant** | Probe 类工具无强终止       | Adverse selection — Probe 被当作 Fact 定价 |
| RC-4 | D7 Verify 节点只判现有 deliverable，**无 input contract 强校验**                                   | 探索性 finalText 蒙混过关  | Hart–Holmström 不完全契约                  |
| RC-5 | `verdict.Reason` 在 session_complete.go 局部变量丢失                                           | D1 渲染拿不到 verify 层语义 | 重复博弈信息链断裂，无法 punishment               |


**并列社会成本病根（共识）**：LLM 将 D2 token 预算当作 Hardin 公地悲剧中的公共池塘 — 个体理性（多读一眼）≠ 集体理性（产出 deliverable）。

---



## 3. 假设 (Hypothesis)

> 4 类正交分解 (Fact / Action / Probe / Experiment) 必须从 MUPS 节点级下沉到 **tool metadata 级**，在信息结构允许的最早节点做 **type revelation**（类型揭示）。



### 3.1 核心假设（H1–H6，原 v1）

1. 每个 tool 自带 `EmissionClass` + `ConvergenceContract` + `IterationBound` + `SourceUncertainty` 4 个 control plane 字段
2. D7 Execute 节点按 `EmissionClass` 路由到 **4 ToolChannel**（与现有 mups/execute/ 4 **PlanKind** Channel 平行 — 旧接口 rename 为 `PlanChannel`，新抽象叫 `ToolChannel`）
3. 每个 ToolChannel 在 register time 就强制挂 **LTL-Lite L4-L6 termination invariant**（与 L0-L3 安全 invariant cross-check，见 H8）
4. **ProbeToolChannel 的** `Bounded(n)` **hard reject 是 commitment device**；`PromptPressure` 是 Schelling focal point（cheap talk），二者配合实现单 session **SPE**
5. D7 Verify 节点用 **VerifyContract 4 元组**强校验 deliverable mandatory，并按 EmissionClass 分配 **burden of proof**（见 §3.3）
6. `verdict.Reason` 透传 `meta["verify_exit_reason"]` 全链路至 D1 render，并 **最小写入 Learn FeedbackMemory**（跨 session reputation 起点）



### 3.2 博弈论评审共识假设（H7–H12，Codex + Composer 2026-07-01）


| ID      | 共识内容                                                                                                                                                                | 来源                           |
| ------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------- |
| **H7**  | **目标均衡双声明**：单 session = ToolChannel `Bounded(n)` 下的 **SPE**；跨 session = Learn `ReputationEvidence` 驱动的 **reputation equilibrium**（Phase A–D 保证前者必要条件，后者最小接入 H6）     | Codex §4.2 + Composer §4.2   |
| **H8**  | LTL-Lite **L4–L6 与 L0–L3 兼容**：`Bounded(n)` 不得 override readonly/destructive 安全 guard；至少 3 条 cross-check 写入 `specs/execute-channels.md`                              | Codex §3.3                   |
| **H9**  | **PlanKind × EmissionClass 交叉一致性**：`task_kind=review` 时 Filter/PlanChannel 不得将 Probe 工具 bound 降为 `OpenEnded`；`OnResult` 对同 tool 反复调用做行为重分类（call_count>3 → 升级 Probe） | Codex §3.2 + Composer §2.1   |
| **H10** | `emission_class` **cheap talk 审计**：Learn 侧维护 `tool → declared_class → drift_rate`（Phase C 最小：仅 `verify_exit_reason`；完整 drift 表可 Phase E）                            | Codex §3.1、§4.3              |
| **H11** | **Phase B shadow mode**：ToolChannel enforce 前 1 周 log-only 并行（`would_reject` 不 block），false positive <5% 再硬切                                                        | Codex §4.4 + Composer §7     |
| **H12** | **PR-A 禁止 pooling 默认**：缺 `EmissionClass` 的 surface 文件 **CI fail**，禁止 silent fallback 为 `Action+OpenEnded`；`read_file`/`grep`/`glob` 必须显式 `EC_Probe + Bounded(15)`   | Codex Open Q#6 + Composer §5 |




### 3.3 VerifyContract 举证规则（H5 细化，双 review 共识）


| EmissionClass | 举证要求                                        |
| ------------- | ------------------------------------------- |
| Fact          | deliverable text 自证                         |
| Action        | state change evidence 必传                    |
| Probe         | `source_quality` / calibrated_confidence 必填 |
| Experiment    | result reproducibility 必传                   |


---



## 4. 三方案对比


| 方案           | 内容                                                                                    | 治标/治本  | 风险                        |
| ------------ | ------------------------------------------------------------------------------------- | ------ | ------------------------- |
| **A (DO)**   | ToolSpec v3 + 4 ToolChannel + VerifyContract + Reason 透传 + Filter v2（三维，不含 workspace） | **治本** | 19 工具重标 + LTL-Lite 新概念    |
| B (partial)  | 仅 4 ToolChannel + VerifyContract，ToolSpec v3 拆 2 步                                    | 半治本    | metadata 不下沉 → 维持 pooling |
| C (rejected) | 仅 Verify synthesize enforce（DM-005）                                                   | **治标** | Execute 子博弈 IC 未建立        |


> **估时声明**：本 demand 不含估时；详细 PR 拆分 + T 点估时见 `tasks.md`。

**选 A**（双 review 共识）：六原语（类型揭示 + 路由 + 终止约束 + 契约补全 + 信号分离 + 信息透传）覆盖 RC-1..RC-5 三大经典失败模式。

---



## 5. P0 验收标准


| ID           | P0 验收标准                                             | 量化指标                                                                  | 验证手段                                   |
| ------------ | --------------------------------------------------- | --------------------------------------------------------------------- | -------------------------------------- |
| **P0-AC-1**  | LLM 在 review 任务上不再 read_file > 15 次                 | iter 16 必须 return `SynthesizeNowSignal` + 注入 system message           | `TestProbeToolChannelBounded` PASS     |
| **P0-AC-2**  | ProbeToolChannel 在 Bounded(n) 时强制 synthesize        | review/edit/test 三 task_kind 100% 触发                                  | `TestProbeToolChannelBounded` × 3 PASS |
| **P0-AC-3**  | D2 截断必附加 TruncateMarker                             | 截断后含 `complete=false`                                                 | `TestMarkerAlwaysAppended` PASS        |
| **P0-AC-4**  | VerifyContract 4 元组 + 举证规则                          | 空 FinalText → FAIL `deliverable_missing`                              | `TestDeliverableMissing` PASS          |
| **P0-AC-5**  | verdict.Reason 透传到 D1 feishu render                 | 红卡含 `(reason: deliverable_missing)`                                   | `TestRenderVerifyExitReason` PASS      |
| **P0-AC-6**  | 19 工具 metadata 全部显式标注                               | `grep -L "EmissionClass:" surface/*.go` = empty                       | grep gate PASS                         |
| **P0-AC-7**  | ToolSpec v3 0 破坏现有 9 字段                             | position literal = 0                                                  | grep gate PASS                         |
| **P0-AC-8**  | **PlanKind Channel rename 为** `PlanChannel`（PR-B 前） | `type Channel interface` 不再与 `ToolChannel` 同名；允许 1-release type alias | compile + grep gate PASS               |
| **P0-AC-9**  | **read_file/grep/glob 显式 Probe+Bounded(15)**        | 三工具 EmissionClass=Probe 且 IterationBound=Bounded(15)                  | surface spec 单测 PASS                   |
| **P0-AC-10** | **禁止 silent metadata default**                      | 缺 EmissionClass 的 surface 文件 build/test fail                          | CI gate PASS                           |


**P0 验收顺序**：PR-A → AC-6/7/9/10；PR-B 前 → AC-8；PR-B（含 shadow）→ AC-1/2/3；PR-C → AC-4/5 + Learn feedback 写入。

---



## 6. P1 验收标准（博弈论共识增量）


| ID          | P1 验收标准                                                                        | 验证手段                                           |
| ----------- | ------------------------------------------------------------------------------ | ---------------------------------------------- |
| **P1-AC-1** | `design.md §2` 含 **Equilibrium Concept**（单 session SPE + 跨 session reputation） | Spec reviewj                                   |
| **P1-AC-2** | `specs/execute-channels.md` 含 L4–L6 vs L0–L3 **≥3 条** cross-check              | Spec review + 单测                               |
| **P1-AC-3** | VerifyContract 按 EmissionClass 举证规则单测                                          | `TestBurdenOfProofByClass` PASS                |
| **P1-AC-4** | `verify_exit_reason` 写入 Learn **FeedbackMemory**（不只 D1 字符串）                    | `TestReasonInFeedbackMemory` PASS              |
| **P1-AC-5** | Phase B **shadow mode**：log `would_reject` 且不 block；切换前 false positive <5%     | shadow 指标 + 运维 sign-off                        |
| **P1-AC-6** | PromptPressure 软警告 baseline（soft@5 后平均剩余 iter）                                 | 10 session baseline 报告                         |
| **P1-AC-7** | PlanKind × task_kind 交叉：review 时 Probe 不得 OpenEnded                            | `TestPerTaskKindFilter` + cross-consistency 单测 |
| **P1-AC-8** | Filter v2 **不含 workspace 维**（defer）                                            | 代码 + spec 审查                                   |


---



## 7. 与上游 DM 的关系

```
DM-20260701-005  D7 Verify synthesize enforce ────┐  治标 — Phase C 协同
DM-20260701-006  D2 tool_result budget profile ──┤
DM-20260630-012  D7 deliverable convergence ──────┘
                       │
                       ↓ (本 change 治本)
DM-20260701-007  Tool metadata control plane + ToolChannel 自治
                       │
                       ├── Phase C: verify_exit_reason → Learn FeedbackMemory (最小 reputation)
                       ├── Phase B: shadow mode → hard enforce
                       └── Phase E (可选): 完整 drift audit + 半自动 emission_class 分类器
```

---



## 8. 风险


| 风险                              | 缓解                                               |
| ------------------------------- | ------------------------------------------------ |
| ToolSpec v3 breaking change     | 6 新字段 R3 默认值；**禁止** silent pooling default（H12）  |
| PlanChannel vs ToolChannel 命名冲突 | PR-B 前 rename + type alias 1-release（P0-AC-8）    |
| LTL-Lite L4-L6 新概念              | MVP fixture + **L0-L3 cross-check**（H8，P1-AC-2）  |
| PromptPressure 被 LLM 忽略         | hard reject @n+1 兜底；shadow + baseline（P1-AC-5/6） |
| `emission_class` cheap talk     | Learn drift 最小接入 + Phase E 完整 audit（H10）         |
| Phase B 硬切 false positive       | **Shadow mode 1 周**（H11，P1-AC-5）                 |
| Filter 维度过载                     | **workspace 维 defer**（P1-AC-8）                   |
| 19 工具标注 cognitive load          | PR-A 含全量迁移；Phase E 可选半自动分类器                      |


---



## 9. 范围



### 9.1 In Scope（相对 v1 增量）

- H7–H12 博弈论共识机制约束
- P0-AC-8..10、P1-AC-1..8
- Learn **最小接入**：`verify_exit_reason` → FeedbackMemory（H6/H10 最小子集）
- Phase B shadow mode（H11）



### 9.2 Out of Scope


| ID         | 不做                               | 理由                             |
| ---------- | -------------------------------- | ------------------------------ |
| OOS-1      | 历史 T/A/F 路径重编号                   | 跨 30+ 历史 change                |
| OOS-2      | 物理目录迁移                           | 各域自治                           |
| OOS-3      | WaveScheduler / D4 delegate 行为变更 | 无关                             |
| OOS-4      | Clawcode TS Tool 接口              | Go 实现                          |
| OOS-5      | L0-L3 安全 invariant **改造**        | 仅 cross-check，不改造              |
| OOS-6      | R4 strict 默认值全量迁移                | PR-A 含 R3 默认；R4 留 Phase E      |
| OOS-7      | Bash 22 zsh rules 改造             | 无关                             |
| OOS-8      | D1/D3/D4/D6 域元数据                 | 域自治                            |
| OOS-9      | 新建外部依赖                           | Pure Go                        |
| **OOS-10** | **Filter v2 workspace 维**        | 双 review 共识 defer（H12/P1-AC-8） |
| **OOS-11** | **完整 drift audit + 半自动分类器**      | Phase E；本 change 仅最小 Learn 接入  |


---



## 10. 澄清记录



### Q1: 博弈论双 review 结论如何并入需求？

**A**: 2026-07-01 — 机制骨架与方案 A 获一致认可；薄弱点在 cheap talk、均衡声明、Learn 接入、命名 focal point、shadow 迁移。共识项写入 §3.2 H7–H12 与 §5/§6 验收标准。详见 `game-theory-review.md` + `game-theory-review-composer.md`。

### Q2: PlanChannel rename 优先级？

**A**: 2026-07-01 — 双 review 均要求 PR-B 前消歧；Composer 升为 P0（P0-AC-8），Codex 原 P1#2，**共识为 P0 门禁**。

### Q3: Learn 节点本 change 做多少？

**A**: 2026-07-01 — **In Scope 最小子集**：Phase C 将 `verify_exit_reason` 写入 FeedbackMemory；完整 `(declared, actual)` drift 表留 Phase E（OOS-11）。

---



## 更新历史

- 2026-07-01：v1 创建 (RC-1..RC-5 + H1-H6 + 三方案 + P0-AC-1..7)
- 2026-07-01：v1.1 博弈论双 review 共识并入（H7-H12、P0-AC-8..10、P1-AC-1..8、Learn 最小接入、shadow mode、PlanChannel rename、workspace defer）

