# D3 LLM Gateway Domain Specification

**Capability:** llm-gateway
**Status:** Active
**Version:** 3.2.0 (V1→V2→V3 S/A 重切 → V3.1 韧性可见性 + 评测探针 + 适配扩展 → V3.2 跨域归位)
**Last Updated:** 2026-06-30 (DM-20260629-003 d3-dsaft-restructuring v3.2.0 S7_Archived)
**Domain SoT:** `d3-domain.md` v1.6.0 — North Star + 5 承诺 + DSAFT 资产 + 边界 SoT
**Guides:** [dsaf-architecture.md](dsaf-architecture.md) / [observability-guide.md](observability-guide.md) / [terminal-state-guide.md](terminal-state-guide.md) / [model-resolution-trace.md](model-resolution-trace.md)

> **精简设计契约（Lite-Mode）**：本文档只放当前符合代码的设计契约（v3.2.0）。**过程需求迭代**（90 个 Gherkin Scenario 详细文本：happy 30 / sad 24 / boundary 18 / concurrent 9 / timeout 9）不进入本文件，留在 `archive/<change-id>/specs/` 各 change 归档目录。详细时间线见 [CHANGELOG.md](CHANGELOG.md)。

---

## Overview

D3 LLM 网关是 **Common Domain**（公共域），向所有消费者域（D1/D2/D4/D5/D6/D7）提供 **5+1 价值流承诺**（5 承诺装置 + 1 横切 Config）：模型路由（S1）/ 流式调用（S2）/ 韧性保护（S3）/ 预算控制（S4）/ 内容守卫（S5）/ 配置加载（S6 横切 + 启动 fail-fast）。D7 是主消费方（DM-020 Turn 编排），D3 仅执行 Gateway 契约；**D2→D3 任何 import / 调用硬阻断**（`lint-d1-imports.sh` 守门 + `d2_d3_ban_test`）。

| 承诺 | Canonical S | ValueFlow Alias | 验证入口 |
|------|-------------|-----------------|----------|
| C1 模型路由 | D3-S1 RouteModel | `D3_Model_Routing` | `D3-S1-A01-T01/T02` |
| C2 流式调用 | D3-S2 StreamChat | `D3_Stream_Chat_Completion` | `D3-S2-A01-T01~T05` |
| C3 韧性保护 | D3-S3 ProtectCall | `D3_Circuit_Breaker_And_Retry` | `D3-S3-A01-T01~T12` |
| C4 预算控制 | D3-S4 BudgetTokens | `D3_Token_Budget_Control` | `D3-S4-A01-T01~T03` |
| C5 内容守卫 | D3-S5 GuardContent | `D3_Content_Safety_Filter` | `D3-S5-A01-T01/T02` |
| 横切：配置加载与启动 fail-fast | D3-S6 ConfigureGateway | `D3_Gateway_Configuration` | `D3-S6-A01-T01` |

**现行实现路径（v3.2.0）**：`D3.InvokeLLM` 经 D7 入口调用；`Adapter` 实现 `IAdapter.Protocol() string` 接口（V3.1 BREAKING 扩展）；Breaker + Retry + Fallback 合并到 D3-S3 ProtectCall；5 个运行时 span op 稳定（R1 Q3 决议，violation 触发再审计）。

### 核心设计原则

1. **承诺装置哲学**（R1 D1）：5+1 S 各自独立验证、独立替换；S 内部 F 编排可调，对外承诺稳定
2. **D7 拥有调用决策权，D3 仅执行 Gateway 契约**：Caller = D7，Gateway = D3；D3 不拥有 Turn 主循环
3. **D2→D3 import 硬阻断**（DM-020）：D2 不得直调 Gateway；D2 上下文准备单独走 S15 Prepare → D3-S2；CI `d2_d3_ban_test` + `lint-d1-imports.sh` 守门
4. **Tier 解析二阶段**（R2 OQ-4 决议）：F02a ResolveTierAlias（alias → tier）+ F02b ResolveDefault（fallback）
5. **D3-S5 GuardContent vs D2-S18 PermissionMode 灰区**（R2 命题 E）：D3 前置过滤拒（critical），D2 兜底（tool execution 权限）
6. **Breaker + Retry + Fallback 合并到 D3-S3 ProtectCall**（R1 D1 决议）；T 末尾加 `<!-- Mechanism: -->` 注释标注机制（R2 命题 A）
7. **启动 fail-fast**（R3 P0 #8）：`NewFromConfig` obs == nil → `ErrObservabilityRequired`（不 silent fallback）
8. **运行时 span 名保持不变**（R1 Q3 决议 + playbook 原则 3）：5 个 active span op 字面量稳定，violation 触发 v4.0 重新审计

