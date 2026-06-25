# Tasks: devrix-d7-6s-bootstrap-slim

**Change ID:** devrix-d7-6s-bootstrap-slim
**Status:** S7_Archived
**Priority:** P2
**Created:** 2026-06-26
**DM:** DM-20260626-007

---

## §1 实施任务 (按阶段)

### 阶段 1: util.go 抽离 (S4 实现 - PR #225)

- [ ] **T1.1** 新建 `internal/bootstrap/util.go`:
  - [ ] `func boolPtr(b bool) *bool`
  - [ ] `func intPtr(i int) *int`
  - [ ] `func strPtr(s string) *string`
  - [ ] `func mapBackgroundStatus(s string) orchtypes.TaskStatus`
- [ ] **T1.2** 修改 `internal/bootstrap/wire_coordinator.go`:
  - [ ] 删除 4 个函数定义（原 14 行）
  - [ ] import 自动引用同 package 的 util.go（无需 import 改动）
- [ ] **T1.3** 验证:
  - [ ] `go build ./...` 0 错误
  - [ ] `go vet ./...` 0 警告
  - [ ] `grep "func boolPtr\|func intPtr\|func strPtr\|func mapBackgroundStatus" internal/bootstrap/wire_coordinator.go` 0 命中
- [ ] **T1.4** Commit + push + PR

### 阶段 2: adapters.go 抽离 (S4 实现 - PR #226)

- [ ] **T2.1** 新建 `internal/bootstrap/adapters.go`:
  - [ ] `type contextEngineAdapter struct { gw, engine, counter }`
  - [ ] `func newContextEngineAdapter(gw, engine, counter) *contextEngineAdapter`
  - [ ] `func (a *contextEngineAdapter) Prepare(ctx, req) (turn.Prepared, error)`
  - [ ] `func (a *contextEngineAdapter) Persist(ctx, state) error`
  - [ ] `func (a *contextEngineAdapter) TokenCount(text) int`
  - [ ] `type turnOrchExecutor struct { orch sessionorchestrator.TurnOrchestrator }`
  - [ ] `func newTurnOrchExecutor(orch) *turnOrchExecutor`
  - [ ] `func (e *turnOrchExecutor) RunTurn(ctx, req) (<-chan *contracts.EngineEvent, error)`
  - [ ] `type gatewayEventPublisher struct { gw *capture.CommunicationGateway }`
  - [ ] `func newGatewayEventPublisher(gw) *gatewayEventPublisher`
  - [ ] `func (p *gatewayEventPublisher) Publish(ctx, ev)` (内部调 `gw.PublishEngineEvent`)
- [ ] **T2.2** 修改 `internal/bootstrap/wire_coordinator.go`:
  - [ ] 删除 3 个 adapter 类型 + 3 个构造器 + 3 个方法（原 61 行）
- [ ] **T2.3** 验证:
  - [ ] `go build ./...` 0 错误
  - [ ] `go vet ./...` 0 警告
  - [ ] `grep "^func new\|^type turnOrchExecutor\|^type gatewayEventPublisher" internal/bootstrap/wire_coordinator.go` 0 命中
  - [ ] `grep "^type contextEngineAdapter" internal/bootstrap/wire_coordinator.go` 0 命中
- [ ] **T2.4** Commit + push + PR

### 阶段 3: S5 + S6 Wire 包装 (S4 实现 - PR #227)

- [ ] **T3.1** 新建 `internal/bootstrap/decision_planning.go`:
  - [ ] `func WireDecisionPlanning(llmInvoker decisionplanning.LLMTaskDecomposer, defaultTier string) decisionplanning.LLMTaskDecomposer`
  - [ ] 内部调 `decisionplanning.NewLLMDecomposer(LLMDecomposerDeps{LLM, DefaultTier})`
- [ ] **T3.2** 修改 `internal/bootstrap/wire_coordinator.go`:
  - [ ] `llmDecomp := decisionplanning.NewLLMDecomposer(...)` → `llmDecomp := WireDecisionPlanning(llmInvoker, llmStack.DefaultModel)`
- [ ] **T3.3** 新建 `internal/bootstrap/mups_pipeline.go`:
  - [ ] `type MUPSPipelinesDeps struct { CtxAdapter, OrchPath, LLMInvoker, DefaultModel, LoopFirst, ObsBridge, PlanMode, SubagentCfg, MaxContextTokens, TaskManager, ToolResultStore, WorkModel, FocusHint, ResolveAwait }`
  - [ ] `func WireMUPSPipeline(deps MUPSPipelinesDeps) (*sessionorchestrator.Orchestrator, *sessionorchestrator.SubTurnRunner, *sessionorchestrator.TurnPrepareWrapper)`
  - [ ] 内部构造 toolExec + ctxPrep + turnOrch + subTurn
