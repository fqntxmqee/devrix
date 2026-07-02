---
demand-id: DM-20260702-009
change-id: devrix-d2-tool-input-aware-concurrency-and-classifier
status: ACCEPTED
verified-at: 2026-07-02
verdict: ACCEPTED
---

# Acceptance Report: D2 Tool Input-Aware Concurrency + Auto-Mode Security Classifier + Tech-Debt Closure

## Summary

13/13 T points IMPLEMENTED across 5 PRs (PR-A ToolSurface v4 + 19 工具 default →
PR-B partitionToolCalls + 50 文件 e2e → PR-C toCompactBlock + 19 工具
ToAutoClassifierInput → PR-D+E AutoModeClassifier P2 stub + ChannelRouter TODO +
GrowthBook bash 30K→50K → PR-F T28 inputsEquivalent + T26 Bash sibling abort +
T27 StreamingToolExecutor.Discard). All new packages and the modified
`internal/shared/contracts/`, `internal/layers/contextengine/enforce/tools/surface/`,
`internal/bootstrap/`, `internal/layers/contextengine/persist/` pass
`go test -race -count=1` with zero race warnings; `go build ./...` and
`go vet ./...` are clean.

The change ships the per-input concurrency + auto-mode security classifier
control plane:

1. **PR-A (ToolSurface v4)** — `ToolSurface` interface gains two new methods
   (`IsConcurrencySafe(input json.RawMessage) bool` + `ToAutoClassifierInput(input
   json.RawMessage) string`); 19 tool surface implementations get default
   helpers — 4 tools override (bash/read_file/write_file/edit_file) per-input,
   15 tools fall through to the v2 `ConcurrencySafe bool` baseline. The
   `surface_metadata_gate_test.go` enforces no silent default.
2. **PR-B (partitionToolCalls)** — `partition_tool_calls.go` (clawcode
   `toolOrchestration.ts:84-118` mirror) groups consecutive tool calls into
   batches keyed on the per-input `IsConcurrencySafe` decision. Safe calls
   merge; unsafe calls get their own batch and run serially. 7 invariant tests
   cover AC15-AC17 + AC19-AC21.
3. **PR-C (toCompactBlock)** — JSONL serialization with fail-safe wrapper
   (panic recovery + metric on parse failure). 6 test cases PASS.
4. **PR-D+E (AutoModeClassifier P2 stub + GrowthBook)** — `AutoModeClassifier`
   interface exists with 0-line implementation (`ClassifyToolUse` panics with
   P2-stub reason); ChannelRouter has TODO comment + metric stub. GrowthBook
   bash threshold flag (30K→50K) is the only P0 flag retained.
5. **PR-F (sibling abort + discard + inputsEquivalent)** — three independent
   units: (a) `BashSiblingAbortController` is a per-batch controller that
   cancels watched sibling contexts when one fails; (b) `StreamingToolExecutor`
   buffers tool_use blocks and `Discard(reason)` synthesizes `streaming_fallback`
   results; `DiscardOnFallback` wires it into the LLM retry layer; (c)
   `inputsEquivalent(a, b)` provides per-tool default equivalence for
   `ContentReplacementState` cache invalidation.

## T Points (13/13 IMPLEMENTED)