### S 层职责（canonical 6 + 1 CROSS）

| S ID | Scenario | 承诺 | ValueFlow Alias | Status |
|------|----------|------|-----------------|--------|
| D3-S1 | RouteModel | C1 模型路由（tier alias → provider + model） | `D3_Model_Routing` | **REGISTRY** |
| D3-S2 | StreamChat | C2 OpenAI SSE 协议 chunk 流 | `D3_Stream_Chat_Completion` | **REGISTRY** |
| D3-S3 | ProtectCall | C3 Provider 故障不阻塞（Breaker + Retry + Fallback 合并） | `D3_Circuit_Breaker_And_Retry` | **REGISTRY** |
| D3-S4 | BudgetTokens | C4 Token 超预算截断/报错 | `D3_Token_Budget_Control` | **REGISTRY** |
| D3-S5 | GuardContent | C5 危险 prompt 拒绝（critical）/ 告警（warning） | `D3_Content_Safety_Filter` | **REGISTRY** |
| D3-S6 | ConfigureGateway | 横切：配置加载与启动 fail-fast | `D3_Gateway_Configuration` | **REGISTRY** |
| D3-X | CROSS | 跨域锚点（归属 `internal/bridges/llm/`，spec 仅占位） | — | **CROSS** |

---

## DSAFT 结构

| 层级 | ID | 名称 | 物理路径 / SoT |
|------|----|------|----------------|
| D | D3 | LLM Gateway | `internal/layers/llmgateway/` |
| S | D3-S1..S6 | 6 个 canonical（5+1 价值流） | 见 d3-domain.md |
| S (跨域) | D3-X | CROSS Bridge / Bootstrap | `internal/bridges/llm/`（归属跨域锚点，R1 D2 决议） |
| A | A1-A6 | 6 个核心活动（每 S 1 A） | 见 `a-registry.md` |
| F | F1-F999 | 30 域内 + CROSS F | 见 `f-registry.md` |
| T | T1-T200 | 35 个测试点（19 P0，34 IMPLEMENTED） | 见 `t-registry.md` |
| Span | — | 5 个运行时 span op（稳定字面量） | 见 `span-registry.md` |

**当前计数（v3.2.0）**：D=1, S=6 (canonical: S1-S6) + S=1 (D3-X CROSS 跨域), A=6, F=30 (域内) + CROSS, T=35 (19 P0, 34 IMPLEMENTED), Span=5 ops。

---

## Scenarios

| ID | Scenario | Responsibility | Status | 验证入口 |
|----|----------|----------------|--------|----------|
| D3-S1 | RouteModel | tier alias 解析 + provider/model 选路 + ResolveTierAlias + ResolveDefault | **REGISTRY** | `D3-S1-A01-T01/T02` |
| D3-S2 | StreamChat | OpenAI SSE chunk 流生成 + adapter 协议扩展 `Protocol() string` | **REGISTRY** | `D3-S2-A01-T01~T05` |
| D3-S3 | ProtectCall | Breaker (5 态 state machine) + Retry (exponential backoff) + Fallback + emit `flow.breaker.*` | **REGISTRY** | `D3-S3-A01-T01~T12` |
| D3-S4 | BudgetTokens | Token 超预算截断/报错 + span event `budget.check.exceeded` | **REGISTRY** | `D3-S4-A01-T01~T03` |
| D3-S5 | GuardContent | safety.Reject (critical) + warn (warning) + EmitSafetyLatencyEvent | **REGISTRY** | `D3-S5-A01-T01/T02` |
| D3-S6 | ConfigureGateway | `NewFromConfig` obs nil → `ErrObservabilityRequired` fail-fast | **REGISTRY** | `D3-S6-A01-T01` |
| D3-X | CROSS Bridge | 归属 `internal/bridges/llm/`，spec 仅占位声明 | **CROSS** | — |

---

## Architecture

