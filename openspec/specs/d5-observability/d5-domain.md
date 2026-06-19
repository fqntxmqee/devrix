# D5 Observability Domain

**Domain ID:** D5
**Slug:** `observability`
**Type:** Common Domain（公共域 / 裁判域）
**Status:** Active — Canonical S21–S24 + S0（v2.1 terminal 设计目标）
**Version:** 1.1.0
**Last Updated:** 2026-06-19
**Depends On:** Shared `config` / `types`；无业务域硬依赖
**Depended By:** D1, D2, D3, D4, D6, D7（全员 Bridge + Registry）
**Cross-Domain SoT:** `d5-boundary.md` · `../architecture/cross-domain-boundaries.md` §D5

> S3 设计稿 · Change `devrix-d5-v2-terminal`（DM-20260619-006）。S7 归档后迁入 `openspec/specs/d5-observability/d5-domain.md`。

---

## Tl;DR（新人入口 · ≤200 字）

D5 是 Devrix 的**横向可观测性基础设施（裁判域）**。不参与业务决策，不创建 Turn span，不阻塞任何域。核心职责：**让任意操作在 Jaeger/Prometheus/Health/Incident 中可交叉验证**。4+1 个价值流场景（S21 Instrument / S22 Export / S23 Diagnose / S24 Configure / S0 Facade）。56 个 Operation 常量（`telemetry.Op*`），41 个测试锚点（T ID 冻结）。v2.1 Terminal = S21–S24 号段 + 物理路径 + 56 ops **长期冻结**。博弈角色：Referee（裁判）+ Auditor（审计）——吹哨不终止比赛。

> 必读：[MUST] `d5-domain.md`（本文）→ [MUST] `d5-boundary.md` → [SHOULD] `observability-guide.md` → [REFERENCE] `gaming-analysis.md`

---

## North Star

**作为全链路可观测性基础设施（裁判域），为任意域操作提供可追踪、可度量、可诊断的遥测数据，并保证 Jaeger / Prometheus / Health / Incident Export 可交叉验证。**

| 可验证承诺 | Canonical S | 消费者可验证 WHAT |
|-----------|-------------|-------------------|
| C1 遥测生成 | D5-S21 Instrument | 任意 operation → Span + Metric + Log + layer/component |
| C2 遥测导出 | D5-S22 Export | Span→OTLP/Console；Metric→Prometheus |
| C3 诊断辅助 | D5-S23 Diagnose | Coverage 对账、Incident、Doctor、Tracker、FaultInject(test) |
| C4 配置管理 | D5-S24 Configure | yaml 切换 exporter/采样；runtime path 计数 |
| C0 横切 Facade | D5-S0 | Init/Shutdown/Bridge/SessionGauge；观测失败不阻断业务 |

---

## 完备性边界（D5 保证什么 / 不保证什么）

| 维度 | D5 保证 | D5 不保证 |
|------|--------|----------|
| 埋点覆盖 | Span 是否存在、是否被触发（Coverage） | 业务关键路径是否被埋（"该埋未埋"不可检测） |
| Trace 完整 | Trace 树可交叉引用（D1→D7→D2） | 业务决策正确性 |
| 导出可复现 | OTLP/Prometheus 数据到达 | 性能 SLA（除已登记 metric） |
| 诊断审计 | Coverage 对账、Incident 举证、Doctor 环境检查 | 准入控制（归 D7-S2） |
| 配置可验证 | yaml 切换 exporter/采样可测 | 配置的业务语义正确性 |

**核心原则：D5 保证"可观测过程诚实"（有没有 span、有没有 hit、能不能导出），不保证"业务决策正确"。**

---

## 博弈论玩家表

```
Principal（用户 / SRE / 平台 Owner）
    ↓ 委托「系统要可解释、可排障」
各业务域玩家（D1/D2/D3/D4/D7/D6）
    ↓ 局部最优：少埋点、快上线、改自己包最省事
D5 Observability（Referee + Auditor + Commitment Device）
    ↓ 产出：Span / Metric / Log / Coverage / Incident
外部观测者（Jaeger / Prometheus / on-call）
    ↓ 验证：Trace 树、zero_hit、Health、export bundle
SRE / on-call（主权消费者 —— 即时反馈回路）
    ↓ 消费 D5 输出进行排障、报警、dashboard 监控
Principal 事后问责
```

