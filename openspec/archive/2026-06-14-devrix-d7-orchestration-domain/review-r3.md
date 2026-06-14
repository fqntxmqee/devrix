---
review-id: R3
title: D7 Orchestration Domain — 三次 Review（运行盲区与稳定均衡）
change-id: devrix-d7-orchestration-domain
demand-id: DM-20260613-001
reviewer: Codex (对话沉淀)
review-date: 2026-06-14
status: S3_Design — R3 DRAFT (未裁决，待 Cursor 接力)
predecessor: review-r2.md (R2, 2026-06-14, Claude)
predecessor-status: R2 接力接口已闭合（v1.0 收尾 P0 全绿）
scope: 仅文档，不开发
---

# D7 Orchestration Domain — Review R3 提议

> 本文不修改 `d7-domain.md`、`layering.md`、`proposal.md`、任何 `specs/` 与 `changes/` 文件，仅作为 R3 review 的命题与决议接口。
> 综述与分析（博弈论 + 控制论 + 状态机）已通过对话完成；本文档只承载**可被后续 reviewer 接力**的命题与最小修复路径。
> R2 §6 接力接口已闭合；R3 关注 R2 之后**运行盲区**的 4 个命题与 1 个 NQ 列表。

---

## 0. 与既有 R1/R2 的关系

- **R1**：11 条语义层决议（域定位、三模型、状态机别名、路由矩阵、S5 分阶段、迁移契约、T02 拆分、HandleInterrupt 子能力、Background facade、配置 SoT、T01 修正）。**无修改地接受。**
- **R2**：5 个结构层命题 + 4 个 OQ + 3 个保留分歧。**全部已闭合**（命题 A/B/C/D/E 接受或部分接受；OQ-1~4 定稿）。
- **R3（本 review）**：R2 之后在**实际运行视角**下浮现的盲区。这些盲区 R1/R2 不曾覆盖，因为它们需要"loop 与 Wave 已并存 + BackgroundRun 与 PlanTask 实际跑数据 + IM 限流实际触发"才能观察到。

R3 的命题都遵循 R2 的"接受或反驳"接口契约：每个命题给出**现象 → 结构分析 → 建议最小修复**，请 reviewer 接受 / 反驳 / 列入 P1 路线图。

---

## 1. 命题 A：BackgroundRun ↔ PlanTask 在重启路径上存在双权威竞态

### 现象

`d7-domain.md` §"Task Model Trinity" 段：

> "`BackgroundRun` ... 目标迁入 D7-S1"
> "**映射：** `WaveTaskNode.ID` 可关联 `PlanTask.ID`；`FlowEvent.TaskID` + `link_tasks` 联动 PlanTask 状态。"

`d7-domain.md` §"Background task registration" scenario：

> "AND v1.0 implementation may remain in `query/background.go` with D7-S1 facade (D7-S1-T07)"

可观察的三个真实冲突（对话论证给出反事实）：

- **冲突 A（重启双源）**：PlanTask 在 `contextengine/tasks/` DiskStore 恢复；BackgroundRun 在 `query/background.go` 残留——**两条独立恢复路径**。`bg_*` 进程崩在中间时不会被 PlanTask 的"33% 进度"覆盖，永久卡住。
- **冲突 B（取消竞态）**：HandleInterrupt 步骤 5 "TaskCancel propagates to WorkerCancel"——"worker handles" 指 Wave worker 还是 SubQuery？文档未指明。真实后果：用户看到 `/stop` 后 0.5s 收到 `stopped` 事件，但 BackgroundRun 仍持有 SubQuery goroutine 在跑 LLM。
- **冲突 C（结果回流两路）**：`bg_8` 完成与 `w_3` 完成都触发 `FlowEvent` 推 D7-S1，去重键未规定。两条都更新同一 `PlanTask[t_2].artifacts`，结果可能不一致。

### 结构分析

