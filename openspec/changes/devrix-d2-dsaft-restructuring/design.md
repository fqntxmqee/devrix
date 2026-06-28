# Design: devrix-d2-dsaft-restructuring (DM-20260629-002)

**Change ID:** `devrix-d2-dsaft-restructuring`
**Demand ID:** DM-20260629-002
**Status:** S3_Design
**Template:** `devrix-d7-dsaft-restructuring` design.md（DM-20260629-001）

---

## §1 目标与范围

D2 v8.2.0 → v9.0.0 6 子 Change 联动 refactoring。详见 `demand.md` + `proposal.md`。

---

## §2 S→A→F 逐层详细设计

### 2.1 S 层（North Star 重整）

#### 2.1.1 当前 v8.2.0 状态

| S ID | Scenario | Status |
|------|----------|--------|
| D2-S15 | PrepareExecutionContext | REGISTRY（4 A） |
| D2-S16 | RunQueryLoop | REMOVED（DM-20260618-010）→ D7-S2-A06 |
| D2-S17 | PersistSessionState | REGISTRY（3 A） |
| D2-S18 | EnforceExecutionPolicy | REGISTRY（7 A） |
| D2-S19 | NestedExecution | DISMANTLED（v6.4.0） |
| D2-S20 | LegacyHarnessFallback | REMOVED（v6.5.0） |

#### 2.1.2 v9.0.0 目标

| S ID | Scenario | ValueFlow Alias | Status |
|------|----------|-----------------|--------|
| D2-S15 | PrepareExecutionContext | **D2_Context_Loading_Compression** | REGISTRY（4 A 拆 8 A） |
| D2-S16 | Materialize Context（v8.2 rename） | **D2_LLM_Ready_Assembly** | REGISTRY（3 A） |
| D2-S17 | PersistSessionState | **D2_Session_State_Persistence** | REGISTRY（3 A） |
| D2-S18 | EnforceExecutionPolicy | **D2_Tool_Permission_Sandbox** | REGISTRY（7 A） |

**S15 拆 8 A 详细映射**（god fn 拆分后）：
- D2-S15-A01 LoadSession（保持）
- D2-S15-A02 RecallMemory（保持）
- D2-S15-A03 CompressContext → 拆 D2-S15-A03-A/B（pipeline 7 step + budget validation）
- D2-S15-A04 AssemblePrompt → 拆 D2-S15-A04-A/B（assembler core + dynamic sections）
- D2-S15-A05-A08 新登记 4 A（compression_steps / compression_budget / prompt_assembler_layers / prompt_dynamic_sections）

#### 2.1.3 Historical Appendix（S1/S9/S10/S19/S20 5 历史 S 独立段）

| Historical S | Status | 备注 |
|---|---|---|
| D2-S1 | PEV, RETIRED | → D2-S15/S16 |
| D2-S9 | Harness, REMOVED | v6.5.0 |
| D2-S10 | QueryLoop, REMOVED | DM-20260618-010 → D7-S2-A06 |
| D2-S19 | NestedExecution, DISMANTLED | v6.4.0 → S15 + S18 |
| D2-S20 | LegacyHarnessFallback, REMOVED | v6.5.0 |

**保留旧编号（DSAFT 原则 3）**：历史 S 不删编号，仅移到独立 §Historical Appendix 段。

### 2.2 A 层（god fn 拆分）

#### 2.2.1 S15 god fn 拆分

**D2-S15-A03 CompressContext god fn `RunForSession()` (109 LOC)**：

```
compression/pipeline.go::RunForSession (109 LOC, god)
   ↓ 拆
   ├─ compression/pipeline.go (<300 行, 主入口)
   │     func RunForSession(ctx, sess, messages, budget) (compressed, report, error)
   │     func newPipelineState() PipelineState
   │
   ├─ compression/compression_steps.go (<500 行, 7 step helper)
   │     func deduplicate(messages) messages
   │     func snip(messages) messages
   │     func fold(messages) messages
   │     func expire(messages) messages
   │     func maybeAutocompact(messages) messages
   │     func assemble(messages) messages
   │     func tokenBlock(messages) error
   │
   └─ compression/compression_budget.go (<300 行, token validation)
         func validateBudget(messages, budget) error
         func compressBudget(report) report
```

**D2-S15-A04 AssemblePrompt god fn `Build()` (55 LOC)**：

