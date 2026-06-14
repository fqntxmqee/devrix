---
review-id: R2
title: D3 LLM Gateway S/A 重切 — 二次 Review（结构层）
change-id: devrix-d3-sa-refine
demand-id: DM-20260614-016
reviewer: Claude（与 R1 同 reviewer，结构层加深）
review-date: 2026-06-14
status: S3-Gate — R2 FINALIZED（R3 接力接口已闭合，全部 12 项接受）
predecessor: review-r1.md (R1, 2026-06-14, 用户 Owner)
predecessor-verdict: APPROVED（3 Decision + 7 澄清全部接受）
successor: review-r3.md (R3, 2026-06-14, Claude 自裁决)
---

# D3 LLM Gateway S/A 重切 — Review R2 二次裁决

> 本文档为二次 Review，对 R1（`review-r1.md`）的 3 个 Decision + 7 项澄清**完全接受**，但对结构层的 5 个命题、4 个 OQ、3 个保留分歧项提出明确答案。
> 本文不修改 `demand.md` / `proposal.md` / `tasks.md` / 任何 `specs/` 与 `changes/` 文件，仅承载**可被 Owner / Codex / 后续 reviewer 接力**的命题与决议接口。
> 综述与分析（含博弈论框架展开、5+1 S 切法的演化均衡评估）已通过对话完成。

---

## 1. 立场：完全接受 R1 全部决议

R1 全部 3 个 Decision（D1 5+1 S 切法 / D2 Bridge 跨域归位 / D3 Safety 留 D3）+ 7 项澄清（Q1 公共域 / Q2 暂不引入 kernel / Q3 运行时字符串保留 / Q4 alias 写入 t-registry / Q5 v1.0+v1.1 合并 / Q6 D3→D5 metric + 复用 EngineEvent / Q7 D6 3 probe）**无修改、无撤回**。本 R2 不再讨论语义层。

R2 关注的层级是**结构层**：在 R1 消歧之后，5+1 S 切法 + Legacy 双轨 + Bridge 跨域归位 + 三段终态 是否形成**稳定均衡**。

---

## 2. 5 个结构层命题（请 Owner 接受或反驳）

### 命题 A：S3 ProtectCall 合并 Breaker+Retry 后，错误归因的"承诺装置"是否可被消费者验证

**现象**

R1 D1 把 D3-S3（Breaker）+ D3-S4（Retry）合并为新 D3-S3（ProtectCall），理由是承诺 C3「Provider 故障不阻塞我」是同一承诺的两个机制。合并后 F 层编排：

```
D3-S3-A01 ShieldAndRetry
  F01 AllowCircuit          (Breaker.Allow)
  F02 RecordOutcome         (RecordSuccess / RecordFailure)
  F03 ManageCircuitState    (Closed/Open/HalfOpen)
  F04 ComputeBackoff        (Full Jitter)
  F05 StreamWithFallback    (Retry.Executor.Stream)
  F06 ShouldRecordBreakerFailure (Cancel/Deadline 不触发)
```

当前 t-registry 中跨 S 的 11 条 P0：
- D3-S3（旧 Breaker）4 条 P0：T01 Closed / T02 Open / T03 HalfOpen→Closed / T04 HalfOpen→Open
- D3-S4（旧 Retry）1 条 P0：T01 Full Jitter 退避
- D3-S2（旧 Gateway）2 条 P0：T04 Retry+CB 联动 / T05 Half-Open 并发探测限制

合并后 P0 T 重排：
- 新 D3-S3-A01-T01..T04：旧 Breaker T01..T04（不动）
- 新 D3-S3-A01-T05..T08：旧 Retry T01..T04（重编号）
- 新 D3-S3-A01-T09..T10：旧 Gateway T04/T05（CB+Retry 联动，跨机制）

**结构分析**

- 错误归因在 S 内部（F 层）仍可定位（6 个 F 各司其职）；
- 但**对消费者的承诺装置 T** 编号不再与"机制"1:1，而是与"F 编排顺序"1:1；
- 风险点：消费者读 t-registry 看到"T05 StreamWithFallback"时会想"为什么 StreamWithFallback 测试在 ProtectCall 而非 StreamChat？"——S 与 T 的对应直觉被打乱。