- [ ] **T3.4** 修改 `internal/bootstrap/wire_coordinator.go`:
  - [ ] 30+ 行 S6 构造 → `turnOrch, subTurn, ctxPrep := WireMUPSPipeline(MUPSPipelinesDeps{...})`
- [ ] **T3.5** 验证:
  - [ ] `go build ./...` 0 错误
  - [ ] `go vet ./...` 0 警告
  - [ ] `go test -race -count=1 ./internal/bootstrap/...` 全 PASS
  - [ ] `go test -race -count=1 ./internal/layers/orchestration/...` 22/22 PASS
- [ ] **T3.6** Commit + push + PR

### 阶段 4: config + obsBridge 抽取 (S4 实现 - PR #228)

- [ ] **T4.1** 修改 `internal/bootstrap/wire_coordinator.go`:
  - [ ] 新增 `type orchestratorConfigs struct { coordCfg, tasksCfg, subagentCfg, maxContextTokens }`
  - [ ] 新增 `func loadOrchestratorConfigs(configFile string) (*orchestratorConfigs, error)`
  - [ ] 抽 52 行 config 加载到辅助函数
  - [ ] InitOrchestration 主体替换为 `cfg, _ := loadOrchestratorConfigs(configFile)`
  - [ ] 引用 `cfg.coordCfg` / `cfg.tasksCfg` / `cfg.subagentCfg` / `cfg.maxContextTokens`
- [ ] **T4.2** 同文件新增:
  - [ ] `func resolveObsBridge(arg interface{}) *observability.Bridge`
  - [ ] InitOrchestration 主体替换 4 行类型断言为 `obsBridge := resolveObsBridge(obsBridgeArg)`
- [ ] **T4.3** 验证:
  - [ ] `go build ./...` 0 错误
  - [ ] `go vet ./...` 0 警告
  - [ ] `wc -l internal/bootstrap/wire_coordinator.go` ≤ 250 行
  - [ ] InitOrchestration 函数体 ≤ 200 行
  - [ ] `grep "LoadConfigFile" internal/bootstrap/wire_coordinator.go` ≤ 1 命中
- [ ] **T4.4** Commit + push + PR

### 阶段 5: 文档同步 (S4 实现 - PR #228 同 PR 内)

- [ ] **T5.1** `openspec/specs/d7-orchestration/d7-domain.md`:
  - [ ] 更新 `**Version:** 2.3.0` → `**Version:** 2.4.0`
  - [ ] 更新 `**Last Updated:** 2026-06-26 (verify-promotion)` → `2026-06-26 (bootstrap-slim)`
  - [ ] 新增 §"Bootstrap Wire 拓扑" 章节（详见 design.md §7.1）
  - [ ] 新增 v2.4.0 changelog row
- [ ] **T5.2** `openspec/specs/d7-orchestration/design.md`:
  - [ ] 更新 `**Version:** 4.3.0` → `**Version:** 4.4.0`
  - [ ] 更新 `**Last Updated:** 2026-06-26 (v4.3 verify-promotion)` → `2026-06-26 (v4.4 bootstrap-slim)`
  - [ ] §"Bootstrap" 章节展开 6 S × WireFunc 函数清单
  - [ ] 新增 v4.4.0 changelog row
- [ ] **T5.3** `openspec/specs/d7-orchestration/t-registry.md`:
  - [ ] 更新 `**Version:** 4.5.0` → `**Version:** 4.6.0`
  - [ ] 更新 `**Last Updated:** 2026-06-26 (verify-promotion)` → `2026-06-26 (bootstrap-slim)`
  - [ ] 新增 D7-S2-A51 章节 4 P0 T (T01-T04), 全部 PLANNED → IMPLEMENTED
  - [ ] 新增 v4.6.0 changelog row
  - [ ] Statistics: 域 Total 222→226, P0 185→189
- [ ] **T5.4** `openspec/t-registry.md` (root):
  - [ ] 更新 `**Version:** 5.5.0` → `**Version:** 5.6.0`
  - [ ] 更新 `**Last Updated:** 2026-06-26 (verify-promotion)` → `2026-06-26 (bootstrap-slim)`
  - [ ] 在 §Overview 域级注册表 D7 Orchestration 行更新 Total 218→222, P0 181→185
  - [ ] 更新 §Overview "**总计**": 540→544, P0 350→354
  - [ ] 新增 DM-20260626-007 增量条目
- [ ] **T5.5** 验证:
  - [ ] 4 文档 git diff 仅包含目标改动
  - [ ] 无意外 cross-reference 破坏

## §2 S4-Gate 验证 (PR merge 前)