- $T$（PlanTask，权威在 `contextengine/tasks/`）与 $B$（BackgroundRun，权威在 `query/background.go`）的**状态机终止事件**有两个观察者：D2 SubQuery 内部（写 $B$ 状态）+ D7-S1 经 FlowEvent（试图驱动 $T$）。
- 这是分布式系统里的**双写（dual-write）**问题。`d7-domain.md` §D7 域边界把两者都说成"D7 暂托管（D2）"——**实际上没有 D7 真正托管**，是 D2 内部一致性问题被话术包装成跨域问题。
- 博弈论视角：**无主之地悲剧**——每个写者都假设对方负责，一致性漂移。R1 §9 决议"Background facade"是治理妥协，不是治理解决。
- 风险升级路径：`d7-domain.md` §D7-S1-T08 PLANNED `TransitionTaskState` 是 v1.1 任务。**v1.0 接受任意状态转换**意味着 PlanTask 的"持续做完"在 v1.0 是 **advisory** 而非 **enforced**。

### 建议最小修复（不修改文档，仅供 R3 评审）

1. `link_tasks` 字段应明确写在 $T$ 侧还是 $B$ 侧；哪个事件触发时由谁写入
2. BackgroundRun 在 `query/background.go` 的 facade **必须**承诺"所有状态变更经 D7-S1"（单权威原则）
3. `D7-S1-T08 TransitionTaskState` v1.1 **应先于** BackgroundRun 迁入 D7-S1，否则状态机不闭合
4. 新增 P1 测试点 `D7-S1-T09 BackgroundRun restart recovery`（重启双源回归）

**给 reviewer 的问题**：
- 接受 1~3 作为 R3 决议？
- 还是 P1 路线图项（接受但延后）？
- 还是反驳（v1.0 暂不解决，靠事后 review）？

---

## 2. 命题 B：v1.0 长程任务入口只剩 `/plan`，UX 激励扭曲

### 现象

`d7-domain.md` §"Orchestration Routing Matrix"：

| 路由 | 条件 | 调度者 |
|------|------|--------|
| PlanPath | PlanMode active **或用户 `/plan`** | D7-S2 → S5-P1 |
| SerialExplore | orchestrate + 单步 | D7-S2 |
| WaveExecute | orchestrate + 多 Worker | D7-S3 |

`d7-domain.md` §"S5 Decision Layer — Phased Roadmap"：

- S5-P2 ClassifyIntent ✅ v1.0 必须
- S5-P3 SynthesizeTaskGraph ⬜ v1.1
- S5-P4 auto_detect → PlanMode ⬜ **v1.2**

`d7-domain.md` §Implementation Status 又写：

> "D7-S5 ClassifyIntent / Shadow：✅ IMPLEMENTED"
> `internal/layers/orchestration/coordinator/{classifier,shadow_classifier}.go`

**关键观察**：

- "orchestrate" 标签的唯一来源是 ClassifyIntent 的 LLM 分支
- v1.0 LLM fallback 默认关闭（"rules_enabled: true, llm_fallback: false"）
- 但 S5-P2 实际是 "**规则 + shadow LLM（不实际用）**"——shadow 知道答案但**生产路径不调用**
- 结果：用户发"帮我把项目里所有 DDD 边界违反的地方都修一下"（长程信号明确）→ 走 FastPath → 单次对话答复

**v1.0 实际可用的长程入口只有 `/plan` 一条**。

### 结构分析

这是**信号博弈**（参考 Spence 教育信号模型）：

- 发送 `/plan` = 高成本信号（多打字 + 审批等待）
- 不带 `/` = 低成本信号
- 系统设计**激励用户过度使用 `/plan`**（不带可能误判为 fast，任务根本启动不了）

均衡：用户先用 `/plan` 试探。对系统不利——D7 应主动消除这种过度信号。

更糟的是**与 R2 §2 命题 A 的暗合**：

- R2 命题 A：权力分配写进 D7-D1 Contract
- 但**"长程入口的可达性"是权力分配的另一面**
- 路由权名义在 D7，实际是"必须会 `/plan` 语法"——**权力被 UX 收回**

