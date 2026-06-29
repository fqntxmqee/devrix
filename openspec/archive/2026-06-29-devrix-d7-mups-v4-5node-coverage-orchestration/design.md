# Design: D7 MUPS 5-node Span 全覆盖 + 目录结构治理

**Change ID:** `devrix-d7-mups-v4-5node-coverage-orchestration`

---

## 1. Architecture

### 1.1 5-node Span 树（修复后）

```
orchSpan (D7_S2_Orchestrator_Process)
  └── mupsSpan (D7_S6_MUPS_Pipeline, kind=Internal)        ← 新增 P0-2
        ├── observeSpan   (D7_S5_Observe_Build)
        ├── planSpan      (D7_S5_TaskGraph_Synthesize)     ← 修复 P0-1
        ├── waveSpan      (D7_S3_Executor_Select)          ← 修复 P0-1
        ├── execSpan      (D7_S6_Channel_Route)            ← 修复 P0-1
        ├── verifySpan    (D7_S4_System_Anomaly_Detect)    ← 修复 P0-1
        └── learnSpan     (D7_S6_Memory_Persist)           ← 修复 P0-1
```

Jaeger 中可按 `D7_MUPS_Pipeline` 一次查询拿到完整 5 节点时序。

### 1.2 learn/ subpackage 依赖图（修复 cycle 后）

```
              ┌──────────┐
              │  learn   │  (facade: type alias + var/const re-export)
              └────┬─────┘
       ┌─────────┬──┴────┬─────────┐
       ▼         ▼       ▼         ▼
   ┌──────┐ ┌──────┐ ┌──────┐ ┌──────────┐
   │asset │ │memory│ │ prior│ │reputation│
   └──┬───┘ └──┬───┘ └──┬───┘ └────┬─────┘
      │        │        │          │
      ▼        ▼        ▼          │
   PendingAsset  LearningAsset  ReputationEvidence ◄──┘
   (常量)        Channel
                 contract
```

- `learner` → 4 subpackages（facade re-export）
- `prior` → `reputation`（BuildAdaptivePrior 用 ReputationEvidence）
- `memory` → `asset`（types only）
- `asset` → 不依赖任何 subpackage（无 cycle）

## 2. Implementation Notes

### 2.1 P0-1 + P0-2 (PR #235)

`sessionorchestrator/spans.go` 新增 6 SpanMeta：

```go
{Name: telemetry.OpD7_S3_Executor_Select, SinceVersion: "2.2.0", Instrumented: true, ...},
{Name: telemetry.OpD7_S4_System_Anomaly_Detect, ...},
{Name: telemetry.OpD7_S5_TaskGraph_Synthesize, ...},
{Name: telemetry.OpD7_S6_Channel_Route, ...},
{Name: telemetry.OpD7_S6_Memory_Persist, ...},
{Name: telemetry.OpD7_S6_MUPS_Pipeline, ...},  // root
```

`telemetry/names.go`：
- 加常量 `OpD7_S6_MUPS_Pipeline = "D7_MUPS_Pipeline"`
- `LayerAndComponent` 加 `D7_MUPS_Pipeline` → `LayerOrchestration, Component("orchestrator")`

`orchestrate_path.go`：
```go
ctx, mupsSpan := startObsSpan(op.obsBridge, ctx, telemetry.OpD7_S6_MUPS_Pipeline,
    tracer.SpanKindInternal,
    tracer.Attribute{Key: "session_id", Value: req.SessionID},
    tracer.Attribute{Key: "pipeline.intent", Value: string(orchtypes.IntentOrchestrate)},
    tracer.Attribute{Key: "pipeline.nodes", Value: "observe,plan,wave,execute,verify,learn"},
)
defer endSpan(mupsSpan)
```

### 2.2 P0-3 (PR #236)

**execute/ 重命名**：4 文件 `git mv` + `channel.go` 改注释引用新文件名。

**learn/ subpackage 拆分**：12 文件 `git mv` 到 4 subpackage。

**Cycle 解决**：`asset/learning_asset.go` 新增 `DefaultPendingMaxRetries = 3` 常量（原 `memory.DefaultScheduledMaxRetries`），`memory.ScheduledMemory.Store` 改用 `asset.DefaultPendingMaxRetries`，`asset_builder.go` 也改用本地常量（不再 import memory）。

**learner.go facade**：
```go
type (
    LearningAsset = asset.LearningAsset
    LearningClass = asset.LearningClass
    AssetBuilder  = asset.AssetBuilder
    ScheduledMemory = memory.ScheduledMemory
    AdaptivePrior = prior.AdaptivePrior
    ReputationStore = reputation.ReputationStore
    // ... 30+ more
)
```

**Test helper**：新增 `memory.ScheduledMemory.ForceExhaustRetry(key)`，替换 test 中的 `l.ScheduledMem.mu.Lock() / l.ScheduledMem.store[key] = ...` 直接 unexported field 访问。

## 3. Verification

| Check | Command | Pass Criterion |
|---|---|---|
| 全项目编译 | `go build ./...` | 0 error |
| D5 可观测性 lint | `go test ./internal/lint/...` | PASS |
| Layer lint | `go test ./internal/lint/layer/...` | PASS |
| Coverage registry | `go test ./internal/layers/observability/diagnose/coverage/...` | PASS, 6 new Op registered |
| D7 orchestration 全包 | `go test -race ./internal/layers/orchestration/...` | 23/23 PASS |
| 全项目测试 | `go test -race ./...` | 全部 PASS（除 pre-existing ci-lint-invariant）|
| 文件名无 channel_ 前缀 | `ls mups/execute/ \| grep -c ^channel_` | 0 |
| learn/ 子目录数 | `ls -d mups/learn/*/ \| wc -l` | 4 |

## 4. Backward Compatibility

通过 `learner.go` 的 type alias + var/const re-export，所有外部 importer（sessionorchestrator, decisionplanning, orchtypes, integration tests）零修改，0 行 import path 替换。

LP-1 / LP-2 / LP-3 / LP-4 / LP-5 全部 100% 兼容（5 节点数据契约未变）。