| 玩家 | 目标函数（局部） | 全局希望 | D5 机制响应 |
|------|------------------|----------|-------------|
| D2/D7 开发者 | 改功能包最快；埋点可后置 | E2E 可排障 | Operation Registry + Coverage 独立于采样 |
| 新功能作者 | 复制粘贴 span 名 | canonical op 一致 | `telemetry.Op*` + WARN unknown op |
| **SRE / on-call** | **3 分钟定位故障域** | **Trace 树一眼可读** | **SpanAttrs 强制注入 layer/component；Runbook 共建** |
| 测试作者 | 单测 mock 观测 | Bridge + NoOp | `NewNoOp()` 不阻断业务 |
| 诊断工具作者 | 功能堆进 diagnose/ | S 层语义清晰 | S23 子承诺 C3a–C3e |
| 架构维护者 | 目录=文档 | 双锚点对齐 | v2.1 删 bridge、补 d5-domain |

> **SRE/on-call 是 D5 输出的主权消费者**，反馈循环短于"事后问责"。D5 的成功指标是过程指标（覆盖率 + trace 完整度）+ 验证指标（MTTR 改善趋势），而非单一覆盖率 KPI。

---

## 与 D7「Mediator」的对称

| 域 | 博弈角色 | 保证什么 | 不保证什么 |
|----|----------|----------|------------|
| D7 | Mediator / Turn Leader | 编排路径可验证、Turn span 存在 | 结论质量 |
| D2 | Execution Follower | Prepare/Tool/Persist 机制 | 走哪条编排路径 |
| **D5** | **Referee + Auditor** | **埋点覆盖、Trace 可交叉引用、导出可复现** | **业务正确性、性能 SLA（除已登记 metric）** |
| D6 | Judge | 评测/守卫 advisory | 不阻塞主路径 |

**D5 的博弈定位：Referee 吹哨（warn），但不终止比赛（fail）。终止比赛的权力归 D7（Leader）或用户（Principal）。**

---

## Out of Scope

| 能力 | 归属 |
|------|------|
| Turn 主循环 / LLM 调用权 | D7 |
| Prepare / ToolRound / 沙箱 | D2 |
| IM ingress / 展示 | D1 |
| LLM 路由 / 熔断 | D3 |
| Agent 生命周期 | D4 |
| 结论质量 / Guard 决策 | D6 |
| Task/Plan 写模型 | D7 workmodel |

---

## DSAFT 资产

### Canonical 价值流 — S21–S24 + S0

| S ID | Scenario | 博弈角色 | Commitment 类型 | Status |
|------|----------|----------|-----------------|--------|
| D5-S21 | Instrument | Signal Producer | Span/Metric/Log 可生成 | ACTIVE |
| D5-S22 | Export | Signal Shipper | 数据到达 OTLP/Prom | ACTIVE |
| D5-S23 | Diagnose | Auditor + Evidence Clerk | Coverage/Health/Export/Tracker | ACTIVE |
| D5-S24 | Configure | Rule Setter | 配置切换可验证 | ACTIVE |
| D5-S0 | Facade | Integration Shell（非独立子博弈） | Init/Bridge/NoOp | ACTIVE |

### 时间属性 × 承诺强度交叉矩阵

| | 事前（ex-ante） | 事中（during） | 事后（ex-post） |
|--------|----------------|---------------|----------------|
| **强承诺** | Doctor 启动阻塞 | — | **Incident 导出失败报警（不可补救）** |
| **弱承诺** | Operation Registry | Coverage 报告 / Tracker | — |

- **事前×强承诺**：Doctor 启动失败硬阻塞（环境不可用则观测无意义）
- **事中×强承诺（空）**：运行时硬阻塞违反 Graceful Degradation 原则
- **事后×强承诺**：Incident export（C3b）事后证据**不可补救**——零容忍失败，必须报警
- **事前×弱承诺**：Operation Registry 编译期已知，但可事后补注册
- **事中×弱承诺**：Coverage 可补救（下个窗口期补埋），Tracker 可降采样
- **事后×弱承诺（空）**：事后证据不能弱

