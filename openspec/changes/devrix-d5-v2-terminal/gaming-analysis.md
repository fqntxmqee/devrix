# D5 Observability — 博弈论分析（裁判域 + 跨域激励）

**Change ID:** devrix-d5-v2-terminal  
**Demand ID:** DM-20260619-006  
**日期:** 2026-06-19  
**状态:** S3 Draft — **供 Owner + Claude 博弈论对焦**  
**关联:** `gaming-analysis.md` 方法论见 `docs/methodology/dsaft-refactoring-playbook.md` §阶段2

---

## 0. 文档目的

| 文档 | 角色 |
|------|------|
| `demand.md` | 可验证 AC + 范围 |
| `proposal.md` | North Star + S 切法 |
| **本文件** | 多方激励、错配根因、Commitment 装置、开放问题 |
| `design.md` | Decision + 物理终态 |
| `d5-boundary.md` | 跨域契约 SoT |
| `d5-domain.md` | 领域 North Star + 博弈角色表 |

---

## 1. 多方博弈位置

```
Principal（用户 / SRE / 平台 Owner）
    ↓ 委托「系统要可解释、可排障」
各业务域玩家（D1/D2/D3/D4/D7/D6）
    ↓ 局部最优：少埋点、快上线、改自己包最省事
D5 Observability（Referee + Auditor）
    ↓ 产出：Span / Metric / Log / Coverage / Incident
外部观测者（Jaeger / Prometheus / on-call）
    ↓ 验证：Trace 树、zero_hit、Health、export bundle
Principal 事后问责
```

**D5 不是业务玩家**，不参与 Turn 编排、不选模型、不执行 Tool。博弈角色：**Referee（裁判）+ Auditor（审计）+ Commitment Device（T/Span 承诺装置）**。

### 1.1 与 D7「Mediator」的对称

| 域 | 博弈角色 | 保证什么 | 不保证什么 |
|----|----------|----------|------------|
| D7 | Mediator / Turn Leader | 编排路径可验证、Turn span 存在 | 结论质量 |
| D2 | Execution Follower | Prepare/Tool/Persist 机制 | 走哪条编排路径 |
| **D5** | **Referee + Auditor** | **埋点覆盖、Trace 可交叉引用、导出可复现** | **业务正确性、性能 SLA（除已登记 metric）** |
| D6 | Judge | 评测/守卫 advisory | 不阻塞主路径 |

**核心洞察：** D5 保证「**可观测过程诚实**」（有没有 span、有没有 hit、能不能导出），不保证「**业务决策正确**」。

---

## 2. 玩家与目标函数

| 玩家 | 目标函数（局部） | 全局希望 | D5 机制响应 |
|------|------------------|----------|-------------|
| D2/D7 开发者 | 改功能包最快；埋点可后置 | E2E 可排障 | Operation Registry + Coverage 独立于采样 |
| 新功能作者 | 复制粘贴 span 名 | canonical op 一致 | `telemetry.Op*` + WARN unknown op |
| SRE | 生产一眼看懂哪层坏了 | layer/component 属性 | `SpanAttrs` 强制注入 |
| 测试作者 | 单测 mock 观测 | Bridge + NoOp | `NewNoOp()` 不阻断业务 |
| 诊断工具作者 | 功能堆进 diagnose/ | S 层语义清晰 | S23 子承诺 C3a–C3e |
| 架构维护者 | 目录=文档 | 双锚点对齐 | v2.1 删 bridge、补 d5-domain |

---

## 3. 现状均衡失灵（重构动机）

### 3.1 S 被 Go 包名绑架（已部分修正）

| 时期 | 均衡 | 问题 |
|------|------|------|
| v1.0 前 | S1–S9 = tracer/metrics/logger… | 消费者思维是「生成遥测」一坨，却被拆成 9 个「模块 S」 |
| v1.0 Registry | S21–S24 价值流 | 注册表对了，spec 主叙事仍 Legacy |
| v2.0 Structure | 物理路径迁移 | bridge 残留，半终态 |
| **v2.1 Terminal** | 三锚闭合 | 本 change 目标 |

**错配模式（Playbook）：** 开发者局部最优（沿用 bridge import）≠ 架构全局最优（单一 canonical 路径）。

### 3.2 「有 T 无 Span 主路径文档」= cheap talk

| 现象 | 博弈后果 |
|------|----------|
| spec 写 `query.loop` 主路径，代码已 D7 Turn | on-call 在 Jaeger 找错 op → 误判 D2 路径 |
| 39 IMPLEMENTED T，spec SoT 仍 S1–S9 | P0 验收锚点分裂 |
| Doctor/Tracker 有 T 无 A | 实现者可随意改语义 |

**修正：** observability-guide + spec v3.0 统一 Trace；a-registry v4.0 补 A07–A10。

### 3.3 S23 膨胀为「诊断万能包」

诊断工具 change（DM-20260616~18）在 **无 S 层扩展** 下堆入 tracker/doctor/faultinject：

| 能力 | 真实博弈功能 | 应登记为 |
|------|-------------|----------|
| Coverage | **Audit** — 埋点是否诚实 | C3a |
| Incident export | **Evidence bundle** — 事后举证 | C3b |
| Doctor | **Pre-flight audit** — 环境是否就绪 | C3c |
| Tracker | **Continuous audit** — 编辑后诊断不阻塞 | C3d |
| FaultInject | **Test-only sabotage** — 测韧性 | C3e |
| DebugFilter | **Signal conditioning** — 降噪 | **C1 / S21**（非 S23） |

**不增 S25 的理由：** 均属「审计/举证」同一子博弈场；用 A 层区分策略即可（原则 2）。

### 3.4 DebugFilter / SessionBridge 归属错配

| 组件 | 错误归属 | 正确博弈语义 |
|------|----------|--------------|
| DebugFilter | T 挂 S23 | **Instrument 管道滤波** → S21-A14 |
| SessionBridge gauge | T 挂 S23 | **Facade 集成面** → S0-A03 |

---

## 4. Canonical S 的博弈角色（切法 A — 价值流）

| S | 价值流 | 博弈角色 | Commitment 类型 | North Star 片段 |
|---|--------|----------|-----------------|-----------------|
| D5-S21 | Instrument | **Signal Producer** | Span/Metric/Log 可生成 | 「任意 op 可观测」 |
| D5-S22 | Export | **Signal Shipper** | 数据到达 OTLP/Prom | 「外部系统能收到」 |
| D5-S23 | Diagnose | **Auditor + Evidence Clerk** | Coverage/Health/Export/Tracker | 「埋点诚实 + 可举证」 |
| D5-S24 | Configure | **Rule Setter** | 配置切换可验证 | 「规则变更可测」 |
| D5-S0 | Facade | **Integration Shell**（非独立子博弈） | Init/Bridge/NoOp | 「业务不被观测拖死」 |

### 4.1 S23 子承诺的博弈分工

| 子承诺 | 角色 | 验证者 |
|--------|------|--------|
| C3a Coverage | **Compliance Auditor** | CI + Health zero_hit |
| C3b Incident | **Evidence Archivist** | `debug export` schema |
| C3c Doctor + Health | **Environment Auditor** | `/doctor` + `/health` |
| C3d Tracker | **Continuous Inspector** | 非阻塞 T + integration |
| C3e FaultInject | **Red Team（testbuild）** | test tag only |

---

## 5. 跨域激励相容（D5 ↔ 业务域）

### 5.1 RecordHit 独立于采样 = 防「选择性失明」

| 策略 | 无 RecordHit 时 | 有 RecordHit 时 |
|------|----------------|-----------------|
| 开发者设 `always_off` 偷懒 | 无 span，看似「没调用」 | Hit 仍涨 → Coverage 发现 |
| 采样降本 | 合理 | Hit 与采样解耦 |

**机制设计：** `Tracer.Start` **无条件** `RecordHit`（spec P0 Requirement）。

### 5.2 Bridge 零侵入 = 防「观测绑架业务」

| 策略 | 后果 |
|------|------|
| 观测 panic 拖垮 Process | 用户流失 |
| `NewNoOp()` + nil 守卫 | 观测失败 → degraded，业务继续 |

**D5 承诺：** Graceful Degradation（Facade S0）。

### 5.3 D2 Tracker 只读边界 = 防「双 SoT」

| 错配 | 均衡 |
|------|------|
| D2 再建 tracker 写模型 | LRU 两套，诊断不一致 |
| D5 `diagnose/tracker` 唯一写主权 | D2 Surface 只读 `Recent()` |

见 `d5-boundary.md` §3。

### 5.4 D7 创建 Turn span，D5 提供命名 = 权责分离

| 谁创建 span | 谁定义 op 常量 | 博弈效果 |
|-------------|---------------|----------|
| D7 `orchestration.turn.*` | D5 `names.go` | Leader 对编排诚实；Referee 对命名诚实 |
| 若 D5 创建 Turn span | — | D5 侵入 Leader 子博弈（越界） |

---

## 6. T 层作为 Commitment Device

```
L4/F 实现变更 → 查关联 T → 测试绿 → 规格同步
```

| 原则 | D5 应用 |
|------|---------|
| T 最稳定 | 41 T ID 本 change **不改号** |
| Legacy 双轨 | canonical_s / canonical_a 列校正 |
| P0 无 Span 证据 | PLANNED T 本 change 闭合 2 条 |

**影子策略风险：** `D5-S21-A05-T01/T02` PLANNED 但功能可能存在 → 规格债务，鼓励「改了不测」。

---

## 7. v2.1 Terminal 的博弈含义

| 动作 | 激励效应 |
|------|----------|
| 删 bridge | 消除「双路径」局部最优 |
| spec v3.0 Canonical | 验收锚点单一 |
| d5-domain + boundary | 跨域谈判有 SoT |
| gaming-analysis（本文） | 后续改 S23 须回答「属于哪条子承诺」 |

---

## 8. 开放问题（供 Claude / Owner 对焦）

| # | 问题 | 选项 | 倾向 |
|---|------|------|------|
| OQ-1 | S23 是否未来拆 S25「TerminalTools」？ | 保持 C3a–C3e / 拆 S25 | **保持** — 除非 Tracker 独立产品化 |
| OQ-2 | Doctor 失败是否应阻塞 Turn？ | 仅 warn / 硬失败 | **仅 warn** — D5 审计不阻塞 D7 Leader |
| OQ-3 | Coverage zero_hit 是否应 CI 阻断发布？ | advisory / gate | **advisory** — 与 D6 Delta 分工；可配置 gate |
| OQ-4 | FaultInject 是否允许 integration 默认开启？ | testbuild only / opt-in env | **testbuild only** — 防生产 sabotage |
| OQ-5 | D6 写指标 vs D5 写指标边界 | D6 经 Bridge / D6 直 OTel | 维持 **D6 直 OTel sink 到 D5 meter**（见 d6-domain） |
| OQ-6 | `legacy_harness` path metric 何时删 | v2.1 / 下一 major | **v2.1 文档标 DEPRECATED**；计数保留至 harness 代码全清 |

