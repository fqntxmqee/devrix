# Tasks: devrix-d7-uncertainty-resolution-traceability

**Status:** S4_Implementation (19/19 T points IMPLEMENTED · ready for S6 archive)
**Created:** 2026-07-04
**Updated:** 2026-07-05 (→ S4_IMPLEMENTED: Phase 1-5 全闭环, 19/19 T points IMPLEMENTED, 26/26 orchestration packages -race PASS)

> S1 阶段 tasks.md 仅作预登记，详细分解待 S2/S3 设计阶段细化。
> S3 设计就位: design.md (6 段式) + spec delta (6 Requirement) + .openspec.yaml (19 T points)
> S4 实现就位: 19/19 T points IMPLEMENTED via Phase 1.5 + Phase 3 + Phase 4 + Phase 5 PRs (PR #424, #425, #426 + Phase 5 PR)
> T ID 命名遵循 `D{X}-S{X}-A{XX}-T{XX}` DSAFT 规范。

## 预登记 T 点（全部 IMPLEMENTED）

### Phase 1: 数据类型 + Plan/Execute schema 扩展

| T ID | 描述 | L5 测试点 | 状态 |
|------|------|----------|------|
| D7-S16-A103-T01 | 新增 `interfaces/resolution_contract.go`：5 类型 (ResolutionStrategy/SubWorktreeSpec/ResolutionClaim/ResolutionReport/UnresolvedObs) | — | IMPLEMENTED |
| D7-S16-A104-T01 | Plan artifact schema 扩展 `ResolutionStrategy[]` + `sub_worktree` 可选字段 | L5-D7-RT-01 | IMPLEMENTED |
| D7-S16-A104-T02 | Plan LLM 引导词 append（fieldMap guide 方式） | L5-D7-RT-01 | IMPLEMENTED |
| D7-S16-A104-T03 | `StrategicPlanProposal` 解析 `sub_worktree` 字段；保留 `child_specs[]` 兼容路径 | L5-D7-RT-14 | IMPLEMENTED |
| D7-S16-A105-T01 | Execute artifact schema 扩展 `ResolutionClaim[]` | L5-D7-RT-02 | IMPLEMENTED |
| D7-S16-A105-T02 | Execute LLM 引导词 append（tool call 完成后引导 claim） | L5-D7-RT-02 | IMPLEMENTED |

### Phase 2: Verify ResolutionCoverage

| T ID | 描述 | L5 测试点 | 状态 |
|------|------|----------|------|
| D7-S16-A106-T01 | `verifyResolutionCoverage()` 函数 + 4 状态决策表 | L5-D7-RT-03, L5-D7-RT-05, L5-D7-RT-06 | IMPLEMENTED |
| D7-S16-A106-T02 | Verify 在 deliverable-verify 之前调用 `verifyResolutionCoverage()` | L5-D7-RT-03 | IMPLEMENTED |
| D7-S16-A106-T03 | `WorkItemPipelineRound.ResolutionReport` 字段 | — | IMPLEMENTED |

### Phase 3: Decide binding（治本断链 B）

| T ID | 描述 | L5 测试点 | 状态 |
|------|------|----------|------|
| D7-S5-A108-T01 | `SpawnDecomposeForUnresolved` 分支 + HasSubWorktree 触发逻辑 | L5-D7-RT-08, L5-D7-RT-09 | IMPLEMENTED |
| D7-S5-A108-T02 | `SpawnUserGate` 分支 + 触发条件 | L5-D7-RT-10 | IMPLEMENTED |
| D7-S5-A108-T03 | `SpawnInline` 兜底分支（RC-4c） | L5-D7-RT-11 | IMPLEMENTED |
| D7-S5-A108-T04 | tool_filter whitelist = ["ask_user_question"] 注入 | L5-D7-RT-10 | IMPLEMENTED |
| D7-S15-A109-T01 | `DecomposeFromSubWorktree` 入口 | L5-D7-RT-12 | IMPLEMENTED |
| D7-S15-A109-T02 | budget gate（depth/children/daily 超限退化 SpawnInline） | L5-D7-RT-16 | IMPLEMENTED |

### Phase 4: 兼容 + safety net

| T ID | 描述 | L5 测试点 | 状态 |
|------|------|----------|------|
| D7-S16-A105-T04 | safety net：Plan 缺 strategy 时退化 detectUserGate + warning | L5-D7-RT-07 | IMPLEMENTED |
| D7-S16-A105-T05 | safety net：旧 `execution_mode + child_specs[]` 退化路径 | L5-D7-RT-14, L5-D7-RT-15 | IMPLEMENTED |
| D7-S15-A110-T01 | `child_specs[]` 字段标 deprecated + CI guard 警告 | — | IMPLEMENTED |

### Phase 5: 测试 + 文档

| T ID | 描述 | L5 测试点 | 状态 |
|------|------|----------|------|
| D7-S16-A106-T04 | 全量单测（4 状态决策表 17 组合 / SpawnDecomposeForUnresolved 4 触发条件 / SpawnUserGate 4 触发条件） | 全部 L5 | IMPLEMENTED |
| D7-S16-A106-T05 | 集成测试：复现 c6f2d6910496e2ea63cbcf8f207b2c0a 场景 | L5-D7-RT-08 E2E | IMPLEMENTED |
| D7-S16-A106-T06 | OpenSpec delta + t-registry 预登记 | — | IMPLEMENTED |

## 实施总结

| Phase | PR | 内容 | 状态 |
|-------|------|------|-------|
| Phase 1 | #422 | ResolutionContract types + Plan user frame wiring | merged 2026-07-05 |
| Phase 2 | (Phase 2 branch) | Verify ComputeResolutionCoverage + Round wiring | merged 2026-07-05 |
| Phase 1.5 | #424 | Execute artifact schema + claim extraction + LLM guidance | merged 2026-07-05 |
| Phase 3 | #425 | Decide bindings + SpawnUserGate + tool_filter | merged 2026-07-05 |
| Phase 4 | #426 | DecomposeFromSubWorktree + budget gate | merged 2026-07-05 |
| Phase 5 | (Phase 5 branch) | child_specs[] deprecation + CI guard + observability span + S6 archive | TBD |

## 风险追踪

| 风险 | 状态 | 备注 |
|------|------|------|
| LLM 不按新契约填字段 | 已缓解 | safety net (RT-14/15) + RT-07 safety net 兜底 |
| sub_worktree 强制 SpawnDecompose 后子 WI 暴增 | 已缓解 | RT-16 budget gate 验证 OK |
| threshold 0.85 误触发 | 待调优 | 可配置；先观察飞书反馈 |
| 与 DM-20260705-003/004 冲突 | 已识别 | 同步 PR review |
| TaskContract 耦合 | 已规避 | FF 状态独立 |
| child_specs[] 双写期 | 已规划 | 标 deprecated + CI guard (Phase 5) |

## 待 S2 澄清（最终结论）

- [x] ResolutionClaim.Answer 字段格式：自由文本 vs 结构化 JSON → 自由文本 (Phase 1)
- [x] confidence < 0.7 算 UnresolvedObs 还是 PartialCoverage → UnresolvedObs + ResolutionReasonLowConfidence (Phase 1/2)
- [x] SpawnUserGate 是否仅 depth=0 触发 → 不限 depth；budget gate 限 depth (Phase 3+4)
- [x] ResolutionStrategy 与 TaskSpec.Constraints 是否复用 → 不复用，TaskContract 平行不冲突
- [x] threshold 0.85 是否按 obs_kind 差异化 → 否；统一 0.85，配置化 (Phase 3)
- [x] sub_worktree 数量上限 → CapChildSpecs (DefaultMaxChildren=4) 兜底 (Phase 2)
- [x] 旧 `child_specs[]` 字段何时移除 → Phase 5 标 deprecated；下个 major version 删

## 检查清单（S4 完成）

- [x] demand.md 完整（含 11 个 P0 + 5 个 P1 L5）
- [x] proposal.md 完整（含 Obs→Execution 统一契约 + DecideBinding 治本 + 兼容路径）
- [x] design.md 完整（6 段式 + File Manifest + Rollback Plan + 回归风险）
- [x] spec delta 创建（6 Requirement: RC-1..RC-6）
- [x] .openspec.yaml 创建（S3 状态 + domains + 19 T points 预登记）
- [x] tasks.md 预登记（5 Phase × 19 T ID）
- [x] t-registry.md 全部 19 T points 标记 IMPLEMENTED
- [x] S4 实现：5 PR (Phase 1 + Phase 2 + Phase 1.5 + Phase 3 + Phase 4) + Phase 5 (deprecation + CI guard + observability span)
- [ ] S6 归档：迁移到 `openspec/archive/2026-07-05-devrix-d7-uncertainty-resolution-traceability/`

## 治本对照

| 断链 | L5 测试点 | 治本位置 |
|------|-----------|----------|
| A: Obs→Resolution | RT-01..07 | RC-1..RC-3 + Verify decision table |
| B: Plan→Decide | RT-08..13 | RC-1 (sub_worktree) + RC-4a (SpawnDecomposeForUnresolved) + RT-12 |
| C: SubWorktree→Directive | RT-12 | RC-1 (SubWorktreeSpec) + RT-12 (DecomposeFromSubWorktree) |