**建议最小修复**

- `t-registry.md` 每个 ProtectCall T 末尾加 `<!-- Mechanism: Breaker / Retry / Cross -->` 注释，让 T ID 与机制的可追溯性不丢失
- `span-registry.md` 5 个 Span 操作名（`llm.stream` / `llm.provider.route` / `llm.circuit_breaker` / `llm.retry` / `llm.adapter.stream`）保持不变（playbook 原则 3）；Span 层级仍在
- 不动 F 编排；仅在 spec / t-registry 加注释

**问 Owner**：T ID 末尾加 `<!-- Mechanism: -->` 注释是否接受？还是仅在设计文档中说明、不进 t-registry？

**Claude 自答：建议接受**（注释是承诺装置的可追溯性增强，不破坏 t-registry 现有结构；与 D7 demand.md §Q3「状态机沿用现行代码词汇」同型——保留可追溯性是 T 层不变契约的核心）。

---

### 命题 B：D3 韧性状态 emit 到 D5 的 metric 粒度选择（provider 单一维度 vs session×provider×model）

**现象**

R1 Q6 决议：D3 → D5 写 metric `llm_breaker_state{provider,state}`（Counter / Gauge 待设计）；D3 → D7 复用现有 EngineEvent，**不新增 D3→D7 直接契约**。

但当前 D3 Breaker 的 `Scope` 字段（`CircuitBreakerConfig.Scope`）默认 `provider`，意味着**所有 model 共享一个 provider 级 Breaker 状态**。

**结构分析**

- 单维度（provider）简单，但 DeepSeek-V4-Flash 故障时 DeepSeek-V4-Pro 也被熔断（粗粒度）
- 多维度（provider × model）更精细，但 SRE dashboard 维度爆炸
- D7 编排者（v1.1 接 EngineEvent）想知道的是"哪条路径可调度"——单维度足够；D6 评测（v1.1 增 3 probe）想知道的是"哪个 model 在 Breaker 状态切换"——多维度必要

**建议最小修复**

- v1.1 metric 设计为 `llm_breaker_state{provider, state}` 单维度（与 R1 Q6 一致）
- D6 probe #2「Breaker 状态切换次数」用 `llm_breaker_state` Gauge 跳变次数统计，与 SRE dashboard 同源
- v1.1 release 后**第一个 issue**评估是否升级到多维度（取决于 v1.0 实际是否出现"某 model 故障拖累同 provider 其他 model"的事件）
- 不写进 v1.0 spec；D5 spec 在 v1.1 阶段补

**问 Owner**：v1.1 metric 维度延后决策是否接受？还是希望 v1.0 就钉死"provider 单一维度"以避免 v1.1 重新设计？

**Claude 自答：建议延后**（与 D7 R2 命题 D 同型——advisory 超时 = pass 最初钉死 50ms 反而成为沉默同意反模式，v1.0 收尾时已修订为 metric 进 t-registry + 告警阈值）；D3 Breaker 粒度也是同类问题，先用 v1.0 单维度跑数据、v1.1 凭数据决定升级）。

---

### 命题 C：v1.0 + v1.1 合并发布的回归矩阵是否可控

**现象**

R1 Q5 决议：v1.0 + v1.1 合并发布；v2.0 物理迁移作为下一个 release。理由是避免「注册表已价值流化但代码目录仍叫 adapter/」的中间态。

合并发布意味着 **v1.0 的 5+1 S 切法 + Legacy alias + Bridge 跨域归位** 与 **v1.1 的韧性状态 emit + D6 3 probe** 一次性合入。

**结构分析**

- v1.0 工作量：4 注册表重排 + 26 条 T alias + layering.md + code-layout.md §4 + cross-domain-boundaries.md（**纯文档**）
- v1.1 工作量：D3 → D5 metric 新增 + D6 3 probe spec 补丁（**少量代码**）
- 回归矩阵：v1.0 跑 11 P0 + 26 全量；v1.1 跑 26 + 3 D6 probe（≈ 29）
- 风险点：v1.1 的 metric 名（`llm_breaker_state`）与 v1.0 注册表同时合入，错位时回滚复杂

