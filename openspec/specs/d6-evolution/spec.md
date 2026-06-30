# D6 Evolution Domain Specification

**Domain:** D6 Evolution
**DSAFT Type:** Supporting
**Version:** 2.5.0
**Last Updated:** 2026-06-30 (DM-20260630-009 d6-spec-lite v2.5.0 S7_Archived)
**Domain SoT:** `d6-domain.md` v1.0.0 — North Star + 3 子系统 + DSAFT 资产 + 边界 SoT

> **精简设计契约（Lite-Mode）**：本文档只放当前符合代码的设计契约（v2.5.0）。**过程需求迭代**（如 d6-sa-refine / d6-evolution-review-fixes 18 条 Requirements 详细 Gherkin）不进入本文件，留在 `archive/<change>/specs/` 各 change 归档目录。详细时间线见 [CHANGELOG.md](CHANGELOG.md)。

---

## Overview

D6 演化域负责 Devrix 系统的自我评估与运行时行为校验。包含三大子系统：**D6-S3 评测引擎**（离线评测管道，10 类探针 v2.2.0）+ **D6-S4 GuardRuntime**（运行时校验智能体路由决策，v2.0 重命名自 Orchestration）+ **D6-S5 VerifyInvariant**（v2.0 物理独立，Invariant fail-closed）。D6-S1（版本检测）/ D6-S2（配置热更新）PLANNED。

| 承诺 | Canonical S | ValueFlow Alias | 验证入口 |
|------|-------------|-----------------|----------|
| 10 类探针离线评测 + Delta 回归 + CI 门禁 | D6-S3 EvalEngine | `D6_Eval_Engine` | `D6-S3-A01-T01~T22` |
| 智能体决策校验 + 跨模型 Judge + 干预 | D6-S4 GuardRuntime | `D6_Guard_Runtime` | `D6-S4-A01~A04-T01` |
| 系统级不变量 fail-closed + Plan 验证 | D6-S5 VerifyInvariant | `D6_Verify_Invariant` | `D6-S5-A01-T01` |
| Invariant 启动期 fail-safe (v2.4.0) | D6-S11 VerifyResilience | `D6_Verify_Resilience` | `D6-S11-A02-T09` |
| Guard 命名空间收敛 + 6 metric 重命名 (v2.4.0) | D6-S12 GuardNamespace | `D6_Guard_Namespace` | `D6-S12-A01~A03-T01~T05` |

### 核心设计原则

1. **LLM-as-Judge 双模型交叉验证**：primary + secondary LLMClient，位置随机化（forward/reversed averaging），Cohen's kappa 校准
2. **Delta 回归 3 档分级**：Red（严重）<-5% / Yellow（轻微）<-2% / Green（正常）≥-2%（R1 D6-S3 P0）
3. **CI Delta 门禁 fail-closed**：`CheckDeltaGate` 检测回归返回 `GateResult{Passed:false}` + 非零退出码（DM-20260621-011 韧性强化）
4. **GuardRuntime 跨模型决策校验**：tool_call / permit / fork 三类决策经 D3 LLM Gateway 跨模型 Judge 验证（R1 D6-S4 P0）
5. **Verify Invariant fail-closed**：启动期 `init()` → `log.Fatalf` 替代 panic（v2.4.0 韧性修复，DM-20260621-011 H-2）
6. **Bridge 清债零容忍**：`internal/layers/evolution/eval/bridge.go` + `orchestration/bridge.go` git ls-files 0 命中（v2.4.0 PR-B）
7. **Guard 命名空间收敛**：Orchestration* → Guard* type alias 保留 v2.5 删；6 个 OTel 指标 `orch_*` → `guard_*`（v2.4.0 PR-B）
8. **三联固化错误处理**：metric + slog.Warn + errors.Join 上抛，禁止 `_, _ =` 静默吞错（v2.4.0 PR-A H-3）

### S 层职责

