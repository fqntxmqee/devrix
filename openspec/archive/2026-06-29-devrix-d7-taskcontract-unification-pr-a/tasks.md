# Tasks: D7 TaskContract 统一 PR-A

**Change ID:** devrix-d7-taskcontract-unification-pr-a
**Demand ID:** DM-20260629-007
**Status:** S4_Tasks
**Total Tasks:** 17 steps → 7 T points → 6 AC
**Estimated:** ~800 LOC 新增 / ~50 LOC 修改

---

## §1 任务总览

| Step | 内容 | 文件 | LOC | AC | T |
|------|------|------|-----|----|----|
| 1 | `interfaces/doc.go` 包文档 | NEW | +30 | AC1, AC2 | — |
| 2 | `interfaces/errors.go` 5 ORCH_* errors | NEW | +80 | AC1, AC2, AC3, AC5 | — |
| 3 | `interfaces/task_spec.go` TaskSpec struct + builder | NEW | +180 | AC1 | T01, T02 |
| 4 | `interfaces/task_report.go` TaskReport + 5 子类型 + builder | NEW | +280 | AC2, AC3, AC4, AC5 | T04, T05 |
| 5 | `interfaces/task_spec_test.go` 单测 | NEW | +200 | AC1 | T01, T02, T03 |
| 6 | `interfaces/task_report_test.go` 单测 | NEW | +280 | AC2, AC3, AC4, AC5 | T04, T05, T06 |
| 7 | `interfaces/taskcontract_test.go` Round-trip | NEW | +150 | AC1, AC2 | T01, T04 |
| 8 | `mups/execute/channel.go` 创建路径 → TaskSpec | MOD | +20 | AC1 | T03 |
| 9 | `mups/execute/{commit,scenario,protocol}.go` 出口 → TaskReport | MOD | +30 | AC2 | T06 |
| 10 | `mups/execute/exploration.go` 全量结果 → TaskReport.Dissent | MOD | +20 | AC3 | T07 |
| 11 | `mups/learn/learner.go` 入口 → TaskReport | MOD | +20 | AC2, AC3 | T06 |
| 12 | `workmodel/workitem.go` 创建路径 → TaskSpec | MOD | +15 | AC1 | T03 |
| 13 | `workmodel/child_downlink.go` 嵌入 TaskSpec 引用 | MOD | +10 | AC1 | T03 |
| 14 | `decisionplanning/decomposer.go` 分解 → TaskSpec + Resource | MOD | +15 | AC1, AC5 | T03, T09 |
| 15 | `decisionplanning/filter.go` Dissent 过滤 | MOD | +15 | AC3 | T07 |
| 16 | `sessionorchestrator/{orchestrator,turn_orchestrator}.go` 消费 | MOD | +30 | AC1, AC2 | T03, T06 |
| 17 | `wavescheduler/scheduler.go` dispatchOne 接收 TaskSpec | MOD | +10 | AC1 | T03 |
| 18 | `orchtypes/errors.go` 新增 5 ORCH_* SentinelError | MOD | +50 | AC1, AC2, AC3, AC5 | — |
| 19 | `specs/d7-orchestration/spec.md` ADDED 3 Requirement | MOD | +100 | AC17 | T10 |
| 20 | `specs/d7-orchestration/d7-domain.md` §8 + §9 | MOD | +30 | AC17 | T11 |
| 21 | `specs/d7-orchestration/{a,f,t,span}-registry.md` 增量 | MOD | +50 | AC17 | T11 |

---

## §2 F-T 映射

