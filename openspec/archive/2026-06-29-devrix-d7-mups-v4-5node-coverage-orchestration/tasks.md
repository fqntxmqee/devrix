# Tasks: D7 MUPS 5-node Span 全覆盖 + 目录结构治理

**Change ID:** `devrix-d7-mups-v4-5node-coverage-orchestration`
**Total Tasks:** 6 (3 P0 + 3 P1)
**Sprint:** d7-v6 follow-up

---

## P0-1 注册 5 节点 Span 到 coverage registry

**File:** `internal/layers/observability/diagnose/coverage/registry_test.go`
**File:** `internal/layers/orchestration/sessionorchestrator/spans.go`
**Effort:** 0.5 天
**AC:** `go test ./internal/layers/observability/diagnose/coverage/...` PASS，期望列表包含 5 个新 Op

| ID | Description | Status |
|---|---|---|
| T01 | 在 `registry_test.go` 期望列表加 5 个 Op：D7_S3_Executor_Select, D7_S4_System_Anomaly_Detect, D7_S5_TaskGraph_Synthesize, D7_S6_Channel_Route, D7_S6_Memory_Persist | DONE |
| T02 | 在 `sessionorchestrator/spans.go` 注册 5 个 SpanMeta（SinceVersion: "2.2.0", Instrumented: true）| DONE |
| T03 | `go test ./internal/layers/observability/diagnose/coverage/...` PASS | DONE |

## P0-2 加 5-node 共享根 Span D7_MUPS_Pipeline

**File:** `internal/layers/orchestration/sessionorchestrator/orchestrate_path.go`
**File:** `internal/layers/observability/instrument/telemetry/names.go`
**Effort:** 1 天
**AC:** Jaeger 中 mupsSpan.parent == orchSpan.SpanContext

| ID | Description | Status |
|---|---|---|
| T01 | telemetry/names.go 加常量 `OpD7_S6_MUPS_Pipeline = "D7_MUPS_Pipeline"` | DONE |
| T02 | LayerAndComponent switch 加 D7_MUPS_Pipeline → LayerOrchestration + orchestrator | DONE |
| T03 | orchestrate_path.go 在 orchSpan 之后启动 mupsSpan，6 个 attributes | DONE |

## P0-3 5-node 目录结构治理

**File:** `internal/layers/orchestration/mups/execute/{commit,exploration,protocol,scenario}.go`
**File:** `internal/layers/orchestration/mups/learn/{asset,memory,reputation,prior}/`
**Effort:** 1.5 天
**AC:** 23 orchestration packages -race PASS；`go vet ./...` 0 error；无 import cycle

| ID | Description | Status |
|---|---|---|
| T01 | execute/ 4 个 channel_*.go 重命名 + channel.go 改引用注释 | DONE |
| T02 | learn/ 17 文件 git mv 到 4 subpackage（asset/memory/reputation/prior）| DONE |
| T03 | 解决 asset ↔ memory import cycle：DefaultPendingMaxRetries 从 memory 上提到 asset | DONE |
| T04 | learner.go 改为 facade：type alias + var/const re-export 全部旧 API | DONE |
| T05 | memory.ScheduledMemory 加 ForceExhaustRetry test helper（替换直接 mu/store 访问）| DONE |
| T06 | 23 orchestration packages -race PASS（含 parent learn + 4 subpackage）| DONE |

## P1 follow-up（已在 #235 / #236 中一并完成）

| ID | Description | Status |
|---|---|---|
| T01 | coverage/registry_test.go 期望 6 个新 Op（5 node + 1 root）| DONE |
| T02 | memory/memory_test.go 全部加 asset. 前缀 + ForceExhaustRetry 改造 | DONE |
| T03 | prior/adaptive_prior_test.go 全部加 reputation. 前缀 | DONE |