---

## 9. 与 D7 / D2 重构的对称总结

| 重构 | Leader/Follower/Referee 变化 |
|------|------------------------------|
| D2 SA Refine | D2 = Follower；QueryLoop 从 D2 剥离 |
| D7 SA Refine + v2 Structure | D7 = Leader；物理路径对齐 S |
| **D5 v2.1 Terminal** | **D5 = Referee 文档与路径终态**；不抢 Turn 主权 |

---

---

## 11. Claude 博弈论对焦（2026-06-19）

**状态:** 对 §8 OQ 1–6 与 `d5-requirements-clarifications.md` §6 Grill Review 1–6 的逐条回应  
**方法:** 建模为多方不完全信息博弈 + 承诺装置评估

---

### 11.1 OQ 回应

#### OQ-1: S23 是否未来拆 S25？

**博弈论视角：不拆——但需设置"拆"的触发条件。**

当前 S23 的 C3a–C3e 属于同一子博弈场（审计/举证），用 A 层区分策略是正确的。从机制设计角度，关键不是"现在拆不拆"，而是**"什么条件下会拆"应该预先声明**。

| 触发条件 | 含义 |
|----------|------|
| Tracker 独立产品化（被外部系统消费） | 不再是内部审计 → 新 S |
| C3e FaultInject 被要求生产可用 | 不再是 testbuild-only → 新博弈语义 |
| C3 子承诺数 > 7 | Schelling 点：超过 7 个子承诺意味着 S 层语义不再清晰 |

**建议：** 在 `d5-domain.md` 中写入 S25 的触发条件，这本身是一种**预先承诺（Pre-commitment）**——防止未来"想拆就拆"的策略漂移。

**倾向：保持（与现有分析一致）。**

---

#### OQ-2: Doctor 失败是否应阻塞 Turn？

**博弈论视角：仅 warn——否则破坏 D5 的 Referee 独立性。**

```
若 Doctor 失败 → Turn 阻塞
  → D5 从 Referee 变成 Gatekeeper
  → D7 Leader 的策略空间被 D5 健康状况约束
  → D5 故障 = 全局故障（道德风险反转）
```

D5 的核心承诺是"审计不阻塞业务"（Graceful Degradation）。Doctor 是**环境审计**，不是**准入控制**。准入控制归 D7-S2（Screening）。

但有一个关键边界：**Doctor 的 warn 必须足够响亮。** silent warn = 无激励效果。需要确保：
- `doctor.fail` 事件在 Health 端点可见
- 持续 fail 触发 `degraded` 状态
- D5 提供 doctor 指标，D7 **可选**读取（非强制）

**倾向：仅 warn（与现有分析一致）。**

---

#### OQ-3: Coverage zero_hit 是否应 CI 阻断发布？

**博弈论视角：advisory 为主，但需区分"可解释 zero-hit"和"不可解释 zero-hit"。**

纯阻断的问题：条件 Operation（如 `context.harness.*`）在正常路径下 zero-hit 是预期的。阻断会惩罚正确行为。

但纯 advisory 的问题：开发者无激励修复**不可解释**的 zero-hit。

**建议分级机制（与 P1 建议对齐）：**

| zero-hit 类型 | CI 行为 | 博弈含义 |
|---------------|---------|----------|
| 条件未触发 | advisory only | 正常，无惩罚 |
| 路径未启用 | advisory only | 正常，无惩罚 |
| 可疑（注册了但无任何触发路径） | **warning**（不阻断） | 信号：可能死代码 |
| **注册表有但 names.go 无** | **hard fail** | 契约破裂 |

**倾向：advisory（与现有分析一致），但建议增加可疑 zero-hit 的 warning 级别。**

---

#### OQ-4: FaultInject 是否允许 integration 默认开启？

**博弈论视角：testbuild only——这是不可谈判的安全边界。**

```
FaultInject 在生产 = 系统内生 sabotage 能力
  → 攻击面：任何能改配置的人都能注入故障
  → 无法区分"测试故障"和"真实攻击"
```

testbuild tag 是一个**硬承诺装置**——编译时强制执行，运行时无法绕过。这与 D5 的 S18（权限检查）同构：**某些能力必须在编译边界上被限制，而非运行时配置。**

但需要确认：**testbuild tag 在生产二进制中是否确实不存在 FaultInject 代码？** 如果只是 feature flag，攻击者改配置即可激活。

**倾向：testbuild only（与现有分析一致）。建议增加编译时验证——生产二进制中 `grep FaultInject` 返回空。**

---

#### OQ-5: D6 写指标 vs D5 写指标边界？

**博弈论视角：维持 D6 经 Bridge → D5 meter。这是正确的产权分配。**

```
D5 拥有: Meter 实例、Label 白名单、Prometheus 端点
D6 拥有: 什么指标、何时写入、指标语义
```

这是典型的**基础设施提供者（D5）vs 基础设施消费者（D6）**的产权分离：
- D5 保证"管道畅通"（meter 可用、label 合法）
- D6 保证"数据有意义"（指标定义、写入时机）

**D6 直连 OTel 的风险：** D6 绕过 D5 的 label allowlist/blocklist → CPR 治理被绕过 → 公地悲剧。

**倾向：维持 D6 经 Bridge 到 D5 meter（与现有分析一致）。**

---

#### OQ-6: `legacy_harness` path metric 何时删？

**博弈论视角：v2.1 标 DEPRECATED，删除时机 = harness 代码全清。**

`runtime_path_resolved_total{path="legacy_harness"}` 是 **ESS 检测器的对照臂**。只要 harness 代码还在，就需要这个 label 来回答"旧均衡是否已被新均衡完全替代"。

过早删除的代价：失去对迁移进度的客观度量。唯一能证明"旧路径已死"的证据就是这个 counter 长期为零。

**建议增加删除条件：连续 2 release 的 `legacy_harness` 计数为零 + harness 代码已物理删除。**

**倾向：v2.1 标 DEPRECATED，保留计数（与现有分析一致）。**

---

### 11.2 Grill Review 回应（d5-requirements-clarifications.md §6）

#### Grill-1: Referee 边界 — D5 审计失败时，系统应 warn 还是 fail？

**博弈论答案：warn。理由已覆盖于 OQ-2。**

补充一个博弈论维度：**审计失败时的 fail 策略会创造"审计者绑架系统"的激励。** D5 可以通过"让自己失败"来阻止整个系统运行——这在委托代理链中是权力的滥用。Referee 不应拥有对比赛的否决权。

**D5 的博弈定位：Referee 吹哨（warn），但不终止比赛（fail）。终止比赛的权力归 D7（Leader）或用户（Principal）。**

---

#### Grill-2: Coverage as gate — zero_hit 是否应成为发布硬门禁？与 D6 Delta gate 如何分工？

**博弈论答案：D5 Coverage 与 D6 Delta 是互补分工，不应合并。**

| 维度 | D5 Coverage Gate | D6 Delta Gate |
|------|-----------------|---------------|
| 检查什么 | 埋点是否诚实（span 是否被触发） | 行为是否合规（输出是否满足约束） |
| 失败含义 | "我们不知道发生了什么" | "我们知道发生了什么，但不对" |
| 博弈类型 | Monitoring game（是否被观察） | Compliance game（是否守规则） |
| 阻断时机 | 可疑 zero-hit（硬契约破裂除外） | Critical 违反（如权限逃逸） |

**分工理由：** Coverage 回答"系统是否可观测"——这是认识论问题。Delta 回答"系统行为是否正确"——这是合规问题。混在一起会模糊责任边界。

**建议：D5 Coverage 提供 advisory 信号给 D6；D6 Delta gate 决定是否阻断。** D5 是证人，D6 是法官——证人不判案。

---

#### Grill-3: S23 子承诺上限 — C3a–C3e 是否已触及「万能 S」边界？

**博弈论答案：当前未触及，但需要显式边界。**

C3a–C3e 均属审计/举证子博弈，语义凝聚力强。但危险信号是：**"诊断辅助"这个词在中文里可以塞进任何东西。**

**建议设置硬边界：**

| 边界 | 规则 |
|------|------|
| 语义边界 | S23 只含"事后审计/举证"；"实时准入控制"归 D7，"即时执行决策"归 D2/D4 |
| 数量边界 | 子承诺数 ≤ 7（超过则拆 S25） |
| 依赖边界 | S23 不 import D2/D4/D7（除 contracts 接口） |

**当前状态：C3a–C3e（5 项），未超边界。但需要让这些边界显式可查。**

---

#### Grill-4: 选择性失明 — RecordHit 独立采样是否足够？是否需要「未 Register 的 op 硬拒绝 Start」？

**博弈论答案：RecordHit 解耦是优秀的机制设计。硬拒绝是不必要的过度约束。**

```
当前：未知 op → 仍创建 span + WARN → 向后兼容 + 可发现
提议：未知 op → 硬拒绝 Start → 新 op 必须先在 registry 注册才能用
```

硬拒绝的问题：
1. **创新障碍**：新功能实验阶段需要快速迭代 span 名，注册表成为瓶颈
2. **过度集中化**：D5 获得对所有域 span 命名的否决权——权力不对称
3. **WARN 已足够**：生产日志中的 `unknown operation` 是可发现的，且不阻断业务

**RecordHit 独立于采样的设计已经解决了核心的"选择性失明"问题。** 硬拒绝会创造新的问题（注册表成为单点阻塞），而收益有限。

**建议：维持现状。WARN + unknown operation 计数已构成足够的激励。**

---

#### Grill-5: FaultInject 伦理 — testbuild only 是否足够防生产误开？

**博弈论答案：testbuild only + 编译时验证 = 足够。**

已覆盖于 OQ-4。补充：**生产二进制中不应存在 `faultinject` 包的 import。** 这是编译边界的硬承诺——如果生产二进制编译通过，但 `faultinject` 代码存在，则边界失效。

**建议验收标准（AC）：生产构建（`go build -tags=""`）的二进制中，`strings devrix | grep -i faultinject` 返回空。**

---

#### Grill-6: Legacy harness metric — `legacy_harness` path label 保留多久？对路径分流观测有何扭曲？

**博弈论答案：保留至 harness 代码物理删除 + 2 release 零计数期。**

`legacy_harness` label 是观测新旧均衡迁移的唯一客观指标。但长期保留的代价：

| 代价 | 说明 |
|------|------|
| 认知负荷 | on-call 看到两个 path，需理解含义 |
| 仪表盘膨胀 | 每个 path label 增加 Prometheus 时间序列 |
| 无限期保留 = 无承诺 | 不设删除条件 = 永远不删 |