| F (Function) | T (Test) | 描述 |
|-------------|---------|------|
| D7-S20-A01-F01 NewTaskSpec | D7-S20-A01-T01 | 构造 + Validate happy path |
| D7-S20-A01-F02 TaskSpec.With* | D7-S20-A01-T02 | 不可变 builder (浅拷贝验证) |
| D7-S20-A01-F03 TaskSpec.Validate | D7-S20-A01-T01 | 同上 |
| D7-S20-A01 (创建点统一) | D7-S20-A01-T03 | Plan / Channel / WorkItem 3 处迁移 |
| D7-S20-A02-F01 NewTaskReport | D7-S20-A02-T04 | 构造 + Validate happy path |
| D7-S20-A02-F02 TaskReport.With* | D7-S20-A02-T05 | 不可变 builder + AppendDissent |
| D7-S20-A02-F03 TaskReport.AppendDissent | D7-S20-A02-T05 + D7-S21-A01-T07 | Dissent 填充 |
| D7-S20-A02 (出口+入口统一) | D7-S20-A02-T06 | Channel.Execute + Learn 节点 |
| D7-S20-A03-F01 spec.md delta | D7-S20-A03-T10 | 3 ADDED Requirement |
| D7-S20-A03-F02 d7-domain.md §8 | D7-S20-A03-T11 | Layer 架构 + interfaces 包章节 |
| D7-S21-A01-F01 Exploration → Dissent | D7-S21-A01-T07 | top-3 截断 + summary hash + Learn 沉淀 |
| D7-S21-A02-F01 Verifier → Blockage | D7-S21-A02-T08 | 3 类 kind 分类 |
| D7-S21-A03-F01 ContextBudget → Resource | D7-S21-A03-T09 | token/time/step 抽取 |

---

## §3 7 个 T 点详细

### T01: D7-S20-A01-T01 NewTaskSpec + Validate happy path [P0]
- **测试目标**：构造合法 TaskSpec（Goal 非空 + TraceID 自动生成）+ Validate 通过
- **测试矩阵**：
  - TC1: NewTaskSpec("fix bug") → TaskSpec{Goal: "fix bug", TraceID: "ts_<uuid8>", ...}
  - TC2: NewTaskSpec("") → ErrTaskSpecGoalEmpty
  - TC3: spec.Validate() 字段校验全过
  - TC4: NewTaskSpec + 9 次连续构造 → 9 个不同 TraceID（uuid 唯一性）
- **位置**：`internal/layers/orchestration/interfaces/task_spec_test.go`

### T02: D7-S20-A01-T02 TaskSpec.With* 不可变 builder [P0]
- **测试目标**：验证 `With*` 返回新副本，原对象不变
- **测试矩阵**：
  - TC1: `s := NewTaskSpec("a"); s2 := s.WithConstraint("k", "v", true); s.Constraints != s2.Constraints`
  - TC2: `s := NewTaskSpec("a"); s2 := s.WithBudget(...); s.CostBudget != s2.CostBudget`
  - TC3: `With*` chain 5 层 → 5 层全部独立对象（不同指针）
  - TC4: 并发安全（-race 测试）：100 goroutines 同时 `With*` 同一原始 spec → 无 data race
- **位置**：`internal/layers/orchestration/interfaces/task_spec_test.go`

### T03: D7-S20-A01-T03 TaskSpec 3 处创建点统一 [P0]
- **测试目标**：验证 Plan / Channel / WorkItem 3 处创建点全部走 `interfaces.NewTaskSpec`
- **测试矩阵**：
  - TC1: `mups/execute/channel.go::NewChannelRequest` 内部调用 `interfaces.NewTaskSpec`
  - TC2: `workmodel/workitem.go::NewWorkItem` 内部调用 `interfaces.NewTaskSpec`
  - TC3: `decisionplanning/decomposer.go::Decompose` 返回 `*interfaces.TaskSpec`
  - TC4: 集成测试：`Channel.Execute(ctx, request)` 内部 `request.Spec` 是有效 TaskSpec
- **位置**：`internal/layers/orchestration/interfaces/taskcontract_test.go` + 集成测试

### T04: D7-S20-A02-T01 NewTaskReport + Validate happy path [P0]
- **测试目标**：构造合法 TaskReport + Validate 通过
- **测试矩阵**：
  - TC1: `NewTaskReport(spec.TraceID)` → TaskReport{TraceID: ..., Result: ResultKindPending, ...}
  - TC2: `NewTaskReport("")` → ErrTaskReportTraceIDEmpty
  - TC3: report.Validate() 字段校验全过
  - TC4: `NewTaskReport` 自动从 spec 继承 TraceID（如果 spec 非空）
- **位置**：`internal/layers/orchestration/interfaces/task_report_test.go`