- [ ] **G1** `go build ./...` 0 错误
- [ ] **G2** `go vet ./...` 0 警告
- [ ] **G3** `go test -race -count=1 ./internal/bootstrap/...` 全 PASS
- [ ] **G4** `go test -race -count=1 ./internal/layers/orchestration/...` 22/22 PASS
- [ ] **G5** LP-1 (TestAutoClose_FullLP1Loop) PASS
- [ ] **G6** LP-2 (TestIntegration_5NodePipeline_End2End) PASS
- [ ] **G7** LP-5 (Cross-session traceability 套件) PASS
- [ ] **G8** `git diff --stat hardening/ escape/circuit_breaker.go sessionorchestrator/autoclose.go` 空
- [ ] **G9** `git diff --stat cmd/devrix/main.go cmd/obs-verify/main.go tests/testutil/d7_stack.go` 空
- [ ] **G10** `git diff --stat internal/layers/orchestration/` 仅含 sessionorchestrator/ (引用 verify.* 0 变化)

## §3 S5 验收 (PR merge 后)

- [ ] **A1** PR #225+#226+#227+#228 全部 squash auto-merge
- [ ] **A2** CI unit tests + layer-lint 全 SUCCESS
- [ ] **A3** `verify-archive.sh` 准备就绪 (待 S6 阶段)
- [ ] **A4** 13 AC 全部 PASS (详见 demand.md §6)
- [ ] **A5** 4 个新 P0 T (D7-S2-A51-T01..T04) 全部 IMPLEMENTED
- [ ] **A6** acceptance-report.md 完整 11 sections (摘要 + T 层验证 + 22 包 baseline + LP 集成 + Baseline stability + Cross-package DAG + spec 同步 + 13 AC + PR 落地 + 经验教训 + 修订记录)

## §4 S6 归档 (S5 完成后)

- [ ] **F1** 移动 `openspec/changes/devrix-d7-6s-bootstrap-slim/` → `openspec/archive/2026-06-26-devrix-d7-6s-bootstrap-slim/`
- [ ] **F2** 更新 `.openspec.yaml` status: `s2_proposal` → `s7_archived`, 4 P0 T PLANNED → IMPLEMENTED
- [ ] **F3** 更新 4 文档 Status 字段
- [ ] **F4** 添加 DM-20260626-007 row 到 `openspec/demand-archive-index.md`
- [ ] **F5** 运行 `verify-archive.sh devrix-d7-6s-bootstrap-slim` 期望 12 PASS / 0 FAIL / 1 WARN
- [ ] **F6** 创建 S6 PR (#229) + auto-merge --auto --squash --delete-branch
- [ ] **F7** 保存 memory 到 `~/.claude/projects/-Users-fukai-workspace/memory/devrix-d7-6s-bootstrap-slim-s7-archived.md`
- [ ] **F8** 更新 MEMORY.md 索引
- [ ] **F9** v6.0.0 follow-up 序列收官声明（DM-20260626-007 完成 = 序列 5/6 S7_Archived + 1/6 S1_Cancelled + 1/1 S7_Archived）

## §5 关键文件清单

### 新建 (4 文件)

- `internal/bootstrap/util.go` (~30 行)
- `internal/bootstrap/adapters.go` (~80 行)
- `internal/bootstrap/decision_planning.go` (~30 行)
- `internal/bootstrap/mups_pipeline.go` (~80 行)

### 修改 (5 文件)

- `internal/bootstrap/wire_coordinator.go` (275 → ≤ 200 行)
- `openspec/specs/d7-orchestration/d7-domain.md` v2.3.0 → v2.4.0
- `openspec/specs/d7-orchestration/design.md` v4.3.0 → v4.4.0
- `openspec/specs/d7-orchestration/t-registry.md` v4.5.0 → v4.6.0
- `openspec/t-registry.md` (root) v5.5.0 → v5.6.0

### 不变 (1 文件 + 3 调用方)

- `internal/bootstrap/turn_wiring.go` (S2 WireTurnInvoker 保持)
- `cmd/devrix/main.go` (调用方 0 变化)
- `cmd/obs-verify/main.go` (调用方 0 变化)
- `tests/testutil/d7_stack.go` (调用方 0 变化)

## §6 风险与回滚

| 风险 | 缓解 | 回滚 |
|------|------|------|
| adapters.go 抽离后类型未导出破坏 | 同 package 内移动 | 删除 adapters.go 还原 wire_coordinator.go |
| WireMUPSPipeline 参数过多 | MUPSPipelinesDeps 结构体聚合 | 还原 inline 30 行 S6 构造 |
| loadOrchestratorConfigs 静默 fallback | 保留原"err 静默"语义 | 还原 inline 52 行 config |
| 5 PR 拆分粒度过细 | 4 PR (PR #225-#228) 4 阶段 | 合并为单 PR |
| S4 PR 未 squash merge 时 S5 验收失败 | S5 阶段必须等所有 4 PR merge | 修复最后一个未 merge PR |

---

## 修订记录

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-26 | 初版：5 阶段 (1-4 实施 + 5 文档同步) × 4 PR 拆分 + S4/S5/S6-Gate 完整任务清单 + 风险回滚 |