**建议在 `design.md` 中写入 `legacy_harness` metric 的退役计划，使"删除"本身成为一个可验证的承诺。**

**倾向：v2.1 标 DEPRECATED；退役条件写入 design.md §6。**

---

### 11.3 补充博弈论视角（现有分析未覆盖）

#### 11.3.1 Bridge 删除 = 烧船（Burning the Ships）

删除 9 个 bridge 包的博弈含义不仅是"代码清理"：

```
桥还存在 → 开发者可以走旧路径 → 局部最优：继续用熟悉的 bridge import
桥被烧毁 → 只剩 canonical 路径 → 强制迁移，消除双路径均衡
```

这是**烧船承诺（Burn-the-Boats Commitment）**——通过消除撤退选项来强制新均衡。D6 已先行（v2.0.1 删 bridge），D5 跟进是正确的信号传递：**"半终态不是终态"是一个可验证的声明。**

**风险：** 如果删除后发现有遗漏的外部 import，回滚成本高于保留 bridge。但当前 grep 已确认仅 D5 内部 5 处依赖，风险可控。

#### 11.3.2 T ID 冻结 = Schelling 点

41 个 T ID 不改号是一个**谢林点（Schelling Point）**——在多方协调中，T ID 作为"已经存在的编号"自然成为各方默认选择的锚点。

```
若允许改号 → D7/D2 的 T 编号也可能被要求改 → 级联震荡
T ID 冻结 → 各方以 T ID 为锚协商 canonical_s/a 列 → 协调成本最低
```

**评价：正确决策。** 用 canonical_s/canonical_a 列做语义校正，比改 T ID 的协调成本低一个数量级。

#### 11.3.3 D5 Referee 独立性的内在张力

现有分析将 D5 定位为 Referee，但有一个未被讨论的张力：

**D5 的 Bridge 注入模式意味着 D5 依赖各域主动调用它。** 

```
真正的足球裁判：独立于球员，主动判罚
D5 Referee：等待各域调用 tracer.Start()，被动记录

→ D5 是"被动记录员"而非"主动裁判"
```

这个区分影响 S23 的诊断工具设计：
- Doctor：主动检查（接近裁判）
- Coverage：被动对账（接近审计）
- Tracker：连续被动记录（接近监控）

**建议：** 在 `d5-domain.md` 中区分 D5 的主动审计能力（Doctor/Health）与被动记录能力（Span/Coverage）。两者的博弈含义不同——主动审计是 Referee，被动记录是 Scorekeeper。

#### 11.3.4 双轨制的博弈成本

spec v3.0 Canonical + Legacy S1–S9 冻结 → 双轨并行。这个设计在 D1/D2/D7 重构中都出现了。

**博弈论视角：双轨是过渡均衡，本身不是终态。** 

| 阶段 | 均衡 | 成本 |
|------|------|------|
| Legacy only | 单轨，局部最优 | 消费者思维被模块名绑架 |
| Canonical + Legacy | 双轨，协调中 | 新人读 spec 困惑；需额外注释 |
| Canonical only | 单轨，全局最优 | 无 |

**D5 v2.1 是双轨→单轨的过渡点。** 当 bridge 删除 + spec v3.0 完成后，Legacy 轨仅存在于 RETIRED 节中——这在博弈论意义上是**接近单轨的**。

---

### 11.4 总结表

| 议题 | 倾向 | 关键理由 |
|------|------|----------|
| OQ-1 S23 拆 S25 | 保持 | 预设触发条件；当前 5 子承诺未超边界 |
| OQ-2 Doctor 阻塞 Turn | 仅 warn | Referee 不否决 Leader；防审计绑架系统 |
| OQ-3 Coverage 阻断 CI | advisory | 区分可解释/不可解释 zero-hit；D5 是证人非判官 |
| OQ-4 FaultInject 生产 | testbuild only | 编译硬边界；需二进制验证 |
| OQ-5 D6→D5 metric 边界 | 经 Bridge | CPR 治理不可绕过 |
| OQ-6 legacy_harness metric | DEPRECATED | 2 release 零计数 + harness 全清后删除 |
| Grill-1 Referee 失败策略 | warn | Referee 吹哨不终止比赛 |
| Grill-2 Coverage vs Delta | 互补分工 | D5 证人 + D6 法官 |
| Grill-3 S23 万能 S 风险 | 设硬边界 | 语义/数量/依赖 三边界 |
| Grill-4 硬拒绝未注册 op | 不需要 | WARN 已足够；硬拒绝过度集中化 |
| Grill-5 FaultInject 安全 | 编译时验证 | 生产二进制中不可存在 |
| Grill-6 legacy metric 保留 | 写入退役计划 | 使"删除"成为可验证承诺 |
| 补充: Bridge 删除 | 烧船承诺 | 强制新均衡，风险可控 |
| 补充: T ID 冻结 | Schelling 点 | 协调成本最低 |
| 补充: Referee 独立性 | 区分主动/被动 | Doctor=裁判，Coverage=计分员 |
| 补充: 双轨成本 | 接近单轨 | bridge 删除后 Legacy 仅剩 RETIRED 节 |

---

### 11.5 建议写入 design.md 的补充项

1. **S25 触发条件**（OQ-1 落地）：`d5-domain.md` 中写入 3 条触发条件
2. **zero-hit 分级**（OQ-3/Grill-2 落地）：Coverage 报告区分条件/路径/可疑三类
3. **S23 硬边界**（Grill-3 落地）：语义/数量/依赖三边界写入 `design.md` §5
4. **`legacy_harness` 退役计划**（OQ-6 落地）：删除条件 + 时间线写入 `design.md` §6
5. **FaultInject 生产验证**（Grill-5 落地）：AC 增加编译后二进制检查
6. **D5 完备性边界**（补充落地）：`d5-domain.md` 中声明"D5 保证什么/不保证什么"

---

## 13. 第二轮博弈论视角（增量 — 2026-06-19）

> §13 是 v1.1.0 之后的第二轮增量。聚焦 §11 未展开的**结构性问题**：信息不对称、ESS、SRE 角色、时间属性、公共物品、多 Referee、承诺强度。不重复既有 OQ/Grill。

---

### 13.1 D5 与业务域之间的「信息不对称 + 道德风险」

**问题：** D5 期望各业务域主动埋点，但业务域开发者拥有 D5 不知道的**私有信息**：

| 私有信息 | 业务域可用，D5 不可见 | 后果 |
|----------|----------------------|------|
| 关键失败路径 | 哪些分支算「异常」由业务语义决定 | D5 只能记录「被埋的点」，无法判断「该埋未埋」 |
| 上下文属性 | user_id / tenant / request 维度 | 业务域可选择性注入属性，影响 on-call 能否定位 |
| 异常吞没 | 是否有 recover()/suppress() | D5 看到 span 存在，丢失了「被吞掉」的信号 |

**博弈后果：** 经典 **委托—代理问题（Principal-Agent）**：
- Principal = D5 Referee（要全局可观测）
- Agent = 各业务域开发者（要本地快速上线）
- Agent 信息优势 → 可能 **cheap talk**（只埋对成功路径的 span，假装合规）

**现有缓解（v2.1 已部分落地）：**

| 装置 | 博弈功能 | 局限 |
|------|---------|------|
| `Operation Registry` (56 ops) | 强制 op 名收敛 | 仅约束「已埋」的；无法检测「该埋未埋」 |
| Coverage 报告 | 揭示「未覆盖路径」 | 仅基于已埋点反推；不能揭示业务关键路径未埋 |
| `WARN unknown op` | 对漂移施压 | 仍允许运行；agent 可选忽略 |
| `RecordHit` | 给业务方自查工具 | 业务方可选择性调用 |

**未对焦缺口：** **如何激励业务域主动暴露私有信息**？当前装置都是「下游审计」，缺「上游承诺激励」。

**建议：**
1. **Coverage 分级（沿用 §11.5 第 2 项）**：把零点击拆为「条件/路径/可疑」三类，给业务域**暴露偏差的成本信号**。
2. **业务域「埋点自检」SDK**：轻量级 lint/hook，写代码时即被提示「此处异常路径未埋」（agent 自检前置，不依赖 D5 运行时）。
3. **D5 与 D6 联合审计**：D5 揭示「埋点稀疏性」，D6 揭示「行为偏差」。两路信号共同放大 cheap talk 的成本。
4. **不推荐：硬拒绝未注册 op**（Grill-4 已记录）—— 这会把信息不对称转为集中化决策，与 v2.1「下放埋点权」的激励结构相反。

---

### 13.2 Operation Registry 作为 Commitment Device：是不是 ESS？

**建模：**

```
业务域开发者策略集：
  A1: 使用 canonical op（合规）
  A2: 使用 ad-hoc string 名（绕开）
  A3: 完全不埋点（放弃）

D5 策略集：
  B1: WARN unknown op（当前）
  B2: 拒绝启动（硬约束）
  B3: 仅 advisory（无信号）
```

**直觉收益矩阵：**

| | B1 WARN | B2 拒绝 | B3 无信号 |
|---|---------|---------|----------|
| A1 合规 | (合规成本, 治理成功) | (合规成本, 治理成功) | (合规成本, 治理失败) |
| A2 ad-hoc | (低合规成本, 漂移) | (启动失败, 协调成本) | (低合规成本, 漂移) |
| A3 不埋 | (零成本, 漂移) | (启动失败, 协调成本) | (零成本, 漂移) |

**当前均衡：** (A2, B1) 是混合策略下业务域最偏好的均衡 ——「偶尔漂移 + WARN 仅是噪声」。

**为什么 v2.1 选 B1 不选 B2：**
- B2 把 D5 变成「编译期法官」，违反 **Referee 不阻塞 Leader** 原则（OQ-2）
- B2 在快速迭代期反激励：业务域宁愿不启动也不愿受 D5 阻塞
- B2 失败模式：业务域 fork registry 本地副本，D5 失去全局可见性

**演化稳定性（ESS）判断：**

- (A1, B1) **不是 ESS** —— A2 在 B1 下适应度略高于 A1（合规成本不为零）
- (A2, B1) **是弱 ESS** —— 但这是「低质量均衡」，与 §3.1「开发者局部最优 ≠ 全局最优」一致

**建议：**
1. **提高 A2 成本，让 (A1, B1) 成为 ESS**：
   - Coverage 报告把 unknown op 列入「可疑清单」（沿用 §11.5 第 2 项）
   - D6 advisory 把 unknown op 标为「路径不可信」，让业务域 owner 承担外部审视
   - **不靠 D5 自己硬拒**（中央集权）
2. **保留 B1 而非 B2**：确保 D5 不变成启动期单点
3. **承认「次优均衡下的最优装置」**：100% 合规不现实，D5 目标是**让漂移可见 + 漂移有成本**，不是「零漂移」