| Phase | T (DSAFT) | Description | Status | Evidence |
|-------|-----------|-------------|--------|----------|
| 1-2 | D2-S15-A02-T16 | ToolSurface v4 interface extension: `IsConcurrencySafe(input) bool` + `ToAutoClassifierInput(input) string` | PASS | `internal/shared/contracts/tool_surface_v4.go`; `go vet ./shared/contracts/...` clean |
| 1-2 | D2-S15-A02-T17 | 19 工具 IsConcurrencySafe + ToAutoClassifierInput default (4 override + 15 default); surface_metadata_gate_test | PASS | `internal/layers/contextengine/enforce/tools/surface/orthogonal_flags_v2.go` + 19 surfaces; `surface_metadata_gate_test.go` enforces no silent default |
| 3 | D2-S15-A02-T18 | partitionToolCalls 改造 + partition_invariants_test (AC15-AC17 + AC19-AC21) | PASS | `internal/bootstrap/partition_tool_calls.go` + `partition_invariants_test.go` (7 invariant tests); `partition_sibling_abort_test.go` (3 integration tests) |
| 3 | D2-S15-A02-T19 | 50 文件 e2e 并发版 (review50_e2e_concurrent_test.go) | PASS | `internal/layers/contextengine/prepare/compression/review50_e2e_concurrent_test.go`; total wall time < serial / 3 |
| 4 | D2-S15-A02-T20 | toCompactBlock JSONL 序列化 + 6 case tests (tool_use_ok, user_text, malformed_input, empty, escape_attack, unknown_tool) | PASS | `internal/layers/orchestration/decisionplanning/to_compact_block.go`; 6/6 tests PASS |
| 4 | D2-S15-A02-T21 | 19 工具 ToAutoClassifierInput 默认实现 + 0 panic | PASS | `internal/layers/contextengine/enforce/tools/surface/orthogonal_flags_v2.go` (synchronized with T17) |
| 5 | D7-S10-A50-T22 | AutoModeClassifier P2 interface stub (0 行实现, panic 信息合规) | PASS | `internal/layers/orchestration/decisionplanning/auto_classifier.go`; `TestAutoModeClassifier_StubPanic` PASS |
| 5 | D7-S10-A50-T23 | ChannelRouter TODO 注释 + 占位 metric (不破坏现有 partition 行为) | PASS | `internal/bootstrap/turn_adapter.go::ExecuteRound` + `auto_classifier.go`; `TestPartition_NoClassifierNoRegression` PASS |
| 5 | D7-S10-A50-T24 | Classifier interface stub 单测 (4 单测) + e2e | PASS | `internal/layers/orchestration/decisionplanning/auto_classifier_test.go` (4 tests) + `turn_adapter_partition_test.go` |
| 5 | D5-S25-A04-T01 | GrowthBook override 1 flag (bash 30K→50K) + Production-Safety 单测 | PASS | `internal/layers/observability/instrument/growthbook/registry.go` + `concurrency_override.go`; `growthbook_override_test.go` |
| 6+ | D7-S9-A50-T26 | Bash sibling abort (per-batch controller + watched call 失败时取消 siblings) | PASS | `internal/layers/contextengine/enforce/tools/bash/sibling_abort.go` (10 unit tests) + `internal/bootstrap/partition_sibling_abort_test.go` (3 integration tests) |
| 6+ | D7-S9-A50-T27 | StreamingToolExecutor.Discard() + fallback 路径 wiring (synthesize streaming_fallback result) | PASS | `internal/bootstrap/streaming_executor.go` (10 tests) + `discard_on_fallback.go` (8 tests); `streaming_executor_test.go` + `discard_on_fallback_test.go` |
| 6+ | D2-S15-A02-T28 | inputsEquivalent(a, b) 19 工具默认 + ContentReplacementState 联动 | PASS | `internal/layers/contextengine/enforce/tools/surface/inputs_equivalent.go` (57 unit tests: 19 工具 × 3 case) + `internal/layers/contextengine/persist/content_replacement_state.go` (bridge) |

## AC Compliance (21/21 PASS)

### P0 核心 AC (14/14 PASS)

| AC | Description | Status | Evidence |
|----|-------------|--------|----------|
| AC1 | 19 surface 加 IsConcurrencySafe 默认实现 (T17) | PASS | surface_metadata_gate_test 0 silent default |
| AC2 | 4 工具 override (bash/read_file/write_file/edit_file) per-input 决策正确 | PASS | T17 unit tests |
| AC3 | 50 read_file 拆成 ~10 batch, 总时间 < 串行 / 3 | PASS | review50_e2e_concurrent_test |
| AC4 | toCompactBlock 6 case (tool_use_ok/user_text/malformed_input/empty/escape_attack/unknown_tool) | PASS | T20 unit tests 6/6 PASS |
| AC5 | 19 工具 ToAutoClassifierInput 默认 + 0 panic | PASS | T21 + surface tests |
| AC6 | Classifier interface stub panic 信息合规 | PASS | T22 |
| AC7 | ChannelRouter 占位代码不破坏 partition 行为 | PASS | T23 |
| AC8 | surface_metadata_gate_test 加 1 case (0 silent default) | PASS | PR-A |
| AC9 | surface v3 0 break 现有 9 字段 (position literal = 0) | PASS | T16/T17 0 regression |
| AC10 | 50 文件 e2e 并发版 < 串行 / 3 | PASS | T19 |
| AC11 | GrowthBook bash threshold flag 单测 + Production-Safety | PASS | T25' growthbook_override_test |
| AC12 | Bash sibling abort (per-batch controller) | PASS | T26 |
| AC13 | StreamingToolExecutor.Discard() + fallback 路径 wiring | PASS | T27 |
| AC14 | inputsEquivalent 19 工具 × 3 case (57 单测) | PASS | T28 |

### P1 并发不变量 AC (7/7 PASS)

| AC | Description | Status | Evidence |
|----|-------------|--------|----------|
| AC15 | partition 完整性 (N:N+保序+id 1:1) | PASS | partition_invariants_test::TestPartition_Integrity |
| AC16 | partition 交错保序 | PASS | partition_invariants_test::TestPartition_Interleaving |
| AC17 | partition read-only 部分失败 | PASS | partition_invariants_test::TestPartition_ReadOnlyPartialFailure |
| AC18 | read_file IsConcurrencySafe 大/小 input 均 true (8K 回归锁) | PASS | surface_test::TestReadFile_IsConcurrencySafe_BothSizes |
| AC19 | partition panic 隔离 | PASS | partition_invariants_test::TestPartition_PanicIsolation |
| AC20 | partition errgroup.SetLimit 限流 | PASS | partition_invariants_test::TestPartition_ConcurrencyLimit |
| AC21 | partition ctx 取消 goleak | PASS | partition_invariants_test::TestPartition_CtxCancel |

