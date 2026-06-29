# Spec Delta: D7 v7.0 TaskContract PR-C (DM-20260629-009)

**Change ID:** devrix-d7-taskcontract-unification-pr-c
**Demand ID:** DM-20260629-009
**Phase:** PR-C (L3 防御运行时层高风险 + L4 收口)
**Status:** S3_Design (待 S3-Gate review)
**Created:** 2026-06-29

> **本 delta 文件遵循 OpenSpec 规范。** S4 实施时合并到 `openspec/specs/d7-orchestration/spec.md` v4.17.0 → v4.18.0 新增 3 ADDED Requirements + 15 Gherkin Scenarios。本文件仅作为 S3-Gate review 锚点 + S4 实施清单。
> 父 PR-A (DM-20260629-007) v4.16.0 → v4.17.0 + 父 PR-B (DM-20260629-008) 已落地 5 + 7 Gherkin Scenarios；本 PR-C 新增 15 Gherkin Scenarios。

---

## ADDED Requirements (PR-C scope 3 NEW)

### Requirement: D7-S18-A13 CoW VersionChain 防御运行时（L3 防御运行时层）

`VersionChain` MUST 提供不可变追加 (Append) + O(1) hash 索引 (RollbackTo) + 24h GC 周期 (head 永远不被 GC) 的 CoW 版本链，并在 Feature Flag `D7_SIMILARITY_CHECK_ENABLED=true`（间接触发，因为 Similarity Check 读 VersionChain）时通过 `workmodel/version_chain.go` 的 `WorkItemVersionChain` 嵌入 `WorkItem` 字段（additive，`omitempty` JSON tag）。

**Priority:** P0
**Layer:** L3 防御运行时层
**Package:** `internal/layers/orchestration/interfaces/version_chain.go` + `internal/layers/orchestration/workmodel/version_chain.go` + `internal/layers/orchestration/workmodel/work_tree.go`
**Contract:** 1 ORCH_* SentinelError (code 7120)
**Hash 算法:** SHA-256 64-char hex（替代 PR-B FNV-1a 16-char）
**T:** D7-S18-A13-T01 / T02 / T03 / T04 / T05

#### Scenario: VersionChain.Append SHA-256 hash 稳定

- GIVEN `NewVersionChain()` 初始化空链
- WHEN `Append(content, reason="commit")` 调用一次
- THEN 返回 `(Hash="<64-char hex>", err=nil)`
- AND `Len() == 1` AND `Head() == <new_hash>`
- AND `Get(<new_hash>)` 返回 entry `{Hash, Parent="", Content, CreatedAt, Reason="commit"}`
- AND Hash 长度固定 64-char (SHA-256) — 区别于 PR-B FNV-1a 16-char

#### Scenario: RollbackTo O(1) hash 索引

- GIVEN VersionChain 已有 3 个 entry (hash_A, hash_B, hash_C，head=hash_C)
- WHEN `RollbackTo(hash_A)` 调用
- THEN `Head() == hash_A` (O(1) 查找)
- AND 中间 hash_B 仍存在（仅 head 变化）
- AND `Get(hash_B) != nil`（GC 24h 后才被清理）

#### Scenario: 24h GC 周期 + head 守护

- GIVEN VersionChain 已有 5 个 entry (created at t0, t6h, t12h, t18h, t24h)
- AND `time.Now() = t25h`
- WHEN `GC(24 * time.Hour)` 调用
- THEN 删除 t0, t6h, t12h, t18h (4 个 entry)
- AND t24h (head) 永远不被 GC
- AND `Len() == 1` AND `Head()` 仍指向 t24h entry
- AND `RollbackTo(t24h)` 仍能 O(1) 查找成功

#### Scenario: CoW 不可变 + race detector clean

- GIVEN VersionChain v1 (head=v1_hash) + 多个 goroutine 并发 Append
- WHEN 100 个 goroutine 各 `Append(content[i], "commit")` 100 次
- THEN 总 append 成功 = 100 次（新 hash 各异）
- AND `Len() == 101` (1 初始 + 100 追加)
- AND `race detector` 报告 0 race
- AND 浅拷贝实现（`vc.entries` + `vc.order` + `vc.head` rehash 复制）