---

### 13.3 SRE 作为「主权消费者」的缺位

**问题：** §2 玩家表没有明确列出 **SRE/on-call**，他们其实是 D5 输出的**主权消费者**（sovereign consumer）。

| 角色 | 当前在表中的定位 | 实际博弈地位 |
|------|------------------|--------------|
| SRE | 隐含在「外部观测者 / Principal 事后问责」 | 实际上是 **D5 Trace 的即时消费者**，反馈循环短于「事后问责」 |
| on-call | 同上 | 唯一被 **报警 + dashboard** 持续激励的群体 |
| 架构师 | §2 表「架构维护者」 | 一次性消费者，不是主权 |

**博弈后果：**
- SRE 不在 D5 设计反馈回路 → D5 优化「spec 完整 / T 实现」而非「on-call 3 分钟定位」
- Coverage「可解释性」是 SRE 视角，但分级（§11.5 第 2 项）尚未邀请 SRE 共建
- FaultInject 的「韧性验证」价值最终落在 SRE 手里，但 SRE 没在 S23 C3a–C3e 的定义方里

**建议：**
1. **把 SRE/on-call 显式加入 §2 玩家表**，明确「主权消费者」地位
2. **`observability-guide.md` §Runbook 邀请 SRE 验收**：Runbook 不是「架构写给 on-call」，是「SRE 与架构共建」
3. **Coverage 分级（§11.5 第 2 项）定义方应包含 SRE**——「可疑 zero-hit」判定需要 on-call 经验
4. **D5 的 SLA 不是覆盖率，是 MTTR**——把 D5 成功指标从「埋了多少」改为「定位多快」

> 与 §11 Grill-1 「Referee 失败策略 = warn」一致 —— SRE 是「收到 warn 的人」，D5 是「吹哨的人」，二者必须解耦。

---

### 13.4 时间维度的不对称：事前承诺 vs 事后审计

**问题：** v2.1 把多个**时间属性不同**的承诺装进同一治理框架，存在隐含不一致。

| 装置 | 时间属性 | 失败模式 | 修复成本 |
|------|---------|---------|---------|
| `Operation Registry` | **事前**（编译期已知） | 上线后才发现遗漏 | 高（需补埋 + 重发版） |
| Coverage 报告 | **事中**（运行期聚合） | 漂移被静默接受 | 中（warn → advisory） |
| Doctor / HealthCheck | **事前 + 事中**（启动检查 + 周期） | 启动失败阻塞 / 周期失效 | 中 |
| Incident export | **事后**（事件触发） | 触发失败 = 证据缺失 | 高（事后补不回来） |
| Tracker | **事中**（连续） | 性能开销 | 低（可降采样） |
| FaultInject | **事前**（testbuild） | testbuild 与生产不一致 | 中（需 invariant 测试） |

**承诺强度分层：**

```
事前承诺（Operation Registry / HealthCheck）→ 强承诺，强可信，高修复成本
事中审计（Coverage / Tracker）           → 弱承诺，可补救
事后举证（Incident export）               → 不可补救，零容忍失败
```

**错配风险：** v2.1 把「事前/事中/事后」混在 S21–S23，可能让承诺强度与博弈角色错配：

- Incident export 放在 S23 C3b（事后举证），但 S23 整体定位是「诊断子博弈场」—— 容易被误贴「可补救诊断」标签，而非「不可补救证据」

**建议：**
1. **`d5-domain.md` 显式标注每个装置的时间属性**（事前 / 事中 / 事后）
2. **S23 子承诺按时间属性再分组**：
   - 事前/事中：Doctor / Coverage / Tracker（C3a, C3c, C3d）
   - 事后：Incident（C3b）—— 单独声明「不可补救」语义
   - 测试：FaultInject（C3e）—— 与生产完全隔离
3. **承诺强度与失败容忍度匹配**：
   - 事前承诺失败 = 阻塞启动（如 Doctor 启动硬阻塞）
   - 事中审计失败 = advisory（Coverage 不阻塞 CI）
   - 事后举证失败 = 报警（Incident 导出失败立即报警，不静默）

---

### 13.5 Coverage 报告作为公共物品：搭便车问题

**问题：** Coverage 报告是 D5 提供的**公共物品**（non-rivalrous, non-excludable）：

- 任何业务域都能读（non-excludable）
- 一份报告服务所有读者（non-rivalrous）

**博弈后果：**

| 读者 | 收益 | 成本 |
|------|------|------|
| D5 自身 | 治理证据 | 生产 Coverage 的全部成本 |
| 业务域 owner | 自查埋点质量 | 读报告 |
| SRE / on-call | 定位可疑路径 | 读报告 |
| 架构师 | 漂移审计 | 读报告 |

**搭便车风险：**

- 业务域 owner 倾向「只埋不查」—— 贡献埋点但不读 Coverage
- SRE 只在事故时读 —— 日常不消费，Coverage 改进反馈弱
- D5 独自承担生产成本，没有强制消费方

**现有缓解：** Coverage advisory（OQ-3）—— 但这进一步削弱了消费激励

**建议：**
1. **嵌入 D6 advisory 流程**——D6 评估业务域行为时引用 D5 Coverage，形成**消费回路**
2. **SRE 周会固定消费项**——把 Coverage 列入常规仪表板，非「事故时查」
3. **业务域 owner 对 Coverage 有最低消费义务**——通过 D7 Turn 报告或 OKR 链路引入
4. **避免「Coverage 阻断 CI」**（Grill-2 已记录）—— 把公共物品变成强制消费会扭曲激励

---

### 13.6 多 Referee 协调风险：D5 + D6 + 业务域 Health 的潜在重复博弈

**问题：** 系统内不只 D5 一个「审计角色」：

| Referee 角色 | 关注 | 失败动作 |
|--------------|------|---------|
| **D5** | 埋点诚实、Trace 完整、Coverage | warn / advisory |
| **D6** | 行为偏差、Delta gate | advisory / 阻断 release |
| **D2/D7 HealthCheck** | 自身模块健康 | degraded / 阻断 |

**潜在协调问题：**

```
场景 A：业务域缺失关键路径埋点
  - D5 Coverage: warn（不阻塞）
  - D6 Delta: advisory（不阻塞）
  - D2 HealthCheck: 不管埋点（不相关）
  → 三个 Referee 都没动 → 漂移累积

场景 B：业务域故意不发 span
  - D5: warn（但业务域忽略）
  - D6: 不查
  - D2: 不管
  → cheap talk 持续
```

**博弈视角：** **多 Referee 协调博弈**。每个 Referee 局部最优 = 都不阻塞；全局最优 = 至少一个 Referee 报警。

**现有装置局限：**
- D5 ↔ D6 协调写在 `cross-domain-boundaries.md` §6（§5.1），但仅 metric 写入路径，不含审计协调
- 没有「跨 Referee 升级路径」—— D5 warn 被忽略时，没有自动升级到 D6 的机制

**建议：**
1. **建立 D5 → D6 升级路径**：
   - D5 Coverage「可疑 zero-hit」自动进入 D6 advisory 输入
   - D6 评估业务域时不只看 Delta，也看 D5 暴露的「该埋未埋」
   - **D5 是证人，D6 是法官**（§11 Grill-2），但需补「证据移交规则」
2. **明确 D2/D7 HealthCheck 不审计埋点**：
   - HealthCheck 只关心功能健康，不应扩展到观测健康（否则变成万能 Referee）
   - 与 §13.4 时间属性一致：HealthCheck「事前」，D5 Coverage「事中」
3. **避免 Referee 之间的否决权竞争**：
   - D5 不可升级为「阻塞启动」（OQ-2）
   - D6 不可吸纳 D5 全部职责（避免万能裁判）
   - 业务域 HealthCheck 不吸收 D5 责任（避免本地 Referee 化）

---

### 13.7 Phase A 文档先合 vs Phase B 代码后改的承诺强度差异

**问题：** demand.md §5 把 change 拆成 Phase A（docs-only）和 Phase B（code）。两阶段**承诺强度**完全不同：

| Phase | 性质 | 承诺强度 | 失败代价 |
|-------|------|---------|---------|
| A | 文档先合并 | 弱（spec 是宣言） | 无运行时影响 |
| B | 代码后改 | 强（编译 + 测试） | 编译失败 / 测试红 |

**博弈后果：**

- Phase A 阶段「文档承诺 vs 代码现实」可能再次出现 v1.0 错配（spec 写 S21–S24，代码仍 bridge）
- Phase A 通过 S3-Gate，但 S3-Gate 不验证代码 —— 存在「文档先于事实」的风险窗口
- 开发者可能把 Phase A 当「承诺缓冲」，等到 Phase B 再补，造成 Phase A 长期与代码不一致

**建议：**
1. **Phase A 合并后立即打 tag `docs/d5-v2-terminal-spec`**，明确「文档承诺」，**不是终态**
2. **Phase B 启动条件额外加一条**：「Phase A 文档与 `git grep <canonical_s>` 对账完成」（不仅是 spec 一致，还要代码侧已无冲突 import）
3. **明确 Phase A 不可逆性边界**：文档一旦合并到 `openspec/specs/`，在 Phase B 完成前不允许 v2.0/legacy 双轨描述回流
4. **避免「分阶段合并」变成「分阶段逃避」**：
   - Phase A 合并 ≠ 完成度提升信号（避免外部误以为 D5 已终态）
   - 需在 CHANGELOG / announcement 显式说明「docs-only 阶段」

---

### 13.8 第二轮增量视角总结表

| 议题 | 倾向 | 与 v1.1.0 关系 | 关键理由 |
|------|------|---------------|---------|
| §13.1 业务域信息不对称 | 维持 WARN+分级，**不硬拒** | 扩展 Grill-4 | 委托-代理问题；硬拒会导致 registry 本地化 |
| §13.2 Registry ESS | (A2,B1) 是当前弱 ESS；通过 Coverage + D6 联动提 A1 适应度 | 扩展 OQ-3 / Grill-4 | 演化稳定策略视角 |
| §13.3 SRE 主权消费者 | 把 SRE 显式加入 §2 玩家表；MTTR 而非覆盖率 | **新议题** | D5 反馈回路缺关键节点 |
| §13.4 时间属性分层 | 显式标注事前/事中/事后；S23 子承诺再分组 | **新议题** | 承诺强度与失败容忍度错配风险 |
| §13.5 Coverage 公共物品 | 嵌入 D6 + SRE 周会 + 业务域 owner | 扩展 Grill-2 | 避免公共物品悲剧 |
| §13.6 多 Referee 协调 | D5→D6 证据移交规则；D2/D7 Health 不审计埋点 | 扩展 §11 Grill-2 | 避免 Referee 重复/缺位 |
| §13.7 Phase A/B 承诺强度 | docs-only 阶段需打 tag + 对账条件 + 显式声明 | **新议题** | 避免「文档先于事实」错配窗口 |

