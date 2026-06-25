# Demand: devrix-d7-6s-bootstrap-slim (DM-20260626-007)

**Demand ID:** DM-20260626-007
**Status:** S1_Activating
**Priority:** P2
**Created:** 2026-06-26
**Change ID:** devrix-d7-6s-bootstrap-slim
**Related:** devrix-d7-six-s-simplification (DM-20260626-001) · devrix-d7-6s-package-merge (DM-20260626-004) · devrix-d7-6s-verify-promotion (DM-20260626-005) · devrix-d7-6s-observe-merge-cancel (DM-20260626-006)

---

## §1 背景

v6.0.0 域升级 follow-up 序列最终 PR (`devrix-d7-6s-bootstrap-slim`, #6)：把 `internal/bootstrap/` 中的 D7 编排层 wire 调用从分散形态收口为 6 S + 1 横切的 7-wire 拓扑（与 S 层博弈角色对齐），并完成 `InitOrchestration` 函数内部 adapter 拆分 + config 加载抽取的清理工作。

### 原始 follow-up scope (来自 v6.0.0 plan)

`openspec/archive/2026-06-26-devrix-d7-six-s-simplification/tasks.md` §"后续 follow-up" 描述：

> #6 devrix-d7-6s-bootstrap-slim — wire 14 → 6 收口

原始设想是：14 S 设计时每个 S 层有独立的 bootstrap 文件/函数（wire_workmodel, wire_session, wire_wave, wire_flow, wire_decision, wire_observe, wire_execute, wire_learn, wire_escape, wire_hardening 等），v6.0.0 精简到 6 S + 1 横切后，bootstrap 拓扑应同步收口。

### 实际状态 (2026-06-26 调研)

经过 v6.0.0 多个 follow-up PR（#1 spec + #2 mups + #3 hardening + #4 turn-merge + #5 verify-promotion）落地后，bootstrap 拓扑已自然收敛。**当前 InitOrchestration 是 6 S + 1 横切的"单一入口"模式**：

```go
// InitOrchestration 主线 (内部 6 S 拆分清晰)
func InitOrchestration(...) error {
    // 横切 Discipline Keeper (Hardening)
    obsBridge → 后续 6 S 共享

    // S1 WorkModel (State Authority) - 0 wire, 注入式构造
    tm := workmodel.NewTaskManagerFromConfig(...)
    wm := sessionorchestrator.NewLocalWorkModel(tm)

    // S2 SessionOrchestrator (Mediator+Turn Leader+Error Recovery)
    llmInvoker := WireTurnInvoker(llmStack)  // → D3 LLM 直达

    // S5 DecisionPlanning+Observe (Info Producer+Quantizer)
    llmDecomp := decisionplanning.NewLLMDecomposer(LLMDecomposerDeps{...})

    // S3 WaveScheduler (Mechanism Designer)
    orchPath := BuildOrchestratePath(sink, llmDecomp, WaveSchedulerDeps{...})
    // └─ WireWaveScheduler (内部)

    // S6 MUPS Pipeline (Pipeline Coordinator+Memory)
    planMode := workmodel.NewPlanMode(newPlanLLMCompleter(llmInvoker, ...), obsBridge)
    toolExec := sessionorchestrator.NewTurnToolExecutor(...)
    subTurn := sessionorchestrator.NewSubTurnRunner(turnOrch, ...)

    // S2 编排入口
    turnOrch := sessionorchestrator.NewOrchestrator(OrchestratorDeps{...})
    orch := sessionorchestrator.NewSessionOrchestrator(...)
    gw.SetOrchestrationEntry(NewEntry(orch))
}
```

**InitOrchestration 已天然是 6 S 单入口**，但存在以下 cleanup 机会：

| 类别 | 当前 | 期望 | 收益 |
|------|------|------|------|
| InitOrchestration 长度 | 275 行（含 config 加载 + obs bridge 类型断言 + 6 S 构造） | ≤ 200 行 | 可读性 +1 |
| 内嵌 adapter 函数 | `newContextEngineAdapter` + `newTurnOrchExecutor` + `newGatewayEventPublisher` (3 个) + 3 个 boolPtr/intPtr/strPtr + mapBackgroundStatus | 移到独立文件 | 文件组织 +1 |
| Config 加载逻辑 | 30 行 LoadConfigFile + coordCfg + tasksCfg + maxContextTokens + subagentCfg | 抽到 `loadOrchestratorConfigs()` 辅助 | 可读性 +1 |
| Wire 函数命名一致性 | `WireTurnInvoker` (S2) + `WireWaveScheduler` (S3) + `WireExecutionFlow` (S4) + `WireDelegate` (D7) + `BuildOrchestratePath` (S3 helper) | 统一为 6 S × WireFunc 函数 + 1 横切 | 拓扑对齐 +1 |
| Wire 文件组织 | 22 个 .go + 14 个 _test.go（多领域混合） | 按 S 层归类到 `bootstrap/d7/` 子目录（可选） | 领域隔离 +1 |

## §2 目标

把 D7 编排层 bootstrap wire 拓扑收口为 **6 S × 1 wire + 1 横切 × 0 wire = 7 入口**，与 v6.0.0 6 S 博弈角色对齐：

| S 层 | 博弈角色 | Wire 函数 | 入口 |
|------|----------|-----------|------|
| **S1 WorkModel** | State Authority | 0 wire (注入式构造) | `InitOrchestration` 内联 |
| **S2 SessionOrchestrator** | Mediator+Turn Leader+Error Recovery | `WireTurnInvoker` (D7→D3) | bootstrap/turn_wiring.go |
| **S3 WaveScheduler** | Mechanism Designer | `WireWaveScheduler` + `BuildOrchestratePath` | bootstrap/wire_wave.go |
| **S4 ExecutionFlow+Verify** | Costly Signaler+Certifier | `WireExecutionFlow` (4 子文件: execution_flow.go + freefork_injection.go + task_notif_injection.go + transcript_writer.go) | bootstrap/execution_flow.go |
| **S5 DecisionPlanning+Observe** | Information Producer+Quantizer | `WireDecisionPlanning` (LLMDecomposer 包装) | bootstrap/decision_planning.go (NEW) |
| **S6 MUPS Pipeline** | Pipeline Coordinator+Memory Curator | `WireMUPSPipeline` (TurnToolExecutor + SubTurnRunner + PreparedTurnAdapter) | bootstrap/mups_pipeline.go (NEW) |
| **横切 Hardening** | Discipline Keeper | 0 wire (Observability Bridge 自动注入) | 隐式 |

清理 InitOrchestration 内部结构：
1. 抽取 config 加载到 `loadOrchestratorConfigs(configFile string) (*orchestratorConfigs, error)` 辅助函数
2. 抽取 obs bridge 类型断言到 `resolveObsBridge(arg interface{}) *observability.Bridge` 辅助函数
3. 拆分 3 个内嵌 adapter 函数（newContextEngineAdapter / newTurnOrchExecutor / newGatewayEventPublisher）到独立文件
4. 统一 6 S × WireFunc 命名约定，让 InitOrchestration 主体是纯组合

## §3 范围 (In Scope)

1. `internal/bootstrap/wire_coordinator.go`:
   - 提取 30+ 行 config 加载代码到 `loadOrchestratorConfigs()` 辅助函数（≤ 80 行）
   - 提取 obsBridgeArg 类型断言到 `resolveObsBridge()` 辅助函数
   - 保留 InitOrchestration 主体为 6 S 组合入口（≤ 200 行）
2. `internal/bootstrap/`:
   - 提取 3 个内嵌 adapter 函数到独立文件：
     - `internal/bootstrap/adapters.go` (NEW): `newContextEngineAdapter` + `newTurnOrchExecutor` + `newGatewayEventPublisher` + `turnOrchExecutor.RunTurn` + `gatewayEventPublisher.Publish`
   - 提取 mapBackgroundStatus + boolPtr + intPtr + strPtr 到 `internal/bootstrap/util.go` (NEW)
   - 新建 `internal/bootstrap/decision_planning.go` (NEW): `WireDecisionPlanning` 包装 LLMDecomposer
   - 新建 `internal/bootstrap/mups_pipeline.go` (NEW): `WireMUPSPipeline` 包装 TurnToolExecutor + SubTurnRunner
3. 文档同步（4 文档）：
   - `openspec/specs/d7-orchestration/d7-domain.md` v2.3.0 → **v2.4.0** (新增 §"Bootstrap Wire 拓扑" 章节)
   - `openspec/specs/d7-orchestration/design.md` v4.3.0 → **v4.4.0** (§"Bootstrap" 章节展开)
   - `openspec/specs/d7-orchestration/t-registry.md` v4.5.0 → **v4.6.0** (新增 D7-S2-A51 T01-T04)
   - `openspec/t-registry.md` (root) v5.5.0 → **v5.6.0** (新增 DM-20260626-007 增量)
4. T 层验证：4 个新 P0 T (D7-S2-A51-T01..T04)
   - T01: 6 S × WireFunc 函数命名一致 + 1 横切 × 0 wire
   - T02: InitOrchestration 主体 ≤ 200 行 + 6 S 组合入口清晰
   - T03: 3 个内嵌 adapter 函数已拆到独立文件 + 0 残留内嵌
   - T04: 22/22 orchestration packages go test -race PASS + LP-1/2/5 100% 兼容 + `InitOrchestration` 调用方 (cmd/devrix + cmd/obs-verify + tests/testutil/d7_stack.go) 0 变化

## §4 范围 (Out of Scope)

- **不动 wire 拓扑行为**: `InitOrchestration` 输入/输出签名 + 内部 S 层调用顺序保持不变
- **不动其他域 wire 函数**: WireIM (D1) + WireAgentFactory/WireDefaultForker/WireMultiAgent (D4) + WireContextV3 (D2) + WireAgentToolRegistry (D4) 保持原样
- **不引入 DI 框架**: 保持手写 wire (避免引入 wire.go 或 fx 等)
- **不重组 bootstrap 子目录**: 22 个 bootstrap 文件维持扁平结构（不引入 `bootstrap/d7/` 子目录）
- **不改 cmd/devrix + cmd/obs-verify + tests/testutil/d7_stack.go**: 调用方 0 变化
- **不删任何 wire 函数**: 7 wire 函数全部保留，只整理 InitOrchestration 内部

## §5 风险评估

| 风险 | 可能性 | 影响 | 缓解 |
|------|--------|------|------|
| adapter 函数抽离后 import 错位 | 中 | 中 (build fail) | 同一 package 内移动, import 路径不变 |
| config 抽离后 LoadConfigFile 调用次数变化 | 低 | 低 (config 行为不变) | 保留所有调用, 抽到辅助函数 |
| WireDecisionPlanning/WireMUPSPipeline 命名引起歧义 | 低 | 低 (只是包装) | 文档说明 wire 拓扑 |
| 测试覆盖漏 adapter | 中 | 中 (test fail) | 跑全量 orchestration packages go test -race |
| LP-1/2/5 集成测试 regression | 低 | 高 (release block) | 全套集成测试 + baseline 22/22 PASS 校验 |

## §6 验收标准 (Acceptance Criteria)

| AC# | 描述 | 验证 |
|-----|------|------|
| AC1 | InitOrchestration 主体 ≤ 200 行 | `wc -l internal/bootstrap/wire_coordinator.go` ≤ 250（含 helper + adapter 引用） |
| AC2 | 3 个内嵌 adapter 函数（newContextEngineAdapter / newTurnOrchExecutor / newGatewayEventPublisher）已拆到 `internal/bootstrap/adapters.go` | `grep "^func new" internal/bootstrap/wire_coordinator.go` 0 命中 |
| AC3 | 4 个 util 函数（boolPtr / intPtr / strPtr / mapBackgroundStatus）已拆到 `internal/bootstrap/util.go` | 同上 |
| AC4 | config 加载已抽到 `loadOrchestratorConfigs()` 辅助函数 | `grep "LoadConfigFile" internal/bootstrap/wire_coordinator.go` ≤ 1 命中（仅辅助函数内） |
| AC5 | 6 S × WireFunc 命名一致（WireTurnInvoker / WireWaveScheduler / WireExecutionFlow / WireDecisionPlanning / WireMUPSPipeline） | `grep -E "^func Wire" internal/bootstrap/*.go` 列出 5 个 Wire* + 1 BuildOrchestratePath |
| AC6 | `go build ./...` 0 错误 | terminal output 0 errors |
| AC7 | `go vet ./...` 0 警告 | terminal output 0 warnings |
| AC8 | 22/22 orchestration packages go test -race PASS | `go test -race -count=1 ./internal/layers/orchestration/...` |
| AC9 | LP-1 (Bayesian reputation TestAutoClose_FullLP1Loop) / LP-2 (5 节点 TestIntegration_5NodePipeline_End2End) / LP-5 (Cross-session traceability) 100% 兼容 | 同 DM-20260626-004/005 baseline |
| AC10 | hardening/ + escape/circuit_breaker.go + sessionorchestrator/autoclose.go git diff 0 变化 | baseline stability 保持 |
| AC11 | InitOrchestration 调用方 (cmd/devrix + cmd/obs-verify + tests/testutil/d7_stack.go) 0 变化 | `git diff` 仅含 internal/bootstrap/ 内部 + 4 文档 |
| AC12 | spec 同步 (4 文档 version bump) | d7-domain v2.3.0→v2.4.0 + design v4.3.0→v4.4.0 + t-registry v4.5.0→v4.6.0 + 根 v5.5.0→v5.6.0 |
| AC13 | verify-archive.sh 12/12 PASS | S6 阶段验证 |
| AC14 | 4 个新 P0 T (D7-S2-A51-T01..T04) 全部 IMPLEMENTED | 域 t-registry D7-S2-A51 row |

## §7 后续 follow-up

本 PR 是 v6.0.0 follow-up 序列的**最终收口**。完成本 PR 后：

- v6.0.0 follow-up 6 PR 序列 5/6 S7_Archived + 1/6 S1_Cancelled + 1/1 S7_Archived (本次)
- D7 编排层进入 v6.0.x 维护阶段（新功能走 v6.0.1 序列）
- bootstrap 拓扑稳定 6 S + 1 横切博弈角色对齐

---

## 修订记录

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-26 | 初版：14 AC × 7 sections 调研 + scope + 风险 + 验收标准 + v6.0.0 follow-up 收官声明 |