### T05: D7-S20-A02-T02 TaskReport.With* + AppendDissent 不可变 [P0]
- **测试目标**：验证 `With*` + `AppendDissent` 返回新副本
- **测试矩阵**：
  - TC1: `r := NewTaskReport(id); r2 := r.WithResult(...); r.Result != r2.Result`
  - TC2: `AppendDissent(entry)` 浅拷贝 + append，原 `r.Dissent` 长度不变
  - TC3: top-3 截断：`AppendDissent` 5 次 → 只保留前 3 个（max）+ 后面静默忽略 + log warn
  - TC4: 并发安全（-race 测试）：100 goroutines 同时 `AppendDissent` 同一原始 report → 无 data race
- **位置**：`internal/layers/orchestration/interfaces/task_report_test.go`

### T06: D7-S20-A02-T03 Channel.Execute 出口 + Learn 节点入口统一 [P0]
- **测试目标**：验证 Channel.Execute 出口 + Learn 节点入口都走 `interfaces.NewTaskReport`
- **测试矩阵**：
  - TC1: `mups/execute/commit.go::Execute` 返回 `*interfaces.TaskReport`
  - TC2: `mups/learn/learner.go::Learn` 接收 `*interfaces.TaskReport`
  - TC3: 集成测试：Worker → Channel.Execute → 5 层 CB → Learn → 全链路 TaskReport 一致
  - TC4: `LearnRequest.Report` 字段存在（additive，老字段保留）
- **位置**：`internal/layers/orchestration/interfaces/taskcontract_test.go` + 集成测试

### T07: D7-S21-A01-T01 Dissent top-3 + summary hash + Learn 沉淀 [P0]
- **测试目标**：验证 Dissent 字段填充规则
- **测试矩阵**：
  - TC1: ExplorationChannel 全量 5 候选 → Dissent 保留 top-3（minority）+ summary hash
  - TC2: 触发条件：`Result.Kind == Indeterminate` 或 `fallbackUsed=true` → Dissent 填充
  - TC3: 触发条件：`Result.Kind == Pass` → Dissent 不填充（默认空）
  - TC4: Learn 节点消费 Dissent → SkillMemory.SOP 写入验证
  - TC5: 同 entry 多次 `AppendDissent` → Learn 节点按 hash dedup
- **位置**：`internal/layers/orchestration/interfaces/task_report_test.go` + `internal/layers/orchestration/mups/learn/dissent_test.go`

### T08: D7-S21-A02-T01 Blockage 字段 3 类 kind 分类 [P0]
- **测试目标**：验证 Blockage 字段结构化分类
- **测试矩阵**：
  - TC1: Verifier reject "missing_input" → Blockage.Kind = Missing
  - TC2: Verifier reject "infeasible_path" → Blockage.Kind = Infeasible
  - TC3: Verifier reject "required_external" → Blockage.Kind = RequiredExternal
  - TC4: Blockage.Description + Source + Traceback 三字段填充完整
  - TC5: 多 Blockage 累积：`WithBlockage` 多次 → BlockageList 追加（不同 kind 可共存）
- **位置**：`internal/layers/orchestration/interfaces/task_report_test.go` + `decisionplanning/filter_test.go`

### T09: D7-S21-A03-T01 Resource 字段 token/time/step 抽取 [P0]
- **测试目标**：验证 Resource 字段从 ContextBudget Phase B 抽取
- **测试矩阵**：
  - TC1: `Resource.TokensUsed = budget.TokensUsed()` + `TokensBudget = spec.CostBudget.Tokens`
  - TC2: `Resource.TimeElapsed = budget.Elapsed()`（单位 ms）
  - TC3: `Resource.StepCount = budget.StepCount()` + `ToolInvocations = budget.ToolCalls()`
  - TC4: 单位一致性：tokens ≥ 0, time ≥ 0, steps ≥ 0, 任意 < 0 → ErrResourceInvalid
  - TC5: Resource 字段不可变：`WithResource` 返回新副本
- **位置**：`internal/layers/orchestration/interfaces/task_report_test.go` + `decisionplanning/decomposer_test.go`

### T10: D7-S20-A03-T01 spec.md v7.0 ADDED Requirement [P0]
- **测试目标**：spec.md 同步 3 ADDED Requirement（TaskSpec + TaskReport + Dissent/Blockage/Resource）
- **测试矩阵**：
  - TC1: `## ADDED Requirements` 章节存在 + 3 个 Requirement 子章节
  - TC2: 每个 Requirement 含 Scenario（Gherkin Given/When/Then）+ `<!-- T: -->` 注释
  - TC3: `spec.md` 版本号 v6.0.x → v7.0.0（minor bump）