**建议最小修复**

- v1.1 metric 与 D6 probe 写为 **feature flag** `d3_resilience_emit_enabled`（默认 false）
- v1.0 收尾时先合入 5+1 S 切法（纯文档）+ Bridge 跨域归位（少量注释）+ alias 表
- v1.1 release 时**单独**翻 `d3_resilience_emit_enabled = true`，回归 P0 + 3 D6 probe
- 即使 v1.0 + v1.1 合并发布，**激活**分两步走，避免 metric 名错位

**问 Owner**：`d3_resilience_emit_enabled` feature flag 是否接受？或者 v1.0 就把 metric 全部开（不退路）？

**Claude 自答：建议 feature flag**（与 D7 R2 OQ-2 同型——`d7_enabled` 默认 false、对内 dogfood 先翻、对外分阶段；D3 metric 也应先 dogfood 再外放）。

---

### 命题 D：S1 RouteModel 的 Tier 解析时机在「Resolve 阶段」还是「Send 阶段」

**现象**

R1 D1 决议：旧 D3-S2-A01-F02 ResolveTier（tier 别名 → 实际 model）挂到新 D3-S1-A01-F02（RouteModel 域内）。

当前实现（`gateway/router.go:Router.Resolve`）流程：

```go
func (r *Router) Resolve(model string) (provider, resolvedModel, error) {
    // 1. Tier alias 解析（fast → MiniMax-M2.7-highspeed）
    resolved := r.resolveTier(model)
    // 2. Provider routing（deepseek-* → deepseek）
    provider := r.matchRouting(resolved)
    return provider, resolved, nil
}
```

**结构分析**

- Tier 解析在 Resolve 阶段，**早于** Breaker.Allow 与 Adapter 选定
- 优点：Breaker state 按 (provider, resolvedModel) 存储准确
- 缺点：tier 解析失败时（未知 tier 透传），routing 阶段才报错；错误归因是 "Tier 解析失败" 还是 "Provider 路由失败" 模糊
- V2.1 已有 Scenario「Unknown tier passes through」验证透传行为（`spec.md §Model Tier Resolution`）

**建议最小修复**

- D3-S1-A01-F02 拆为 F02a ResolveTierAlias + F02b ResolveDefault，T ID 末尾加注释 `<!-- Tier / Default -->` 区分
- 当前 Scenario「Unknown tier passes through」挂在 F02a；新 Scenario「Empty model defaults to DefaultProvider」挂在 F02b
- 错误归因：`ErrUnknownTier`（F02a 抛）vs `ErrNoRoute`（F02b 抛）vs `ErrUnsupportedModel`（F02b 抛）—— 三者签名不同
- 不改运行时行为（透传 + 默认回填）；仅细化 F 边界

**问 Owner**：F02 拆分为 F02a/F02b 是否接受？还是保持单一 F02、错误码区分即可？

**Claude 自答：建议拆分**（playbook 原则 6「F 是可被 A 编排的最小业务/技术逻辑单元」——Tier alias 解析与 Empty model 默认是两种不同的最小单元，合并违反"高内聚低耦合"）。

---

### 命题 E：D3-S5 GuardContent 与 D2-S18 PermissionMode 长期边界

**现象**

R1 D3 决议 Safety 留 D3（D3-S7 → D3-S5 GuardContent），理由是 Safety 当前在「流式调用前最后一道门」位置（`Filter.Check` → `IGateway.Stream`），属 D3 边界；D2-S18 PermissionMode 语义是「允许哪些 tool 调用」，与「过滤哪些 prompt 内容」不同。

`cross-domain-boundaries.md`（v1.0 阶段产出）声明：

| 边界 | D3 SoT | 邻域 SoT |
|------|--------|---------|
| Prompt 内容过滤 | **D3-S5 GuardContent** | D5 audit log；D6 Safety 评测 |
| 工具执行权限 | D2-S18 PermissionMode | D3 不参与 |

**结构分析**