```
prepare/prompt/assembler.go::Build (55 LOC, god)
   ↓ 拆
   ├─ prepare/prompt/assembler.go (<300 行, core)
   │     func Build(ctx, req) (SystemPrompt, BuildReport, error)
   │
   ├─ prepare/prompt/prompt_assembler_layers.go (<300 行, buildCoreLayer + buildLayer3)
   │     func buildCoreLayer(ctx, req) Layer
   │     func buildLayer3(ctx, req) Layer
   │
   └─ prepare/prompt/prompt_dynamic_sections.go (<200 行, buildDynamicSections)
         func buildDynamicSections(ctx, req) []Section
```

#### 2.2.2 S16 god fn 拆分

**D2-S16-A20 Materialize god fn `Materialize()` (30 LOC)**：

```
materialize/materializer.go::Materialize (30 LOC, god)
   ↓ 拆
   ├─ materialize/materializer.go (<200 行, core)
   │     func Materialize(ctx, req) (Materialized, error)
   │
   ├─ materialize/materialize_prompts.go (<300 行, buildSystemPrompt + buildWaveSystemPrompt)
   │     func buildSystemPrompt(ctx, req) string
   │     func buildWaveSystemPrompt(ctx, req) string
   │
   └─ materialize/materialize_compressor.go (<300 行, compressMessages)
         func compressMessages(ctx, messages) messages
```

#### 2.2.3 S18 god fn 拆分

**D2-S18-A08 Sandbox AST god fn `Analyze()` (39 LOC)**：

```
enforce/tools/sandboxast/analyzer.go::Analyze (39 LOC, god)
   ↓ 拆
   ├─ enforce/tools/sandboxast/analyzer.go (<200 行, core)
   │     func Analyze(ctx, src) (Report, error)
   │
   ├─ enforce/tools/sandboxast/ast_walker.go (<300 行, walk function)
   │     func walk(node ast.Node, rules *RuleSet) []Finding
   │
   └─ enforce/tools/sandboxast/ast_rules.go (<300 行, rule registry)
         func defaultRules() *RuleSet
         func loadCustomRules(path) *RuleSet
```

**D2-S18-A09 Background god fn `RunBackground()` (26 LOC)**：

```
enforce/background.go::RunBackground (26 LOC, god)
   ↓ 拆
   ├─ enforce/background_registry.go (<300 行, CRUD)
   │     func Register(task) error
   │     func Get(id) Task
   │     func List() []Task
   │     func Delete(id) error
   │
   └─ enforce/background_notifications.go (<200 行, queue integration)
         func drainNotifications(ctx) []Notification
         func enqueueNotification(ctx, n) error
```

### 2.3 F 层（路径修正 + Historical appendix）

#### 2.3.1 9 F 路径修正

| F ID | 旧路径 | 新路径 |
|---|---|---|
| D2-S15-A02-F01..F02 | `prepare/memory/longterm.go` | `prepare/memory/recall.go` + `persist/memory/store.go`（v8.2 P4 split） |
| D2-S15-A03-F01..F03 | `compression/pipeline.go` | `compression/pipeline.go` + `compression/compression_steps.go` + `compression/compression_budget.go`（PR-2 拆出） |
| D2-S15-A04-F01..F03 | `prepare/prompt/assembler.go` | `prepare/prompt/assembler.go` + `prompt/prompt_assembler_layers.go` + `prompt/prompt_dynamic_sections.go`（PR-2 拆出） |
| D2-S17-A02-F01..F03 | `persist/commit_window.go` | `persist/commit_window/` 子目录展开 |
| D2-S18-A01-F01..F03 | `permission/` | `permission/` 子目录展开 |
| D2-S18-A02-F01..F05 | `tools/` | `enforce/tools/{surface,sandboxast,builtin}/` 子目录展开 |
| D2-S16-A20-F01..F03 | `materialize/` | `materialize/{prompts,compressor}.go` 子文件展开 |
| t-registry `D2-S15-A02-T01..T04` | `prepare/conversation/repair_test.go` | 路径加 `prepare/conversation/` 前缀 |
| t-registry `D2-S18-A02-T01..T15` | `enforce/tools/*_test.go` | 路径加 `enforce/tools/` 前缀 |

#### 2.3.2 Historical appendix 段