### S23 子承诺（按时间属性分组）

| 子承诺 | 角色 | 时间属性 | 验证者 |
|--------|------|---------|--------|
| C3c Doctor + Health | Environment Auditor | **事前 + 事中** | `/doctor` + `/health` |
| C3a Coverage | Compliance Auditor | **事中** | CI + Health zero_hit |
| C3d Tracker | Continuous Inspector | **事中** | 非阻塞 T + integration |
| C3b Incident | Evidence Archivist | **事后（不可补救）** | `debug export` schema |
| C3e FaultInject | Red Team | **测试（与生产隔离）** | test tag only |

### S23 硬边界与 S25 触发条件

**硬边界（语义/数量/依赖）：**

| 边界 | 规则 |
|------|------|
| 语义边界 | S23 只含"事后审计/举证"；"实时准入控制"归 D7，"即时执行决策"归 D2/D4 |
| 数量边界 | 子承诺数 ≤ 7（超过则拆 S25） |
| 依赖边界 | S23 不 import D2/D4/D7（除 contracts 接口） |

**S25 触发条件（预先承诺 — 防止策略漂移）：**

| 触发条件 | 含义 |
|----------|------|
| Tracker 独立产品化（被外部系统消费） | 不再是内部审计 → 新 S |
| C3e FaultInject 被要求生产可用 | 不再是 testbuild-only → 新博弈语义 |
| C3 子承诺数 > 7 | Schelling 点：超过 7 个子承诺意味着 S 层语义不再清晰 |

### S23 子承诺新增的举证责任

任何提议新增 C3f 的提案，必须证明：
1. 该能力无法归入现有 C3a–C3e
2. 该能力的消费者与现有子承诺的消费者不同
3. 新增不导致 S23 超过 7 个子承诺上限

### 登记规模（终态目标 — v2.1 后复核）

| 层 | 数量 | SoT 文件 |
|----|------|----------|
| S | 5（S0+S21–S24） | `d5-domain.md` + `spec.md` |
| A | 30 | `a-registry.md` v4.0 |
| F | ~45 | `f-registry.md` v3.0 |
| T | 41（14 P0） | `t-registry.md` v3.2 |
| Span ops | 56 | `span-registry.md` |

### Legacy Module Index（冻结）— D5-S1–S9

| Module | Canonical |
|--------|-----------|
| S1–S3, S6 | S21 |
| S4 | S22 |
| S5, S8 | S23 |
| S7, S9 | S24 |

### 物理路径映射

| Canonical S | 物理路径 |
|-------------|----------|
| S21 | `instrument/{tracer,metrics,logger,telemetry}/` |
| S22 | `export/` |
| S23 | `diagnose/{coverage,incident,doctor,tracker,faultinject}/` + `health.go` |
| S24 | `configure/{settings,runtime}/` + `config.go` |
| S0 | `observability.go`, `bridge.go` |

---

## Terminal 冻结声明

**v2.1 Terminal 是 D5 的终态闭合。以下资产声明为长期冻结：**

| 冻结对象 | 范围 | 变更需 |
|----------|------|--------|
| S 号段 | S21–S24 + S0 | D5 Owner + 2 个业务域 Owner NACK 权 |
| 物理路径 | `instrument/` / `export/` / `diagnose/` / `configure/` | D5 Owner Review |
| Operation Registry | 56 ops（不增删，除非 RETIRED 清理） | D5 Owner Review + span-registry 更新 |
| T ID | 41 T（不改号，仅 canonical_s/a 列校正） | 跨域 Review（T 层宪法） |

**稳定期承诺：** v2.1 归档后至少 2 个 release cycle（~2 个月）不进行 S 层或物理路径变更。

---

## 各域 Bridge 删除时间线（公共知识）

