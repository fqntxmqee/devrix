# Implementation Tasks: devrix-harness-bootstrap

**Change ID:** devrix-harness-bootstrap
**Demand ID:** DM-20260609-004
**Status:** S4 Complete / S5 Ready (2026-06-10)
**Based on:** design.md, specs/context-engine/spec.md, specs/observability/spec.md

---

V5a 拆为 **两个 PR** 交付，降低单分支变更风险与 review 负担：

| PR | 聚焦 | Milestone | 工时 |
|----|------|-----------|------|
| PR1「内核」 | Harness 骨架 + 工具装配 | M1 + M2 + M3 | ~44h |
| PR2「装配」 | 上下文注入 + 可观测性 + 测试 | M4 + M5 + M6 + M7 | ~47h |

---

## PR1「内核」（M1 + M2 + M3，~44h）

### Milestone 1: 配置与类型基础

### Definition of Done
- [ ] HarnessConfig 编译通过
- [ ] L5-2-9-01 已登记

### Tasks

- [ ] **T1**: 新增 `HarnessConfig` + `devrix.yaml` 示例块
  - L4: L4-CTX-HARNESS
  - L5: —
  - Estimate: 2h

- [ ] **T2**: 扩展 `types/context.go`（HarnessSessionState, TranscriptStore, BootstrapReport）
  - L4: L4-CTX-HARNESS, L4-CTX-TRANSCRIPT
  - L5: L5-2-9-01
  - Estimate: 3h

- [ ] **T3**: 定义 `harness/contracts.go`（IHarnessBootstrap, IDeferredInit, IToolPoolFilter, IPromptRouter）
  - L4: L4-CTX-HARNESS
  - L5: —
  - Estimate: 2h

**Quality Gate:**
- [ ] `./scripts/test-unit.sh` 通过
- [ ] 配置默认值 `harness.enabled=false`

---

## Milestone 2: Harness Bootstrap 核心

- [ ] **T4**: 实现 `harness/workspace.go`（WorkDir 扫描 + WorkspaceContext）
  - L4: L4-CTX-HARNESS
  - L5: L5-2-9-02
  - Estimate: 4h

- [ ] **T5**: 实现 `harness/bootstrap.go`（阶段 1-4 编排 + BootstrapReport）
  - L4: L4-CTX-HARNESS
  - L5: L5-2-9-01, L5-2-9-03
  - Estimate: 6h

- [ ] **T6**: 实现 `harness/deferred.go`（NoOpDeferredInit + trusted gate）
  - L4: L4-CTX-HARNESS
  - L5: L5-2-9-04
  - Estimate: 2h

- [ ] **T7**: 单元测试 `harness/bootstrap_test.go`, `workspace_test.go`
  - L4: L4-CTX-HARNESS
  - L5: L5-2-9-01, L5-2-9-02, L5-2-9-03, L5-2-9-04
  - Estimate: 4h

**Quality Gate:**
- [ ] Bootstrap stage 顺序 deterministic
- [ ] trusted=false 时 deferred_init 全 false

---

## Milestone 3: ToolPool、Router 与 Preflight

- [ ] **T8**: 实现 `harness/toolpool.go`（simple_mode / MCP / deny 过滤）
  - L4: L4-CTX-TOOLPOOL
  - L5: L5-2-9-05
  - Estimate: 4h

- [ ] **T9**: 实现 `harness/router.go`（PromptRouter 关键词计分）
  - L4: L4-CTX-ROUTER
  - L5: L5-2-9-06
  - Estimate: 3h

- [ ] **T9b**: 实现 `harness/preflight.go`（RuleBasedPreflight + ToolRelevanceFilter；PR1 用 provisional context）
  - L4: L4-CTX-PREFLIGHT
  - L5: L5-2-9-09
  - Estimate: 6h
  - Note: PR1 不测完整 XML context；block mode config 校验拒绝

- [ ] **T10**: 单元测试 toolpool / router / preflight
  - L5: L5-2-9-05, L5-2-9-06, L5-2-9-09
  - Estimate: 5h

**Quality Gate:**
- [ ] preflight.mode=block 在 V5a 未实现（应 compile-time 或 config 校验拒绝）
- [ ] tool filter 可独立 disabled
- [ ] `harness.enabled=false` 退化路径单元覆盖（零行为变化断言）

- [ ] **T10b**: 轻量集成测试 `tests/integration/context_harness_bootstrap_smoke_test.go`
  - 验证场景：enabled=false 零行为变化、enabled=true 完整 bootstrap 流程
  - Tag: `integration && d2`（与 PR2 全量集成测试共用标签，但仅 smoke 子集）
  - L5: L5-2-9-01, L5-2-9-03
  - Estimate: 3h

---

---

## PR2「装配」（M4 + M5 + M6 + M7，~47h）

### Milestone 4: Workspace、Transcript 与 Process 集成

- [ ] **T11**: 实现 `harness/transcript.go` + `session_log.go`（双轨 log）
  - L4: L4-CTX-TRANSCRIPT
  - L5: L5-2-9-07
  - Estimate: 4h

- [ ] **T11b**: 实现 `harness/system_prompt_assembler.go` + `templates/*.md`（§十 SoT）
  - L4: L4-CTX-WORKSPACE
  - L5: L5-2-9-10
  - Estimate: 8h

