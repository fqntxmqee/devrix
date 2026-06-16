# Proposal: D7 Loop-First 路由（Clawcode 对齐）

**Change ID:** `devrix-d7-loop-first-routing`
**Demand ID:** `DM-20260616-002`
**Status:** Draft
**Created:** 2026-06-16

---

## Problem Statement

Devrix D7 在 **ingress** 用规则引擎决定「简单 → FastPath / 复杂 → OrchestratePath」，与 Clawcode「单 Query Loop + Tool 门控」.harness 哲学相反。该设计在 IM 场景已造成：

1. **误路由**：CJK 问候语触发 Wave（confidence 阈值 + regex 边界）
2. **过度编排**：短消息被 TaskDecomposer 拆图，SubAgent 发出 `"started"` 等中间态
3. **重复投递**：正交路径 + EventPublisher sink 叠加，同一 EngineEvent 多次到达 D1

Clawcode 证明：复杂度判断应发生在 **loop 内**（LLM 读 tool 描述后决定是否 Plan / Agent / Fork），而非 **ingress 规则**。

## Proposed Solution

### 方案 A：Loop-First（推荐）

**Ingress 三路：**

```
Skip → 空 channel
Command → CommandHandler（零 LLM，保持）
Default → TurnOrchestrator.RunTurn（所有非命令消息）
```

**Loop 内 Tool 门控（对齐 Clawcode s03/s04）：**

| Tool | Clawcode 对标 | Devrix 实现 |
|---|---|---|
| `enter_plan_mode` | EnterPlanModeTool | `workmodel.PlanMode.Enter` |
| `delegate_wave` | AgentTool + Wave 语义 | 内部调 `OrchestratePath.Run` |
| `delegate_agent` | AgentTool / fork | 现有 D4 delegate（Turn tool surface 暴露） |

**投递契约：**

- Turn 返回的 channel = **唯一** IM 主路径
- `emit()` / `FastPath` **禁止** mirror 到 `EventPublisher`
- `PublishEngineEvent` 仅用于 flow hub → worker_progress 等 out-of-band

### 方案 B：增强 RuleClassifier（不推荐）

修 CJK regex + 降 threshold + 加更多 fast pattern。

**否决理由：** 不解决语义误判；每加一种语言/场景需维护规则；与 Clawcode 方向相反。

### 方案 C：LLM Ingress Classifier（defer v1.2）

用 LLM 在 ingress 判 IntentOrchestrate。

**否决理由：** 增加延迟与成本；Clawcode 也不用 ingress LLM；Shadow 已存在可后续评估。

## Scope

### In Scope

- `coordinator.routing_mode` 配置
- Classifier/Orchestrator ingress 简化
- Turn 编排 tool 注册与 prompt
- 单路径投递修复
- L5 测试 + spec delta

### Out of Scope

- Coordinator Mode 全量移植
- 删除 Wave / Plan 能力
- Feishu UI 改版

## Impact Analysis

| Component | Change | Details |
|---|---|---|
| `coordinator/classifier.go` | Modify | Default → Turn；Orchestrate 规则 deprecated |
| `coordinator/orchestrator.go` | Modify | switch 缩为 3 路；threshold 门控 conditional |
| `coordinator/fastpath.go` | Modify | 去 sink mirror |
| `coordinator/orchestrate_path.go` | Modify | 仅 tool 调用；emit 不 sink |
| `turn/` + bootstrap | Add | 编排 tool handler |
| `capture/gateway.go` | Modify | 单投递守卫（hasActiveProcess） |
| `shared/config/coordinator.go` | Add | `routing_mode` |
| `openspec/specs/d7-orchestration/` | Modify | 路由矩阵 delta |

## Architecture Considerations

### 与现有资产的关系

| 已有资产 | Loop-First 中的角色 |
|---|---|
| `TurnOrchestrator` | **主路径**（= Clawcode QueryEngine） |
| `FastPath` | 薄包装 Turn；可保留命名，语义变为 MainLoop |
| `OrchestratePath` | **Library 路径**，由 `delegate_wave` tool 调用 |
| `CommandHandler` | 不变 |
| `RuleClassifier` | 缩规则面；Shadow 观测保留 |
| `WaveScheduler` | 不变，触发方式变 |

### 配置

```yaml
coordinator:
  routing_mode: loop_first   # default; legacy: rule_orchestrate
  fast_path_threshold: 90    # 仅 rule_orchestrate 生效
  command_first: true
```

## Success Criteria

- [ ] AC1–AC9（见 demand.md §7）全部满足
- [ ] 飞书「你好」真机验收：单回复、无 wave 日志
- [ ] P0 L5 测试 CI 全绿

## Risks & Mitigations

| Risk | Prob | Impact | Mitigation |
|---|---|---|---|
| LLM 不调用 delegate_wave | Med | Med | Tool prompt + integration stub + D6 probe |
| 双投递回归 | Med | High | L5-04 门禁；禁止 emit sink |
| 004 用户依赖 auto-orchestrate | Low | Med | `rule_orchestrate` 回滚 |
| PR 超 400 行 | Med | Low | Phase 1/2 拆分 |
