# Review: D7 TaskContract 统一 PR-B — Pessimistic Commit + Rule-based Fallback Design Review

**Change ID:** devrix-d7-taskcontract-unification-pr-b
**Demand ID:** DM-20260629-008
**Phase:** S3-Gate Review
**Reviewer:** Cursor (Claude Code)
**Created:** 2026-06-29
**Status:** S3-Gate PASS

---

## 1. 评审范围

`openspec/changes/devrix-d7-taskcontract-unification-pr-b/`:
- `demand.md` (DM-20260629-008)
- `proposal.md`
- `design.md` (六段式 ①-⑥ + 5 附录)
- `tasks.md` (S4-S6 30 任务)

父设计引用：`openspec/archive/2026-06-29-devrix-d7-taskcontract-unification/design.md` (DM-20260629-006, 648 行)。

## 2. 评审维度 (data/logic/boundary/call/exception per `feedback-design-doc-review-focus.md`)

### 2.1 数据 (Data)

| 项 | 评估 | 备注 |
|----|------|------|
| ConvergenceBudget 值对象 | ✅ | 5 字段 + 1 不可变 builder |
| FallbackPolicy 3 态 | ✅ | iota +1 防 zero value 误用 |
| MVPArtifact 5 字段 | ✅ | 与 TaskReport.WithMVPArtifact 桥接 |
| PessimisticCommitGuard interface | ✅ | 3 method + 4 SentinelError |
| 与 PR-A 字段语义一致 | ✅ | MVPArtifact 复用 With* immutable 模式 |

### 2.2 逻辑 (Logic)

| 项 | 评估 | 备注 |
|----|------|------|
| 5 类触发条件 → 决策树 | ✅ | 见 design §3.1 + 父设计 §3.4 |
| 5 层 CB 升级路径 | ✅ | 仅 L1 升级 → NotifyPessimistic，L2-L5 不变 |
| Rule-based 4 候选规则 | ✅ | default min_uncertainty + env 切换 |
| Feature Flag 默认 disabled | ✅ | IV-6 不变量 |
| 0 行为变更承诺 | ✅ | 与 PR-A 一致原则 |

### 2.3 边界 (Boundary)

| 项 | 评估 | 备注 |
|----|------|------|
| interfaces/ 0 import D7 子包 | ✅ | IV-1 守 |
| escape/ 只新增不修改 | ✅ | 1 NEW + 2 MOD (additive) |
| mups/execute/channel.go 仅 +15 行 | ✅ | Execute 出口决策 |
| 错误码区间 7110-7113 | ✅ | 7110-7119 PR-B / 7120-7129 PR-C 区间划分清晰 |
| Feature Flag env 注入 | ✅ | d7-bootstrap/wire.go 集中 |

### 2.4 调用 (Call)

| 项 | 评估 | 备注 |
|----|------|------|
| Channel.Execute → guard.Evaluate | ✅ | mups/execute/channel.go +15 LOC |
| CB L1 → engine.NotifyPessimistic | ✅ | escape/circuit_breaker.go +20 LOC |
| engine.NotifyPessimistic → fallback.Resolve | ✅ | escape/engine.go +30 LOC |
| Resolve → FallbackPolicy.Select | ✅ | escape/fallback.go 3 实现 |
| fallback.BuildMVPArtifact → TaskReport.WithMVPArtifact | ✅ | 复用 PR-A With* 模式 |

### 2.5 异常 (Exception)

| 项 | 评估 | 备注 |
|----|------|------|
| 4 ORCH_PESSIMISTIC_* / ORCH_FALLBACK_* (7110-7113) | ✅ | sharederrors.WithCode 模式与 PR-A 一致 |
| ErrORCHPessimisticTriggered (7110) | ✅ | 资源耗尽 / CB L1 / 空证 PASS |
| ErrORCHPessimisticMVPEmpty (7111) | ✅ | MVPArtifact 输出为空防御 |
| ErrORCHFallbackRuleInvalid (7112) | ✅ | Rule-based 规则未识别 |
| ErrORCHFallbackAbortTimeout (7113) | ✅ | FallbackAbort 超时 |
| Feature Flag 灰度 false negative | ⚠️ | staging 24h 烟测前置 |

## 3. 关键不变量 (4+2)

| IV | 内容 | 验证 |
|----|------|------|
| IV-1 | interfaces/ 0 import D7 子包 | `grep -r 'orchestration/' interfaces/ \| grep -v _test` → 0 |
| IV-2 | FallbackPolicy immutable enum | `const` + 无 setter |
| IV-3 | ConvergenceBudget immutable | `c := *b` 浅拷贝 + With* builder |
| IV-4 | MVPArtifact immutable | 构造函数 + 字段全 basic type |
| IV-5 | PessimisticCommitGuard 4 错误码仅 escape+interfaces | `grep sharederrors 跨包引用` 仅在 2 包 |
| IV-6 | Feature Flag 默认 disabled (新增) | `D7_PESSIMISTIC_COMMIT_ENABLED` env check |

