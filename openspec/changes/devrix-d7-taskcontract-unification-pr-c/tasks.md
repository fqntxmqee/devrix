# Tasks: D7 TaskContract 统一 PR-C — CoW VersionChain + Similarity Check + Hard Evidence (L3 防御运行时层)

**Change ID:** devrix-d7-taskcontract-unification-pr-c
**Demand ID:** DM-20260629-009
**Phase:** S3_Design (S3-Gate 待 S3-Gate review)
**Status:** S3_Design (PR-C S3 design + tasks 完成; S4 实现待 S3-Gate 通过)
**Created:** 2026-06-29

---

## Phase 1: S4 Implementation (P0 T 矩阵) ⏳ 13/13 PLANNED

### 1.1 interfaces/ 纯类型层（AC13 + AC14 + AC15 核心 3 NEW）

- [ ] **D7-S18-A13-T01 (AC13)**: VersionChain interface + Hash type
  - [ ] `internal/layers/orchestration/interfaces/version_chain.go` (NEW, ~80 LOC) - Hash type (SHA-256 64-char hex) + VersionChainEntry struct + NewVersionChain + Append + RollbackTo + Get + Head + GC + Len
  - [ ] `sharederrors.WithCode(7120)` ORCH_COW_VERSION_CHAIN_BROKEN SentinelError
  - [ ] 验证：`grep -r 'orchestration/' interfaces/ | grep -v _test` → 0 行 (IV-1)
  - [ ] 验证：SHA-256 升级（替代 PR-B FNV-1a 16-char）

- [ ] **D7-S18-A13-T02 (AC13)**: CoW 不可变语义 + 24h GC head 守护
  - [ ] `Append` / `RollbackTo` / `GC` 全部浅拷贝返回新副本（不可变）
  - [ ] `GC(ttl)` 显式排除 `head`（IV-3 不变量）
  - [ ] 验证：race detector clean（map[Hash]*VersionChainEntry rehash 复制）

- [ ] **D7-S18-A14-T01 (AC14)**: SimilarityCheck + Jaccard 算法
  - [ ] `internal/layers/orchestration/interfaces/similarity_check.go` (NEW, ~80 LOC) - SimilarityConfig + SimilarityResult + DefaultInterceptThreshold (0.85) + DefaultWarnThreshold (0.70) + DefaultLookbackN (5) + Jaccard + CheckSimilarity
  - [ ] `sharederrors.WithCode(7121)` ORCH_SIMILARITY_INTERCEPTED SentinelError
  - [ ] 验证：Jaccard O(|A|+|B|) + map[string]struct{} 哈希集合 P99 < 0.1ms

- [ ] **D7-S18-A14-T02 (AC14)**: 相似度拦截 + 0.7-0.85 边界 slog.Warn
  - [ ] `> 0.85` → `Similar=true` → ErrSimilarityCheckIntercepted (7121) → FallbackAbort
  - [ ] `0.70-0.85` → `Warn=true` → slog.Warn (不阻塞) (IV-6 不变量)
  - [ ] `< 0.70` → 正常通过

- [ ] **D7-S18-A15-T01 (AC15)**: HardEvidence + kind-specific 配置
  - [ ] `internal/layers/orchestration/interfaces/hard_evidence.go` (NEW, ~50 LOC) - HardEvidence struct + Kind + TestResult/LogExcerpt/ArtifactHash/EntityHash/CoherenceScore + Verified() + ExtractHardEvidence
  - [ ] `sharederrors.WithCode(7122)` ORCH_HARD_EVIDENCE_MISSING SentinelError
  - [ ] kind="code": `TestResult != nil && (CoveragePct >= 1 || LogExcerpt != "" || ArtifactHash != "")`
  - [ ] kind="chat": `CoherenceScore >= 0.5 || EntityHash != ""`
  - [ ] kind="unknown": 同 code (保守) (IV-5 不变量)