### 建议最小修复

1. R3 评审明确："S5-P2 = 规则 + shadow" 在 v1.0 **等价于 no-op for routing**——文档需在 §Implementation Status 加"实际无 LLM fallback"旁注（**不**改 `d7-domain.md`，仅 R3 决议承认）
2. 提议把 S5-P4 `auto_detect` **提前到 v1.1**（v1.2 太久）
3. v1.0 临时方案：消息长度 > 50 token 或包含"帮我…所有…每个" → 默认 orchestrate（启发式）

**给 reviewer 的问题**：
- 接受 1~3？
- 还是只接受 1（文档澄清），2 推迟到 v1.1 路线图？

---

## 3. 命题 C：WaveScheduler "detached task context" 设计在 normal-end 路径存在沉默失败

### 现象

`d7-domain.md` §"Interrupt handler cancels active orchestration (/stop)" 步骤 1：

> "`WaveScheduler.CancelAll(sessionID)` — explicit; Wave task ctx is detached from Process (`wave/scheduler.go`)"

`d7-domain.md` §"Normal Process end does not cancel Wave"：

> "AND only HandleInterrupt (`/stop`) triggers step 1 above"

R2 §2 命题 E 已确立：**Process 先于 Wave 取消是错的；正确顺序是 Wave→D4→Process→stopped Event→TaskCancel→WorkerCancel**。R2 §6 接力接口 5 已接受此契约。

**但 R2 未覆盖 normal-end 路径**——`/stop` 之外的 Process 结束（如 LLM 自然完成对话轮次）不触发 CancelAll，detached Wave 继续跑。

### 结构分析

设 Process 生命周期 $P = [P_{\text{start}}, P_{\text{end}}]$，Wave 任务生命周期 $W = [W_{\text{start}}, W_{\text{end}}]$。设计要求 $W \supseteq P$（detached 存活）。$P_{\text{end}}$ 有两种：

- **正常结束**（无 `/stop`）：$W$ 持续到自然完成 → 四个失败模式
- **异常中断**（`/stop`）：$W_{\text{end}} = \text{CancelAll} \text{ time}$ → 取消竞态

四个 normal-end 失败模式（对话反事实）：

1. **孤儿 Wave 任务**：新消息进 D1 时旧 Wave 还在跑，IM 同屏显示新旧两套进度
2. **幽灵写**：Wave worker 完成时 `PlanTask[t_2]` 已被 `/plan` 取消，artifacts 写错目标
3. **goroutine 泄漏**：detached ctx 不被 GC，LLM streaming goroutine 累积
4. **slot 永久占用**：HandleInterrupt 失败时 `cursor=1, claude_code=1, subagent=3` 占满，后续所有 Wave 阻塞

D6 视角的"治理盲区"——detached Wave 在 D6 无 `Process.parent` 引用，**进程级孤儿检测做不到**。权力大于责任，激励不相容。

### 建议最小修复

1. 明确 Wave 任务的**最大允许 detached 时长**（建议 30 min，超时由 WaveScheduler 内部 watchdog kill）
2. normal-end 触发 `WaveScheduler.NotifyProcessEnded(sessionID)`，Wave 在 grace period 内完成；未完成由 Wave 标记"degraded"并在 IM 显示
3. HandleInterrupt 步骤 1 的 `CancelAll` 失败重试 N 次 + 强制清理 slot
4. D6 metric 新增 `wave_orphan_count` 与 `wave_detached_seconds_p99`

**给 reviewer 的问题**：
- 接受 detached 设计的**对称性原则**（Process end 也应 cancel Wave）？
- 还是保留当前非对称设计，补 1~4 的治理补丁？

---

## 4. 命题 D：D2 → D1 IM 限流参数与 D7 FlowEvent 频率存在隐式耦合

### 现象

`d7-domain.md` §Configuration：

```yaml
context_engine:
  execution_flow:
    enabled: false
    link_tasks: true
    im_progress: true
    tool_summary_throttle_ms: 500
    event_buffer_size: 32
```