## 4. 与 PR-A 边界 (5 维度对齐)

| 维度 | PR-A | PR-B | 评估 |
|------|------|------|------|
| 物理位置 | interfaces/ (7 NEW) | interfaces/contracts.go + escape/ (3 NEW) | ✅ |
| 接口层 | TaskSpec/TaskReport (L1) | PessimisticCommitGuard (L3) | ✅ |
| 字段语义 | 5+2 字段 (L2) | FallbackPolicy + ConvergenceBudget (L3) | ✅ |
| 调用点 | ChannelRequest.Spec / LearnRequest.Report (additive) | Channel.Execute 出口 + EscapeEngine (additive) | ✅ |
| 行为变更 | 0 (默认 use) | 0 (Feature Flag 默认 disabled) | ✅ |

## 5. 复用 PR-A 资产 (5 项)

- `interfaces.TaskSpec` (PessimisticCommitGuard.Evaluate 接收 `*TaskSpec`)
- `interfaces.TaskReport` (出口返回 `*TaskReport`)
- `interfaces.Dissent` (MVPArtifact.RiskWarnings 可与 Dissent 互补)
- `sharederrors.WithCode` 模式 (4 错误码 7110-7113)
- `interfaces/ 0 import` 原则 (IV-1)

## 6. 六段式自检 (per `devrix-architecture-design-six-segment-migration` DM-20260629-007)

- [x] **① 设计目标与约束** (3 类约束显式列出 + 5 关键决策)
- [x] **② 领域模型** (3 聚合根 + 1 interface + 6 不变量)
- [x] **③ 业务流程** (5 类触发 + 5 层 CB 升级)
- [x] **④ 接口契约** (PessimisticCommitGuard interface + 2 值对象 + 1 builder 扩展)
- [x] **⑤ 实现路径** (7 NEW + 4 MODIFIED + 8 步实施顺序)
- [x] **⑥ 验证与风险** (6 验证项 + 4 风险 + Rollback + 回归)
- [x] **5 附录** (File Manifest + Rollback + 回归 + S3 Checklist + 下一步)

## 7. 4 AC + 7 T 矩阵 (design §3 验证)

| AC | T | 实施 | 评估 |
|----|---|------|------|
| AC11 Pessimistic Commit | T01 happy | interfaces/contracts.go::Evaluate | ✅ |
| AC11 | T02 资源耗尽 → MVP | escape/fallback.go::FallbackPessimistic | ✅ |
| AC11 | T03 5 类触发 | escape/engine + circuit_breaker | ✅ |
| AC12 Rule-based | T01 4 规则 | escape/fallback.go::FallbackRuleBased | ✅ |
| AC12 | T02 env 切换 | escape/fallback.go | ✅ |
| AC16 Feature Flag | T01 env-gated | d7-bootstrap/wire.go | ✅ |
| AC18 可观测 | T01 Span + Metric | escape/engine.go + hardening | ✅ |

## 8. 风险评估 (4 类 + 缓解)

| 风险 | 概率 | 缓解 |
|------|------|------|
| Feature Flag 灰度 false negative | 中 | 灰度分桶 1% → 10% → 50% → 100% |
| 4 候选规则实现复杂度 | 低 | 先 min_uncertainty，其他 3 个 stub + TODO v7.0.1 |
| CB L1 Pessimistic action 复杂度 | 中 | 复用 PR-A Blockage 字段，L1 仅 action 之一 |
| 错误码区间 7100-7119 划分 | 0 | 区间划分：7100-7109 PR-A / 7110-7119 PR-B / 7120-7129 PR-C |

## 9. 与 DM-20260629-006 父设计一致性

| 父设计章节 | PR-B 实施 | 一致性 |
|------------|-----------|--------|
| §1.2 4 维不均衡 | L3 防御运行时层补足 | ✅ |
| §3.4 决策树 | 完整沿用 + 实施点 | ✅ |
| §④ 4 聚合根 | ConvergenceBudget + MVPArtifact + FallbackPolicy 3 落地 | ✅ |
| §5.1 5 层 CB | L1 升级 → Pessimistic action | ✅ |
| §5.3 CoW VersionChain | 留 PR-C 实施 | ✅ (scope 边界) |
| §B Rollback Plan | 沿用 D7_PESSIMISTIC_COMMIT_ENABLED 关停 | ✅ |
| §7 验证策略 | 6 验证项 + 灰度 staging 24h | ✅ |
| §6 实施周期 | 4.5 周 → PR-B 1 周 | ✅ (符合 4-Layer × 3-Phase 拆分) |

## 10. S3-Gate Verdict

**PASS** — 10 维度评估全绿，6 段式自检完整，4 AC + 7 T 矩阵设计清晰，与 PR-A 边界清晰 (0 行为变更承诺 + 错误码区间划分 + IV 不变量 6 项)，可推进 S4 实施。

---

**Reviewer:** Cursor (Claude Code)
**Date:** 2026-06-29
**Verdict:** S3-Gate PASS → 可推进 S4 Implementation
