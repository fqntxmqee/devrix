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

## 10. 修订记录

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-19 | 初稿：devrix-d5-v2-terminal S3 |
