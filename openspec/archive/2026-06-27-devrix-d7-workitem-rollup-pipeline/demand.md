# Demand: D7 WorkItem Rollup 闭环 — 大任务可交付、可验收、可审计

**Demand ID:** DM-20260627-001  
**Created:** 2026-06-27  
**Reporter:** 用户（飞书 `review d2领域代码` + Jaeger trace 排查）  
**Priority:** P0  
**Sprint:** d7-worktree-closure  

---

## 原始反馈

> 1. 大任务（如 review 整个 d2 领域）运行 12+ 分钟，**最终总结不清晰**，用户看不到结构化 deliverable。  
> 2. LLM 口头说「并行 explore」，trace 显示 **11 轮串行 MUPS**，无 Wave/SubWorktree span。  
> 3. WorkTree 子节点跑完后，**父层未汇总**；`complete` 事件 Content 为空。  
> 4. 需要按 OpenSpec 先确认 **设计方案**（拆解理由、验收标准、向上传播内容），再编码。

## 1. 现象（Trace: `58e6c55dd4d42284e4c2bed3ebeda28b`）

| 指标 | 观测值 |
|------|--------|
| Session | `sess_1782526784130_9000` |
| 指令 | `review d2领域代码` |
| 时长 | ~754s，3331 spans |
| MUPS Pipeline | 11 次（11 个不同 `wi_*`） |
| TaskGraph | 11 次，`node_count=1` |
| SubTurn Iteration | 66 次 |
| Wave / SubWorktree | 0 |
| Root WorkItem | `wi_d44b61f0` Round1 **verdict=fail**（max_iters），spawn=none |
| 实际拆解 | `todo_write` checklist + `free_fork`×22，**绕过 SpawnPolicy** |
| complete | `summary_len=0`，飞书 `contentLen=28` |

## 2. L1–L5 映射（草案）

| 层级 | 映射 |
|------|------|
| **L1** | D7 Orchestration |
| **L2** | 不确定大任务递归探索与交付（open-ended review / multi-hypothesis） |
| **L3-BE** | RunSessionTurnLoop × WorkItem MUPS × WorkTree spawn/rollup |
| **L3-FE** | 飞书结构化回复卡 + 任务总结卡（D1） |
| **L4** | 见 proposal §Capabilities |
| **L5** | 见 `specs/d7-orchestration/spec_delta.md` + tasks.md T 标注 |

## 3. 业务目标

1. **每层 WorkItem** 能回答：为何拆 sub、解决什么、如何验收、向上反馈什么。  
2. **父层 Rollup Round**：子树 terminal 后，父再跑 MUPS synthesize，产出用户 deliverable。  
3. **向上传播分层**：Structured（强制）+ Summary（可选）+ 终局 complete(summary)。  
4. **向下监控受控**：Spawn 审计链；禁止 LLM 工具侧私自分解替代 D7 Decide。  
5. **并行路径**（Phase 2）：`SpawnParallelExplore` 接 Wave，非 stub。

## 4. 验收标准（Demand 级）

- [ ] **P0** 给定 decompose 场景（≥2 子 WorkItem 全 terminal），父 WorkItem 执行 **Round 2+** MUPS，Jaeger 可见同一 `wi_*` 两次 `D7_MUPS_Pipeline`。  
- [ ] **P0** 给定 **trace 重放场景**（root spawn=none + todo_write checklist）：root 2× MUPS、无 checklist MUPS、complete 含 P0/P1 结构。  
- [ ] **P0** 父 Round 2 Observe 含子 `ArtifactSummary` 或 **Virtual Checklist Bubble**（Path B）。  
- [ ] **P0** Session `complete` 事件 `Content` 含终局 summary（len≥阈值，review 类含 P0/P1 结构或等价章节）。  
- [ ] **P1** `SpawnParallelExplore` 非 no-op；trace 含 Wave schedule span。  
- [ ] **P1** `todo_write` / 非审计拆解不替代 `SpawnDecompose` 持久化子项（或显式 PromoteChecklist → Implement）。  
- [ ] 集成测试 + 关联 L5 全绿（见 tasks.md）。

## 5. 关联

- 设计 SoT：`workitem-pipeline-unification-design.md`（G1–G5，Phase D 遗留）  
- Context SoT：`workitem-context-graph-design.md`（§6 垂直 bubble）  
- 技术债：`openspec/tech-debt/worktree-v2-deferred.md`（TD-WT-02 Wave 投影）  
- 先例 change：`devrix-d7-mups-v4-5node-coverage-orchestration`

## 6. 澄清 Q&A（S2）

| # | 问题 | 决议（待用户确认可在 proposal Review 调整） |
|---|------|---------------------------------------------|
| Q1 | Rollup 是否强制对所有 decompose 父节点？ | **是**（Path A）；**Root goal + checklist** 走 Path B Fallback（spawn=none 亦触发） |
| Q5 | todo_write checklist 如何处理？ | **GetFocus 跳过 ephemeral**；rollup 用 Virtual Checklist Bubble，**不**对每条 checklist 跑 MUPS |
| Q2 | complete 空 Content hotfix 是否回滚？ | **是** — 空 Content 仅用于抑制 D7 内部 metadata；deliverable summary 必须写入 |
| Q3 | Phase 1 是否包含 ParallelExplore？ | **否** — Phase 1 = Rollup + Bubble；Phase 2 = Wave + DecomposeProposer LLM |
| Q4 | review 类 FailureCriteria 谁定义？ | Plan 模板 + Directive 后缀；Verify 仍 deterministic，Phase 3 可接 LLM Verifier |
| Q6 | TurnLoop×WorkTree×MUPS 主线终审？ | 见 **review-r1.md**；**2026-06-27 决议已冻结**，S4 开发中 |
| Q7 | R1-V1 Rollup verify？ | **IT stub + 生产 heuristic**（len≥500、P0/P1、planning 黑名单） |
| Q8 | R1-V2 Learn？ | Rollup 终局 Verdict 写 Reputation |