---

### 13.9 第二轮建议写入 design.md 的补充项（增量）

1. **`d5-domain.md` 加入 SRE/on-call 玩家**（§13.3 落地）：§2 玩家表显式登记主权消费者
2. **`d5-domain.md` 加入时间属性标注**（§13.4 落地）：每个装置标注事前/事中/事后
3. **D5 → D6 证据移交规则**（§13.6 落地）：写入 `cross-domain-boundaries.md` §6
4. **Phase A tag + 对账条件**（§13.7 落地）：写入 `design.md` §Phase B 启动条件
5. **D5 成功指标从覆盖率改 MTTR**（§13.3 落地）：写入 `observability-guide.md` §Runbook 顶部声明

---

## 14. Claude 第二轮对焦（2026-06-19）

**对象:** §13 MiniMax 增量（13.1–13.7）
**方法:** 逐条对焦 + 保留/修正/补充 三类裁决

---

### 14.1 对 §13.1（业务域信息不对称）的回应

**基本同意诊断，但不同意全部处方。**

委托-代理诊断是准确的：D5（Principal）要全局可观测，业务域（Agent）有私有信息优势。当前装置确实都是"下游审计"。

**同意点：**
- Coverage 分级（区分条件/路径/可疑零命中）——已在 §11.5 建议，两轮一致
- D5↔D6 联合审计——D5 揭示埋点稀疏性 + D6 揭示行为偏差，互补信号

**不同意点："埋点自检 SDK"**

```
埋点自检 SDK（lint/hook 在写代码时提示）
  → D5 侵入业务域开发环境
  → D5 从 Referee 变成 Linter
  → 边界模糊：D5 管"运行时是否可观测"，不管"写代码时怎么埋"
```

这与 §11 Grill-4 的"不硬拒绝未注册 op"逻辑同构——D5 不应成为业务域开发流程的瓶颈。SDK 方案让 D5 跨入了 IDE/CLI 工具链，这是 D5 不该拥有的权力。

**替代方案：Coverage 报告反向推送**

不推送 SDK 到业务域，而是把 Coverage 报告**推送给业务域 owner**：

| 当前 | 改进 |
|------|------|
| Coverage 报告被动等待消费 | D5 生成报告后，按域拆分并通知域 owner |
| 域 owner 不知道自己的埋点质量 | 域 owner 在 D7 Turn 报告/OKR 链路中看到自己的 Coverage |

**裁决：** 同意诊断 + Coverage 分级 + D6 联动。不同意埋点自检 SDK。替代方案为 Coverage 报告反向推送。

---

### 14.2 对 §13.2（Registry ESS）的回应

**最精彩的增量。收益矩阵和 ESS 判断准确。有一处修正。**

(A2, B1) = 弱 ESS，这是对的。但"弱 ESS"在这里不是缺陷——它是**约束条件下的最优均衡**：

```
强 ESS (A1, B2) = 硬拒绝未注册 op → D5 成为编译期单点 → 业务域 fork registry 本地副本
弱 ESS (A2, B1) = WARN + unknown op 可见 → 漂移有成本但可容忍 → 业务域不逃离
```

**关键洞察：在 D5 无法强制业务域合规的前提下，(A2, B1) 是帕累托最优。** 任何试图强化到 (A1, B2) 的尝试都会触发业务域的策略迁移（fork registry → D5 失去全局可见性 → 更差的均衡）。

**修正：提升 A2 成本的方式**

codex 建议通过 Coverage 报告 + D6 advisory 提高 A2 成本。方向正确，但我想补充一个**更轻量且更有效的机制**：

| 机制 | 如何提高 A2 成本 | 为何有效 |
|------|------------------|----------|
| WARN 日志聚合 | 将 `unknown operation` WARN 聚合为 metric `devrix_unknown_op_total{operation}` | 各域 owner 无法忽略可计数的指标 |
| 域归属 | unknown op 携带 `devrix.layer` 属性 → 可追溯到域 | 域 owner 不能推卸责任 |
| **PR 模板** | PR 模板中加入 "新增 Span 是否已在 Registry 注册" 勾选 | 事前承诺，最低摩擦 |

WARN 日志在 Jaeger 中不可见——但**聚合为 metric 后在 Prometheus 中可见**，且进入 on-call 仪表板。这比 SDK/lint 轻量得多，且不跨 D5 边界。

**裁决：** 同意 ESS 分析 + 提升 A2 成本方向。补充 WARN→metric 聚合 + PR 模板勾选，替代重量的 SDK/lint。

---

### 14.3 对 §13.3（SRE 主权消费者）的回应

**最重要的增量。SRE 缺位是真实的架构盲区。但 MTTR 作为 D5 成功指标有保留。**

**完全同意：**
- SRE/on-call 应显式加入 §2 玩家表——他们是 D5 输出的主权消费者
- `observability-guide.md` Runbook 应是 SRE 与架构共建，不是架构单向写给 on-call
- Coverage 分级应邀请 SRE 参与——"可疑零命中"的判定需要生产经验

**保留：MTTR 不应替代覆盖率作为 D5 的唯一成功指标**

```
MTTR = Mean Time To Recovery
  → D5 可以影响 MTTR（好的 trace 降低定位时间）
  → 但 MTTR 也取决于：代码熟悉度、on-call 经验、事故复杂性
  → D5 对 MTTR 的"可控方差"有限
```

覆盖率与 MTTR 的关系是**手段与结果**的关系：

| 指标 | 类型 | D5 可控性 |
|------|------|-----------|
| 覆盖率（56 ops hit rate） | 过程指标（leading indicator） | 高 |
| MTTR | 结果指标（lagging indicator） | 中低 |
| Jaeger trace 可交叉引用率 | 过程指标 | 高 |

**建议折中：D5 成功指标用"覆盖率"和"trace 完整度"作为过程指标，MTTR 作为 D5 贡献的验证指标，而非 D5 的 KPI。**

**裁决：** 同意 SRE 显式加入玩家表 + Runbook 共建。保留 MTTR 的定位——作为验证指标而非 KPI。D5 成功指标双轨：过程指标（覆盖率/trace 完整度）+ 验证指标（MTTR 改善趋势）。

---

### 14.4 对 §13.4（时间属性分层）的回应

**完全同意。这是 v2.1 文档栈的结构性改进。**

事前/事中/事后的分层是正确的，尤其是 Incident export 被标记为"不可补救"——这与 §11 的"事后举证 = 非否认性机制"一致。

**补充一个视角：时间属性 × 承诺装置强度的交叉矩阵**

| | 事前（ex-ante） | 事中（during） | 事后（ex-post） |
|--------|----------------|---------------|----------------|
| **强承诺** | Doctor 启动阻塞 | — | Incident 导出失败报警 |
| **弱承诺** | Operation Registry | Coverage 报告 / Tracker | — |

**空白格的含义：**
- 事中×强承诺（空）：在运行时做硬阻塞违反 Graceful Degradation 原则
- 事后×弱承诺（空）：Incident 不能弱——事后证据不可补救，必须强承诺

这支持了 codex 的建议：Incident export（C3b）应从 S23 中单独标注"不可补救"语义。

**裁决：** 完全同意。建议补充交叉矩阵写入 `d5-domain.md`。

---

### 14.5 对 §13.5（Coverage 公共物品）的回应

**诊断准确。对"强制消费"的方案持谨慎态度。**

Coverage 确实是公共物品（非竞争性 + 非排他性）。搭便车问题是真实的。

**同意：**
- Coverage → D6 advisory 输入，形成消费回路
- SRE 周会固定消费项（列入常规仪表板）

**保留：业务域 owner "最低消费义务"**

"义务"暗示强制——强制消费公共物品会把 Coverage 变成负担：
- 域 owner 为了满足"最低消费"而刷 Coverage 数字（Goodhart 定律）
- Coverage 从诊断工具变成合规工具 → 数据质量下降

**替代方案：用透明度替代强制**

| 强制消费 | 透明度机制 |
|----------|-----------|
| 域 owner 必须每月查看 Coverage | Coverage 按域分组，公开可见（dashboard） |
| 不达标有惩罚 | 不达标在 PR review / OKR review 中被自然提及 |
| D5 定义"达标线" | 各域 owner 与 SRE 共同定义自己域的"合理零命中" |

**核心原则：Coverage 是诊断工具，不是合规工具。** 一旦变成合规工具，业务域的激励会从"诚实埋点"转变为"满足指标"——这对 D5 的博弈价值是破坏性的。

**裁决：** 同意公共物品诊断 + 嵌入 D6 + SRE 周会。不同意强制消费义务。替代方案为透明度机制（按域分组的公开 dashboard）。

---

### 14.6 对 §13.6（多 Referee 协调）的回应

**同意诊断 + 同意 D5→D6 证据移交规则。补充 Referee 之间的信息结构。**

**完全同意：**
- D5 → D6 升级路径：Coverage 可疑零命中自动进入 D6 advisory
- D2/D7 HealthCheck 不审计埋点
- 避免 Referee 否决权竞争

**补充：三个 Referee 的信息结构不对称**

| Referee | 信息类型 | 时间 | 范围 |
|---------|----------|------|------|
| D5 | 可观测性（span 是否存在） | 实时/事后 | 全系统 |
| D6 | 行为正确性（输出是否合规） | 事后 | 全系统 |
| D2/D7 Health | 功能健康（模块是否存活） | 实时 | 单域 |

**D5 的信息是 D6 判断的输入，但不是唯一输入。** 这决定了 D5→D6 的"证据移交"是单向的、非强制的——D5 提供证据，D6 决定是否采纳。

**裁决：** 完全同意 + 补充信息结构不对称表格。证据移交规则写入 `cross-domain-boundaries.md`。

---

### 14.7 对 §13.7（Phase A/B 承诺强度）的回应

**同意诊断，补充一个更强的保护机制。**

Phase A 文档先合并 + Phase B 代码后改的风险是真实的——文档与代码的时间窗口不一致曾是 v1.0 的根因。

**完全同意：**
- Phase A 合并后打 tag
- Phase B 启动条件包含对账
- 文档合并后不允许双轨回流

**补充：Phase A 应包含一个"不可逆的代码锚点"**

纯文档的 Phase A 可以永远不与代码一致。建议 Phase A 至少包含**一项代码变更**作为锚点：

| 锚点候选 | 说明 |
|----------|------|
| `spec.md` v3.0 中的 version 字段 | 代码中 `version.go` 同步更新（如果存在） |
| `t-registry` canonical_s 列校正 | 这是数据文件，修改不涉及业务逻辑 |
| A-registry v4.0 路径同步 | 纯注册表，零业务风险 |