- [ ] **D7-S18-A15-T02 (AC15)**: Verifier reject 空证 PASS
  - [ ] `internal/layers/orchestration/executionflow/verify/verifier.go` (MOD, +50 LOC)
  - [ ] `if report.Result.Kind == VerdictPass && !HardEvidence.Verified()`:
    - [ ] → `Result.Kind = VerdictPartial` (强制)
    - [ ] → `Blockage.RequiredExternal = ["hard_evidence"]`
    - [ ] → emit `"d7.s18.hard.evidence.reject"` (kind=ev.Kind)
    - [ ] → metric `hard_evidence_reject_count{kind=ev.Kind}++`
    - [ ] → return `ErrHardEvidenceMissing` (7122)

### 1.2 workmodel/ 防御运行时层（2 NEW + 1 MOD）

- [ ] **D7-S18-A13-T03 (AC13)**: workmodel/version_chain.go CoW impl + 24h GC worker
  - [ ] `internal/layers/orchestration/workmodel/version_chain.go` (NEW, ~120 LOC) - WorkItemVersionChain 封装 + GC 周期 worker (24h)
  - [ ] `internal/layers/orchestration/workmodel/work_tree.go` (MOD, +30 LOC) - VersionChain 嵌入 WorkItem (additive, `omitempty` JSON tag)
  - [ ] 验证：`WorkItem.VersionChain == nil` 视为无 chain (老 WorkItem 完全不变)

- [ ] **D7-S18-A14-T03 (AC14)**: workmodel/similarity.go Jaccard + boundary
  - [ ] `internal/layers/orchestration/workmodel/similarity.go` (NEW, ~80 LOC) - DecomposeSimilarityChecker 封装 + LastN 提取
  - [ ] `internal/layers/orchestration/decisionplanning/decomposer.go` (MOD, +40 LOC) - Similarity Check 入口

- [ ] **D7-S18-A15-T03 (AC15)**: Verifier kind-specific 配置
  - [ ] `internal/layers/orchestration/executionflow/verify/verifier.go` (MOD, +20 LOC) - kind-specific 配置读取（code/chat 分开）
  - [ ] 验证：chat 任务不被 test 强制拒绝

### 1.3 单元测试（5 NEW）✅

- [ ] **D7-S18-A13-T04 (AC13)**: interfaces/version_chain_test.go
  - [ ] `internal/layers/orchestration/interfaces/version_chain_test.go` (NEW, ~150 LOC, ≥ 5 tests)
  - [ ] TestVersionChain_Append_SHA256 (64-char hex 验证)
  - [ ] TestVersionChain_RollbackTo_O1
  - [ ] TestVersionChain_GC_HeadProtected
  - [ ] TestVersionChain_Immutable (CoW 不可变)
  - [ ] TestVersionChain_HashCollision_Resilient

- [ ] **D7-S18-A14-T04 (AC14)**: interfaces/similarity_check_test.go
  - [ ] `internal/layers/orchestration/interfaces/similarity_check_test.go` (NEW, ~120 LOC, ≥ 4 tests)
  - [ ] TestJaccard_Basic
  - [ ] TestCheckSimilarity_HighIntercept
  - [ ] TestCheckSimilarity_MidBoundary_Warn
  - [ ] TestCheckSimilarity_LowPass

- [ ] **D7-S18-A15-T04 (AC15)**: interfaces/hard_evidence_test.go
  - [ ] `internal/layers/orchestration/interfaces/hard_evidence_test.go` (NEW, ~100 LOC, ≥ 4 tests)
  - [ ] TestHardEvidence_Code_Verified
  - [ ] TestHardEvidence_Code_Rejected (TestCoveragePct=0 + LogExcerpt="" + ArtifactHash="")
  - [ ] TestHardEvidence_Chat_Verified
  - [ ] TestHardEvidence_Chat_Rejected (CoherenceScore<0.5 + EntityHash="")

- [ ] **D7-S18-A13-T05 (AC13)**: workmodel/version_chain_test.go
  - [ ] `internal/layers/orchestration/workmodel/version_chain_test.go` (NEW, ~150 LOC, ≥ 5 tests)
  - [ ] TestWorkItemVersionChain_Append_WithSHA256
  - [ ] TestWorkItemVersionChain_GC_24hCycle
  - [ ] TestWorkItemVersionChain_RaceCondition (race detector)
  - [ ] TestWorkItemVersionChain_HeadProtected
  - [ ] TestWorkItemVersionChain_NilSafe (老 WorkItem `VersionChain == nil` 兼容)