```
## §Historical Appendix

### D2-S1: PEV (RETIRED v6.4.0)
- D2-S1-A01-T01..T04: PEV Execute / Gateway 四握 → D2-S10-A01-T34 (REMOVED → D7-S2-A06)
- D2-S1-A02-T02,T05,T06,T09: Verify commands (RETIRED)
- D2-S1-A02-T10: Shell injection → D2-S8-A01-T01

### D2-S9: Harness (REMOVED v6.5.0)
- D2-S9-A01-T01..T08: harness bootstrap / prefetch / tool_pool / preflight

### D2-S10: QueryLoop (REMOVED DM-20260618-010 → D7-S2-A06)
- D2-S10-A01-T01..T34: query.Loop / QueryLLMCaller / `query_loop.enabled`

### D2-S19: NestedExecution (DISMANTLED v6.4.0)
- D2-S19-A01-T01: Explore read-only → D2-S18-A02-T01
- D2-S19-A02-T01: Fork identical prefix → D2-S15-A02-T01

### D2-S20: LegacyHarnessFallback (REMOVED v6.5.0)
- D2-S20-A01-T01..T02: default skip harness / legacy bootstrap 一次
```

### 2.4 Boundary Decision（DM-018 slice-c + fixture type）

#### 2.4.1 DM-018 slice-c 物理迁移

**当前状态**：`nested/flow_report.go`（D2-S19 DISMANTLED 残留）持有 FlowEvent ownership，违反 D2 Follower 边界（D2 不应编排）。

**目标**：物理迁 `orchestration/executionflow/bridge/subquery_bridge.go`。

**约束**：
- D2→D7 已有 Hard Ban（D2-THIN-T01）
- D7→D2 反向 import 通过新建 `internal/shared/bridge/` 打破 cycle
- 不删旧实现（DSAFT 原则 3）

**治理常量**：
```go
// internal/layers/contextengine/orchtypes/boundary_decision.go (NEW, DM-20260629-002)
package orchtypes

// BoundaryDM018SliceC marks D2 nested/flow_report.go → D7 executionflow/bridge/subquery_bridge.go.
// v6.0.x 临时放在 D2 域（含 orchtypes/），归属待 v7.0 重新评估。
const BoundaryDM018SliceC = "boundary-debt:dm-018-slice-c-v7.0"
```

#### 2.4.2 fixture type 标

**当前状态**：`summarizer_fixture.go` + `prepared_turn_fixture.go` 在 D2 根跨 D2 + D7 使用。

**目标**：t-registry 显式标 fixture type。

```markdown
| D2-FIXTURE-A01-T01 | summarizer_fixture.go | fixture:cross-domain | — |
| D2-FIXTURE-A02-T01 | prepared_turn_fixture.go | fixture:cross-domain | — |
```

**约束**：fixture 物理位置不变（无 import cycle 风险）。

---

## §3 Span Evidence Coverage（94% 目标）

### 3.1 26 dead ops 删除清单

#### 3.1.1 Harness（6 ops）

| Span | 删除原因 |
|---|---|
| `context.harness.bootstrap.run` | D2-S20 REMOVED v6.5.0 |
| `context.harness.bootstrap.stage` | 同上 |
| `context.harness.tool_pool` | 同上 |
| `context.harness.preflight` | 同上 |
| `context.harness.route` | 同上 |
| `context.system_prompt.build` | 已合并到 `context.process` span |

#### 3.1.2 QueryLoop（3 ops）

| Span | 删除原因 |
|---|---|
| `query.loop.run` | DM-20260618-010 REMOVED → D7-S2-A06 |
| `query.loop.iterate` | 同上 |
| `query.loop.tool_batch` | 同上 |

#### 3.1.3 Context（8 ops）

| Span | 删除原因 |
|---|---|
| `context.snapshot.load` | 0 production emit |
| `context.system_prompt.load` | 已合并到 `context.process` |
| `context.longterm.recall` | 0 production emit（v8.2 P4 split 后） |
| `context.longterm.store` | 同上 |
| `context.tools.register` | 0 production emit |
| `context.memory.snapshot.save` | 已合并到 `context.process` |
| `context.token.count` | 0 production emit |
| `context.fork.subagent` | 0 production emit |

#### 3.1.4 Tool（2 ops）

| Span | 删除原因 |
|---|---|
| `tool.execute.single` | 0 production emit（D7 turn_adapter 接管） |
| `tool.execute.permission` | 0 production emit（同上） |

#### 3.1.5 Task/Plan（7 ops）

| Span | 删除原因 |
|---|---|
| `task.plan.create` | 已迁 D7 workmodel |
| `task.plan.execute` | 同上 |
| `task.plan.complete` | 同上 |
| `task.plan_mode.enter` | 同上 |
| `task.plan_mode.exit` | 同上 |
| `task.manager.register` | 同上 |
| `task.manager.dispatch` | 同上 |

### 3.2 4 active ops 保留

