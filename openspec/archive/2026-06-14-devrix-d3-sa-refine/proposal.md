# Proposal: D3 LLM Gateway S/A 重切 — 价值流化

**Change ID:** devrix-d3-sa-refine
**Demand ID:** DM-20260614-016
**Status:** S2_Clarified (Review R1 incorporated)
**Phase Scope:** D + S（不含 A/F/T 编排；编排在 design.md 阶段产出）

---

## 1. Background

D3 LLM Gateway 自 V1（2026-06-07，DM-004）至 V2.1（2026-06-14，V2 增强 + Safety + ModelTier）共迭代 3 个版本、26 条 T（25 IMPLEMENTED + 1 PLANNED），功能完整。但 **D3 是当前 D1–D7 架构中唯一未做价值流切法的核心/公共域**：

| 域 | 价值流 S 数 | 价值流化状态 |
|----|------------|------------|
| D1 Communication | 6 (S13–S18) | ✅ v2.0 完成 |
| D2 Context Engine | 6 (S15–S20 Canonical) | ✅ v2.0 完成 |
| **D3 LLM Gateway** | **0 / 7** | ❌ **本 change 处理** |
| D4 Multi-Agent | 10 (S1–S10) | ⚠️ 偏技术角色（待后续） |
| D5/D6 | 9/4 | ⚠️ 支撑/公共域技术角色可接受 |
| D7 Orchestration | 5 (S1–S5) | ✅ 完成 |

D3 当前 7 个 S（Adapter / Gateway / Breaker / Retry / Token / Config / Safety）**全部为技术角色词**，且物理目录（`adapter/` `gateway/` `breaker/` `retry/` `token/` `config/` `safety/`）与之 1:1 绑定，违反 `openspec/specs/architecture/code-layout.md §2` 明确**禁止**「技术角色词作为 L2 scenario-slug」的规定。

> 本 change 是 `dsaft-refactoring-playbook.md` 的**首次 D 域应用**，输出可作为后续 D2/D4/D5/D6 价值流化重构样板。

---

## 2. Problem Statement

### 2.1 价值流承诺无法对应到 S

D3 是公共域（横向能力），对消费者（D2/D4）提供 5 类**可验证承诺**：

1. **路由承诺**：给我模型名/tier，我返回正确 Provider + 模型
2. **流式承诺**：给我 chat 请求，我流式返回 chunk
3. **韧性承诺**：Provider 故障不阻塞我
4. **预算承诺**：上下文不超 token 预算
5. **安全承诺**：恶意内容不能穿过 gateway

但当前 7 个 S 是「实现机制」而非「承诺能力」，承诺 3（韧性）被拆为 S3（Breaker）+ S4（Retry）两个 S，导致：

- **错误归因错位**：Provider 失败可能是 Breaker 拒 / Retry 退避 / Adapter 解析错，三者在不同 S，告警与路由决策无统一动线
- **新加 Provider 容易、运维困难**：实现者只碰 `adapter/`（S1），但故障时 SRE 不知该查 S3 还是 S4

### 2.2 跨域边界模糊

| # | 问题 |
|---|------|
| P1 | D3-S7 Safety 与 D2-S18 PermissionMode 职责未声明，Safety 在 V2.1 是「补丁场景」（为放新代码而加） |
| P2 | D3-S2-A01 RouteLLMCall 挂载了 bridge（`AdaptToContextEngine`）与 bootstrap（`WireLLMStack`）两个异质 F，混在「路由」A 下 |
| P3 | D3 韧性状态（Breaker Closed/Open/HalfOpen）仅 D3 内部使用，不 emit 到 D5/D7，导致 D7 编排者不知 Provider 已被熔断 |

### 2.3 注册表内部失同步

| # | 失同步项 | 影响 |
|---|---------|------|
| S1 | `spec.md` / `layering.md` 列到 D3-S6；`a-registry.md` 实际有 D3-S7 Safety | 评审 / 检索时易漏查 |
| S2 | `code-layout.md §4` 缺 D3 scenario-slug 注册表（D1/D2/D7 都有，唯 D3 漏登） | 物理路径决策无锚点 |

### 2.4 物理路径违反 code-layout.md §2

D3 子目录 7/7 命中 `code-layout.md §2` 禁止名单（`gateway` `adapters` 等技术角色词），与 D1 v2.0 价值流化后的 `capture/` `channel/` `delivery/` 风格不一致。

---

## 3. Proposed Solution

