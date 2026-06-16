# Tasks: D7 Loop-First 路由

**Change ID:** `devrix-d7-loop-first-routing`
**Demand ID:** `DM-20260616-002`

---

## Milestone 1: 单路径投递 + Ingress 默认 Turn（Phase 1 PR）

### Definition of Done

- [x] L5-01 / L5-03 / L5-04 / AC1 / AC3 / AC8 满足
- [x] 飞书「你好」真机验收通过（integration stub 覆盖，见 acceptance-report）
- [x] `-race` 全绿

### Tasks

- [x] **T1**: 修复 EngineEvent 单投递（FastPath 去 sink mirror、Orchestrate emit 去 Publish、Agent sink hasActiveProcess 守卫）
- [x] **T2**: 新增 `RoutingMode` 配置（`loop_first` | `rule_orchestrate`），默认 `loop_first`
- [x] **T3**: Classifier `loop_first` 模式 — 非 Skip/Command → IntentFast(100)；移除 threshold 降级
- [x] **T4**: Orchestrator 在 `loop_first` 下不进入 OrchestratePath ingress case
- [x] **T5**: Turn system prompt 注入 loop-first guidance（问候直接答、勿无故 delegate）
- [x] **T6**: 单测 + integration — greeting 无 wave 事件；command 零 LLM；单投递计数

---

## Milestone 2: Tool 门控编排（Phase 2 PR）

### Definition of Done

- [x] L5-02 / L5-05 / AC4 满足
- [x] integration stub 覆盖 delegate_wave → OrchestratePath

### Tasks

- [x] **T7**: 实现 `enter_plan_mode` handler
- [x] **T8**: 实现 `delegate_wave` handler — 内部调 OrchestratePath，事件转发到 Turn channel
- [x] **T9**: bootstrap 接线 — Turn OrchestratorDeps 注册 orchestration tools
- [x] **T10**: Tool prompt / schema
- [x] **T11**: Integration test — stub LLM 返回 delegate_wave → 断言 OrchestratePath 调用

---

## Milestone 3: 观测 + 文档（Phase 3 PR 或合入 Phase 2）

### Tasks

- [x] **T12**: Metrics — `orchestration.tool.delegate_wave` counter；route label `turn`
- [x] **T13**: 更新 `openspec/specs/d7-orchestration/spec.md` + a-registry 路由矩阵
- [x] **T14**: ShadowClassifier 对照指标（loop_first 下 tail shadow 仍运行，不改变路由）
- [x] **T15**: 验收报告 + 飞书真机 L5-01 证据

---

## Completion Checklist

- [x] 全部 P0 L5 通过
- [x] `routing_mode` documented
- [x] 领域文档同步（S5 → specs/）
- [x] 准备 S7 归档
