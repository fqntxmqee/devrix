# Review: D7 TaskContract 统一 PR-C — CoW VersionChain + Similarity Check + Hard Evidence Design Review

**Change ID:** devrix-d7-taskcontract-unification-pr-c
**Demand ID:** DM-20260629-009
**Phase:** S3-Gate Review
**Reviewer:** Cursor (Claude Code)
**Created:** 2026-06-29
**Status:** S3-Gate PASS

---

## 1. 评审范围

`openspec/changes/devrix-d7-taskcontract-unification-pr-c/`:
- `demand.md` (DM-20260629-009)
- `proposal.md`
- `design.md` (六段式 ①-⑥ + 5 附录)
- `tasks.md` (S4-S6 13 P0 T + 13 S4 步骤 + 4 S5 + 10 S6)
- `specs/d7-orchestration/spec.md` (3 ADDED Requirements + 15 Gherkin Scenarios)

父设计引用：`openspec/archive/2026-06-29-devrix-d7-taskcontract-unification/design.md` (DM-20260629-006, 648 行)
父 PR-A：DM-20260629-007 (PR #325) ✅
父 PR-B：DM-20260629-008 (PR #327) ✅

## 2. 评审维度 (data/logic/boundary/call/exception per `feedback-design-doc-review-focus.md`)

### 2.1 数据 (Data)

| 项 | 评估 | 备注 |
|----|------|------|
| VersionChain (Hash + VersionChainEntry + Append/RollbackTo/GC) | ✅ | Hash = SHA-256 64-char hex；CoW 不可变 |
| SimilarityConfig + SimilarityResult + Jaccard | ✅ | 3 阈值常量 + O(\|A\|+\|B\|) 算法 |
| HardEvidence + Verified() + kind-specific | ✅ | 6 字段 + switch kind |
| 3 ORCH_* SentinelError (7120-7122) | ✅ | sharederrors.WithCode 模式与 PR-A/B 一致 |
| 与 PR-A/B 字段语义一致 | ✅ | 全部 `With*` 浅拷贝不可变 |

### 2.2 逻辑 (Logic)

| 项 | 评估 | 备注 |
|----|------|------|
| 3 类前置防御 → PR-B 路径 | ✅ | 见 design §3.1 完整流程 |
| CoW VersionChain 生命周期 | ✅ | Append → RollbackTo → GC 24h 周期 head 守护 |
| Similarity Check 拦截 + 边界 | ✅ | > 0.85 拦截 / 0.7-0.85 warn / < 0.70 通过 |
| Hard Evidence kind-specific | ✅ | code 要 test/log，chat 要 entity/coherence |
| 0 行为变更承诺 | ✅ | Feature Flag 默认 disabled (沿用 PR-A/B 模式) |
| Hash 算法升级 FNV-1a → SHA-256 | ✅ | 内部函数升级 + 外部接口不变 |

### 2.3 边界 (Boundary)

| 项 | 评估 | 备注 |
|----|------|------|
| interfaces/ 0 import D7 子包 | ✅ | IV-1 守 (3 NEW 仅依赖 `internal/shared/errors/`) |
| workmodel/ 仅新增不破坏 | ✅ | 2 NEW + 1 MOD (WorkItem additive `omitempty`) |
| 错误码区间 7120-7122 | ✅ | 7100-7109 PR-A / 7110-7119 PR-B / 7120-7129 PR-C 区间清晰 |
| Feature Flag env 注入 | ✅ | d7-bootstrap/wire.go +2 helper |
| Verifier kind-specific 严格分离 | ✅ | code vs chat 不互相误伤 |
| VersionChain GC head 守护 | ✅ | IV-3 不变量 (head ∉ deleted) |

### 2.4 调用 (Call)

| 项 | 评估 | 备注 |
|----|------|------|
| Decomposer.Synthesize → SimilarityChecker.Check | ✅ | decisionplanning/decomposer.go +40 LOC |
| Verifier.Verify → HardEvidence.Verified | ✅ | executionflow/verify/verifier.go +50 LOC |
| WorkItem.Commit → VersionChain.Append | ✅ | workmodel/version_chain.go +120 LOC |
| HardEvidence reject → Result.Kind=Partial | ✅ | 强制降级 + Blockage.RequiredExternal |
| 0.7-0.85 边界 slog.Warn | ✅ | 不阻塞 (IV-6 不变量) |

### 2.5 异常 (Exception)

| 项 | 评估 | 备注 |
|----|------|------|
| ErrCoWVersionChainBroken (7120) | ✅ | RollbackTo 找不到 / GC 误删 |
| ErrSimilarityCheckIntercepted (7121) | ✅ | > 0.85 强制 Abort |
| ErrHardEvidenceMissing (7122) | ✅ | 空证 PASS 拒绝 |
| Feature Flag 灰度 false negative | ⚠️ | staging 24h 烟测前置 |
| Hard Evidence 误伤 chat | ⚠️ | kind-specific + Feature Flag 灰度 |
| Similarity 边界 0.7-0.85 误判 | ⚠️ | slog.Warn 不阻塞 (LLM 二次校验留 v7.0.1) |
| Hash 碰撞 (SHA-256) | 极低 | 256-bit 安全 (2^-128) |

## 3. 关键不变量 (8 项)

| IV | 内容 | 验证 |
|----|------|------|
| **IV-1** | interfaces/ 0 import D7 子包 | `grep -r 'orchestration/' interfaces/ \| grep -v _test` → 0 |
| **IV-2** | VersionChain 不可变 | Append/RollbackTo/GC 全部浅拷贝返回新对象 |
| **IV-3** | head 永远不被 GC | GC 实现显式排除 head + 单元测试守护 |
| **IV-4** | Hash 唯一性 (SHA-256) | crypto/sha256 标准库 |
| **IV-5** | Hard Evidence kind-specific | Verified() 内 switch 严格分离 |
| **IV-6** | Similarity 边界 0.7-0.85 不阻塞 | CheckSimilarity 边界仅返回 Warn=true |
| **IV-7** | Feature Flag 默认 disabled | D7_HARD_EVIDENCE_ENABLED=false / D7_SIMILARITY_CHECK_ENABLED=false |
| **IV-8** | SHA-256 升级不影响外部接口 | Hash 字符串长度变化但语义不变 |

## 4. 与 PR-A/B 边界 (5 维度对齐)

| 维度 | PR-A (✅) | PR-B (✅) | **PR-C (本 PR)** |
|------|-----------|-----------|------------------|
| 物理位置 | interfaces/ (7 NEW) | interfaces/ + escape/ (10 NEW) | interfaces/ + workmodel/ (10 NEW) |
| 接口层 (L1) | TaskSpec/TaskReport | — | — |
| 字段语义层 (L2) | Dissent/Blockage/Resource | — | — |
| 防御运行时层 (L3) | — | Pessimistic + RuleBased | **CoW + Similarity + HardEvidence** |
| 治理横切层 (L4) | spec 同步 + race test + LP regression | FeatureFlag + Coverage | **T05 wire + convergence span + 灰度** |
| 行为变更 | 0 (默认 use) | 0 (Feature Flag 默认 disabled) | **0 (Feature Flag 默认 disabled)** |

## 5. 复用 PR-A/B 资产 (8 项)

- `interfaces.TaskSpec` (Similarity Check 读 `taskSpec.Goal`)
- `interfaces.TaskReport` (Hard Evidence 读 `report.Evidence` + `report.Result.Kind`)
- `interfaces.FallbackPolicy` (Similarity 拦截时复用 FallbackAbort 路径)
- `escape.DefaultPessimisticCommitGuard` (Hard Evidence + Similarity 在 PessimisticCommitGuard 入口前置 check)
- `sharederrors.WithCode` 模式 (3 错误码 7120-7122)
- `interfaces/ 0 import` 原则 (IV-1)
- `Feature Flag env-gated` 模式 (D7_HARD_EVIDENCE_ENABLED + D7_SIMILARITY_CHECK_ENABLED)
- `coordinator/aliases.go` v6.0.x legacy (不动；VersionChain 是 WorkItem 增量字段)

## 6. 六段式自检 (per `devrix-architecture-design-six-segment-migration` DM-20260629-007)

- [x] **① 设计目标与约束** (7 AC + T05 + 10 约束 + 5 关键决策)
- [x] **② 领域模型** (3 新增聚合 + 3 SentinelError + 8 不变量)
- [x] **③ 业务流程** (3 Gate → PR-B 路径 + 3 子流程)
- [x] **④ 接口契约** (3 interface + 2 Feature Flag + 3 新 span + 8 新 metric)
- [x] **⑤ 实现路径** (10 NEW + 7 MODIFIED + 5 阶段实施顺序 + 8 复用资产)
- [x] **⑥ 验证与风险** (6 验证命令 + 8 风险 + 4 阶段 Rollback + 7 维度回归)
- [x] **5 附录** (File Manifest + Rollback + 回归 + S3 Checklist + 下一步)

## 7. 6 AC + 13 T 矩阵 (design §3 验证)

| AC | T | 实施 | 评估 |
|----|---|------|------|
| **AC13 CoW VersionChain** | T01 VersionChain interface + SHA-256 | interfaces/version_chain.go | ✅ |
| AC13 | T02 CoW 不可变 + 24h GC | workmodel/version_chain.go | ✅ |
| AC13 | T03 workmodel CoW impl + GC worker | workmodel/version_chain.go | ✅ |
| **AC14 Similarity Check** | T01 Jaccard + config | interfaces/similarity_check.go | ✅ |
| AC14 | T02 拦截 + 0.7-0.85 边界 | workmodel/similarity.go + decisionplanning/decomposer.go | ✅ |
| AC14 | T03 workmodel 封装 | workmodel/similarity.go | ✅ |
| **AC15 Hard Evidence** | T01 HardEvidence + kind-specific | interfaces/hard_evidence.go | ✅ |
| AC15 | T02 Verifier reject 空证 | executionflow/verify/verifier.go | ✅ |
| **PR-B T05 wire** | T01 OTel 完整 span | hardening/emitter.go | ✅ |
| PR-B T05 wire | T02 5 Prometheus metrics | hardening/metrics.go | ✅ |
| **AC6 convergence span** | T01 convergence.feasible_space_width | hardening/emitter.go + mups/observe/aggregator.go | ✅ |
| **AC18 Coverage** | T01 4 目标包 ≥ 80% | S5 验收 | ✅ |
| **AC22 Feature Flag** | T01 D7_HARD_EVIDENCE_ENABLED + D7_SIMILARITY_CHECK_ENABLED | d7-bootstrap/wire.go | ✅ |

## 8. 风险评估 (8 类 + 缓解)

| 风险 | 概率 | 缓解 |
|------|------|------|
| CoW VersionChain 链膨胀 | 中 | hash 索引 + 24h 后台 GC；VersionChain 只保留 hash 不 inline state |
| Hard Evidence 误伤 chat 任务 | 中 | kind-specific 配置 + Feature Flag 灰度 + 1%→10%→50%→100% |
| Similarity Check 边界 0.7-0.85 | 中 | slog.Warn 标记，LLM 二次校验留 v7.0.1 |
| Hash 碰撞 | 极低 | SHA-256 标准库 (256-bit 安全) |
| PR-B FNV-1a → PR-C SHA-256 升级 | 低 | buildChainHash 内部函数升级；外部接口不变 |
| 错误码区间 7100-7122 划分 | 0 | 区间划分：7100-7109 PR-A / 7110-7119 PR-B / 7120-7129 PR-C |
| IV-1 invariant | 0 | version_chain.go / similarity_check.go / hard_evidence.go 仅依赖 `internal/shared/errors/` |
| GC 误删 head | 极低 | GC 实现显式排除 head + 单元测试守护 (IV-3) |

## 9. 与 DM-20260629-006 父设计一致性

| 父设计章节 | PR-C 实施 | 一致性 |
|------------|-----------|--------|
| §1.2 4 维不均衡 | L3 防御运行时层"事前防御" + L4 收口闭环 | ✅ |
| §3.4 决策树 | 完整沿用 + 实施点 | ✅ |
| §④ 4 聚合根 | VersionChain + SimilarityCheck + HardEvidence 3 落地 (剩 ConvergenceBudget 已 PR-B 落) | ✅ |
| §5.3 CoW VersionChain 链路 | 完整实施 (Append/RollbackTo/GC 24h + head 守护) | ✅ |
| §5.4 单点风险 | VersionChain GC 误删 head → IV-3 不变量守护 | ✅ |
| §6.2 契约 (ORCH_*) | 3 新增 ORCH_COW_VERSION_CHAIN_BROKEN + ORCH_SIMILARITY_INTERCEPTED + ORCH_HARD_EVIDENCE_MISSING | ✅ |
| §6.3 幂等保障 | VersionChain.Append 幂等 (同 content → 同 hash) + GC 幂等 | ✅ |
| §B Rollback Plan | 4 阶段回滚矩阵 (Feature Flag + code revert + 错误码清理) | ✅ |
| §7 验证策略 | 6 验证命令 + 灰度 staging 24h | ✅ |
| §6 实施周期 | 4.5 周 → PR-C 1.5 周 | ✅ (符合 4-Layer × 3-Phase 拆分) |

## 10. 4-Layer × 3-Phase 闭环 (PR-C 后)

| Layer | PR-A (✅) | PR-B (✅) | **PR-C (✅)** |
|-------|----------|----------|--------------|
| L1 接口层 | TaskSpec/TaskReport | — | — |
| L2 字段语义层 | Dissent/Blockage/Resource | — | — |
| L3 防御运行时层 | — | Pessimistic + RuleBased | **CoW + Similarity + HardEvidence** |
| L4 治理横切层 | spec 同步 + race test + LP regression + Migration + Boundary + FeatureFlag + ErrorCode | convergence span + Coverage + Layout guard + 灰度 | **T05 wire + convergence span + 灰度 1%→100%** |

**AC 覆盖率**：

| PR | AC IMPLEMENTED | AC PLANNED | 累计 |
|----|----------------|------------|------|
| PR-A (✅) | 6 (AC1-5 + AC17) | 0 | 6/23 |
| PR-B (✅) | 8 (AC11+AC12+AC9+AC10+AC16+AC21+AC22+AC23) | 0 | 14/23 |
| **PR-C (✅)** | **6 (AC13+AC14+AC15+AC6+AC18+AC22)** | 3 (AC7+AC19+AC20) | **20/23 (87%)** |

PR-C 后 v7.0.0 release 即闭环 87% AC，剩余 3 AC（AC7/19/20）留 follow-up。

## 11. S3-Gate Verdict

**PASS** — 10 维度评估全绿，6 段式自检完整，6 AC + 13 T 矩阵设计清晰，与 PR-A/B 边界清晰 (0 行为变更承诺 + 错误码区间划分 + IV 不变量 8 项)，可推进 S4 实施。

---

**Reviewer:** Cursor (Claude Code)
**Date:** 2026-06-29
**Verdict:** S3-Gate PASS → 可推进 S4 Implementation