### 3.1 D 层（不变）

**D3 LLM Gateway** 保持公共域身份，向上提供 5 类可验证承诺，**不调整 D 层职责边界**。

### 3.2 S 层（5+1 价值流切法）

```
D3（LLM Gateway / 公共域）
├── D3-S1 RouteModel          # 承诺 C1：模型名/tier → Provider + 模型
├── D3-S2 StreamChat          # 承诺 C2：流式 + SSE + 中止控制
├── D3-S3 ProtectCall         # 承诺 C3：Breaker + Retry + Fallback 一体化
├── D3-S4 BudgetTokens        # 承诺 C4：Token 计数 + 预算检查 + 截断
├── D3-S5 GuardContent        # 承诺 C5：内容安全过滤
└── D3-S6 ConfigureGateway    # 横切：配置加载 + 验证（不属 5 类承诺）
```

**S ↔ 承诺 1:1 对应表：**

| S ID | Scenario | 对应承诺 | 用户/消费者可验证 WHAT | 旧 S 归属（冻结追溯） |
|------|----------|---------|----------------------|---------------------|
| D3-S1 | RouteModel | C1 路由承诺 | `Resolve("fast")` → `MiniMax-M2.7-highspeed` | D3-S2 Gateway (Resolve/ResolveTier) |
| D3-S2 | StreamChat | C2 流式承诺 | `Stream(ctx, req)` 流式返回 chunk，ctx.Done 关闭 chan | D3-S1 Adapter + D3-S2 Gateway (Stream 主实现) |
| D3-S3 | ProtectCall | C3 韧性承诺 | Provider 故障经 Breaker 拒 / Retry 退避 / Fallback 切换，**不阻塞消费者** | D3-S3 Breaker + D3-S4 Retry |
| D3-S4 | BudgetTokens | C4 预算承诺 | `CountMessages + CheckBudget` 后超限截断或报错 | D3-S5 Token |
| D3-S5 | GuardContent | C5 安全承诺 | `Filter.Check` 命中 critical pattern → Allowed=false | D3-S7 Safety |
| D3-S6 | ConfigureGateway | 横切支撑 | 改 yaml → 切换 Provider/调整预算，**不需改代码** | D3-S6 Config |

### 3.3 scenario-slug 注册表（v1.0 阶段写入 `code-layout.md §4`）

| S ID | Scenario | scenario-slug | 目标目录 | 当前路径 | 迁移阶段 |
|------|----------|---------------|----------|---------|---------|
| D3-S1 | RouteModel | `route` | `llmgateway/route/` | `gateway/router.go` | v2.0 |
| D3-S2 | StreamChat | `stream` | `llmgateway/stream/` | `adapter/openai_*.go` + `gateway/gateway.go` (Stream 主实现) | v2.0 |
| D3-S3 | ProtectCall | `protect` | `llmgateway/protect/` | `breaker/` + `retry/` | v2.0 |
| D3-S4 | BudgetTokens | `budget` | `llmgateway/budget/` | `token/` | v2.0 |
| D3-S5 | GuardContent | `guard` | `llmgateway/guard/` | `safety/` | v2.0 |
| D3-S6 | ConfigureGateway | `configure` | `llmgateway/configure/` | `config/` + `shared/config/llmgateway.go` | v2.0 |
| — | Domain Kernel | `kernel` | `llmgateway/kernel/` | （v1.0 不引入；factories 与 re-export 在根包） | — |

> 全部 slug 语义化、Go 合法目录名（`code-layout.md §2.2`），无技术角色词，无下划线。

### 3.4 Legacy 双轨（v1.0 阶段交付的 alias 表）

v1.0 注册表重排时，**所有旧 S/A/T ID 写入 `t-registry.md §Legacy Archive`**，不破坏既有 26 条 T 的追溯：