**为什么需要代码锚点：** 纯文档承诺的博弈强度为零——cheap talk。一个轻量代码锚点（如 t-registry 的 canonical_s 列）将文档承诺与可验证的代码变更绑定，使 Phase A 不再是"纯宣言"。

**裁决：** 同意全部建议 + 补充 Phase A 代码锚点（至少包含 t-registry canonical_s 列校正或等效的一行代码变更）。

---

### 14.8 第二轮总结表

| codex § | 议题 | 裁决 | 关键差异/补充 |
|----------|------|------|---------------|
| 13.1 | 业务域信息不对称 | 同意诊断；**不同意 SDK** | 替代：Coverage 反向推送 + WARN metric 聚合 |
| 13.2 | Registry ESS | 同意；补充低摩擦提 A2 成本 | WARN→metric + PR 模板勾选 |
| 13.3 | SRE 主权消费者 | 同意 SRE 入表；**保留 MTTR 定位** | 双轨：过程指标（覆盖）+ 验证指标（MTTR） |
| 13.4 | 时间属性分层 | 完全同意 | 补充交叉矩阵（时间×承诺强度） |
| 13.5 | Coverage 公共物品 | 同意诊断；**不同意强制消费** | 替代：透明度机制（按域公开 dashboard） |
| 13.6 | 多 Referee 协调 | 完全同意 | 补充信息结构不对称表格 |
| 13.7 | Phase A/B 承诺强度 | 同意；补充代码锚点 | Phase A 必须包含 ≥1 个代码变更 |

---

### 14.9 两轮合并的最终落地清单

以下是从 §11 + §13 + §14 中提炼的、应写入 `design.md` / `d5-domain.md` / `cross-domain-boundaries.md` 的最终条目：

| # | 条目 | 写入位置 | 来源 |
|---|------|----------|------|
| 1 | SRE/on-call 显式加入玩家表 | `d5-domain.md` §玩家 | §13.3 |
| 2 | 时间属性标注（事前/事中/事后） | `d5-domain.md` §装置 | §13.4 |
| 3 | 时间×承诺强度交叉矩阵 | `d5-domain.md` §装置 | §14.4 |
| 4 | S25 触发条件（3 条） | `d5-domain.md` §S23 | §11.1 OQ-1 |
| 5 | S23 硬边界（语义/数量/依赖） | `design.md` §5 | §11.2 Grill-3 |
| 6 | D5 完备性边界声明 | `d5-domain.md` | §11.5 |
| 7 | Coverage 零命中分级（条件/路径/可疑） | `coverage.md` + `observability-guide.md` | §11.5 + §13.1 |
| 8 | D5→D6 证据移交规则 | `cross-domain-boundaries.md` §6 | §13.6 |
| 9 | Phase A 代码锚点要求（≥1 个代码变更） | `design.md` §10 | §14.7 |
| 10 | Phase B 启动对账条件 | `design.md` §10 | §13.7 |
| 11 | `legacy_harness` metric 退役计划 | `design.md` §6 | §11.1 OQ-6 |
| 12 | FaultInject 生产二进制验证 AC | `demand.md` §AC | §11.2 Grill-5 |
| 13 | D5 成功指标：过程（覆盖/trace 完整度）+ 验证（MTTR 趋势） | `observability-guide.md` | §14.3 |
| 14 | WARN unknown op 聚合为 metric `devrix_unknown_op_total` | `observability-guide.md` | §14.2 |
| 15 | PR 模板增加"Span 是否在 Registry 注册"勾选 | `.github/PULL_REQUEST_TEMPLATE.md` | §14.2 |

---

## 16. DSAFT 架构方法论 Review（2026-06-19）

> **方法：** 依照 `dsaft-refactoring-playbook.md` §2 四条分析轴展开：DSAFT 分层合规 → 用户动线 → 博弈论/机制设计 → OpenSpec 交付。
> **前置阅读：** 已消化 `dsaft-methodology.md` v4.0、Playbook §1–§10、`a-registry.md` v3.0、`f-registry.md` v2.0、`design.md`、`demand.md`、`proposal.md`、`tasks.md`、`design-review.md`、`d5-requirements-clarifications.md` 及本文件 §1–§15。

---

### 16.1 DSAFT 分层合规检查

#### 16.1.1 D 层：领域边界

| 检查项 | 结论 | 证据 |
|--------|------|------|
| D5 领域类型正确 | ✅ 公共域 | D5 无自有业务实体；通过 Bridge 为 D1–D7 提供横向可观测性 |
| D5 不侵入其他域 | ✅ | `d5-boundary.md` §7 明确 D5 不创建 Turn span、不阻塞 D7 Leader |
| 跨域依赖方向正确 | ✅ | 所有 D* → D5 通过 Bridge 注入；D5 不 import D2/D4/D7 业务包 |
| 域内核不泄漏 | ⚠️ 待验证 | bridge 删除后，需确认 `bridge.go`（S0 Facade）不暴露 instrument 内部实现细节 |

**根因评估：** D 层定位清晰。唯一风险是 Facade 层 `bridge.go` 可能在 bridge 删除后过度暴露 instrument 包内部结构——但这属于 A 层契约设计问题，不影响 D 层边界。

#### 16.1.2 S 层：场景切法

S 层是 DSAFT 中最容易被 Go 包结构绑架的层。v2.1 的核心贡献就是将 S 从「模块名」纠正为「价值流」。

| 检查项 | 结论 | 证据 |
|--------|------|------|
| S 表达价值流而非模块 | ✅ | S21 Instrument / S22 Export / S23 Diagnose / S24 Configure / S0 Facade — 均为用户目标 |
| Legacy S1–S9 已冻结 | ✅ | `a-registry.md` Legacy Module Index 完整；`spec.md` v3.0 分离 Legacy 节 |
| 未出现「S = 目录名」 | ✅ | `instrument/` 目录对应 S21 而非 S1；`diagnose/` 目录对应 S23 而非 S5/S8 |
| S 数量合理 | ✅ | 4+1 S，远低于 Playbook 原则 4 的「万能 S」警戒线 |

**关键检验：S23 是否接近「万能 S」？**

S23 从原始的「Coverage + Incident + Health」3 项能力膨胀到 5 项子承诺（C3a–C3e）。判定标准来自 Playbook 原则 4：

> 不让一个 D 的 S 层膨胀成万能场景。

| 维度 | 评估 |
|------|------|
| 语义凝聚力 | 5 个子承诺均属「审计/举证」——强凝聚 |
| 物理内聚 | 均在 `diagnose/` 子目录下——物理合理 |
| 消费者一致性 | 消费者均为 SRE / on-call / CI——用户群一致 |
| 子承诺上限 | 当前 5，设计上限 7（§11.2 Grill-3）——未触及 |

**结论：当前未触及「万能 S」红线。** 但需注意：Grill-3 中建议的 3 条硬边界（语义/数量/依赖）尚未写入 `design.md` §5。**这属于 S3-Gate 待闭合项。**

#### 16.1.3 A 层：活动追溯链

| 检查项 | 结论 | 证据 |
|--------|------|------|
| 每个 A 有 S 归属 | ✅ | 27 A 分布在 S21(13) + S22(2) + S23(6) + S24(4) + S0(2) |
| A 表达对外动作 | ✅ | CreateSpan / ExportSpans / AssessCoverage / HealthCheck 均为可观测动作 |
| 新增 A 有合理 S 归属 | ✅ | A14 FilterDebugLog → S21（管道滤波）；S0-A03 TrackActiveSessions → S0（Facade 集成面） |
| A 编号唯一 | ✅ | v4.0 草案中 S21-A14 / S23-A07–A10 / S0-A03 无冲突 |

**两个结构性问题：**

**问题 1：A 层「双锚点」断裂（Doctor T↔A 错位）。**

```
D5-S23-A03 = GenerateDailyReport（Coverage 报告生成）
D5-S23-A03-T01 = RunDoctorChecks（Doctor 诊断检查）
                ↑ T 编号指向 A03，但语义属于 A10
```

`design.md` §5 已识别：终态方案是 T ID 冻结 + `canonical_a` 列指向 A10。**这是 DSAFT 合规的已知债务，处理方式正确（T 不改号），但需要在 `a-registry.md` v4.0 中显式登记 A10，并确保 `t-registry.md` 的 `canonical_a` 列已填充。**

**问题 2：a-registry.md v3.0 的 Code Location 列仍指向旧路径。**

例如 D5-S21-A01 的 Code Location 是 `tracer/tracer.go`——这是 bridge 路径，不是 canonical 路径（应为 `instrument/tracer/tracer.go`）。Phase A 的 A6 任务（a-registry v4.0）需要将所有 Code Location 更新为 canonical 路径。**当前未更新 → S3-Gate 文档一致性 AC 未满足。**

#### 16.1.4 F 层：功能点注册

| 检查项 | 结论 | 证据 |
|--------|------|------|
| 每个 F 有 A 归属 | ✅ | f-registry v2.0 中所有 F 均挂载到 Legacy A ID |
| F 注册表版本滞后 | ⚠️ | f-registry v2.0 仍使用 Legacy S1–S9 编号（`D5-S1-A01-F01`） |
| Canonical 路径缺失 | ⚠️ | f-registry v2.0 Code Location 指向 `tracer/tracer.go` 等旧路径 |

**关键问题：f-registry 的「v3.0 计划」与 a-registry 的「v4.0 计划」不同步。**

- a-registry v3.0 已是 Canonical S21–S24 编号（2026-06-15）
- f-registry v2.0 仍使用 Legacy S1–S9 编号（2026-06-14）
- tasks.md A7 是「f-registry v3.0：canonical_s 列 + 诊断 F + 路径更新」

**这意味着 F 层注册表已落后 A 层注册表一个版本周期。** Phase A 执行时，f-registry v3.0 需要：
1. 所有 F ID 从 `D5-S1-*` 改为 `D5-S21-*` 等 canonical 前缀
2. 增 `canonical_s` 列
3. Code Location 更新为 `instrument/` 等真实路径

这与 Playbook 原则 3（T 是安全网）一致：**F 层变更必须查关联 T**。f-registry 的 canonical 更新会触及 T 层引用——需在 Phase A 执行时重点关注。

#### 16.1.5 T 层：测试锚点

| 检查项 | 结论 | 证据 |
|--------|------|------|
| T ID 不变量 | ✅ | design §2 Decision 明确「T ID 不变」 |
| canonical 列校正 | ⚠️ 待执行 | tasks.md A10 是 `t-registry v3.2：canonical_s/canonical_a 校正` |
| PLANNED T 闭合计划 | ✅ 可行 | 3 条 PLANNED → IMPLEMENTED，闭合路径清晰 |
| P0 T 覆盖率 | ✅ | 39 IMPLEMENTED + 2 PLANNED → 闭合后 41 |
| T↔Span 映射 | ⚠️ 待文档 | observability-guide.md 中的 Span↔T P0 绑定矩阵尚未创建 |