#### Scenario: WorkItem.VersionChain 嵌入 (additive)

- GIVEN 老 WorkItem `VersionChain == nil` (PR-A 时期创建)
- WHEN `WorkItem.Commit(state)` 调用
- THEN 触发 lazy init `VersionChain = NewVersionChain()`
- AND 首次 `Append` 正常
- AND JSON 序列化 `{"version_chain": [...]}` 出现（omitempty 保证老 WorkItem 序列化无差异）
- AND 旧 `WorkItem.GetState()` 调用方完全无感知

#### Scenario: CoW VersionChain 链断裂 (GC 误删)

- GIVEN `RollbackTo(hash_X)` 调用
- AND hash_X 已被 24h GC 删除（非 head）
- WHEN 校验
- THEN 返回 `ErrCoWVersionChainBroken` (7120)
- AND `Remediation`: "trigger VersionChain.GC with longer TTL or rollback to head"
- AND `d7.s18.worktree.versionchain.append` Span 不 emit（Append 失败）

### Requirement: D7-S18-A14 Similarity Check 防御运行时（L3 防御运行时层）

`SimilarityCheck` MUST 在 `decompose.TaskGraphSynthesize` 入口通过 token-level Jaccard 相似度算法检测当前 `TaskSpec.Goal` 与 WorkItem.VersionChain 中最近 N=5 个 snapshot 的相似度，并在 Feature Flag `D7_SIMILARITY_CHECK_ENABLED=true` 时触发拦截（> 0.85 → FallbackAbort）或 warn（0.70-0.85 → slog.Warn 不阻塞）。

**Priority:** P0
**Layer:** L3 防御运行时层
**Package:** `internal/layers/orchestration/interfaces/similarity_check.go` + `internal/layers/orchestration/workmodel/similarity.go` + `internal/layers/orchestration/decisionplanning/decomposer.go`
**Contract:** 1 ORCH_* SentinelError (code 7121)
**Algorithm:** token-level Jaccard (O(|A|+|B|)，map[string]struct{} 哈希集合，P99 < 0.1ms)
**T:** D7-S18-A14-T01 / T02 / T03 / T04 / T05

#### Scenario: Jaccard 基础算法

- GIVEN `a = ["hello", "world"]` AND `b = ["hello", "go"]`
- WHEN `Jaccard(a, b)` 调用
- THEN 返回 `0.333...` (|{"hello"}| / |{"world", "go", "hello"}| = 1/3)
- AND 算法 O(|a|+|b|) P99 < 0.1ms（map 哈希集合）

#### Scenario: 相似度 > 0.85 强制 Abort

- GIVEN `current = "实现 HTTP server 支持 GET POST"` AND WorkItem.VersionChain 最后 1 个 entry Content = "实现 HTTP server 支持 GET POST 和 DELETE"
- AND Jaccard > 0.85 (0.75 vs 0.85 边界)
- AND `D7_SIMILARITY_CHECK_ENABLED=true`
- WHEN `DecomposeSimilarityChecker.Check(current)` 调用
- THEN 返回 `ErrSimilarityCheckIntercepted` (7121)
- AND FallbackAbort 触发 → `Result.Kind = Failed`
- AND emit `d7.s18.similarity.check.intercept` Span
- AND metric `similarity_check_intercept_count{boundary="high"}++`

#### Scenario: 0.70-0.85 边界 slog.Warn

- GIVEN `current = "实现 HTTP server"` AND WorkItem.VersionChain 最后 1 个 entry Content = "实现 HTTP server 支持 GET POST"
- AND Jaccard = 0.50 (低相似，< 0.70)
- AND `D7_SIMILARITY_CHECK_ENABLED=true`
- WHEN `DecomposeSimilarityChecker.Check(current)` 调用
- THEN 返回 `(SimilarityResult{Similar=false, Warn=false, Score=0.50, MatchedHash="..."}, nil)`
- AND 正常通过（不阻塞）
- AND 0.7-0.85 边界仅 slog.Warn 标记（不触发 LLM 二次校验，留 v7.0.1）