| S ID | Scenario | 职责 | Status |
|------|----------|------|--------|
| D6-S3 | EvalEngine | 评测管道编排 + 10 类探针 + Judge 评分 + Delta 回归 + CI 门禁 | **REGISTRY** |
| D6-S4 | GuardRuntime | 决策校验 + 干预执行 + Observer 桥接 + Judge 适配 | **REGISTRY** |
| D6-S5 | VerifyInvariant | Invariant 注册校验 + Plan 路径验证 | **REGISTRY** |
| D6-S11 | VerifyResilience (v2.4.0) | Invariant 启动期 fail-safe | **REGISTRY** |
| D6-S12 | GuardNamespace (v2.4.0) | bridge 清债 + Orchestration*→Guard* 重命名 + metric 重命名 | **REGISTRY** |

**PLANNED**：D6-S1（版本检测）+ D6-S2（配置热更新）。

---

## DSAFT 结构

| 层级 | ID | 名称 | 物理路径 / SoT |
|------|----|------|----------------|
| D | D6 | Evolution | `internal/layers/evolution/` |
| S | D6-S3 | Eval Engine | `evaluate/` (engine.go + judge.go + delta.go + tune.go + dataset.go + probe.go + gateway_llm.go + mock_llm.go + 10 probe files) |
| S | D6-S4 | Guard Runtime | `guard/` (validator.go + intervention.go + observer.go + judge_adapter.go + metrics.go) |
| S | D6-S5 | Verify Invariant | `verify/` (invariant.go + plan.go, v2.4.0 由 `_invariant.go` 重命名) |
| A | A1-A20 | 20 Activities | `a-registry.md` |
| F | F1-F15 | 15 Function Points | `f-registry.md` |
| T | T1-T22 | 22 Test Points | `t-registry.md` |

**当前计数（v2.5.0）**：D=1, S=5 (canonical: S3-S5 + v2.4 韧性 S11-S12) + 2 PLANNED (S1-S2), A=20, F=15, T=22, Probe=10 (v2.2.0: 7 + 3)。

---

## Scenarios

| ID | Scenario | Responsibility | Status | 验证入口 |
|----|----------|----------------|--------|----------|
| D6-S3 | EvalEngine | LoadDataset→Sample→RunProbes(×N)→Aggregate→Delta→Baseline→CheckGate | **REGISTRY** | `D6-S3-A01-T01~T22` |
| D6-S4 | GuardRuntime | AgentDecision→GuardObserver→preFilter→Judge→Intervention | **REGISTRY** | `D6-S4-A01~A04-T01` |
| D6-S5 | VerifyInvariant | Init→Register→Check→Plan verify | **REGISTRY** | `D6-S5-A01-T01` |
| D6-S11 | VerifyResilience | parseVerifyInvariants() → log.Fatalf 替代 panic | **REGISTRY** | `D6-S11-A02-T09` |
| D6-S12 | GuardNamespace | bridge 清债 + Orchestration*→Guard* + orch_*→guard_* + Wait/tasks.Fail 三联 | **REGISTRY** | `D6-S12-A01~A03-T01~T05` |

---

## Architecture

```
                    ┌─────────────────────────────────────┐
                    │  D6 Evolution (支撑域)                │
                    │                                     │
D3 LLMGateway ──────│→ EvalEngine (S3) ─ JudgeManager     │
  Judge + Bridge    │   ├─ 10 Probes (v2.2.0: 7 + 3)      │
                    │   ├─ DeltaAnalyzer (Red/Yellow/Green)│
                    │   ├─ CI Gate (CheckDeltaGate)        │
                    │   └─ TuneGenerator                   │
                    │                                     │
D4 AgentObserver ──│→ GuardRuntime (S4)                   │
  agent.forked     │   ├─ GuardObserver → DecisionRecord  │
  agent.joined     │   ├─ preFilter (trusted_tools)       │
  permission_      │   ├─ RuntimeJudge (D3 cross-model)   │
    required       │   └─ InterventionExecutor            │
                    │       (terminate / reroute /        │
                    │        updateState)                 │
                    │                                     │
D7 PlanMode ───────│→ VerifyInvariant (S5)                │
  Plan tool call   │   ├─ InvariantRegistry               │
                    │   ├─ parseVerifyInvariants()         │
                    │   │   (init() → log.Fatalf v2.4.0) │
                    │   └─ PlanVerifier.Verify             │
                    └─────────────────────────────────────┘
```