- [ ] **D7-S18-A14-T05 (AC14)**: workmodel/similarity_test.go
  - [ ] `internal/layers/orchestration/workmodel/similarity_test.go` (NEW, ~100 LOC, ≥ 4 tests)
  - [ ] TestJaccard_Empty
  - [ ] TestJaccard_Identical
  - [ ] TestJaccard_Disjoint
  - [ ] TestDecomposeSimilarityChecker_HighIntercept

### 1.4 PR-B T05 收口（继承 PLANNED → IMPLEMENTED）

- [ ] **D7-S18-A11-T05 (PR-B 收口)**: hardening/emitter.go + metrics.go
  - [ ] `internal/layers/orchestration/hardening/emitter.go` (MOD, +40 LOC) - d7.s18.pessimistic.commit.emit 完整 OTel span 注册
  - [ ] `internal/layers/orchestration/hardening/metrics.go` (MOD, +50 LOC) - 5 Prometheus metrics 注册 (pessimistic_commit_trigger_count + fallback_rule_select_total + mvp_artifact_generated_total + pessimistic_commit_latency_us + fallback_rule_apply_total)
  - [ ] 替换 `engine.go::NotifyPessimistic` 内 `slog.Info` 占位为完整 OTel + Prometheus wire
  - [ ] 验证：Jaeger 可见 + Prometheus scrape 可见

### 1.5 L4 治理收口（3 项）

- [ ] **D7-S19-A01-T01 (AC6)**: convergence.feasible_space_width span
  - [ ] `internal/layers/orchestration/hardening/emitter.go` (MOD, +20 LOC) - 沿用 v6.0.x `d7.s10.observe.aggregate.complete` span 命名空间，不新增
  - [ ] `internal/layers/orchestration/mups/observe/aggregator.go` (MOD, +30 LOC) - 聚合结束 emit convergence.feasible_space_width attr
  - [ ] 验证：Observe 节点聚合结束自动 emit

- [ ] **D7-S18 横切-T01 (AC18)**: Coverage ≥ 80% (4 个目标包)
  - [ ] `internal/layers/orchestration/interfaces/` 覆盖率 ≥ 80%
  - [ ] `internal/layers/orchestration/escape/` 覆盖率 ≥ 80%
  - [ ] `internal/layers/orchestration/workmodel/` 覆盖率 ≥ 80%（新增）
  - [ ] `internal/layers/orchestration/executionflow/verify/` 覆盖率 ≥ 80%（Hard Evidence 落地）

- [ ] **D7-S18 横切-T02 (AC22)**: Feature Flag 灰度
  - [ ] `internal/layers/orchestration/d7-bootstrap/wire.go` (MOD, +20 LOC) - HardEvidenceEnabled + SimilarityCheckEnabled env helpers
  - [ ] `D7_HARD_EVIDENCE_ENABLED` (default `false`) + `D7_SIMILARITY_CHECK_ENABLED` (default `false`)
  - [ ] 验证：default false 时 0 行为变更 (LP-1/LP-2/LP-5 100% 兼容)

### 1.6 spec 文档同步（6 MOD）⏳

- [ ] **D7-S18-A13-T06 (AC18)**: spec.md v4.17.0 → v4.18.0
  - [ ] 新增 3 ADDED Requirements（D7-S18-A13 CoW VersionChain + D7-S18-A14 Similarity Check + D7-S18-A15 Hard Evidence）
  - [ ] 新增 15 Gherkin Scenarios（VersionChain 5 + Similarity 4 + HardEvidence 4 + T05 wire 1 + AC6 1）
  - [ ] 新增 §Scenario D7-S18-A13/A14/A15 Section（13 P0 T 矩阵）
  - [ ] 修订记录 4.18.0 entry

