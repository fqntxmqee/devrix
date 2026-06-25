# D7 Orchestration Spec (bootstrap-slim Delta)

**Change ID:** devrix-d7-6s-bootstrap-slim
**Status:** S3_Design
**Priority:** P2
**Created:** 2026-06-26
**DM:** DM-20260626-007

---

## §1 Background

v6.0.0 域升级 follow-up #6 (DM-20260626-007) — bootstrap 拓扑收口:

- `internal/bootstrap/wire_coordinator.go` InitOrchestration (275 行) 内部 3 个 adapter 函数 + 4 个 util 函数 + 30 行 config 加载拆出
- 6 S × WireFunc 命名一致 (新增 `WireDecisionPlanning` + `WireMUPSPipeline` 包装)
- InitOrchestration 主体降到 ≤ 200 行, 6 S 组合入口清晰

**0 行为变化** (pure refactor):
- InitOrchestration 外部接口 100% 不变
- InitOrchestration 内部 6 S 构造顺序 100% 不变
- 3 个调用方 (cmd/devrix + cmd/obs-verify + tests/testutil/d7_stack.go) 0 变化
- hardening/ + escape/circuit_breaker.go + sessionorchestrator/autoclose.go git diff 0 变化

## §2 Gherkin 验收规格

### Scenario: 6 S × WireFunc 命名一致

```gherkin
Feature: Bootstrap Wire 拓扑收口 (DM-20260626-007)

  Background:
    Given v6.0.0 域升级 6 S + 1 横切博弈角色对齐已完成 (DM-20260626-001)
    And D7 编排层 InitOrchestration 当前 275 行混杂 config + adapter + util

  Scenario: 6 S × WireFunc 命名一致 (T01)
    Given 6 S + 1 横切 (S1 0 wire + S2-S6 各 1 wire + 横切 0 wire)
    When grep -E "^func Wire" internal/bootstrap/*.go
    Then 列出 5 个 Wire* (WireTurnInvoker + WireWaveScheduler + WireExecutionFlow + WireDecisionPlanning + WireMUPSPipeline)
    And 列出 1 个 BuildOrchestratePath (S3 helper)
    And 横切 Hardening 通过 d7spans.SetBridge 隐式注入, 不暴露 Wire 函数

  Scenario: InitOrchestration 主体 ≤ 200 行 (T02)
    Given wire_coordinator.go 当前 275 行 InitOrchestration
    When 抽 30+ 行 config 加载到 loadOrchestratorConfigs() 辅助
    And 抽 3 个内嵌 adapter 函数到 adapters.go
    And 抽 4 个 util 函数到 util.go
    And 用 WireDecisionPlanning + WireMUPSPipeline 包装替换 inline 30+ 行 S5 + S6 构造
    Then wc -l internal/bootstrap/wire_coordinator.go ≤ 250 行
    And InitOrchestration 函数体 ≤ 200 行

  Scenario: 3 个内嵌 adapter 函数已拆到 adapters.go (T03)
    Given wire_coordinator.go 内嵌 newContextEngineAdapter + newTurnOrchExecutor + newGatewayEventPublisher
    When 拆到独立文件 internal/bootstrap/adapters.go
    Then adapters.go 包含 3 个 adapter 类型 + 3 个构造器 + 3 个方法
    And wire_coordinator.go 0 残留
    And 4 个 util 函数 (boolPtr + intPtr + strPtr + mapBackgroundStatus) 拆到 util.go

  Scenario: 22/22 orchestration packages go test -race PASS (T04)
    Given 22 包 orchestration baseline 持平 (DM-20260626-005)
    When go test -race -count=1 ./internal/layers/orchestration/...
    Then 22/22 包全部 PASS
    And LP-1 (TestAutoClose_FullLP1Loop) PASS
    And LP-2 (TestIntegration_5NodePipeline_End2End) PASS
    And LP-5 (Cross-session traceability 套件) PASS
    And hardening/ + escape/circuit_breaker.go + sessionorchestrator/autoclose.go git diff 0 变化
    And cmd/devrix + cmd/obs-verify + tests/testutil/d7_stack.go 0 变化
```

## §3 0 函数签名变化表

| 函数/类型 | 改前位置 | 改后位置 | 签名变化 |
|-----------|----------|----------|----------|
| `boolPtr(b bool) *bool` | wire_coordinator.go | util.go | **0** (pure 移动) |
| `intPtr(i int) *int` | wire_coordinator.go | util.go | **0** |
| `strPtr(s string) *string` | wire_coordinator.go | util.go | **0** |
| `mapBackgroundStatus(s string) orchtypes.TaskStatus` | wire_coordinator.go | util.go | **0** |
| `contextEngineAdapter` 类型 + `newContextEngineAdapter` + 3 方法 | wire_coordinator.go | adapters.go | **0** (未导出类型, 纯移动) |
| `turnOrchExecutor` 类型 + `newTurnOrchExecutor` + `RunTurn` | wire_coordinator.go | adapters.go | **0** |
| `gatewayEventPublisher` 类型 + `newGatewayEventPublisher` + `Publish` | wire_coordinator.go | adapters.go | **0** |
| `WireDecisionPlanning` (NEW) | (N/A) | decision_planning.go | (新) |
| `MUPSPipelinesDeps` (NEW) | (N/A) | mups_pipeline.go | (新 struct) |
| `WireMUPSPipeline` (NEW) | (N/A) | mups_pipeline.go | (新 wire 函数) |
| `loadOrchestratorConfigs` (NEW) | (N/A) | wire_coordinator.go | (新 helper) |
| `resolveObsBridge` (NEW) | (N/A) | wire_coordinator.go | (新 helper) |
| `InitOrchestration(...)` | wire_coordinator.go | wire_coordinator.go | **0** (输入 6 参数 + 输出 error 保持) |

## §4 0 行为变化测试矩阵

| 测试函数 | 改前 | 改后 | 行为变化 |
|----------|------|------|----------|
| `TestInitOrchestration_*` (如有) | (原行为) | (同) | **0** |
| `TestAutoClose_FullLP1Loop` (sessionorchestrator) | PASS | PASS | **0** |
| `TestIntegration_5NodePipeline_End2End` (escape) | PASS | PASS | **0** |
| `TestSessionOrchestrator_BuildObserveRequest_*` | PASS | PASS | **0** |
| `TestResolveAgentWorkDir_*` (bootstrap) | PASS | PASS | **0** (bootstrap 内部) |
| `TestContextEngineBuilder_*` (bootstrap) | PASS | PASS | **0** |
| 22/22 orchestration packages | PASS | PASS | **0** |

---

## 修订记录

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-26 | 初版：4 Gherkin scenarios + 0 函数签名变化表 + 0 行为变化测试矩阵 + v6.0.0 收官声明 |