| Span | 覆盖 T | 触发点 |
|---|---|---|
| `context.process` | ~80 T | `kernel.ContextEngine.Process()` 主入口 |
| `context.compression.run` | ~50 T | `compression/pipeline.go::RunForSession()` |
| `context.compression.step` | ~40 T | `compression/compression_steps.go::step()` |
| `context.materialize` | ~50 T | `materialize/materializer.go::Materialize()` |

### 3.3 期望覆盖率

| 类别 | 数量 | 覆盖率 |
|---|---|---|
| 全部 T | 232 | 100% |
| 映射到 active ops | 220 | 94% |
| 未映射 T（标 `—`） | 12 | 6% |
| compile-time check / internal helper | 12 | — |

**期望**：94%（对齐 D7 实际达成）。

---

## §4 Coverage Guard CI

### 4.1 scripts/d2-span-coverage.sh

```bash
#!/usr/bin/env bash
set -euo pipefail

# D2 Span Evidence Coverage Guard
# 期望：94%（220/232 T 映射到 4 active ops）

D2_REGISTRY=openspec/specs/d2-context-engine/t-registry.md
ACTIVE_OPS=(
  "context.process"
  "context.compression.run"
  "context.compression.step"
  "context.materialize"
)
THRESHOLD=80

total=$(grep -cE "^\| D2-(S|FIXTURE)-" "$D2_REGISTRY" || true)
mapped=$(grep -cE "^\| D2-(S|FIXTURE)-.*\| (context\.(process|compression\.(run|step)|materialize)) " "$D2_REGISTRY" || true)
coverage=$((mapped * 100 / total))

echo "D2 Span Evidence Coverage: ${coverage}% (${mapped}/${total} T rows)"
echo "Threshold: ${THRESHOLD}%"

if [ "$coverage" -lt "$THRESHOLD" ]; then
  echo "FAIL: Coverage ${coverage}% < threshold ${THRESHOLD}%"
  exit 1
fi

echo "PASS: Coverage ${coverage}% >= threshold ${THRESHOLD}%"
```

### 4.2 集成位置

- PR-7 落地后接入 `verify-archive.sh` 第 13 项（与 D7 `verify-archive.sh` 对齐）
- 期望 12/12 → 13/13 PASS

---

## §5 Cross-Domain Boundary

### 5.1 DM-018 slice-c 跨域迁移路径

```
D2 internal/layers/contextengine/nested/flow_report.go
   ↓ 物理迁移
D7 internal/layers/orchestration/executionflow/bridge/subquery_bridge.go
```

**反向 import 处理**：
- D7 → D2 反向 import 通过 `internal/shared/bridge/` 打破 cycle
- D2 不再持有 FlowEvent ownership（D7-S4 ExecutionFlow + Verify 已 14 ExitReason 落地）

### 5.2 fixture type 跨域标记

| Fixture | 物理位置 | 跨域使用 | t-registry 标记 |
|---|---|---|---|
| `summarizer_fixture.go` | D2 根 | D2 + D7 | fixture:cross-domain |
| `prepared_turn_fixture.go` | D2 根 | D2 + D7 | fixture:cross-domain |

**约束**：
- fixture 物理位置不变（无 import cycle 风险）
- t-registry 显式标 fixture type 给未来跨域 fixture 重新分配留决策空间

---

## §6 ValueFlow Alias 详细设计

### 6.1 4 S ValueFlow Alias

| S 名 | ValueFlow Alias | 用户感知层 |
|---|---|---|
| S15 PrepareExecutionContext | **D2_Context_Loading_Compression** | 用户想"在不超 token 预算的前提下拿到完整上下文" |
| S16 Materialize Context | **D2_LLM_Ready_Assembly** | 用户想"按调用场景拿到 LLM-ready 上下文" |
| S17 PersistSessionState | **D2_Session_State_Persistence** | 用户想"会话状态可恢复 / 可审计" |
| S18 EnforceExecutionPolicy | **D2_Tool_Permission_Sandbox** | 用户想"工具调用按权限受控执行" |

### 6.2 加 `D2_` 前缀理由

- D7 alias（如 `Multi-Step Task Coordination`）未加 `D7_` 前缀（v6.0.0 博弈角色对齐精简时未加）
- D2 alias 加 `D2_` 前缀避免跨域 alias 命名冲突
- 用户感知层保留 D2 原语义

---

## §7 修订记录

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-29 | 初版：6 子 Change 联动设计 + 5 god fn 拆 10 文件 + 26 dead ops 删 + 94% Span Evidence 覆盖率 + DM-018 slice-c 跨域迁移 |