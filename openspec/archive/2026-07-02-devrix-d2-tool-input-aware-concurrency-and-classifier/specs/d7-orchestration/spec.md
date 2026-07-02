# Delta: D7 Orchestration — Execute Node Partition + Bash Sibling Abort + Discard + AutoClassifier Stub

**Change ID:** `devrix-d2-tool-input-aware-concurrency-and-classifier`
**Demand ID:** DM-20260702-009
**Affects:** D7-S9 (Execute) + D7-S10 (Verify)

---

## ADDED

### Requirement: D7-S9-A50 partitionToolCalls 集成

`ExecuteRound` SHALL 调用 `partitionToolCalls(calls, surfaces)` 替换单层 errgroup:

- 调用 `buildConcurrencyByTool(surfaces)` 一次性构建 baseline
- 调用 `IsConcurrencySafeForTool(name, input, surfaces, concurrency)` per-input
- batch 间串行 + batch 内并发

#### Scenario: AC3 50 read_file 拆成 ~10 batch, 总时间 < 串行 / 3

- GIVEN 50 read_file calls (同一 file, 不同 path)
- WHEN ExecuteRound
- THEN 拆成 1 个 safe batch (50 calls 合并)
- AND wall time < serial / 3

### Requirement: D7-S9-A50 Bash Sibling Abort (T26)

`BashSiblingAbortController` SHALL 提供 per-batch 控制:

- `Register(callID, toolName) (ctx, cancel, ok)` — 返回 sibling ctx (含 cancel)
- `Unregister(callID)` — 清理
- `AbortSiblings(callID, reason) bool` — 取消其它 watched siblings (idempotent)
- `Close()` — 释放所有 ctx

#### Scenario: AC12 Watched Call 失败时取消 Siblings

- GIVEN 3 个 bash calls (A, B, C) 同 batch
- WHEN A 失败 (result.Error != "")
- THEN AbortSiblings(A.ID, reason) 取消 B + C 的 sibling ctx
- AND B + C 在下个 ctx check 看到 ctx.Done()
- AND B + C 通过 ExecuteFunc 返 synthetic cancel result

#### Scenario: Non-Watched Tool 不参与

- GIVEN bash + read_file 同 batch
- WHEN bash 失败
- THEN 仅 watched siblings 被取消
- AND read_file 继续执行 (read-only 语义保留)

#### Scenario: Idempotent AbortSiblings

- GIVEN 多次调用 AbortSiblings(A.ID, reason)
- WHEN 第一次 → 取消 siblings
- THEN 第二次/第三次 → false (no-op, sync.Once wrapped)

### Requirement: D7-S9-A50 StreamingToolExecutor.Discard() (T27)

`StreamingToolExecutor` SHALL 提供 per-LLM-iteration buffer:

- `Buffer(call)` — 累积 streamed tool_use
- `Buffered()` / `BufferedCount()` — read-only 快照
- `Discard(reason) []ToolResult` — 合成 streaming_fallback sentinel results
- `IsDiscarded()` / `DiscardReason()` — 状态查询
- `DiscardOnFallback.OnFallback(reason) []ToolResult` — wiring (atomic counter + idempotent)

#### Scenario: AC13 Discard Synthesizes streaming_fallback Results

- GIVEN 3 buffered tool_use calls (IDs c1, c2, c3)
- WHEN Discard("primary_503")
- THEN 返 3 个 ToolResult, 每个 Error = "streaming_fallback: primary_503"
- AND ToolCallID 顺序与 buffered 一致 (c1, c2, c3)
- AND executor 状态变更为 discarded

#### Scenario: Nil Receiver / Nil Executor 防御

- GIVEN nil *StreamingToolExecutor
- WHEN 调用任何方法
- THEN 全部 no-op, 不 panic

### Requirement: D7-S10-A50 AutoModeClassifier P2 Interface Stub (T22'-T23')

`AutoModeClassifier` interface SHALL 存在 (0 行实现):

- `ClassifyToolUse(ctx, transcript) (YoloResult, error)`
- `YoloResult{Decision, Reason, Source}` (Source: "anthropic" | "external" | "rule-fallback")
- 当前 stub panic("P2 interface, not implemented; see gaming-debate-round3-convergence.md")

ChannelRouter TODO 占位:
- `internal/bootstrap/turn_adapter.go::ExecuteRound` partitionToolCalls 之后 batch 跑之前预留 `ClassifyToolUse` 调用点
- 当前直接走 default allow
- TODO 注释 + metric stub

#### Scenario: AC5/AC6 Stub Panic 信息合规

- GIVEN AutoModeClassifier interface
- WHEN 调用 ClassifyToolUse
- THEN panic 信息含 "P2 interface, not implemented" + "gaming-debate-round3-convergence.md"

#### Scenario: AC7 ChannelRouter 占位代码不破坏 partition 行为

- GIVEN ExecuteRound 集成 stub call (但当前直接 default allow)
- WHEN ExecuteRound 运行
- THEN partition 行为完整 (跟 PR-B 一致)
- AND 0 行为变化

#### Scenario: 触发升 P1 实施的条件

- 主触发: `verify_contract.deny_rate` 7 天滑动 > 5%
- 次触发: devrix 真实 incident 涉及 auto-mode 误判 (任意 1 次)
- 任何触发 → 开 `devrix-d2-tool-input-aware-concurrency-and-classifier-pr-d-followup` Change 实施

## Cross-Reference

- d2-spec-delta: ToolSurface v4 + 19 工具 default + partition + toCompactBlock + inputsEquivalent
- d5-spec-delta: GrowthBook override 1 flag