**Playbook 原则 5 检验（可观测性是 T 的生产延伸）：**

> P0 测试点应有 Span 或 acceptance 证据；「有 T 无 Span」视为规格债务。

当前 41 T 中，`D5-S23-A03-T01/T02`（Doctor）的 Span 证据链尚未在 observability-guide.md 中建立。Phase A 的 A3 任务（observability-guide.md）需要补齐 Span↔T 矩阵。

#### 16.1.6 分层合规总结

| 层级 | 状态 | 待闭合项 |
|------|------|----------|
| D | ✅ 合规 | Facade 不泄漏 instrument 内部（bridge 删除后验证） |
| S | ✅ 合规 | S23 硬边界写入 design.md §5（Grill-3 落地） |
| A | ⚠️ 部分合规 | a-registry v4.0 Code Location 更新（A6）；A10 登记 + canonical_a 列 |
| F | ⚠️ 版本滞后 | f-registry v3.0 canonical 编号迁移（A7） |
| T | ⚠️ 部分合规 | t-registry v3.2 canonical 列（A10）；Span↔T 矩阵（A3） |

**系统性根因：实现先于注册表。** S23 诊断能力（Doctor/Tracker/FaultInject）实现早于 a-registry v4.0，导致 A 编号错位（A03 被 GenerateDailyReport 和 Doctor T 同时引用）。这与 Playbook 阶段 1 的「实现先于规格」错配模式完全一致。

---

### 16.2 用户动线分析

#### 16.2.1 North Star 可验证性

按照 Playbook 阶段 3 的标准，North Star 应回答 3 个问题：

| 问题 | D5 答案 | 质量 |
|------|---------|------|
| 用户要达成什么？ | 「全链路可追踪、可度量、可诊断的遥测基础设施」 | ✅ 清晰 |
| 可验证承诺是什么？ | C1 遥测生成 / C2 遥测导出 / C3 诊断辅助 / C4 配置管理 / C0 Facade | ✅ 可 E2E 验收 |
| 什么不在本域？ | Turn 编排 (D7)、Tool 执行 (D2)、评测 (D6)、IM ingress (D1)、LLM 路由 (D3)、Agent 生命周期 (D4) | ✅ 边界清晰 |

**评价：** North Star 质量高。一句话可翻译为「让任意域操作在 Jaeger/Prometheus/Health/Incident 中可交叉验证」——消费者（SRE/on-call/架构师）可直接验证。

#### 16.2.2 承诺→S→验收路径

| 承诺 | S | 验收消费者 | 验收证据 | 证据可及性 |
|------|---|-----------|----------|-----------|
| C1 遥测生成 | S21 | 任意域开发者 | `tracer_test` + span 属性含 layer/component | ✅ 单元测试 + integration |
| C2 遥测导出 | S22 | SRE/on-call | Jaeger 中可见 D7 Turn trace | ✅ OTLP event test |
| C3 诊断辅助 | S23 | SRE/on-call/CI | Coverage 报告 / `debug export` / Health endpoint | ⚠️ 5 子承诺，验收分散 |
| C4 配置管理 | S24 | 架构维护者 | yaml 切换 exporter 可验证；runtime path 计数准确 | ✅ config validate |
| C0 Facade | S0 | 所有消费者 | Init/Shutdown 不 panic；Bridge 零侵入 | ✅ bootstrap test |

**C3 的验收分散问题：** 5 个子承诺（C3a–C3e）对应 5 个独立的验收消费者和证据链。当前 `acceptance-report.md` 尚未撰写，但 `design-review.md` 指出了这一风险——C3 是唯一一个「一个承诺对应多个独立验证」的条目。**建议在 S5 验收报告中按子承诺分节验收。**

#### 16.2.3 消费者主权

| 消费者 | 在 D5 设计中被考虑程度 | 问题 |
|--------|----------------------|------|
| 业务域开发者（D2/D7 等） | 高 — Bridge 注入、Op 常量 | ✅ |
| SRE/on-call | 中 — observability-guide 已有 Runbook 节 | ⚠️ SRE 未显式列入 §2 玩家表（§13.3 已识别） |
| CI/CD pipeline | 中 — Coverage 报告、Health endpoint | ⚠️ Coverage gate 分级未落地（§11.5 建议项 2） |
| 架构维护者 | 高 — a/f/t 注册表、d5-domain、boundary 文档 | ✅ |

**主要缺口：SRE 主权消费者地位未显式确认。** 这在 §13.3（MiniMax 第二轮）和 §14.3（Claude 第二轮）中已充分讨论，且 §14.9 最终落地清单第 1 条已明确「SRE/on-call 显式加入玩家表」。**该条目尚未执行。**

#### 16.2.4 E2E 动线（以 on-call 排障为例）

```
on-call 收到 Prometheus 报警（devrix_unknown_op_total > 0）
  → 查 Jaeger: 哪个 span 被标记 unknown？
    → D5 提供: trace 树 + layer/component 属性 → 定位到域
  → 查 D5 Coverage: 该域的核心路径是否有其他零命中？
    → D5 提供: 按域分组的 Coverage 报告 → 识别模式
  → 查 D5 Health: Doctor 是否发现环境问题？
    → D5 提供: /health endpoint → 排除环境因素
  → 结论: 埋点遗漏 or 采样配置 or 路径退化？
```

这条动线的完整性取决于：observability-guide.md 的 Runbook 是否覆盖了从报警到定位的完整链路。当前 `design.md` §8.2 仅列出了 observability-guide.md 的结构骨架——**Phase A 执行时，Runbook 需要经 SRE 验收（§14.3 共识）。**

---

### 16.3 博弈论/机制设计交叉验证

> 本 change 的博弈论讨论已在 §1–§15 中深度展开（3 轮、跨 Agent）。本节省略重复论证，聚焦**博弈论结论与 DSAFT 原则的交叉验证**。

#### 16.3.1 博弈论发现对 DSAFT 分层的映射

| 博弈论发现 | 对应的 DSAFT 原则 | 验证 |
|-----------|------------------|------|
| D5 Referee 不阻塞 Leader（OQ-2） | 原则 4：跨域问题在 D 边界决策 | D5 不拥有 Turn 否决权——D 边界正确 |
| S23 子承诺代替新 S 号（OQ-1） | 原则 2：S 与 A 职责分离 | 审计子博弈在 A 层差异化——策略差异下沉 F |
| T ID 冻结 + canonical 列校正 | 原则 3：T 是安全网 | T 不变契约——Schelling 点 |
| RecordHit 独立于采样（Grill-4） | 原则 5：可观测性是 T 的生产延伸 | P0 验收不受采样策略影响 |
| bridge 删除 = 烧船承诺 | 原则 6：分阶段终态 | v1.0→v2.0→v2.1 渐进闭合，不跳步 |
| D5↔D6 证据移交（§13.6） | 原则 4：不让一个 D 的 S 层膨胀 | D5 证人 + D6 法官——职责不合并 |

**结论：博弈论分析验证了 DSAFT 分层的合理性。没有发现博弈论结论与 DSAFT 原则相矛盾的情况。**

#### 16.3.2 博弈论发现对 Phase A 执行的影响

| 合并落地清单条目（§14.9） | 对应 Phase A 任务 | 执行状态 |
|--------------------------|-------------------|----------|
| #1 SRE 入玩家表 | A1 d5-domain.md | 待执行 |
| #2 时间属性标注 | A1 d5-domain.md | 待执行 |
| #3 时间×承诺交叉矩阵 | A1 d5-domain.md | 待执行 |
| #4 S25 触发条件 | A1 d5-domain.md | 待执行 |
| #5 S23 硬边界 | A8 design.md v3.0 | 待执行 |
| #6 D5 完备性边界声明 | A1 d5-domain.md | 待执行 |
| #7 Coverage 零命中分级 | A3 observability-guide.md + coverage.md | 待执行 |
| #8 D5→D6 证据移交规则 | A5 cross-domain-boundaries.md | 待执行 |
| #9 Phase A 代码锚点 | A6/A7/A10 registry 更新 | **关键——见下文 §16.4.2** |
| #10 Phase B 启动对账条件 | A8 design.md §10 | 待执行 |
| #11 legacy_harness 退役计划 | A8 design.md §6 | 待执行 |
| #12 FaultInject 生产验证 AC | A4 terminal-state-guide.md | 待执行 |
| #13 D5 成功指标双轨 | A3 observability-guide.md | 待执行 |
| #14 WARN metric 聚合 | A3 observability-guide.md | 待执行 |
| #15 PR 模板 Span 注册勾选 | 超出 Phase A 范围 | 独立 PR |

**关键发现：15 条合并落地清单中，14 条属于 Phase A docs 任务，1 条（PR 模板）超出范围。** 这意味着 Phase A 不仅是「规格终态」，也是博弈论对焦的落地载体。**如果 Phase A 不执行这 14 条，博弈论讨论就没有转化为架构约束。**

#### 16.3.3 博弈论视角的残余风险

| 风险 | 描述 | 建议 |
|------|------|------|
| Phase A 合并 ≠ 博弈论共识 | 文档合并后，外部 reader（新开发者/新 SRE）可能不读 §1–§15 | `d5-domain.md` 应包含博弈论要点的 1 页摘要 |
| D5 成功指标未定义 | MTTR vs 覆盖率的讨论（§14.3）未写入 design.md | 写入 `observability-guide.md` 顶部 |
| Referee 独立性无保障机制 | 当前仅靠文档约定 | 长期可考虑 CI 检查 D5 不 import 业务域包 |

---

### 16.4 OpenSpec 交付评估

#### 16.4.1 Phase 拆分检查

| 检查项 | 结论 | 证据 |
|--------|------|------|
| Phase A 纯文档 | ✅ | tasks.md A1–A12 全部 docs-only |
| Phase B 纯代码 | ✅ | B1 根目录归位 + B2 bridge 删除 + B3 T 闭合 |
| A 与 B 可并行 | ⚠️ 有条件 | B1（git mv）与 A 无冲突；B2（删 bridge）依赖 A 定稿的 canonical 路径 |
| 每 Phase 有独立 PR | ✅ | `docs/d5-v2-terminal-spec` + `chore/d5-root-file-relocate` + `chore/d5-bridge-removal` |
| 每 Phase 有 Gate | ✅ | Phase A → S3-Gate；Phase B → go test + integration + layer-lint |

**风险：Phase A 「docs-only」的承诺强度问题。**

这是 §13.7 和 §14.7 的核心讨论点。Phase A 如果只有文档，可能永远不与代码一致——这在 v1.0 时期已经发生过。§14.7 的建议（Phase A 至少包含一个「不可逆的代码锚点」）尚未被 `tasks.md` 或 `design.md` 采纳。

