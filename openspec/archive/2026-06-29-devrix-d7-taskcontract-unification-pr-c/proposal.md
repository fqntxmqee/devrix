---
demand_id: DM-20260629-009
change_id: devrix-d7-taskcontract-unification-pr-c
title: D7 v7.0 TaskContract 统一 PR-C — CoW VersionChain + Similarity Check + Hard Evidence (L3 防御运行时层)
status: S2_Proposal
priority: P0
parent_demand: DM-20260629-006
parent_changes:
  - devrix-d7-taskcontract-unification (DESIGN ONLY, S7_Archived)
  - devrix-d7-taskcontract-unification-pr-a (DM-20260629-007, S7_Archived, PR #325)
  - devrix-d7-taskcontract-unification-pr-b (DM-20260629-008, S7_Archived, PR #327)
---

# Proposal: D7 TaskContract 统一 PR-C — CoW VersionChain + Similarity Check + Hard Evidence

## 1. Background

PR-A (DM-20260629-007) 落地 L1 接口层 + L2 字段语义层（TaskSpec/TaskReport/Dissent/Blockage/Resource）。
PR-B (DM-20260629-008) 落地 L3 防御运行时层第 1 阶段：**Pessimistic Commit + Rule-based Fallback**（4 候选规则 + 5 类触发条件 + Feature Flag 灰度）。

PR-B 把 L3 "**事后降级**" 解决——资源耗尽/CB 触发/多轮 INDETERMINATE 时通过 Pessimistic MVP 兜底输出。
但 L3 还有 "**事前防御**" 缺口（按父设计 §3.4 决策树）：

- **子层 Commit 可覆盖父层认知**（AC13 CoW VersionChain）→ 子层 Replan 时无法回溯历史 WHY 决策链，回滚/审计/AAR 困难
- **子层惰性层层转包烧 token**（AC14 Similarity Check）→ 同一 directive 多次相似子任务无防御机制
- **Verifier "空证 PASS" 防御缺失**（AC15 Hard Evidence）→ silent corruption：编译过但 runtime panic / 测试 N/A / 关键 invariant 未断言仍判定 PASS
- **PR-B T05 Span/Metric 完整 wire 仍 PLANNED** → 灰度期无 metric 数据支撑 1%→10%→50%→100% 分桶

PR-C 是 v7.0 演进第三阶段（**最终 PR，闭环 4-Layer × 3-Phase**），落地 L3 高风险 + L4 收口。

## 2. Approach

### 2.1 核心架构

```
Channel.Execute 结束
    ↓
[Check] 3 类前置防御（PR-C 新增）
    ├─ AC15 Hard Evidence Gate
    │   ├─ kind=code → TestResult.TestCoveragePct ≥ 1 OR LogExcerpt 非空 OR ArtifactHash 非空
    │   ├─ kind=chat → CoherenceScore ≥ 0.5 OR EntityHash 非空
    │   └─ 不通过 → Result.Kind = Partial + Blockage.RequiredExternal = ["hard_evidence"]
    ├─ AC14 Similarity Check
    │   ├─ 与 VersionChain 中最近 N=5 个 snapshot 做 token-level Jaccard
    │   ├─ 相似度 > 0.85 → 强制 Abort（防 token 烧光）
    │   └─ 0.7-0.85 边界 → slog.Warn 标记 (PR-C 暂不升级 LLM 二次校验)
    └─ AC13 CoW VersionChain
        ├─ 每次 commit → New Hash (SHA-256) → VersionChain[newhash] = newState
        ├─ 父 snapshot 只读 + 子层继承 reference
        └─ 24h GC 周期 + 当前 pointer 不删
    ↓
[PessimisticCommitGuard.Evaluate]  ←  PR-B 已有
    ├─ ok → 正常返回
    └─ blocked → Pessimistic / RuleBased / Abort（PR-B 路径）
    ↓
[NotifyPessimistic emit Span/Metric]  ←  PR-C T05 完整 wire
    ├─ Span: d7.s18.pessimistic.commit.emit (OTel 完整)
    └─ 5 Metrics: pessimistic_commit_trigger_count + fallback_rule_select_total{rule=...} + mvp_artifact_generated_total + pessimistic_commit_latency_us + fallback_rule_apply_total
```

### 2.2 关键设计决策

1. **3 新增 interfaces/ 纯类型文件**（与 PR-A/B 一致原则：pure types，0 import D7 子包）：
   - `interfaces/version_chain.go`（~80 LOC）— Hash + VersionChain + Append + RollbackTo + GC
   - `interfaces/similarity_check.go`（~80 LOC）— SimilarityConfig + Check + Jaccard 算法
   - `interfaces/hard_evidence.go`（~50 LOC）— HardEvidence + Verified + kind-specific 配置
2. **Hash 算法升级**：PR-B FNV-1a → **PR-C SHA-256**（替代 `crypto/sha256` 标准库），父 chain hash 统一升级
3. **Similarity 算法**：B. token-level Jaccard（O(1)，无 LLM 调用）默认；C. 边界 LLM 二次校验（0.7-0.85）留 v7.0.1
4. **Hard Evidence kind-specific**：code 要 test/log，chat 要 entity_hash/coherence_score，**Feature Flag `D7_HARD_EVIDENCE_ENABLED` 默认 disabled** 灰度
5. **CoW VersionChain GC 周期 24h**：参考 D5 `metrics.go` retention 经验；当前 pointer 不删
6. **additive embedding**：VersionChain 嵌入 WorkItem（PR-C 改造 workmodel/work_tree.go），老 WorkItem 路径完全不变（`VersionChain == nil` 视为无 chain）
7. **PR-B T05 收口**：slog.Info 占位 → 完整 OTel span + 5 Prometheus metrics，**不引入新 Span 命名空间**（沿用 PR-B 已登记的 `d7.s18.pessimistic.commit.emit`）

### 2.3 复用 PR-A/B 资产

| 资产 | 复用方式 |
|------|----------|
| `interfaces.TaskSpec` | CoW VersionChain 接收 `*TaskSpec` 引用作为 snapshot 基础 |
| `interfaces.TaskReport` | Hard Evidence 校验读取 `Report.Evidence` + `Report.Result.Kind` |
| `interfaces.FallbackPolicy` | Similarity Check 拦截时复用 FallbackAbort 路径 |
| `escape.DefaultPessimisticCommitGuard` | Hard Evidence + Similarity 在 PessimisticCommitGuard 入口前置 check |
| `sharederrors.WithCode` 模式 | 3 ORCH_COW_* / ORCH_SIMILARITY_* / ORCH_HARD_EVIDENCE_* (7120-7122) |
| `interfaces/ 0 import` 原则 | PR-C version_chain.go / similarity_check.go / hard_evidence.go 仍守 IV-1 |
| `Feature Flag env-gated` 模式 | D7_HARD_EVIDENCE_ENABLED + D7_SIMILARITY_CHECK_ENABLED 默认 disabled |

## 3. 3 AC + 8 T + T05 矩阵

| AC | Activity | T | 实施位置 | 状态 |
|----|----------|---|----------|------|
| **AC13** | D7-S18-A13 CoW VersionChain | T01 VersionChain.Append SHA-256 hash | interfaces/version_chain.go | PLANNED |
| AC13 | D7-S18-A13 | T02 RollbackTo O(1) hash 索引 | interfaces/version_chain.go | PLANNED |
| AC13 | D7-S18-A13 | T03 24h GC 周期 (不删当前 pointer) | workmodel/version_chain.go | PLANNED |
| **AC14** | D7-S18-A14 Similarity Check | T01 token-level Jaccard 算法 | interfaces/similarity_check.go | PLANNED |
| AC14 | D7-S18-A14 | T02 > 0.85 强制 Abort | decisionplanning/decomposer.go | PLANNED |
| AC14 | D7-S18-A14 | T03 0.7-0.85 边界 slog.Warn | decisionplanning/decomposer.go | PLANNED |
| **AC15** | D7-S18-A15 Hard Evidence | T01 kind-specific 配置 (code/chat) | interfaces/hard_evidence.go | PLANNED |
| AC15 | D7-S18-A15 | T02 Verifier reject 空证 PASS | executionflow/verify/verifier.go | PLANNED |
| **PR-B T05** | 收口 PR-B Span/Metric wire | T01 d7.s18.pessimistic.commit.emit OTel 完整 | hardening/emitter.go | PLANNED |
| PR-B T05 | 收口 PR-B Span/Metric wire | T02 5 Prometheus metrics emit | hardening/metrics.go | PLANNED |
| **AC6** | D7-S19-A01 convergence span | T01 `convergence.feasible_space_width` OTel span | hardening/emitter.go + mups/observe/aggregator.go | PLANNED |
| **AC18** | 横切 Coverage | T01 interfaces + escape + workmodel + verify ≥ 80% | S5 验收 | PLANNED |
| **AC22** | 横切 Feature Flag | T01 D7_HARD_EVIDENCE_ENABLED + D7_SIMILARITY_CHECK_ENABLED 灰度 | d7-bootstrap/wire.go | PLANNED |

**PR-C Total:** 13 P0 T (核心 8 + PR-B T05 2 + AC6 1 + AC18 1 + AC22 1)

## 4. Span / Metric / ErrorCode 清单

### 4.1 Span (3 个 P0 新增)

| Op | Kind | Component | 触发点 | AC |
|----|------|-----------|--------|----|
| `d7.s18.hard.evidence.reject` | INTERNAL | executionflow/verify | Verifier 拒绝空证 | AC15 |
| `d7.s18.worktree.versionchain.append` | INTERNAL | workmodel | VersionChain 追加 | AC13 |
| `d7.s18.similarity.check.intercept` | INTERNAL | decisionplanning | 相似度 > 0.85 拦截 | AC14 |

PR-A 已登记 5 个 Span + PR-B 新增 1 个 = 6 个；PR-C 新增 3 个 → **合计 9 个**（v7.0 spec）。
AC6 `convergence.feasible_space_width` 沿用 v6.0.x 已有的 `d7.s10.observe.aggregate.complete` Span（**不新增** Span 命名空间）。

### 4.2 Metric (8 个新增，PR-B T05 收口 5 + PR-C 新增 3)

| Name | Unit | 触发点 | AC |
|------|------|--------|----|
| `pessimistic_commit_trigger_count` | count | PR-B Evaluate 返回 blocked | PR-B T05 |
| `fallback_rule_select_total{rule=...}` | count | FallbackRuleBased 选中 | PR-B T05 |
| `mvp_artifact_generated_total` | count | BuildMVPArtifact 出口 | PR-B T05 |
| `pessimistic_commit_latency_us` | microseconds | Evaluate() 计时 | PR-B T05 |
| `fallback_rule_apply_total` | count | FallbackRule 实际应用 | PR-B T05 |
| `hard_evidence_reject_count{kind=code/chat}` | count | Verifier 拒绝空证 | AC15 |
| `similarity_check_intercept_count{boundary=high/mid}` | count | 相似度拦截 | AC14 |
| `versionchain_append_total` | count | VersionChain.Append | AC13 |

### 4.3 ErrorCode (3 个新增)

| Code | Name | 范围 |
|------|------|------|
| `ORCH_COW_VERSION_CHAIN_BROKEN` | VersionChain 链断裂 / GC 误删 | 7120 |
| `ORCH_SIMILARITY_INTERCEPTED` | 相似度 > 0.85 强制 Abort | 7121 |
| `ORCH_HARD_EVIDENCE_MISSING` | Verifier 拒绝空证 | 7122 |

## 5. 文件清单 (10 NEW + 4 MODIFIED)

### NEW (10)
- `internal/layers/orchestration/interfaces/version_chain.go` (~80 LOC) — Hash + VersionChain + Append + RollbackTo
- `internal/layers/orchestration/interfaces/similarity_check.go` (~80 LOC) — SimilarityConfig + Check + Jaccard
- `internal/layers/orchestration/interfaces/hard_evidence.go` (~50 LOC) — HardEvidence + Verified + kind-specific
- `internal/layers/orchestration/workmodel/version_chain.go` (~120 LOC) — CoW impl + 24h GC worker
- `internal/layers/orchestration/workmodel/similarity.go` (~80 LOC) — Jaccard 算法 + 边界检测
- `internal/layers/orchestration/interfaces/version_chain_test.go` (~150 LOC)
- `internal/layers/orchestration/interfaces/similarity_check_test.go` (~120 LOC)
- `internal/layers/orchestration/interfaces/hard_evidence_test.go` (~100 LOC)
- `internal/layers/orchestration/workmodel/version_chain_test.go` (~150 LOC)
- `internal/layers/orchestration/workmodel/similarity_test.go` (~100 LOC)

### MODIFIED (4)
- `internal/layers/orchestration/executionflow/verify/verifier.go` (Hard Evidence reject)
- `internal/layers/orchestration/decisionplanning/decomposer.go` (Similarity Check 入口)
- `internal/layers/orchestration/workmodel/work_tree.go` (VersionChain 嵌入)
- `internal/layers/orchestration/d7-bootstrap/wire.go` (Feature Flag 注入 + D7_SIMILARITY_CHECK_ENABLED + D7_HARD_EVIDENCE_ENABLED)

### SPEC DOCS (6 同步)
- `openspec/specs/d7-orchestration/spec.md` (新增 3 ADDED Requirements for D7-S18)
- `openspec/specs/d7-orchestration/d7-domain.md` (§8 Layer 4-Layer × 3-Phase PR-C 落地)
- `openspec/specs/d7-orchestration/a-registry.md` (D7-S18-A13/A14/A15 3 A entries)
- `openspec/specs/d7-orchestration/f-registry.md` (D7-S18-A13/A14/A15 F entries)
- `openspec/specs/d7-orchestration/span-registry.md` (3 个新 P0 span ops)
- `openspec/specs/d7-orchestration/t-registry.md` (13 个 P0 T 登记)

## 6. 行为变更范围

**0 行为变更**（PR-C 与 PR-A/B 同样原则）：
- `D7_HARD_EVIDENCE_ENABLED=false` (default) 时 Verifier 不拒绝空证
- `D7_SIMILARITY_CHECK_ENABLED=false` (default) 时 Similarity Check 不触发
- VersionChain 嵌入 WorkItem：`VersionChain == nil` 视为无 chain（PR-C 启用前老 WorkItem 完全不变）
- LP-1/LP-2/LP-5 集成测试 100% 兼容（Feature Flag 默认 disabled）

## 7. 验证策略

- `go test -race -count=1 ./internal/layers/orchestration/...` → 24+ packages PASS
- `go test -cover ./internal/layers/orchestration/interfaces/` → ≥ 80%
- `go test -cover ./internal/layers/orchestration/escape/` → ≥ 80%
- `go test -cover ./internal/layers/orchestration/workmodel/` → ≥ 80%（新增）
- `go test -cover ./internal/layers/orchestration/executionflow/verify/` → ≥ 80%
- LP-1/LP-2/LP-5 集成测试 100% 兼容（Feature Flag 默认 disabled）
- 灰度验证：`D7_HARD_EVIDENCE_ENABLED=true` + `D7_SIMILARITY_CHECK_ENABLED=true` staging 烟测 24h 后再 prod

## 8. 风险

| 风险 | 概率 | 缓解 |
|------|------|------|
| CoW VersionChain 链膨胀 | 中 | hash 索引 + 24h 后台 GC；VersionChain 只保留 hash 不 inline state |
| Hard Evidence 误伤 chat 任务 | 中 | kind-specific 配置 + Feature Flag 灰度 + 1%→10%→50%→100% |
| Similarity Check 边界 0.7-0.85 | 中 | slog.Warn 标记，LLM 二次校验留 v7.0.1 |
| Hash 碰撞 (SHA-256) | 极低 | crypto/sha256 标准库；256-bit 安全 |
| PR-B FNV-1a → PR-C SHA-256 升级 | 低 | buildChainHash 内部函数，外部接口不变（仅哈希字符串长度） |
| 4 个 ORCH_* 错误码 + PR-A/B 共占 7100-7122 区间 | 0 | 区间划分：7100-7109 PR-A / 7110-7119 PR-B / **7120-7129 PR-C** |
| IV-1 invariant（interfaces/ 0 import） | 0 | version_chain.go / similarity_check.go / hard_evidence.go 仅依赖 `internal/shared/errors/` |

## 9. 时间线

| 阶段 | 日期 | 输出 |
|------|------|------|
| S1-S3 设计 (本 Change) | 2026-06-29 | proposal + design + tasks + review-design + specs/ |
| S3-Gate | 2026-06-30 | review-design.md PASS |
| S4 实现 | 2026-06-30 ~ 2026-07-03 | 10 NEW + 4 MODIFIED + spec sync |
| S5 验收 | 2026-07-04 | 24+ packages -race PASS + 覆盖率 ≥ 80% |
| S6 归档 | 2026-07-04 | archive/ + PR auto-merge |
| **总计** | **~5 天** | PR-C S7_Archived + v7.0.0 minor release |

## 10. 后续路径

PR-C S7_Archived → **v7.0.0 minor release 闭环 4-Layer × 3-Phase**。

可选 follow-up Change（按用户确认按需立）：
- `devrix-d7-adaptive-threshold` (AC7) — AdaptiveThreshold 接入 RunTurn
- `devrix-d7-performance-budget` (AC19) — Performance benchmark 收敛
- `devrix-d7-security-classification` (AC20) — TaskSpec.Classification 标签
- `devrix-d7-similarity-llm-boundary` — 0.7-0.85 边界 LLM 二次校验
- `devrix-d7-versionchain-distributed` — VersionChain 跨 session 持久化