#### Scenario: < 0.70 正常通过

- GIVEN `current = "完全不同的任务"` AND WorkItem.VersionChain 多个 entry
- AND max(Jaccard) < 0.70
- AND `D7_SIMILARITY_CHECK_ENABLED=true`
- WHEN `DecomposeSimilarityChecker.Check(current)` 调用
- THEN 返回 `(SimilarityResult{Similar=false, Warn=false, Score=max, MatchedHash="..."}, nil)`
- AND 正常通过 decompose 流程
- AND LookbackN=5 (PR-C fixed；v7.0.1 可配)

#### Scenario: LookbackN=5 取最近 5 个

- GIVEN WorkItem.VersionChain 已有 10 个 entry
- WHEN `DecomposeSimilarityChecker.Check(current)` 调用
- THEN 仅取最后 5 个 entry (head 倒数 5 个) 计算 Jaccard
- AND 不全量遍历（性能保证）

### Requirement: D7-S18-A15 Hard Evidence 防御运行时（L3 防御运行时层）

`HardEvidence` MUST 在 `Verifier.Verify(artifact, report)` 出口校验 `TaskReport.Result.Kind == VerdictPass` 时检查 kind-specific 最小证据集（code 要 test/log/artifact_hash，chat 要 coherence_score/entity_hash），并在缺失时强制降级为 `VerdictPartial` + `Blockage.RequiredExternal=["hard_evidence"]`，Feature Flag `D7_HARD_EVIDENCE_ENABLED=true` 启用。

**Priority:** P0
**Layer:** L3 防御运行时层
**Package:** `internal/layers/orchestration/interfaces/hard_evidence.go` + `internal/layers/orchestration/executionflow/verify/verifier.go`
**Contract:** 1 ORCH_* SentinelError (code 7122)
**T:** D7-S18-A15-T01 / T02 / T03 / T04

#### Scenario: code 任务 kind-specific 验证

- GIVEN `TaskReport.Result.Kind = VerdictPass` AND kind="code"
- AND `TestResult.CoveragePct = 80.5` (≥ 1)
- WHEN `HardEvidence.Verified()` 调用
- THEN 返回 `true`
- AND Verifier 正常通过

#### Scenario: code 任务空证拒绝

- GIVEN `TaskReport.Result.Kind = VerdictPass` AND kind="code"
- AND `TestResult.CoveragePct = 0` AND `LogExcerpt = ""` AND `ArtifactHash = ""`
- AND `D7_HARD_EVIDENCE_ENABLED=true`
- WHEN `Verifier.Verify(artifact, report)` 调用
- THEN 强制 `Result.Kind = VerdictPartial`
- AND `Blockage.RequiredExternal = ["hard_evidence"]`
- AND emit `d7.s18.hard.evidence.reject` Span (kind="code")
- AND metric `hard_evidence_reject_count{kind="code"}++`
- AND 返回 `ErrHardEvidenceMissing` (7122)

#### Scenario: chat 任务 kind-specific 验证

- GIVEN `TaskReport.Result.Kind = VerdictPass` AND kind="chat"
- AND `CoherenceScore = 0.85` (≥ 0.5)
- WHEN `HardEvidence.Verified()` 调用
- THEN 返回 `true` (chat 不需要 test，仅需 coherence)

#### Scenario: chat 任务空证拒绝（避免误伤）

- GIVEN `TaskReport.Result.Kind = VerdictPass` AND kind="chat"
- AND `CoherenceScore = 0.3` AND `EntityHash = ""`
- AND `D7_HARD_EVIDENCE_ENABLED=true`
- WHEN `Verifier.Verify(artifact, report)` 调用
- THEN 强制 `Result.Kind = VerdictPartial` + `Blockage.RequiredExternal`
- AND metric `hard_evidence_reject_count{kind="chat"}++`
- AND chat 任务不被 test 强制拒绝（kind-specific 严格分离）

