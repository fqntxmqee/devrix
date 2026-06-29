---
demand_id: DM-20260629-009
change_id: devrix-d7-taskcontract-unification-pr-c
parent_demand: DM-20260629-006
parent_changes:
  - devrix-d7-taskcontract-unification (DESIGN ONLY, S7_Archived)
  - devrix-d7-taskcontract-unification-pr-a (DM-20260629-007, S7_Archived, PR #325)
  - devrix-d7-taskcontract-unification-pr-b (DM-20260629-008, S7_Archived, PR #327)
title: D7 v7.0 TaskContract 统一 PR-C — CoW VersionChain + Similarity Check + Hard Evidence (L3 防御运行时层)
status: S1_Demand
reporter: 2026-06-29 启动 v7.0 演进第三阶段（最终 PR，闭环 4-Layer × 3-Phase）
priority: P0
dsaft_domain: D7 Orchestration (核心域)
dsaft_layer: L3 (防御运行时层)
---

# Demand: D7 TaskContract 统一 PR-C — CoW VersionChain + Similarity Check + Hard Evidence

## 1. 背景

PR-A (DM-20260629-007) 落地 L1 接口层 (TaskSpec/TaskReport) + L2 字段语义层 (Dissent/Blockage/Resource)。
PR-B (DM-20260629-008) 落地 L3 防御运行时层第 1 阶段：Pessimistic Commit + Rule-based Fallback + 4 候选规则 + 5 类触发条件。

PR-B 把 L3 "**事后降级**" 解决——资源耗尽/CB 触发/多轮 INDETERMINATE 时通过 Pessimistic MVP 兜底输出。
但 L3 还有 "**事前防御**" 缺口：

- **子层 Commit 可覆盖父层认知**（无 CoW）→ 子层 Replan 时无法回溯历史 WHY 决策链，回滚/审计/AAR 困难
- **子层惰性层层转包烧 token**（无 Similarity Check）→ 同一 directive 多次相似子任务无防御机制
- **Verifier 可被 "官话废话" 骗过**（无 Hard Evidence）→ silent corruption：编译过但 runtime panic / 测试 N/A / 关键 invariant 未断言仍判定 PASS
- **PR-B T05 Span/Metric 完整 wire 仍 PLANNED**（占位 slog.Info）→ 灰度期无可观测数据支撑

PR-C 是 v7.0 4-Layer × 3-Phase **最终 PR**，落地 L3 高风险 + L4 收口，闭环 23 AC。

## 2. 业务目标

| 痛点 | 业务影响 | PR-C AC |
|------|---------|---------|
| 子层 Replan 无版本链 | 历史 WHY 决策丢失，回滚/AAR 困难 | **AC13 CoW VersionChain** |
| 子层相似子任务无防御 | token 烧光、无降级信号 | **AC14 Similarity Check > 0.85 拦截** |
| Verifier "空证 PASS" | silent corruption 通过 | **AC15 Hard Evidence 至少 1 项** |
| PR-B T05 占位 | 灰度期无 metric 数据 | **PR-B T05 Span/Metric 完整 wire** |
| 收敛宽度不可观测 | Observe 节点无度量 | **AC6 `convergence.feasible_space_width` span** |
| Feature Flag 灰度无可观测 | 灰度分桶无数据支撑 | **AC22 Feature Flag (继承 PR-B 模式)** |

## 3. 范围（IN SCOPE）

### 3.1 核心 3 ACs（L3 防御运行时层 高风险）

| AC | Name | Activity | 实施位置 |
|----|------|----------|----------|
| **AC13** | CoW VersionChain 不可变追加 + 24h GC | D7-S18-A13 | `interfaces/version_chain.go` + `workmodel/version_chain.go` |
| **AC14** | Similarity Check token-level Jaccard > 0.85 拦截 | D7-S18-A14 | `interfaces/similarity_check.go` + `workmodel/similarity.go` + `decisionplanning/decomposer.go` |
| **AC15** | Hard Evidence (TestCoverage/LogExcerpt/ArtifactHash ≥ 1) | D7-S18-A15 | `interfaces/hard_evidence.go` + `executionflow/verify/verifier.go` |

### 3.2 PR-B T05 收口（继承 PLANNED）

| T | Name | 实施位置 |
|---|------|----------|
| **T05** | `d7.s18.pessimistic.commit.emit` 完整 OTel span + 5 Prometheus metrics | `hardening/emitter.go` + `hardening/metrics.go` |

### 3.3 L4 治理收口（核心 3 项）

| AC | Name | 实施位置 |
|----|------|----------|
| **AC6** | `convergence.feasible_space_width` span (Observe 节点聚合结束) | `hardening/emitter.go` + `mups/observe/aggregator.go` |
| **AC18** | Coverage ≥ 80% (interfaces/ + escape/ + workmodel/ + verify/) | S5 验收硬指标 |
| **AC22** | Feature Flag 灰度（D7_SIMILARITY_CHECK_ENABLED 默认 disabled） | `d7-bootstrap/wire.go` |

## 4. 非目标（OUT OF SCOPE）

| AC | Status | 说明 |
|----|--------|------|
| AC7 AdaptiveThreshold 接入 RunTurn | ⬜ PLANNED | 留 PR-C 后 follow-up `devrix-d7-adaptive-threshold` Change |
| AC8 Layout guard 守护（interfaces/ 白名单） | ✅ DONE | PR-A 已落地，PR-C 维持 IV-1 验证 |
| AC9 race test / AC10 LP regression | ✅ DONE | PR-A/B 已 PASS，PR-C 维持不退化 |
| AC19 Performance Budget | ⬜ PLANNED | 留 PR-C 后 follow-up（benchmark 已就位在 PR-A） |
| AC20 Security Class | ⬜ PLANNED | 留 PR-C 后 follow-up（无新增敏感数据） |
| AC21 Cross-Domain Boundary | ⬜ PLANNED | D2/D4/D6 跨域消费点 PR-A 已就位 |
| AC16 Migration Plan | ✅ DONE | PR-A 已落地 type alias 兼容 |
| AC23 Error Code | ✅ DONE | PR-A/B 已落地 9 ORCH_*，PR-C 新增 3 个 |
| 5 层 CB L2-L5 行为变更 | — | unchanged |
| LP-1/LP-2/LP-5 行为变更 | — | Feature Flag 默认 disabled 保证 100% 兼容 |

## 5. 验收硬指标

| 指标 | 目标 | 验证 |
|------|------|------|
| `interfaces/` 覆盖率 | ≥ 80% | `go test -cover` |
| `escape/` 覆盖率 | ≥ 80% | 同上 |
| `workmodel/` 覆盖率 | ≥ 80%（新增） | 同上 |
| `executionflow/verify/` 覆盖率 | ≥ 80%（Hard Evidence 落地） | 同上 |
| `mups/execute/` 覆盖率 | ≥ 80%（baseline 79.2% 提升） | 同上 |
| 24/24 orchestration packages `-race` PASS | 100% | `go test -race -count=1` |
| LP-1/LP-2/LP-5 闭环 | 100% 兼容 | `go test -tags "integration d7"` |
| IV-1 invariant | 0 import D7 子包 | `grep -r 'orchestration/' internal/layers/orchestration/interfaces/` |
| Feature Flag smoke | 默认 disabled 0 行为变更 | `D7_SIMILARITY_CHECK_ENABLED=false go test` |
| New error code range | 7120-7129 | sharederrors.WithCode |

## 6. 风险

| 风险 | 概率 | 缓解 |
|------|------|------|
| CoW VersionChain 链膨胀 | 中 | hash 索引 + 24h 后台 GC；VersionChain 只保留 hash 不 inline state |
| Hard Evidence 误伤 chat 任务 | 中 | kind-specific 配置：code 要 test/log，chat 要 entity_hash/coherence_score |
| Similarity Check 边界（0.7-0.85）需 LLM 二次校验 | 中 | 默认走 O(1) 拦截；仅 0.7-0.85 边界升级 LLM（v7.0.1 后续） |
| Hash 碰撞 (FNV-1a) | 低 | PR-C 改 SHA-256（替代 PR-B 的 FNV-1a placeholder） |
| Hard Evidence 强制 reject 影响旧 LP-1 测试 | 中 | Verifier 加 Feature Flag 灰度，LP-1 测试维持 hard-evidence=true |

## 7. 时间线

| 阶段 | 日期 | 输出 |
|------|------|------|
| S1-S3 设计 | 2026-06-29 | proposal + design + tasks + review-design + specs/ |
| S4 实现 | 2026-06-30 ~ 2026-07-03 | ~10 NEW + 4 MODIFIED + spec sync |
| S5 验收 | 2026-07-04 | 24+ packages -race PASS + 覆盖率 ≥ 80% |
| S6 归档 | 2026-07-04 | archive/ + PR auto-merge + v7.0.0 release |
| **总计** | **~5 天** | PR-C S7_Archived + v7.0.0 minor release |

## 8. 后续路径

PR-C S7_Archived → v7.0.0 minor release 闭环。

可选 follow-up Change（按用户确认按需立）：
- `devrix-d7-adaptive-threshold` (AC7) — AdaptiveThreshold 接入 RunTurn
- `devrix-d7-performance-budget` (AC19) — Performance benchmark 收敛
- `devrix-d7-security-classification` (AC20) — TaskSpec.Classification 标签
- `devrix-d7-similarity-llm-boundary` — 0.7-0.85 边界 LLM 二次校验
- `devrix-d7-versionchain-distributed` — VersionChain 跨 session 持久化