| 旧 S ID | 新 S ID | 旧 T ID（前缀） | 新 T ID | 改 T 数 | 备注 |
|--------|--------|----------------|---------|---------|------|
| D3-S1 Adapter | D3-S2 StreamChat | D3-S1-A01-T* | D3-S2-A01-T* | 4 | A 不变，T ID 末位不变 |
| D3-S2 Gateway | D3-S1 RouteModel | D3-S2-A01-T* | D3-S1-A01-T* | 7 | A 不变，T ID 末位不变 |
| D3-S3 Breaker | D3-S3 ProtectCall | D3-S3-A01-T01..T05 | D3-S3-A01-T01..T05 | 5 | S 名变 A 编号变，T ID 不变 |
| D3-S4 Retry | D3-S3 ProtectCall | D3-S4-A01-T01..T04 | D3-S3-A01-T05..T08 | 4 | Retry 整体并入 ProtectCall 的 T05+ |
| D3-S5 Token | D3-S4 BudgetTokens | D3-S5-A01-T01..T03 | D3-S4-A01-T01..T03 | 3 | S 号变 A 编号变，T ID 不变 |
| D3-S6 Config | D3-S6 ConfigureGateway | D3-S6-A01-T01 | D3-S6-A01-T01 | 1 | 仅 S 名变 |
| D3-S7 Safety | D3-S5 GuardContent | D3-S7-A01-T01/T02 | D3-S5-A01-T01/T02 | 2 | S 号变 A 编号变，T ID 不变 |
| — | CROSS | D3-S2-A01-F04/F05 | D3-S0-A01-F01/F02（CROSS 段） | 0 | Bridge 跨域归位（详见 Decision D2） |

**改 T 合计：26 条**（含 Bridge 跨域归位的 2 个 F 改写为 CROSS 段，不增不减）。

### 3.5 跨域边界声明（v1.0 阶段产出新文件）

新增 `openspec/specs/architecture/cross-domain-boundaries.md`，明确：

| 边界 | D3 SoT | 邻域 SoT | 备注 |
|------|--------|---------|------|
| Prompt 内容过滤 | **D3-S5 GuardContent** | D5 Observability 负责 audit log；D6 负责 Safety 评测 | 「过滤哪些 prompt」属 D3 边界；D2-S18 PermissionMode 负责「允许哪些 tool」 |
| Provider 韧性状态 | **D3-S3 ProtectCall** | D5 接收 metric（`llm_breaker_state{provider,state}`）；D7 编排者通过现有 EngineEvent 复用 | v1.1 阶段实施；v1.0 仅声明边界 |
| LLM 调用总成本 | **D3-S4 BudgetTokens** | D7 编排决策（不超预算） | 双方通过 config 解耦，无直接契约 |
| 工具 schema 注入 | **D3-S2 StreamChat**（Request.Tools 字段） | D2 工具注册表（源数据） | D2 → D3 单向；D3 不查 D2 工具库 |
| Adapter 协议 | **D3-S2 StreamChat**（OpenAI-compatible） | 无 | 第三方契约；D3 主权 |

### 3.6 三段终态（v1.0 / v1.1 / v2.0）

| 版本 | 范围 | 关键产出 | 风险 |
|------|------|---------|------|
| **v1.0 Registry** | 4 注册表重排 + Legacy alias + layering.md 同步 + code-layout.md §4 补 D3 + cross-domain-boundaries.md 新建 + Bridge 跨域归位 | 5+1 S 价值流化 + 26 条 T 全绿 | 低（纯文档 + 注释） |
| **v1.1 Traceability** | 4 表追溯（S→A→F→T→Span）+ D3→D5/D7 韧性状态 emit + D6 评测点暴露（3 probe） | 跨域可观测 + 评测闭环 | 中（新增 span/metric 名） |
| **v2.0 Structure** | 物理路径迁移到 scenario-slug + contracts.go 拆分到子包 + re-export 桥接 1 周期 + 旧路径 dead code 清理 | 物理目录与价值流 S 1:1 对齐 | **高**（需 T 全绿 + 1 周期兼容） |

> **v1.0 + v1.1 合并发布**（R1 Q5 决议），避免「注册表已价值流化但代码目录仍叫 adapter/」的中间态；v2.0 物理迁移作为下一个独立 release。

---

## 4. Success Metrics

| 指标 | 当前基线 | 目标（v1.0） | 目标（v2.0） |
|------|---------|-------------|------------|
| D3 价值流 S 数 | 0 / 7 | 5 + 1（横切） | 5 + 1（横切） |
| scenario-slug 语义化率 | 0 / 7 | 6 / 6 + 1（kernel 暂不引入） | 6 / 6 + 1 |
| P0 T 全绿数 | 11 | 11（保持） | 11（保持） |
| T 总数 | 26 | 26（保持，alias 100% 覆盖） | 26（保持，alias 仍可追溯） |
| Bridge / Bootstrap F 误挂 D3 内部 A 数 | 2（F04/F05） | 0（迁 CROSS 段） | 0 |
| D3 韧性状态对 D5/D7 可见性 | ❌ | v1.1 接入 | 持续 |
| D6 Safety 评测点 | 0 | 0 | v1.1 增 3 probe |
| 跨域 import 违规（D3 import D2/D4） | 0 | 0（保持） | 0（保持） |
| `go vet` 警告（v2.0 物理迁移） | — | — | 0 新增 |