**建议：** 在执行 Phase A 时，至少将 **t-registry 的 canonical_s/canonical_a 列校正**（tasks.md A10）从 Phase A 分离出来，与 A6（a-registry v4.0）一起作为 Phase A 的「代码锚点」。这两个注册表的更新是数据文件修改，不涉及业务逻辑，零运行风险，但提供了不可逆的承诺信号——「canonical 编号已写入注册表，spec 不再是纯宣言」。

#### 16.4.2 注册表完整性检查

| 注册表 | 当前版本 | 目标版本 | 增量 | Phase A 任务 |
|--------|----------|----------|------|-------------|
| spec.md | v2.0.0 | v3.0.0 | Canonical 主叙事 + Legacy 下沉 | A2 |
| a-registry | v3.0 | v4.0 | +4 A（S21-A14, S23-A07~A10, S0-A03）；Code Location 更新 | A6 |
| f-registry | v2.0 | v3.0 | canonical_s 列 + 诊断 F + 路径更新 | A7 |
| t-registry | v3.1 | v3.2 | canonical_s/canonical_a 校正 + PLANNED → IMPLEMENTED | A10 |
| span-registry | 当前 | 更新 | query.loop → RETIRED only | A9 |
| coverage.md | 当前 | 更新 | 56 ops 同步 | A9 |

**Playbook 原则 6（分阶段终态）检查：**

> v1.0 闭合 Registry 共识 → v1.1 可追溯 → v2.0 物理结构。每阶段独立 S5/S7。

D5 的演进路径：
- v1.0 Registry（2026-06-15）：✅ S/A/F/T 注册表 + Gherkin + Legacy 双轨
- v1.1 Traceability（部分完成）：spec.md + span-registry + coverage
- v2.0 Structure（2026-06-15）：✅ 物理迁移，但 bridge 残留
- **v2.1 Terminal（本 change）**：规格 SoT + 语义闭合 + bridge 清债

**评价：v1.1 阶段的「可追溯」层（T↔Span↔契约）部分被跳过，直接进入 v2.0。** 这解释了为什么 observability-guide.md（Span↔T 矩阵）和 t-registry canonical 列校正现在仍是待办项——它们在 v1.1 轨道上就应该完成。v2.1 本质上是在补 v1.1 的债 + 完成 v2.0 的清尾。

#### 16.4.3 T 锚定质量

| 指标 | 值 | 评价 |
|------|-----|------|
| T 总量 | 41 | 适中 |
| IMPLEMENTED | 39 | 高完成度 |
| PLANNED | 2 | 闭合路径清晰 |
| T↔A 错位 | 1 (Doctor T↔A03/A10) | 已知债务，处理正确 |
| T 编号变更 | 0 | 冻结策略执行到位 |

**Playbook 的 T 锚定标准：**

> T 是不变契约；改 S 不能破坏 IMPLEMENTED T。

v2.1 完全遵守了这一标准——所有 T ID 不变，仅通过 `canonical_s`/`canonical_a` 列做语义校正。这是正确的。

**但需注意：** 如果 f-registry v3.0 从 Legacy 编号（`D5-S1-A01-F01`）改为 Canonical 编号（`D5-S21-A01-F01`），被引用的 T ID 中如果有 `D5-S1-A01-F01-T01` 格式的，也需要同步更新 F 层前缀。**tasks.md 中未显式列出 F 层编号变更对 T 层的影响——需要在 Phase A 执行时检查。**

#### 16.4.4 S3-Gate 状态与进入 S4 的前置条件

| S3-Gate 检查项 | 状态 | 备注 |
|---------------|------|------|
| 层归属正确 | ✅ | design-review.md §2 |
| demand→proposal→design→specs 链路完整 | ✅ | design-review.md §3 |
| P0 AC 有 Scenario | ✅ | happy + sad path 齐全 |
| T 层映射完整 | ⚠️ | t-registry canonical 列待校正（A10） |
| DM ID 无冲突 | ✅ | DM-20260619-006 新需求 |
| 非平凡选择有 Decision | ✅ | design.md §2 六项 |
| Grill Review 有记录 | ✅ | 本文件 §11–§14 |

**design-review.md 结论：Approved with Suggestions。** 3 条 Suggestion 中：
1. Owner + Claude 博弈论对焦 → **已完成**（§11–§14）
2. Draft PR `docs/d5-v2-terminal-spec` → **待创建**
3. demand-archive-index 登记 → **S7 时处理**

**进入 S4 的实质前置条件：**
- [x] S3-Gate Approved
- [ ] Owner 确认 Review 结论（design-review.md 标记为「建议确认」）
- [ ] Phase A Draft PR 创建
- [ ] 15 条合并落地清单中至少与 d5-domain.md 直接相关的条目（#1–#6）写入 Phase A 产物
- [ ] f-registry v3.0 的 F ID 编号迁移方案确认（影响 T 层引用）

---

### 16.5 发现清单与建议

#### Critical（应在 Phase A 执行前解决）

| # | 发现 | 建议 | 写入位置 |
|---|------|------|----------|
| C1 | f-registry F ID 从 Legacy 迁移到 Canonical 时可能影响 T 层引用 | Phase A 执行前，grep 所有 `D5-S1-*-F*-T*` 格式的 T ID，确认变更范围 | tasks.md A7 备注 |
| C2 | Phase A 缺少代码锚点——纯文档承诺强度为零（§14.7） | tasks.md A6（a-registry v4.0）+ A10（t-registry canonical 列）作为 Phase A 的代码锚点，不推迟到 Phase B3 | tasks.md Phase A 增加 A6/A10 的「代码锚点」标注 |

#### Warning（应在 Phase A 产物中包含）

| # | 发现 | 建议 | 写入位置 |
|---|------|------|----------|
| W1 | a-registry v3.0 Code Location 列指向 bridge 路径 | A6 执行时全部更新为 `instrument/` / `export/` / `diagnose/` / `configure/` 路径 | a-registry.md v4.0 |
| W2 | S23 硬边界（语义/数量/依赖）未写入 design.md | Phase A 的 A8 任务（design.md v3.0）中写入 §5 | design.md §5 |
| W3 | observability-guide.md Runbook 未包含 SRE 验收流程 | A3 执行时 Runbook 节包含「SRE 验收清单」+「以 on-call 排障动线组织」 | observability-guide.md §4 |
| W4 | S23 子承诺 C3b（Incident export）的「不可补救」语义未显式标注 | `d5-domain.md` 中按时间属性（事前/事中/事后）分组 S23 子承诺 | d5-domain.md |
| W5 | D5 成功指标（覆盖率 vs MTTR）未定义 | `observability-guide.md` 顶部声明：过程指标（覆盖率/trace 完整度）+ 验证指标（MTTR 趋势） | observability-guide.md §0 |
| W6 | S25 触发条件未写入 d5-domain.md | `d5-domain.md` S23 节写入 3 条触发条件（§11.1 OQ-1） | d5-domain.md |

#### Info（Phase B 或后续 change 考虑）

| # | 发现 | 建议 |
|---|------|------|
| I1 | f-registry v3.0 缺失诊断 F 点（Tracker diff/LRU/linter、Doctor checks、FaultInject hook） | A7 执行时补充，字数按 design.md §8.4 估算约 15-25 F |
| I2 | S23 合并落地清单中 14/15 条属于 Phase A——Phase A 的信息密度高于预期 | Phase A 执行时可能需要拆分为 A1–A4（领域文档）+ A5–A8（注册表更新）+ A9–A12（同步更新）三个子批次 |
| I3 | PR 模板 Span 注册勾选（§14.9 #15）超出 D5 范围 | 独立 change 或作为 `openspec/specs/project/git-workflow.md` 的修订 |

---

### 16.6 补充发现：Span 运行时命名规范校正

在 DSAFT Review 之后，发现当前 `spans-registry.md` 和 `names.go` 中的 Span 运行时字符串存在 **S 编号与场景名冗余** 的问题。

**问题：** 当前格式 `D{N}_S{N}_{场景}_{动作}_{细节}` 中，S 编号与场景语义名表达同一事物：

```
D1_S13_Capture_Message_Receive
      ↑                   ↑
      S13 = Capture 场景，后面又写了 Capture → 重复
```

**校正：** 运行时字符串采用 `D{N}_{场景名称}_{动作}_{细节}`，去掉中间冗余的 S 编号。

| 当前（冗余） | 校正后 |
|-------------|--------|
| `D1_S13_Capture_Message_Receive` | `D1_Capture_Message_Receive` |
| `D7_S2_Orchestration_Turn_Run` | `D7_Orchestration_Turn_Run` |
| `D2_S2_Context_Process` | `D2_Context_Process` |

**DSAFT 分层含义：** 场景名称（`Capture`/`Orchestration`/`Context`）本身已唯一映射到 S 层，无需 S 编号重复标注。Go 常量名（`OpD1_S13_Capture_Message_Receive`）保留 S 编号前缀用于 DSAFT 追溯。

**影响范围：** `design.md` §2 新增 Decision Q7；`d5-domain.md` 新增命名规范节；`spans-registry.md` 格式描述已修正。

---

### 16.7 一句话总结

**DSAFT 分层体检结论：D 层/S 层/T 层锚点质量高；A 层/F 层注册表版本滞后是主要债务（实现先于规格），Phase A 正好偿还；博弈论对焦已充分完成且与 DSAFT 原则一致，14/15 条落地清单在 Phase A 中可一次收口；最大的结构性风险是 Phase A 「纯文档」的承诺强度——建议以 a-registry/t-registry 更新作为不可逆代码锚点。**

---

## 15. 修订记录

| Version | Date | Changes |
|---------|------|---------|
| 1.4.1 | 2026-06-19 | §16.6：Span 运行时命名规范校正（`D{N}_{场景}_{动作}_{细节}`，去 S 编号冗余）；同步更新 `design.md` §2 Decision Q7 + `d5-domain.md` 命名规范节 + `spans-registry.md` |
| 1.4.0 | 2026-06-19 | DSAFT 架构方法论 Review（§16）：四条分析轴 × 分层体检 + 发现清单（2C/6W/3I） |
| 1.3.0 | 2026-06-19 | Claude 第二轮对焦：对 §13.1–13.7 逐条回应 + 两轮合并落地清单（15 条） |
| 1.2.0 | 2026-06-19 | MiniMax 第二轮增量：§13.1–13.7（信息不对称/ESS/SRE 缺位/时间属性/公共物品/多 Referee/Phase 强度）+ 总结表 + 落地建议 |
| 1.1.0 | 2026-06-19 | Claude 博弈论对焦：OQ 1–6 + Grill 1–6 + 补充视角 + 总结表 |