| 域 | Change | 日期 | 状态 |
|----|--------|------|------|
| D6 | v2.0.1 bridge 删除 | 2026-06-15 | ✅ 已完成 — 先行验证"bridge 可安全删除" |
| D7 | v2 Structure | 2026-06-19 | ✅ 已完成 — 终态闭合模式参考 |
| **D5** | **v2.1 Terminal** | **2026-06-19** | **🔄 进行中 — 第三步级联跟随者** |
| D2 | v2.2 closure | 2026-06-19 | ✅ 已完成 — 平行级联 |
| D1 / D4 | 尚未进入 terminal | TBD | 待启动 — 三个先例（D6+D7+D5）构成足够强的信息级联 |

> D6→D7→D5 形成**序贯信息级联**：D6 用最低风险验证假说 → D7 扩展模式 → D5 成为第三个先例。D5↔D2 为**平行级联**（同期但独立）。

---

## 规格文档索引

| 文档 | 用途 | 阅读优先级 |
|------|------|-----------|
| `d5-domain.md`（本文） | 领域 SoT | **MUST** |
| `spec.md` v3.0 | Gherkin 验收 | **MUST** |
| `d5-boundary.md` | 跨域契约 | **MUST** |
| `observability-guide.md` | Span↔T、Trace、Runbook | **SHOULD** |
| `terminal-state-guide.md` | 终态叠合 | SHOULD |
| `gaming-analysis.md` | 博弈论推导（change 根目录） | REFERENCE |
| `d5-requirements-clarifications.md` | Review 澄清 + Grill 问题 | REFERENCE |
| `design.md` | 六段式 + Decision | REFERENCE |
| `a-registry` / `f-registry` / `t-registry` | A/F/T | REFERENCE |
| `span-registry.md` / `coverage.md` | Ops + 染色手册 | REFERENCE |
| `layer-delta.md` | §v2.1-Terminal | REFERENCE |
| `dsaft-architecture.md` | 五层计数 Stub | REFERENCE |
| `../architecture/code-layout.md` §4.6 | scenario-slug | REFERENCE |

---

## Operation / Span 命名规范

D5 作为 Operation Registry 的拥有者，定义全仓统一的 Span 命名规范。

**格式：** `D{N}_{场景名称}_{动作}_{细节}`

| 段 | 含义 | DSAFT 对应 | 示例 |
|----|------|-----------|------|
| `D{N}` | 域编号 | D 层 | `D1`, `D7` |
| `{场景名称}` | 场景语义名（非 S 编号） | S 层 | `Capture`, `Orchestration` |
| `{动作}` | 业务动作 | A 层 | `Message`, `Turn` |
| `{细节}` | 操作细节（可多层） | F 层 | `Receive`, `Run` |

**示例：**

| Span 名称 | 拆解 |
|-----------|------|
| `D1_Capture_Message_Receive` | D1 · Capture 场景 · Message 动作 · Receive 细节 |
| `D7_Orchestration_Turn_Run` | D7 · Orchestration 场景 · Turn 动作 · Run 细节 |
| `D2_Context_Process` | D2 · Context 场景 · Process 动作 |
| `D3_LLM_Stream` | D3 · LLM 场景 · Stream 动作 |

**规则：**
- 全部大写，`_` 分隔
- **不在中间插入 S 编号**（场景名称已唯一标识场景）
- Go 常量名仍保留 `OpD{N}_S{N}_...` 前缀以保持 DSAFT 追溯（如 `OpD1_S13_Capture_Message_Receive`），但**运行时字符串值**使用本规范

**归属：** 所有 Span name 必须在 `names.go` 中以 `Op*` 常量定义，在 `coverage/registry.go` 中注册。各域使用 `telemetry.Op*` 常量，禁止 ad-hoc 字符串。

---

## 跨域契约（摘要）

见 `d5-boundary.md`。要点：D7 创建 Turn span；D5 提供 Op 常量；D2 Tracker 只读；RecordHit 独立于采样。

---

## 修订记录

| Version | Date | Changes |
|---------|------|---------|
| 1.1.0 | 2026-06-19 | 博弈论共识落地：Tl;DR 新人入口、SRE/on-call 玩家表、完备性边界声明、时间属性×承诺强度交叉矩阵、S25 触发条件、子承诺举证责任、Terminal 冻结声明、各域 Bridge 删除时间线、文档阅读优先级标注 |
| 1.0.0 | 2026-06-19 | S3 设计稿 v2.1 terminal |