#### Scenario: Feature Flag 默认 disabled 0 行为变更

- GIVEN `D7_HARD_EVIDENCE_ENABLED=false` (default)
- AND `TaskReport.Result.Kind = VerdictPass` AND 空证
- WHEN `Verifier.Verify(artifact, report)` 调用
- THEN 正常通过（不触发 HardEvidence 校验）
- AND `Result.Kind` 保持 VerdictPass
- AND LP-1/LP-2/LP-5 集成测试 100% 兼容

#### Scenario: PR-B T05 Span/Metric 完整 wire 收口

- GIVEN `d7.s18.pessimistic.commit.emit` Span 已登记（PR-B 阶段）
- AND `pessimistic_commit_trigger_count` 等 5 metrics 已定义（PR-B 阶段）
- WHEN `escape/engine.go::NotifyPessimistic` 调用
- THEN 完整 OTel span 注册（替换 PR-B slog.Info 占位）
- AND 5 Prometheus metrics 实际 emit (pessimistic_commit_trigger_count + fallback_rule_select_total{rule=...} + mvp_artifact_generated_total + pessimistic_commit_latency_us + fallback_rule_apply_total)
- AND Jaeger trace 可见 + Prometheus scrape 可见

#### Scenario: AC6 convergence.feasible_space_width span

- GIVEN Observe 节点聚合结束
- WHEN `mups/observe/aggregator.go::Aggregate` 出口
- THEN emit `d7.s10.observe.aggregate.complete` Span（沿用 v6.0.x 命名空间）
- AND 附加属性 `convergence.feasible_space_width` (float, [0, 1])
- AND 不新增 Span 命名空间（PR-C 收口，不破坏 v6.0.x span-registry）

#### Scenario: WorkItem.VersionChain 序列化兼容 (omitempty)

- GIVEN 老 WorkItem (PR-A 时期) `VersionChain == nil`
- WHEN `json.Marshal(workItem)` 调用
- THEN 输出 JSON `{"state": {...}}` (无 `version_chain` 字段)
- AND 序列化与 PR-A 时期完全一致（不破坏 v6.0.x 消费者）

#### Scenario: SHA-256 升级对比 PR-B FNV-1a

- GIVEN PR-B `buildChainHash` 使用 FNV-1a (16-char hex)
- AND PR-C 升级为 SHA-256 (64-char hex, `crypto/sha256` 标准库)
- WHEN `VersionChain.Append(content, reason)` 调用
- THEN 内部使用 SHA-256
- AND 外部 Hash 字符串长度变化（16→64 char）但语义不变
- AND 碰撞概率从 2^-64 提升到 2^-128

#### Scenario: Feature Flag 灰度全链路

- GIVEN `D7_HARD_EVIDENCE_ENABLED=false` AND `D7_SIMILARITY_CHECK_ENABLED=false` (both default)
- WHEN `Verifer.Verify` AND `DecomposeSimilarityChecker.Check` 调用
- THEN 两者都正常通过（0 行为变更）
- AND LP-1/LP-2/LP-5 集成测试 100% 兼容
- AND 灰度路径：`D7_HARD_EVIDENCE_ENABLED=true` + `D7_SIMILARITY_CHECK_ENABLED=true` staging 烟测 24h 后再 prod

#### Scenario: IV-1 invariant 守护（interfaces/ 0 import）

- GIVEN 3 NEW interfaces 文件 (version_chain.go + similarity_check.go + hard_evidence.go)
- WHEN `grep -r 'orchestration/' internal/layers/orchestration/interfaces/ | grep -v _test.go` 调用
- THEN 输出 0 行
- AND 仅允许 import `internal/shared/errors/` 等白名单
- AND 整个 v7.0 不会因 interfaces 循环依赖崩溃