```
D7 Orchestration (Caller, 主消费 DM-020)
    └→ D3 InvokeLLM (composition root)
        ├→ D3-S1 RouteModel ─ tier alias → provider/model + Tier Alias F02a + Default F02b
        ├→ D3-S2 StreamChat ─ OpenAI SSE chunk 流 + IAdapter.Protocol() string (V3.1 BREAKING)
        ├→ D3-S3 ProtectCall ─ Breaker (5 态 state machine) + Retry (exp backoff) + Fallback
        ├→ D3-S4 BudgetTokens ─ 超预算截断/报错 + span event budget.check.exceeded
        ├→ D3-S5 GuardContent ─ safety.Reject (critical) / warn (warning)
        ├→ D3-S6 ConfigureGateway ─ NewFromConfig + 启动 fail-fast (R3 P0 #8)
        └→ D3-X CROSS Bridge ─ internal/bridges/llm/ (跨域锚点归属, R1 D2 决议)

D5 Observability ← emit hook (D3 → D5: span + metric)
D6 Evolution ← 探针接入 (Tier resolve probe)
D7 → D3 emit flow.breaker.* → D1 适配器展示 EngineEvent
```

### 域边界

| D3 拥有 | D3 调用（不拥有） | D3 不拥有 |
|---------|------------------|----------|
| Provider model 路由解析 | D7 InvokeLLM 编排 | Turn 主循环（D7-S2） |
| OpenAI SSE chunk 流生成 | — | Session 上下文（D2-S15） |
| Breaker / Retry / Fallback / CircuitOpenError | — | 工具权限（D2-S18） |
| Token 预算执行 + safety 前置过滤 | — | IM 展示（D1） |
| Adapter 协议扩展 `Protocol() string` | — | Span / Metric 基础设施（D5） |
| 启动 fail-fast 验证 | — | 结论质量评测（D6） |

**Hard Ban**：
- **D2→D3 import 硬阻断**（DM-020）；CI `d2_d3_ban_test` + `lint-d1-imports.sh` 守门
- **D2 直调 Gateway 退役**：`routeLegacyD2` 等绕过路径已 RETIRED

**Boundary Debt**（4 项 RESOLVED，治理常量 in `orchtypes/boundary_decision.go`）：
- `BoundaryD2D3ImportBan` — D2→D3 import 硬阻断（v1.0）
- `BoundaryD3S5VsD2S18Grayzone` — D3-S5 GuardContent vs D2-S18 PermissionMode 灰区（v3.0 R2 命题 E）
- `BoundaryD3S4BudgetSpanInjection` — D3-S4 BudgetTokens → D3_LLM_Stream 注入模式（v3.2.0 R1 Q3）
- `BoundaryD3S6FailFastOnObsNil` — D3-S6 启动期 → D5 Observability fail-fast（v1.1 R3 P0 #8）

---

## 关键 Scenario 范式

> **1 个 canonical Gherkin 示例**。完整 90 个 Scenario 分布在 `archive/<change>/specs/` 各 change 目录。

### 范式：D3-S3 ProtectCall Breaker Open 路径（DSAFT S3-A01）

#### Scenario: Provider 5xx 触发 Breaker Open 后降级 Fallback

- **GIVEN** D3-S3 ProtectCall 配置 provider retry=3 + breaker threshold=5
- **WHEN** provider 返回连续 5 次 5xx 错误
- **THEN** Breaker State Machine 转 **Open** 态
- **AND** emit `flow.breaker.open` EngineEvent（`llm_breaker_state` metric = 1）
- **AND** 后续请求走 **Fallback**（不阻塞用户）
- **AND** span event `safety.check.duration_ms` 仍 emit（D3-S5 兜底）

---

## 关键链路口

1. **主链**：D7-S2 InvokeLLM → D3 InvokeLLM 编排 → S1 Route → S2 Stream → S3 Protect → S5 Safety → S4 Budget
2. **FastPath 单轮**：跳过 D2 context 准备，D7 直调 D3-S2 StreamChat
3. **Breaker 状态链**：D3-S3 emit `flow.breaker.*` → D1 适配器展示（EngineEvent 消费方）
4. **Tier 解析链**：F02a ResolveTierAlias（alias → tier）+ F02b ResolveDefault（fallback）
5. **内容守卫灰区链**：D3-S5 GuardContent（前置过滤 critical 拒）+ D2-S18 PermissionMode（tool execution 兜底）
6. **Hard Ban 链**：D2→D3 import = 0（`d2_d3_ban_test` + `lint-d1-imports.sh` CI 守门）；D2 直调 Gateway 0（D2 不得拥有 D3 Gateway）