- **位置**：`openspec/specs/d7-orchestration/spec.md`

### T11: D7-S20-A03-T02 d7-domain.md §8 + a/f/t/span-registry 增量 [P0]
- **测试目标**：spec 5 文件同步（d7-domain.md + a/f/t/span-registry.md）
- **测试矩阵**：
  - TC1: `d7-domain.md` 新增 §8 Layer 架构说明 + §9 interfaces 包章节
  - TC2: `a-registry.md` 新增 6 个 A（D7-S20-A01/A02/A03 + D7-S21-A01/A02/A03）
  - TC3: `f-registry.md` 新增 11 个 F
  - TC4: `t-registry.md` 新增 4 个 S（D7-S20/21/22/23）+ 7 个 T 点
  - TC5: `span-registry.md` 新增 5 个 span（d7.s20.* × 2 + d7.s21.* × 3）
- **位置**：`openspec/specs/d7-orchestration/{d7-domain,a-registry,f-registry,t-registry,span-registry}.md`

---

## §4 任务依赖图

```
Step 1 (doc.go) ─────────────────────────┐
Step 2 (errors.go) ──────────────────────┤
Step 3 (task_spec.go) ───────────────────┤
Step 4 (task_report.go) ─────────────────┤
Step 5 (task_spec_test.go) ──→ 依赖 Step 3│
Step 6 (task_report_test.go) ─→ 依赖 Step 4
Step 7 (taskcontract_test.go) ─→ 依赖 Step 3, 4
                                        │
Step 8-17 (D7 子包修改) ──→ 依赖 Step 3, 4
                                        │
Step 18 (orchtypes/errors.go) ──→ 依赖 Step 2
                                        │
Step 19-21 (spec 文档同步) ──→ 依赖 Step 3-18 (并行)
```

**执行顺序**：
1. **Phase 1**（interfaces 包骨架）：Step 1-7（顺序执行，约 3 天）
2. **Phase 2**（D7 子包修改）：Step 8-17（可部分并行，约 2 天）
3. **Phase 3**（spec 同步）：Step 19-21（约 1 天）
4. **Phase 4**（验证）：22/22 race + LP-1/LP-2/LP-5 + Coverage + P99 benchmark（约 1 天）

---

## §5 S5 验收门槛

- [ ] Step 1-21 全部完成
- [ ] 22/22 orchestration packages `go test -race -count=1` PASS
- [ ] LP-1 / LP-2 / LP-5 集成测试 100% PASS（回归测试集）
- [ ] `interfaces` 包 `go test -cover` ≥ 80%
- [ ] `interfaces` 包 0 import D7 任何子包（layout guard 静态检查）
- [ ] TaskSpec / TaskReport 构造 P99 < 1ms（benchmark）
- [ ] 5 个 ORCH_* SentinelError 定义完整
- [ ] 5+1 个 spec 文档同步完成（git diff 非空）
- [ ] T01-T11 全部 PASS（7 个 T 点 × 子 case 总计 50+ 测试用例）

---

## §6 与父 DESIGN 的差异（PR-A scope 限定）

| 维度 | 父 DESIGN (DM-20260629-006) | PR-A scope (DM-20260629-007) |
|------|----------------------------|------------------------------|
| S 编号 | D7-S16/17/18/19 | **D7-S20/21/22/23**（重映射）|
| AC 数量 | 23 AC（全部 3 PR）| 6 AC（仅 PR-A）|
| T 点数量 | 27 个 | 11 个（7 T 点 × 子 case 总计 50+ 用例）|
| 文件清单 | 7 新增 + 19 修改 + 6 spec | 7 新增 + 14 修改 + 6 spec（无 boundary/benchmark/security/hard_evidence/version_chain/mvp_artifact）|
| L3 防御运行时 | 5 AC 全部 | 0（PR-B + PR-C 实施）|
| L4 治理横切层 | 12 AC 全部 | 1 AC（仅 AC17 spec 同步）|
| 风险等级 | 中-高 | **低** |

PR-A 是父 DESIGN 的**第一阶段实施**，不替代父 DESIGN 的完整性。PR-B / PR-C 在 PR-A 闭环基础上推进。