# Proposal: D7 MUPS 5-node Span 全覆盖 + 目录结构治理

**Change ID:** `devrix-d7-mups-v4-5node-coverage-orchestration`
**Demand ID:** DM-20260625-019
**Priority:** P0
**Sprint:** d7-v6 follow-up
**PR Count:** 2
**Status:** S7_Archived (2026-06-29, FULL — PR #235+#236 merged)
**SoT:** 用户反馈 "D7领域的5节点Span是没有生效吗？请修复。另外5节点的目录非常混乱。"

---

## 1. Background

D7 v6.0.0 6 S + 1 横切结构已稳定（`devrix-d7-six-s-simplification` DM-20260626-001），MUPS v4 5-node 管道（Observe → Plan → Wave → Execute → Verify → Learn）已 S7_Archived（DM-20260625-002/003/004 系列）。但有 2 个遗留问题：

### 1.1 5-node Span 注册缺失（P0 观测盲点）

5 个节点 Span（EmitTaskGraphSynthesize、EmitExecutorSelect、EmitChannelRoute、EmitSystemAnomalyDetect、EmitMemoryPersist）在 hardening/emitter.go 中已实现，但在 coverage registry (`internal/layers/observability/diagnose/coverage/registry.go`) 中未注册。后果：

- `coverage scan` 报告 0% 覆盖（D5 可观测性面板显示"未实现"）
- 端到端 5 节点链路在 Jaeger 中无对应 Op
- LP-1 闭环验证缺少全链路追踪证据

且 5 节点无共享根 Span（`D7_MUPS_Pipeline`），Jaeger 中无法把"一次 turn"作为单一 Span 树查看。

### 1.2 5-node 目录结构混乱（P0 认知负担）

**mups/execute/**：5 个 channel_*.go 重复前缀（channel_commit / channel_exploration / channel_protocol / channel_scenario / channel.go），读文件要绕过前缀。

**mups/learn/**：17 个文件平铺，子领域无边界：
- asset（LearningAsset + Builder + Content × 3）
- memory（3-channel SkillMemory / FeedbackMemory / ScheduledMemory）
- reputation（ReputationEvidence + ReputationStore + BayesianUpdate）
- prior（AdaptivePrior + BetaPrior）

4 个子领域互相 import 容易形成 cycle（learner → asset → memory → asset 真实存在），新成员无法快速定位职责。

## 2. Problem Statement

需要 2 个 P0 fix：

1. **P0-1 + P0-2**：注册 5 节点 Span 到 coverage + 加共享根 Span `D7_MUPS_Pipeline`
2. **P0-3**：物理重构 5-node 目录（execute 去前缀 + learn 子包化），打破 asset ↔ memory cycle

## 3. Goals

| Goal | Metric | Target |
|---|---|---|
| 5-node Span 在 coverage 报告中 | FilesScanned ≥ 5 | ≥ 5（当前 0）|
| Jaeger 中能查看一次 turn 的 5 节点 Span 树 | mupsSpan.parent = orchSpan | yes |
| mups/execute/ 文件名无 channel_ 前缀 | 0 文件匹配 `^channel_` | 0 |
| mups/learn/ 拆为 4 subpackage | 4 子目录（asset/memory/reputation/prior）| 4 |
| 无 import cycle | `go build ./...` | 0 error |
| 23 orchestration packages -race PASS | packages | 23/23 |

## 4. Non-Goals

- 不改任何函数签名（pure physical migration）
- 不改任何业务逻辑（仅文件移动 + import 替换）
- 不动 spec 文档（已在 S6 归档）
- 不动 5-node 5 节点之间的数据契约

## 5. Solution

### 5.1 P0-1 + P0-2 (PR #235) — 5-node Span 治理

- `sessionorchestrator/spans.go`：新增 6 个 SpanMeta（5 node + 1 root）
- `coverage/registry_test.go`：把 6 个新 Op 加入期望列表
- `telemetry/names.go`：新增 `OpD7_S6_MUPS_Pipeline` 常量 + LayerAndComponent 路由
- `orchestrate_path.go`：在 OrchestratePath.Run 中，orchSpan 之后启动 mupsSpan（kind=Internal），作为 5 节点的根 Span

### 5.2 P0-3 (PR #236) — 5-node 目录结构治理

**execute/** (4 文件重命名 + 1 文件改注释)：
```
channel_commit.go    → commit.go
channel_exploration.go → exploration.go
channel_protocol.go  → protocol.go
channel_scenario.go  → scenario.go
channel.go           → (保留，更新引用注释)
```

**learn/** (12 文件 git mv 到 4 subpackage)：
```
asset/      ← learning_asset.go, asset_builder.go, asset_content.go (+ 3 _test.go)
memory/     ← memory.go (+ memory_test.go)
reputation/ ← reputation_evidence.go, reputation_store.go (+ 2 _test.go)  (重命名 → evidence.go, store.go)
prior/      ← adaptive_prior.go (+ adaptive_prior_test.go)
```

**Import cycle 解决**：原 `learner → asset → memory → asset` cycle。把 `DefaultScheduledMaxRetries` 常量从 `memory/memory.go` 上提到 `asset/learning_asset.go`（语义属于 PendingAssetContent 默认值），asset 不再需要 import memory。

**learner.go 角色变化**：从"业务实现"变为"4 subpackage 的 facade"——通过 type alias + var/const re-export 保留旧 API 兼容（`learn.LearningAsset`、`learn.NewAssetBuilder`、`learn.DefaultDeveloperPrior` 等），外部 importer 零修改。

## 6. Risks

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| 5-node Spans 加错位置（orchSpan 之外） | Low | High | P0-2 测试：Jaeger 中 mupsSpan.parent == orchSpan.SpanContext |
| execute/ 重命名漏改 import | Medium | High | 1 文件用 channel.go，4 文件重命名，`go build ./...` 验证 |
| learn/ subpackage split 引入 cycle | High | High | 先拆 prior+reputation（无交叉），再拆 memory+asset（互相 import），上提常量解决 |
| 外部 importer 漏改 | High | High | learner.go facade re-export 100% 旧 API，外部零修改 |
| test file 引用 unexported field (mu) | Low | Low | 加 memory.ScheduledMemory.ForceExhaustRetry test helper |

## 7. Rollout

PR 拆分：
- **PR #235**（P0-1 + P0-2）：5-node Span + root span（约 200 行）
- **PR #236**（P0-3）：5-node 目录结构治理（约 24 文件 git mv + import 替换 + facade re-export）

依赖：PR #235 必须先 merge（因为 #236 不动 Span 治理代码，但需要 #235 已经在 master 上以便 #236 rebase 干净）。

## 8. Open Questions

无（5 节点 Span + 目录结构的具体形态都已与用户对齐）。