- [ ] **D7-S18-A13-T07**: d7-domain.md v2.8.0 → v2.9.0
  - [ ] §8 Layer 架构 L3 行更新（PR-C 3/3 IMPLEMENTED ✅）
  - [ ] §8 Phase 3 (PR-C) 行更新（6 AC + 13 T + 本 PR + AC6/T05 wire 收口）
  - [ ] §9 interfaces 包章节扩展（13 文件清单 + 3 ORCH_* SentinelError 7120-7122 + IV-6/IV-7/IV-8 不变量 + Additive 嵌入 WorkItem.VersionChain）
  - [ ] §DSAFT 资产登记规模更新（A 57→60 / F 93→100 / T 246→259 / Span 32→35 / Metric 13→21）
  - [ ] 修订记录 2.9.0 entry

- [ ] **D7-S18-A13-T08**: a-registry.md v5.2.0 → v5.3.0
  - [ ] D7-S18-A13/A14/A15 3 A entries
  - [ ] 修订记录 5.3.0 entry

- [ ] **D7-S18-A13-T09**: f-registry.md v5.2.0 → v5.3.0
  - [ ] D7-S18-A13 F01-F04 (4 F CoW) + D7-S18-A14 F01-F03 (3 F Similarity) + D7-S18-A15 F01-F02 (2 F Hard Evidence) = 9 F entries
  - [ ] F 总数 93 → 100 IMPLEMENTED
  - [ ] D7-S18-A11 F01-F05 (PR-B) 状态维持 IMPLEMENTED

- [ ] **D7-S18-A13-T10**: span-registry.md v4.4.0 → v4.5.0
  - [ ] +3 P0 span ops (D7_Orchestration_Hard_Evidence_Reject + D7_Orchestration_Worktree_Versionchain_Append + D7_Orchestration_Similarity_Check_Intercept)
  - [ ] Span 总数 32 → 35

- [ ] **D7-S18-A13-T11**: t-registry.md v4.15.0 → v4.16.0
  - [ ] +13 P0 T entries (D7-S18-A13 T01-T05 + D7-S18-A14 T01-T05 + D7-S18-A15 T01-T03)
  - [ ] T 总数 246 → 259

---

## Phase 2: S5 Verification ✅ 验证策略

- [ ] **D7-S18-A13-T12 (AC18)**: 4 目标包覆盖率
  - [ ] `go test -cover ./internal/layers/orchestration/interfaces/` → ≥ 80% (含 3 NEW version_chain/similarity_check/hard_evidence)
  - [ ] `go test -cover ./internal/layers/orchestration/escape/` → ≥ 80% (PR-B 85.0% 维持)
  - [ ] `go test -cover ./internal/layers/orchestration/workmodel/` → ≥ 80% (新增)
  - [ ] `go test -cover ./internal/layers/orchestration/executionflow/verify/` → ≥ 80% (Hard Evidence 落地)

- [ ] **D7-S18-A13-T13 (AC9)**: 全 orchestration 包 race PASS
  - [ ] `go test -race -count=1 -timeout 180s ./internal/layers/orchestration/...` → 24/24 packages PASS
  - [ ] 验证：CoW VersionChain 不可变 + race detector clean

- [ ] **D7-S18-A13-T14 (AC10)**: LP-1/LP-2/LP-5 集成测试
  - [ ] `go test -tags "integration d7" -run "TestAcceptance_LP1_|TestAcceptance_LP2_|TestAcceptance_LP5_" ./tests/integration/d7/` → 7/7 PASS
  - [ ] 验证：Feature Flag 默认 disabled 100% 兼容

- [ ] **D7-S18-A13-T15 (AC22)**: Feature Flag smoke test
  - [ ] `D7_HARD_EVIDENCE_ENABLED=false D7_SIMILARITY_CHECK_ENABLED=false go test -count=1 ./internal/layers/orchestration/...` → PASS (0 行为变更)
  - [ ] 灰度验证：`D7_HARD_EVIDENCE_ENABLED=true` + `D7_SIMILARITY_CHECK_ENABLED=true` staging 烟测 24h 后再 prod

---

## Phase 3: S6 Archive ⏳