四个字段是**D2 → D1 IM 协议**痕迹：`tool_summary_throttle_ms: 500` 防刷屏，`event_buffer_size: 32` 流量整形。

D7 引入后 FlowEvent 频率保守估计提升 3 倍（原 D2 + Wave 进度 + BackgroundRun + link_tasks 联动）。限流参数配置**仍在 D2**。

### 结构分析

设 FlowEvent 到达率 $\lambda(t)$，D1 IM 推送速率上限 $\mu$。稳定性条件 $\lambda(t) \leq \mu$。

引入 D7 后 $\lambda$ 提升 3 倍，500ms 限流下**有效信息密度下降**——D7 增强可观察性反而让用户感知更糟。

配置归属**与执行归属错位**：
- D2 配置限流，但**实际产生事件的是 D7**
- D7 想控制 IM 推送节奏时，无配置入口
- D1 拥有 ingress 但**不拥有限流配置**

博弈论上：限流参数是"公共池塘资源"——D2 写、D7 读、D1 受影响，**无主之地**。

### 建议最小修复

1. v1.0：限流参数从 `context_engine.execution_flow` 提升到 `orchestration.im_throttle`（D7 拥有语义）
2. v1.1：`event_buffer_size` 与 `im_progress` 同步迁移
3. 评审接受 R3 决议后**记入 P1**，v1.0 收尾不动（避免回归）

**给 reviewer 的问题**：
- 接受 v1.0 不动 / v1.1 迁移路径？
- 还是 P2 路线图项？

---

## 5. NQ（New Questions）— 留待 R4 或专门 issue

| NQ | 问题 | 来源 |
|----|------|------|
| NQ-1 | Loop 拆三段（执行原子 / 会话编排 / 协议适配）的具体接口签名 | R3 对话"博弈论视角"得出，但需 R3+ 进一步设计 |
| NQ-2 | `d7_enabled=false` 的 "bit-identical" 承诺是否可被弱化为 "behaviorally equivalent" | R2 §5 P0 #4 隐含未议 |
| NQ-3 | Shadow Classify 的"命中即生效"门槛（流量阈值 + 误判率阈值） | R2 §3 OQ-3 决议 B 改进版未给阈值 |
| NQ-4 | BackgroundRun DiskStore 路径是否复用 `context_engine.tasks.store_dir` 还是独立 | 命题 A 修复 2 的前置 |
| NQ-5 | WaveScheduler detached watchdog 的 owner（D7-S3 内部？D6 外部？） | 命题 C 修复 1 的前置 |

---

## 6. 与 d7-domain.md v2.3.0 的关系

**本文不修改 `d7-domain.md` 任何字符。**

R3 提议的所有修复均为"评审决议"层级，不进入 spec。若 reviewer 接受任一命题，下一步：
- 接收方应在 `changes/devrix-d7-orchestration-domain/` 下开新 change（或在 `tech-debt/` 下登记）
- 该 change 的 proposal/design/spec 才动 `d7-domain.md` 文本

这是与 R2 §5 P0 同步规则一致的做法——R2 强调"v1.0 收尾硬要求 = 文档同步"，R3 同样主张"修复不绕过 change 流程"。

---

## 7. 评审检查清单

- [ ] 命题 A（BackgroundRun ↔ PlanTask 双权威）接受 1~4？
- [ ] 命题 B（长程入口 UX）接受 1~3？
- [ ] 命题 C（Wave detached normal-end 失败）接受 1~4？
- [ ] 命题 D（IM 限流配置归属）接受 1~3？
- [ ] NQ-1~5 是否需要升级为 R3 命题（升级即填入 §1~4）？
- [ ] R3 整体是否应进入 v1.0 P0 / v1.1 P1 / v1.2+ P2？

---

**维护**：R3 决议由后续 reviewer（Cursor / 人工）填入 §1~4 末尾"给 reviewer 的问题"位置；不接受时回退为 `[REFUSED: 理由]`。NQ 接受时升级到 §1~4 或独立 issue。
