# D3 LLM Gateway Domain

**Domain ID:** D3
**Slug:** `llmgateway`
**Type:** Common Domain
**Status:** Active — Canonical S1–S6（5+1 价值流承诺）
**Version:** 1.0.0
**Last Updated:** 2026-06-16
**Depends On:** D5 (Observability — emit hook)
**Depended By:** **D7（主消费方，DM-020）**；D1/D4/D6（间接）
**Hard Ban:** D2→D3 import（DM-020）；D2 不得直调 Gateway
**Cross-Domain SoT:** `../d7-orchestration/d3-boundary.md`

---

## North Star

**向所有消费者提供可独立验证的 LLM 横向能力：路由、流式调用、韧性保护、预算控制、内容守卫——由 D7 拥有调用决策权，D3 仅执行 Gateway 契约。**

| 可验证承诺 | Canonical S |
|-----------|-------------|
| C1 模型路由（tier alias → provider + model） | D3-S1 RouteModel |
| C2 流式 SSE chunk 流 | D3-S2 StreamChat |
| C3 Provider 故障不阻塞用户 | D3-S3 ProtectCall |
| C4 Token 超预算截断/报错 | D3-S4 BudgetTokens |
| C5 危险 prompt 拒绝/告警 | D3-S5 GuardContent |
| 配置加载与启动 fail-fast | D3-S6 ConfigureGateway |

---

## Out of Scope

| 能力 | 归属 | 备注 |
|------|------|------|
| Turn 主循环 / Invoke 编排 | D7-S2 | `InvokeLLM` 入口 |
| Session 上下文 / Prompt 组装 | D2-S15 | D2→D3 ban |
| 工具权限 / PlanMode | D2-S18 | 与 S5 内容守卫灰区见 spec §灰区 |
| IM 展示 / EngineEvent 路由 | D1 | D3 可 emit `flow.breaker.*` |
| Span/Metric 基础设施 | D5 | D3 仅 emit |
| 结论质量评测 | D6 | Tier 探针配套 |

---

## DSAFT 资产

### Canonical 价值流 — D3-S1–S6

| S ID | Scenario | 承诺 | Status |
|------|----------|------|--------|
| D3-S1 | RouteModel | C1 | IMPLEMENTED |
| D3-S2 | StreamChat | C2 | IMPLEMENTED |
| D3-S3 | ProtectCall | C3 | IMPLEMENTED |
| D3-S4 | BudgetTokens | C4 | IMPLEMENTED |
| D3-S5 | GuardContent | C5 | IMPLEMENTED |
| D3-S6 | ConfigureGateway | 横切 | IMPLEMENTED |

### 登记规模（Canonical）

| 层 | 数量 | SoT 文件 |
|----|------|----------|
| A | 6（每 S 1 A） | `a-registry.md` |
| F | 30 域内 + CROSS | `f-registry.md` |
| T | 35（19 P0，34 IMPLEMENTED） | `t-registry.md` |
| Span | 5 ops（运行时名稳定） | `span-registry.md` |

---

## 与 D7 关系（Caller / Gateway）

> 完整矩阵见 [`../d7-orchestration/d3-boundary.md`](../d7-orchestration/d3-boundary.md)。

| D7 动作 | D3 响应 |
|---------|---------|
| `InvokeLLM` / StreamChat | S1 Route → S2 Stream → S3 Protect |
| FastPath 单轮 | 同上，无 D2 参与 |
| Breaker 状态展示 | S3 emit `flow.breaker.*` → D1 |

**禁止：** D2 import `llmgateway`（CI `d2_d3_ban_test`）。

---

## 规格文档索引

| 文档 | 用途 |
|------|------|
| `spec.md` | Gherkin 验收规格（厚 SoT） |
| `terminal-state-guide.md` | D7→D3 调用链、S 编排、Trace 摘要 |
| `observability-guide.md` | Span↔T、Trace 树、P0 Runbook |
| `design.md` | 六段式详细设计 |
| `dsaft-architecture.md` | Stub — DSAFT 五层计数 |
| `a-registry.md` / `f-registry.md` / `t-registry.md` | A/F/T 登记 SoT |
| `span-registry.md` | Span operation 登记 SoT |
| `layer-delta.md` | V2→V3 演进 Delta |

---

## 修订记录

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-16 | 初版：薄领域 SoT；厚规格保留 `spec.md`；对齐 D1/D7 `*-domain.md` 模式 |