- 当前边界清晰，但**长期**可能出现两类能力需要 D2-S18 与 D3-S5 协作：
  - **场景 X**：用户 prompt 包含「用 curl 调用内部 API 拿 token」——这既是 prompt 内容（应 D3 过滤）也是 tool execution policy（应 D2 限制）
  - **场景 Y**：用户 prompt 包含「用 read_file 读 ~/.ssh/id_rsa」——同理
- 长期边界将出现"灰区"：内容是 D3 决策、执行是 D2 决策，但**实际拒绝**只能发生一次（要么 D3 拒、要么 D2 拒）
- V2.1 已 IMPLEMENTED 3 条 T（critical reject / medium warn / custom patterns）；未涉及场景 X/Y

**建议最小修复**

- v1.0 不动 Safety 实现（`filter.go` + `patterns.go` 行为不变）
- `cross-domain-boundaries.md` 加一条「**灰区声明**」：当 prompt 内容与 tool execution 存在交叉时，D3 优先拒（前置过滤）；D2-S18 仍保留"tool schema 不暴露"作为兜底
- D6 评测 v1.1 probe #3（Token 预算触发率）扩展为「**Safety 拒绝率**」probe（v1.1 第三个 issue 评估）
- 不写进 v1.0 spec；`cross-domain-boundaries.md` 段落加灰区声明即可

**问 Owner**：`cross-domain-boundaries.md` 加「灰区声明 + D3 优先拒」是否接受？还是 v1.0 暂不引入灰区讨论、留 v1.1？

**Claude 自答：建议接受灰区声明**（与 D7 R2 命题 E 同型——HandleInterrupt 顺序写进契约才稳定；D3/D2 灰区同样需要契约化；不写进契约的边界会被运行时具体决策模糊化）。

---

## 3. OQ-1~4 最终决议

| OQ | 问题 | Claude 建议 | 最终决议 | 备注 |
|----|------|-------------|----------|------|
| **OQ-1** | T ID 末尾加 `<!-- Mechanism: -->` 注释（命题 A） | 接受 | **接受**（与 spec.md §8 T 设计模板一致） | t-registry.md v3.0.0 写入 |
| **OQ-2** | v1.1 metric 维度延后决策（命题 B） | 延后 | **接受延后**（v1.0 跑数据，v1.1 凭数据决定） | d5-observability spec 不动 |
| **OQ-3** | `d3_resilience_emit_enabled` feature flag（命题 C） | 接受 | **接受 feature flag**（与 D7 `d7_enabled` 风格一致） | config.yaml 加默认 false 段 |
| **OQ-4** | D3-S1-A01-F02 拆 F02a/F02b（命题 D） | 接受 | **接受拆分**（playbook 原则 6） | a-registry.md + f-registry.md v3.0.0 重排 |

---

## 4. 保留分歧的 3 项（Owner 裁决）

### 4.1 灰区声明是否进 v1.0 spec（命题 E 延伸）

**Claude 建议**：`cross-domain-boundaries.md` v1.0 加灰区段；spec.md v3.0.0 不动。

**Owner 裁决**：[待 R3 评审]

### 4.2 D3-S5 GuardContent 的 Pattern 默认值是否进入 v1.0 重审

**Claude 建议**：v1.0 不重审 Pattern（V2.1 IMPLEMENTED）；v1.1 第一个 issue 评估「是否应加 D6 自动 pattern 挖掘」。

**Owner 裁决**：[待 R3 评审]

### 4.3 v2.0 物理迁移时 `contracts.go` 拆分粒度

**Claude 建议**：`Request` / `Chunk` / `TokenUsage` 留根包（kernel 性质）；`CircuitState` / `CircuitBreakerConfig` 移入 `protect/` 子包；`RetryConfig` 移入 `protect/` 子包；`ToolSchema` / `ToolCall` 留根包（与 D2 types.Message 共享）。

**Owner 裁决**：[待 R3 评审]

---

## 5. v1.0 收尾的硬要求（按风险排序）

### P0（不达不收尾）