- [ ] **T11c**: 重构 `memory.EnrichWithLongTermRecall` → 返回 entries 供 Assembler（不再 += SystemPrompt）；压缩 pipeline 改为 messages-only
  - L4: L4-CTX-MEMORY, L4-CTX-WORKSPACE
  - L5: L5-2-9-10, L5-2-9-13
  - Estimate: 5h

- [ ] **T12**: `engine.go` 集成 Assembler（压缩后 Build → sc.SystemPrompt → PEV）
  - L4: L4-CTX-HARNESS, L4-CTX-STATE
  - L5: L5-2-9-01, L5-2-9-07, L5-2-9-08
  - Estimate: 6h

- [ ] **T13**: `bootstrap/context_engine.go` 注入 Harness 依赖
  - L4: L4-CTX-HARNESS
  - L5: L5-2-9-08
  - Estimate: 2h

- [ ] **T14**: PEV 消费 HarnessSessionState.VisibleTools（禁止绕过 ToolPool 全量 ListTools）
  - L4: L4-CTX-PEV, L4-CTX-TOOLPOOL
  - L5: L5-2-9-05, L5-2-9-14
  - Estimate: 4h

**Quality Gate:**
- [ ] harness.enabled=false 时 V4 行为 bit-identical（回归测试）
- [ ] info 事件含 bootstrap stage metadata

---

## Milestone 5: 集成测试与文档

- [ ] **T15**: 集成测试 `tests/integration/context_harness_bootstrap_test.go`
  - L5: L5-2-9-08
  - Estimate: 4h

- [ ] **T15b**: 集成测试 `tests/integration/context_harness_obs_test.go`（Jaeger span 树 + parent-child）
  - L5: L5-2-9-11, L5-2-9-15
  - Estimate: 5h

- [ ] **T16**: 验收测试 `tests/acceptance/p0/ctx_harness_bootstrap_test.go`
  - L5: L5-2-9-01 ~ L5-2-9-11（P0 子集）
  - Estimate: 5h

---

## Milestone 6: 可观测性（Jaeger）

- [ ] **T20**: `telemetry/names.go` 新增 6 个 harness Operation + `LayerAndComponent` 分支
  - L5: L5-5-5-02
  - Estimate: 1h

- [ ] **T21**: `coverage/registry.go` 登记 harness OperationMeta（SinceVersion 2.1.0）
  - L5: L5-5-5-02
  - Estimate: 1h

- [ ] **T22**: `harness/tracing.go` + 各模块 `startHarnessSpan` 埋点（**MUST ctx 传播**）
  - L4: L4-CTX-HARNESS, L4-CTX-WORKSPACE
  - L5: L5-2-9-11, L5-2-9-15
  - Estimate: 4h

- [ ] **T23**: 更新 `telemetry/names_test.go`、`coverage/registry_test.go` expected 列表
  - L5: L5-5-5-02
  - Estimate: 1h

**Quality Gate:**
- [ ] `go test ./internal/layers/observability/...` 通过
- [ ] CoverageReport 中 harness operations 在 Process 后 `operations_hit` 增加

---

## Milestone 7: 文档与 L5 登记

- [ ] **T17**: 更新 `openspec/l5-registry.md` D2-S9 + D5 L5-5-5-02
  - L5: L5-2-9-*, L5-5-5-02
  - Estimate: 1h

- [ ] **T18**: 更新 `docs/context-engine-design.md` 附录 V5 + §十一/十二
  - Estimate: 2h

- [ ] **T19**: 更新 `openspec/project.md` D2-S9 场景行
  - Estimate: 1h

**Quality Gate:**
- [ ] `./scripts/test-domain.sh d2` 通过
- [ ] P0 L5 全绿

---

## 工时汇总

| PR | Milestone | 估算 | 累计 |
|----|-----------|------|------|
| PR1「内核」 | M1 配置类型 | 7h | |
| PR1「内核」 | M2 Bootstrap | 16h | |
| PR1「内核」 | M3 ToolPool/Router/Preflight | 21h | ~44h |
| PR2「装配」 | M4 集成 | 22h | |
| PR2「装配」 | M5 集成测试 | 14h | |
| PR2「装配」 | M6 Jaeger 埋点 | 7h | |
| PR2「装配」 | M7 文档 L5 | 4h | ~47h |
| **合计** | — | **~91h** | |

---

## Completion Checklist（一次归档）

### PR1「内核」合入门禁（S4 Gate → Merge）
- [x] M1/M2/M3 所有任务完成
- [x] PR1 轻量集成测试通过
- [x] `harness.enabled=false` 回归通过（V4 行为 bit-identical）
- [x] **`openspec validate devrix-harness-bootstrap` 通过**
- [ ] S4 代码审查通过
- [ ] → **合入 main，不归档**

### PR2「装配」合入门禁（S4 Gate → Merge）
- [x] M4/M5/M6/M7 所有任务完成
- [x] L5-2-9-11 + L5-5-5-02 IMPLEMENTED
- [x] P0 L5 全绿
- [ ] S4 代码审查通过
- [ ] → **合入 main**

### S5 验收 + S6 归档（PR2 合入后一次性执行）
- [x] acceptance-report.md 填写
- [ ] harness.enabled 灰度文档就绪（见 devrix.yaml 注释 + design §四）
- [ ] 覆盖率 ≥ 80%（待 CI/人工确认）
- [ ] 准备 `/openspec-archive devrix-harness-bootstrap`