---

## 5. Implementation Plan（Phase 概要，不估时）

> **不估时**（playbook 原则 + OpenSpec S2 阶段约束）。各 Phase 内容描述而非时间盒。

### Phase A — 文档澄清（S1→S2）

- `demand.md`（DM-20260614-016）
- `proposal.md`（本文件，S2 产物）
- `tasks.md`（S2 阶段写任务分解骨架；不含估时）
- `openspec/specs/d3-llm-gateway/spec.md` 重写草案

### Phase B — v1.0 Registry 重排

- `a-registry.md` / `f-registry.md` / `t-registry.md` / `span-registry.md` 全表重排
- `t-registry.md` §Legacy Archive 写入 26 条 alias
- `layering.md §D3` 同步（注册 5+1 S）
- `code-layout.md §4` 补 D3 scenario-slug 注册表
- 新建 `openspec/specs/architecture/cross-domain-boundaries.md`

### Phase C — v1.0 验证

- `grep -r "D3-S[1-7]" openspec/specs/` 与 a/f/t-registry 无失同步
- 所有 26 条 T（含 11 P0）保持 IMPLEMENTED 状态
- Bridge 跨域归位：D3 内部 f-registry 移除 F04/F05；CROSS 段新增
- `demand-archive-index.md` 末尾追加 D3 入口
- 产出 `acceptance-report（v1.0）`

### Phase D — v1.1 韧性可见性

- D3 → D5：新增 metric `llm_breaker_state{provider,state}`（Counter / Gauge 待设计）
- D3 → D7：复用现有 EngineEvent（`FlowStarted` / `FlowFailed`），不新增 D3→D7 直接契约
- D6：增 3 个 probe（Tier 解析正确性、Breaker 状态切换次数、Token 预算触发率）

### Phase E — v1.1 验证

- Span 完整性（`span-registry.md` 与 d5-observability 全链路一致）
- 跨域回归：D2/D4 消费 D3 行为不变
- 产出 `acceptance-report（v1.1）`

### Phase F — v2.0 物理迁移

- 子目录改名 → 价值流 scenario-slug（`route/` `stream/` `protect/` `budget/` `guard/` `configure/`）
- `contracts.go` 拆分到各子包；根 `contracts.go` 保留 re-export 一发布周期
- 旧路径（`adapter/` `gateway/` `breaker/` `retry/` `token/` `config/` `safety/`）保留 re-export 一发布周期
- `bridges/llm/` 路径不变（跨域锚点）
- `layering.md §Domain Layout` 更新

### Phase G — v2.0 验证 + 归档

- 完整 `go build ./...` 与 `go test ./...` 全绿
- `go vet` 无新增警告
- 旧路径 dead code 清理
- change 包归档到 `openspec/archive/2026-MM-DD-devrix-d3-sa-refine/`
- `demand-archive-index.md` 标记 D3 价值流化完成
- 产出 `acceptance-report（v2.0）` + S7 归档报告

---

## 6. Risks & Mitigations

| 风险 | 可能性 | 影响 | 缓解 |
|------|--------|------|------|
| v1.0 T ID 改名导致外部 dashboard / 告警 grep 失效 | 中 | 中 | metric / span 名保持不变（仅 `T:` 注释与注册表 ID 改）；Legacy alias 写入 `t-registry.md §Legacy Archive` |
| v2.0 物理迁移破坏 V2.1 IMPLEMENTED 状态 | 中 | 高 | re-export 桥接包 + 1 发布周期兼容 + 完整 P0 回归（Phase G 验证） |
| Bridge 归位（D2-1）打破 D2 bootstrap 调用方 | 中 | 中 | Phase B 优先于 v2.0；D2 已有 WireContextLLM 调用方清单（consumer: 1 处）；桥接后回归 |
| 价值流 S 切法与 D7 编排期望冲突 | 低 | 中 | D7 编排者进 S2 Review；D3 韧性状态 emit（v1.1）作为解耦手段 |
| Safety 归属论证不充分被 D2/D6 挑战 | 中 | 中 | Decision D3-1 写入 demand.md §5；与 D2-S18 边界声明写入 `cross-domain-boundaries.md` |
| v1.0 + v1.1 合并发布窗口内出现 v1.0 单独回归需求 | 中 | 中 | v1.1 的 D3→D5/D7 emit 设计为**可选**（feature flag `d3_resilience_emit_enabled`），可分批合入 |
| 26 条 T alias 覆盖率 < 100% | 低 | 高 | Phase B 末尾必须 100% 覆盖才进 Phase C；alias 表由 `devrix/scripts/check_t_aliases.py`（新写，Phase B 产出）校验 |