- [ ] **S6-1**: 创建 acceptance-report.md (4 AC + 13 T mapping, 覆盖率, race detector, smoke test results)
- [ ] **S6-2**: 更新 .openspec.yaml status S3_Design → s7_archived
- [ ] **S6-3**: `git mv` archive changes/ → archive/2026-06-29-devrix-d7-taskcontract-unification-pr-c/
- [ ] **S6-4**: 更新 proposal.md status 字段 (verify-archive.sh 强制)
- [ ] **S6-5**: 更新 demand-archive-index.md + DM-20260629-009 row
- [ ] **S6-6**: `bash scripts/verify-archive.sh` → 12/12 PASS
- [ ] **S6-7**: `git push -u origin feat/devrix-d7-taskcontract-unification-pr-c`
- [ ] **S6-8**: `gh pr create --title "feat(d7): v7.0 taskcontract pr-c cow versionchain + similarity check + hard evidence (DM-20260629-009)" --body "..."`
- [ ] **S6-9**: `gh pr merge --auto --squash --delete-branch`
- [ ] **S6-10**: `bash scripts/devrix.sh build && bash scripts/devrix.sh restart` (per feedback-devrix-restart-via-script.md)

---

## 任务总览表

| 阶段 | 子项数 | 状态 | 输出 |
|------|--------|------|------|
| S3-Gate (review-design.md) | 12 维度 | ⏳ PLANNED | review-design.md PASS |
| S4 Phase 1 (interfaces) | 6 T | ⏳ PLANNED | 3 NEW + 3 test (~580 LOC) |
| S4 Phase 2 (workmodel) | 4 T | ⏳ PLANNED | 2 NEW + 2 test (~330 LOC) + 1 MOD |
| S4 Phase 3 (MOD) | 5 T | ⏳ PLANNED | 5 MODIFIED (~190 LOC) + 1 NEW test |
| S4 Phase 4 (spec sync) | 6 T | ⏳ PLANNED | 6 spec 文件 |
| S4 Phase 5 (整合) | 0 T | ⏳ PLANNED | race + coverage + LP smoke |
| S5 Verify | 4 T | ⏳ PLANNED | 4/4 PASS |
| S6 Archive | 10 步骤 | ⏳ PLANNED | archive + PR + restart |
| **总计** | **13 P0 T + 13 S4 步骤 + 4 S5 + 10 S6** | ⏳ PLANNED | **PR-C S7_Archived + v7.0.0 release** |

---

## 关键风险与缓解

| 风险 | 概率 | 缓解 | 状态 |
|------|------|------|------|
| CoW VersionChain 链膨胀 | 中 | hash 索引 + 24h 后台 GC；VersionChain 只保留 hash 不 inline state | ⏳ S4 Phase 2 |
| Hard Evidence 误伤 chat 任务 | 中 | kind-specific 配置 + Feature Flag 灰度 + 1%→10%→50%→100% | ⏳ S4 Phase 1.1 |
| Similarity Check 边界 0.7-0.85 误判 | 中 | 仅 slog.Warn 标记，不阻塞；LLM 二次校验留 v7.0.1 | ⏳ S4 Phase 1.1 |
| Hash 碰撞 | 极低 | SHA-256 标准库 (256-bit 安全) | ⏳ S4 Phase 1.1 |
| PR-B FNV-1a → PR-C SHA-256 升级 | 低 | buildChainHash 内部函数升级；外部接口不变 | ⏳ S4 Phase 1.1 |
| 错误码区间冲突 | 0 | 区间划分：7100-7109 PR-A / 7110-7119 PR-B / 7120-7129 PR-C | ⏳ S4 Phase 1.1 |
| IV-1 invariant 破坏 | 0 | version_chain.go / similarity_check.go / hard_evidence.go 仅依赖 `internal/shared/errors/` | ⏳ S4 Phase 1.1 |
| GC 误删 head | 极低 | GC 实现显式排除 head + 单元测试守护 (IV-3) | ⏳ S4 Phase 2 |
| VersionChain 嵌入 WorkItem → JSON 序列化 break | 低 | `omitempty` JSON tag + serialization_test.go | ⏳ S4 Phase 2 |
| T05 wire 复杂度（PR-B 收口） | 中 | 沿用 PR-B 已登记的 span 命名空间 + 5 metrics 复用 sharederrors.WithCode 模式 | ⏳ S4 Phase 1.4 |