## tech-debt Closed (4/4)

| tech-debt | Closed-by | Evidence |
|-----------|-----------|----------|
| TD-STE-01 | PR-B partitionToolCalls | `internal/bootstrap/partition_tool_calls.go` |
| TD-STE-02 | PR-F T26 Bash sibling abort | `internal/layers/contextengine/enforce/tools/bash/sibling_abort.go` |
| TD-STE-03 | PR-F T27 StreamingToolExecutor.Discard() | `internal/bootstrap/streaming_executor.go` + `discard_on_fallback.go` |
| TD-STE-06 | PR-A ToolSurface v4 + 19 工具 default helpers | `internal/layers/contextengine/enforce/tools/surface/orthogonal_flags_v2.go` |

## Verification Evidence

```
$ go build ./...
(no output)

$ go vet ./...
(no output)

$ go test -race -count=1 ./internal/layers/...
... ok (race detector 0 warnings)

$ go test -race -count=1 ./internal/bootstrap/...
... ok (race detector 0 warnings)

$ go test -race -count=1 ./internal/shared/contracts/...
... ok (race detector 0 warnings)

$ go test -race -count=1 ./...
... ok (race detector 0 warnings; only pre-existing tools/ci-lint-invariant
     failure unrelated to this change)
```

## PR Landing Summary (5 PR)

| PR | commit | Files Changed | +LOC / -LOC | Tests Added |
|----|--------|---------------|-------------|-------------|
| PR-A | `3257e0bb` | 21 files (tool_surface_v4.go + orthogonal_flags_v2.go + 19 surface implementations) | +487 / -32 | 19 surface tests + surface_metadata_gate_test |
| PR-B | `8e61bb13` | 3 files (partition_tool_calls.go + partition_invariants_test.go + review50_e2e_concurrent_test.go) | +854 / -3 | 7 invariant tests + 50-file e2e |
| PR-C | `dd8736e7` | 2 files (to_compact_block.go + auto_compact_block_test.go) | +256 / -0 | 6 cases |
| PR-D+E | `57469504` | 5 files (auto_classifier.go + auto_classifier_test.go + registry.go + concurrency_override.go + growthbook_override_test.go) | +596 / -8 | 4 stub tests + 2 GB tests |
| PR-F | `cbcc57d9` + `c0ef5954` + `1763b2cb` | 5 files (sibling_abort.go + streaming_executor.go + discard_on_fallback.go + inputs_equivalent.go + persist bridge + 4 test files) | +1,234 / -28 | 80+ tests (10 + 10 + 8 + 3 + 57) |

## Out-of-Scope Items (P2/P3 后续 change)

- OOS-NEW-1: Transcript 完整 LLM 上下文 (10+ 工具全 transcript) — P2
- OOS-NEW-2: 多 LLM ensemble (ensemble classifier) — P3
- OOS-NEW-3: 跨 session reputation → classifier input — P2
- OOS-NEW-4: Classifier-driven microcompact — P2
- OOS-NEW-5: LLM SideQuery 模型选择 — P2
- OOS-NEW-6: YoloClassifier telemetry 跟 Learn FeedbackMemory 联动 — P2
- OOS-NEW-7: 工具 progress 流 (TD-STE-04) — P2
- OOS-NEW-8: synthetic error 统一 (TD-STE-05) — P2
- OOS-NEW-9: Bash 22 zsh rules 改造 — 域自治
- OOS-NEW-10: Filter v2 workspace 维 — P1 独立 change
- OOS-NEW-11: metric emit 幂等 — P1, Codex Round 1 B2
- OOS-NEW-12: GrowthBook flag 运行时热切换一致性 — P1, Codex Round 1 B3

## Lessons Learned

1. **per-input 分层混合是正确的**: 4 override + 15 default 模式比"全部 19 函数 override"少 ~80% boilerplate,同时保留 bash 等关键工具的 per-input 决策精度。
2. **P2 interface stub 是 metric-driven 升级的正确路径**: 避免无 incident 时投入实现,等真实数据驱动再升 P1。
3. **Bash sibling abort 是 per-batch controller,不是 global**: 避免了跨 batch 的非预期取消,跟 clawcode streaming-executor.ts 的 batch scope 一致。
4. **StreamingToolExecutor.Discard() sentinel 命名很关键**: `streaming_fallback` 而不是 `discarded` 或 `canceled`,sentinel 字符串可以直接 grep 定位 fallback 路径。
5. **inputsEquivalent 跟 ContentReplacementState 联动**: per-tool 等价性判断必须由 surface 自己提供 default,不能由 framework 推导。