---

## 7. Out of Scope

明确**不属于**本 change 的范围：

| 项 | 原因 / 归属 |
|---|-----------|
| Provider 适配器重写（DeepSeekAdapter / MiniMaxAdapter 行为不变） | 行为冻结，v1.0–v2.0 不动实现 |
| D2-S4 Token（Context Engine）vs D3-S4 BudgetTokens 合并 | 属 D2 change 范畴（DM-20260614-009 已规划）；本 change 仅在 cross-domain-boundaries.md 声明边界 |
| Safety filter 与 D2-S18 PermissionMode 合并 | 需另立 D2 change；本 change 仅论证归属（Decision D3-1） |
| V3 计划（Anthropic Adapter / Rate Limiter / 负载均衡） | 属未来 change；本 change 不预留 S 编号 |
| D3 公共域对**新消费者**（非 D2/D4）的开放（如 CLI / Web UI） | 属未来 change；ILLMGateway 契约已具备扩展性 |
| D3 kernel/ 子包引入 | v1.0 不引入；`Request/Chunk/CircuitState` 等核心类型在根 `contracts.go` 已是事实 kernel |
| 多模型路由策略（业务路由，跨域编排） | 属 D7 Orchestrator；D3 仅提供 tier 别名解析，不做业务路由 |
| Audit 日志（合规审计） | 属 D5 Observability；D3 仅 emit metric，audit 由 D5 决定 |

---

## 8. 与 D7 重构案例的对照

D7 升格（`openspec/archive/2026-06-14-devrix-d7-orchestration-domain/`）是 dsaft-refactoring-playbook 的首案，本 change 复用其模板与术语：

| 维度 | D7（已归档） | D3（本 change） |
|------|------------|---------------|
| 核心矛盾 | D2 职责溢出 + ORCH 身份不足 | D3 S 切法为技术角色词 + 公共域身份未贯彻 |
| 关键 Decision | 三模型 Task 职责分离 + 编排路由矩阵 | 价值流 S 切法 + Bridge 跨域归位 + Safety 归属 |
| 迁移期 | `d7_enabled` feature flag + 4 组合回归矩阵 | T ID alias 表 + metric 名不变 + re-export 桥接 |
| 范围 | v1.0 Registry → v1.1 Traceability → v2.0 Structure | **同**（v1.0 + v1.1 合并发布；v2.0 独立 release） |
| 跨域耦合 | D1→D2 → D1→D7 | D3 内部混搭 bridge/bootstrap → CROSS 段 |

---

## 9. 评审入口（供 S2 → S3 推进）

| 文档 | 用途 | 状态 |
|------|------|------|
| `demand.md` | 需求澄清 SoT + Review R1 决议 | ✅ S2_Clarified |
| `proposal.md`（本文件） | D + S 切法 | ✅ S2 产物 |
| `openspec/specs/d3-llm-gateway/spec.md` | 重写（S2 输出，但本 change 内交付 v1.0 草案） | ⬜ Phase B 写 |
| `openspec/specs/d3-llm-gateway/design.md` | A + F 编排 | ⬜ S3 阶段（**本 change 不产出**，留待 S3-Gate 后另立 change） |
| `tasks.md` | 任务分解（无代码） | ⬜ Phase A 末尾 |
| `openspec/specs/architecture/cross-domain-boundaries.md` | D3 vs D2/D5/D6 边界 | ⬜ Phase B 新建 |
| `openspec/specs/architecture/code-layout.md §4` | D3 scenario-slug 注册表 | ⬜ Phase B 补 |
| `demand-archive-index.md` | D3 入口追加 | ⬜ Phase C |

---

**Revision History**

| 版本 | 日期 | 变更 |
|------|------|------|
| 0.1 | 2026-06-14 | 初稿：5+1 S 切法 + 三段终态 + 8 项 Out of Scope |