### 域边界

| D6 拥有 | D6 调用（不拥有） | D6 不拥有 |
|---------|------------------|----------|
| 评测集 + 探针实现 + JudgeManager | D3 LLM Gateway（Judge 真实调用） | 评测数据集领域知识 |
| Delta 回归 + CI 门禁 + 调优建议 | D4 AgentObserver（决策事件源） | 业务 Span 触发点（各域） |
| 干预执行（terminate/reroute/updateState） | D5 Observability（OTel 指标） | OTel Collector 部署 |
| Invariant 注册 + 校验 + Plan 验证 | D7 PlanMode（Plan 路径源） | 业务 invariant 定义（各域） |
| v2.4.0 韧性：panic → log.Fatalf + 三联固化 | — | bridge.go（已清债 v2.4.0） |
| v2.4.0 命名空间：Guard* + guard_* 6 metric | — | Orchestration*（type alias v2.5 删） |

---

## 关键 Scenario 范式

### 范式：D6-S3 Tier Resolution Probe ≥ 99%（v2.2.0 跨 D3-D6 锚点）

#### Scenario: Tier Resolution 命中率 ≥ 99%

- **GIVEN** D3 Gateway 路由决策序列（带 `tier` 属性）+ `llm_tier_resolve_total{outcome=hit|fallback|error}` 三桶计数
- **WHEN** `TierResolutionProbe.Run` 被调用
- **THEN** 统计 `llm_tier_resolve_total{outcome=hit}` 占比
- **AND** `hit / (hit + fallback + error) ≥ 99%` → Score = 1.0
- **AND** `< 99%` → Score = hit_ratio，标记 Yellow（轻微回归）
- **AND** `error > 0` → 触发 Red（严重回归）
- **AND** 上报 D5 dashboard `d3_tier_resolution` 面板 + 桶分布校验（fallback_ratio / error_ratio）

---

## 关键链路口

1. **Eval 链**：LoadDataset → ProbeRegistry(10 类) → JudgeManager(D3 Gateway) → DeltaAnalyzer → CheckDeltaGate (CI) → TuneGenerator
2. **Guard 链**：AgentDecision → D4 AgentObserver → preFilter → RuntimeJudge (D3 cross-model) → Intervention (terminate/reroute/updateState)
3. **Verify 链**：Init → parseVerifyInvariants() (fail-closed log.Fatalf v2.4.0) → Check() → PlanVerifier.Verify (D7 PlanMode)
4. **OTel 指标链**：`guard_decisions_total` + `guard_validations_total` + `guard_interventions_total` + `guard_judge_latency_seconds` + `guard_observer_active` + `guard_decisions_by_stage`（v2.4.0 重命名 `orch_*` → `guard_*`）
5. **韧性修复链**（v2.4.0）：bridge 清债 + Orchestration*→Guard* type alias + invariant.go 激活（_invariant.go 重命名）+ Wait/tasks.Fail metric+slog+errors.Join 三联固化
6. **跨域锚点链**：D6-S3 Tier Resolution 接 D3 Gateway `llm_tier_resolve_total` / D6-S3 Breaker Anomaly 接 D3 `llm_breaker_transitions_total` / D6-S3 Safety Latency 接 D3-S5 `safety.check.duration_ms` / D6-S4 GuardObserver 接 D4 `agent.{forked,joined}` + `permission_required`

---

## 附录：总览

- **当前活跃 Requirement 数**：5 canonical（每段 1 句 + 1 canonical Gherkin，详见 archive 详细文本）
- **历史 Requirement 详细文本（18 条）**：在 `archive/<change>/specs/` 各 change 目录（详见 CHANGELOG.md）
- **PLANNED**：D6-S1 版本检测 + D6-S2 配置热更新
- **当前 spec 版本**：v2.5.0
- **下一次架构级变更触发**：D6 域升级 v3.0+ 或 GuardRuntime/VerifyInvariant 跨域契约变化时重新审计 Boundary Debt Decisions