| # | 硬要求 | 状态 |
|---|--------|------|
| 1 | OQ-1~4 定稿（本文档 §3） | ✅ |
| 2 | `t-registry.md` v3.0.0 写入 `<!-- Mechanism: -->` 注释 + 26 条 Legacy alias | ⬜ Phase B |
| 3 | `a-registry.md` / `f-registry.md` v3.0.0 重排（含 F02a/F02b 拆分） | ⬜ Phase B |
| 4 | `code-layout.md §4` 补 D3 scenario-slug 注册表 | ⬜ Phase B |
| 5 | `cross-domain-boundaries.md` v1.0.0 新建（含 D3-S5 灰区声明） | ⬜ Phase B |
| 6 | `demand-archive-index.md` 末尾追加 D3 入口 | ⬜ Phase C |
| 7 | 11 P0 T + 26 全量 T 保持 IMPLEMENTED 全绿 | ⬜ Phase C |

### P1（v1.0 release 后第一个 issue）

| # | 议题 |
|---|------|
| 8 | D6 3 probe 接入（命题 B 衍生，v1.1 实施） |
| 9 | D3 Breaker 粒度升级评估（命题 B） |
| 10 | D3-S5 Pattern 自动挖掘评估（§4.2 保留分歧） |
| 11 | `d3_resilience_emit_enabled` feature flag 默认值与 dogfood 计划 |

### P2（v1.1 路线图输入）

| # | 议题 |
|---|------|
| 12 | v2.0 物理迁移（`route/` `stream/` `protect/` `budget/` `guard/` `configure/`） |
| 13 | D2-S4 Token vs D3-S4 BudgetTokens 合并评估（属 D2 change 范畴） |
| 14 | D3 Adapter 协议扩展性（V3 计划：Anthropic） |

---

## 6. 接力接口（R3 闭合）

| # | 命题 / OQ / 分歧 | Claude 建议 | R3 决议 | 闭合位置 |
|---|------------------|------------|---------|---------|
| 1 | 命题 A（T ID `<!-- Mechanism: -->` 注释） | 接受 | **[ACCEPTED]** | R3 §6.6 + R3 P0 #2 |
| 2 | 命题 B（v1.1 metric 维度延后） | 延后决策 | **[ACCEPTED]** | R3 §6.6 + R3 P1 #10 |
| 3 | 命题 C（feature flag） | 接受 | **[ACCEPTED]** | R3 §6.6 + R3 P1 #13 |
| 4 | 命题 D（F02 拆分） | 接受 | **[ACCEPTED]** | R3 §6.6 + R3 P0 #3 |
| 5 | 命题 E（灰区声明） | 接受 | **[ACCEPTED]** | R3 §6.6 + R3 P0 #5 |
| 6 | OQ-1 | 接受 | **[ACCEPTED]** | R3 §6.6 + R3 P0 #2 |
| 7 | OQ-2 | 接受 | **[ACCEPTED]** | R3 §6.6 + R3 P1 #10 |
| 8 | OQ-3 | 接受 | **[ACCEPTED]** | R3 §6.6 + R3 P1 #13 |
| 9 | OQ-4 | 接受 | **[ACCEPTED]** | R3 §6.6 + R3 P0 #3 |
| 10 | 分歧 4.1（灰区进 v1.0） | 接受 | **[ACCEPTED]** | R3 §6.6 + R3 P0 #5 |
| 11 | 分歧 4.2（Pattern 自动挖掘） | v1.1 第一个 issue | **[ACCEPTED]** | R3 §6.6 + R3 P1 #12 |
| 12 | 分歧 4.3（contracts.go 拆分粒度） | 见 §4.3 | **[ACCEPTED: v2.0 Phase F 决策]** | R3 §6.6 + R3 P2 #25 (NQ-6) |

**闭合状态**：R2 §6 全部 12 项已由 R3 闭合；S3-Gate 全部接力接口就位。

---

**维护**：R2 接力接口由 R3 闭合（2026-06-14）；本 R2 状态推进到 `S3-Gate — R2 FINALIZED`。下一步：进入 S3 阶段产物（spec.md v3.0.0 / design.md v3.0.0 / 4 注册表 v3.0.0），按 tasks.md Phase B 启动；R3 增补的 P0 #8（factory.go fail-fast）作为 v1.0 收尾的额外硬要求同步实施。
