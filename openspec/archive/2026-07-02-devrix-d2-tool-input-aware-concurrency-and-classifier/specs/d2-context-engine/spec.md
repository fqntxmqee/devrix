# Delta: D2 Context Engine — Tool Input-Aware Concurrency + Classifier + Tech-Debt Closure

**Change ID:** `devrix-d2-tool-input-aware-concurrency-and-classifier`
**Demand ID:** DM-20260702-009
**Affects:** D2-S15 (Prepare — per-input concurrency + classifier projection + inputsEquivalent)

---

## ADDED

### Requirement: D2-S15-A02 ToolSurface v4 — per-input concurrency safety decision

`ToolSurface` interface SHALL include 2 new methods:

- `IsConcurrencySafe(input json.RawMessage) bool` — per-input concurrency safety decision
- `ToAutoClassifierInput(input json.RawMessage) string` — per-tool auto-mode classifier projection

#### Scenario: ToolSurface v4 interface 0 break existing methods

- GIVEN existing 9 v2 + 6 v3 ToolSurface methods
- WHEN 加 IsConcurrencySafe + ToAutoClassifierInput
- THEN 所有现有 ToolSurface 实现编译 0 break (因为提供 default helper)
- AND grep `IsConcurrencySafe(input json.RawMessage) bool` 仅 1 命中 (interface)

#### Scenario: 19 工具 IsConcurrencySafe 分层混合 (4 override + 15 default)

- GIVEN 19 工具 surface (bash/read_file/write_file/edit_file/grep/glob/lsp_*/free_fork/tracker/verify_*/ask_user_question/background_task/tool_search/web_fetch/web_search/mcp_*)
- WHEN 全部加 IsConcurrencySafe 默认实现
- THEN bash override per-input `isReadOnlyBashCommand(command)` → true/false
- AND read_file override 永远 true (read-only, 无 size-based 决策)
- AND write_file override 永远 false (写并发会乱序)
- AND edit_file override per-input 同 target 路径互斥 → false
- AND 其余 15 工具走 default table (true for read-only / false for write-side-effect)
- AND surface_metadata_gate_test 0 silent default

### Requirement: D2-S15-A02 partitionToolCalls — Clawcode Mirror

`partitionToolCalls(calls, surfaces)` SHALL group consecutive tool calls into batches:

- Calls with `IsConcurrencySafe=true` 合并入当前 batch (clawcode "consecutive safe merge")
- Calls with `IsConcurrencySafe=false` 独占 batch
- 返回 `[]Batch` 含 `IsConcurrencySafe` + `Calls` + `Indices`

#### Scenario: AC15 完整性 (N:N + 保序 + id 1:1)

- GIVEN N 个 tool calls
- WHEN partition
- THEN 总 batch 数 ≤ N (合并导致减少)
- AND results[Indices[i]] 对应 calls[i] (id 1:1 保序)
- AND 每 call 出现在且仅出现在一个 batch

#### Scenario: AC20 errgroup.SetLimit 限流

- GIVEN concurrencyLimit = 3
- WHEN 执行 batch 内 N=10 个 safe calls
- THEN 同时最多 3 个 goroutine 运行

### Requirement: D2-S15-A02 toCompactBlock — JSONL Serialization

`toCompactBlock(block, role, surfaceLookup)` SHALL 返回 tool_use JSONL 序列化:

- block.Type == "tool_use" → `{"<tool_name>": "<encoded>"}` 其中 encoded = surface.ToAutoClassifierInput(input)
- block.Type == "text" → `{"<role>": "<text>"}`
- Parse failure → raw input + emit metric (fail-open)
- Panic recovery wrap (fail-safe)

#### Scenario: 6 case (tool_use_ok, user_text, malformed_input, empty, escape_attack, unknown_tool)

- GIVEN 6 cases
- WHEN apply toCompactBlock
- THEN 6/6 PASS, 0 panic, malformed metric emit

### Requirement: D2-S15-A02 inputsEquivalent — Per-Tool Default

`inputsEquivalent(toolName, a, b) bool` SHALL 判断两个 input 语义等价:

- 19 工具 per-tool 默认实现 (depth-first 字段比较 + 字段顺序无关)
- 4 工具 override: bash (parse AST) / read_file/write_file/edit_file (path-only)

#### Scenario: 19 工具 × 3 case (相同 / 字段顺序不同 / 完全不同)

- GIVEN 19 工具 × 3 case = 57 cases
- WHEN apply inputsEquivalent
- THEN 相同 → true; 字段顺序不同 → true; 完全不同 → false (除 mcp_* 0 行为变化)
- AND 0 panic

#### Scenario: ContentReplacementState Bridge

- GIVEN ContentReplacementState 缓存 (path → input_hash)
- WHEN file 内容变化
- THEN inputsEquivalent(oldInput, newInput) 判断 cache invalidation
- AND false → invalidation 触发

## REMOVED

(none — 现有 9 字段 ToolSpec 0 break)

## MODIFIED

### D2-S15 PrepareExecutionContext — partitionToolCalls 集成

- `internal/bootstrap/turn_adapter.go::ExecuteRound` 改造点 line 277
- 替换单层 errgroup 为两层 (batch 间串行 + batch 内并发)
- 复用 errgroup.SetLimit 限流 (AC20)
- panic isolation per call (AC19)

## Cross-Reference

- d7-spec-delta: D7 Execute 节点 partition + Bash sibling abort + Discard
- d5-spec-delta: LTL-Lite L4-L6 终止不变量 cross-check 配套