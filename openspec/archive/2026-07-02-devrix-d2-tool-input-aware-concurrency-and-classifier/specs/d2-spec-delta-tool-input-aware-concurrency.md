# D2 Spec Delta: Tool Input-Aware Concurrency + Auto-Mode Classifier + Tech-Debt Closure

**Change:** devrix-d2-tool-input-aware-concurrency-and-classifier (DM-20260702-009)
**Archived:** 2026-07-02
**Status:** S7_Archived

## Delta Summary

5 PR 收口，13 T IMPLEMENTED，21 AC PASS，4 tech-debt closed (TD-STE-01/02/03/06)。
本 delta 取代 d2-context-engine spec.md 中关于 ToolSurface interface v3 → v4 的
所有引用，并补充 19 工具默认实现决策表。

## Delta 1: ToolSurface Interface v4 扩展 (T16-T17)

### New Methods

```go
type ToolSurface interface {
    // ... existing 9 v2 + 6 v3 methods (UNCHANGED) ...

    // T16: per-input concurrency safety decision.
    // Default impl returns ToolSpec.ConcurrencySafe (v2 static bool)
    // for back-compat. Tools that need per-input logic override.
    // Fail-safe: NEVER panic; on parse failure, return false.
    IsConcurrencySafe(input json.RawMessage) bool

    // T16: per-tool auto-mode classifier projection.
    // Returns compact, security-relevant string (e.g. "ls -la" for Bash).
    // Returns '' to skip this tool in classifier transcript.
    // Fail-safe: on parse failure, return raw input + emit metric.
    ToAutoClassifierInput(input json.RawMessage) string
}
```

### 19 工具 Decision Table (分层混合: 4 override + 15 default)

| 工具 | override? | IsConcurrencySafe(input) | ToAutoClassifierInput(input) |
|------|-----------|--------------------------|------------------------------|
| **bash** | ✅ override | per-input: `isReadOnlyBashCommand(command)` → true/false | command (e.g. "ls -la") |
| **read_file** | ✅ override | 永远 true (read-only, 天然并发安全, 无 size-based 决策 — 跟 v2 `orthogonal_flags.go:22` 一致) | "" (无安全相关性, 跳过) |
| **write_file** | ✅ override | 永远 false (写并发会乱序) | "path: content (first 200 chars)" |
| **edit_file** | ✅ override | per-input: 同 target 路径互斥 → false | "path: old → new (first 200 chars)" |
| grep | ❌ default | true (read-only) | "" |
| glob | ❌ default | true (read-only) | "" |
| lsp_goto_definition | ❌ default | true (read-only) | "" |
| lsp_hover | ❌ default | true (read-only) | "" |
| lsp_references | ❌ default | true (read-only) | "" |
| lsp_workspace_symbol | ❌ default | true (read-only) | "" |
| lsp_code_action | ❌ default | false (server side-effect) | "action: kind" |
| free_fork | ❌ default | false (spawn 副作用) | "agent: prompt (first 200 chars)" |
| tracker | ❌ default | true (read-only) | "" |
| verify_* | ❌ default | false (重资源) | "verify: kind" |
| ask_user_question | ❌ default | false (interactive) | "question: text" |
| background_task | ❌ default | false (spawn 副作用) | "task: description" |
| tool_search | ❌ default | true (read-only) | "" |
| web_fetch | ❌ default | false (网络副作用) | "url" |
| web_search | ❌ default | false (per-host rate-limit) | "query" |
| mcp_* | ❌ default | false (保守, 未知 mcp 协议) | "server.tool: input (first 200 chars)" |

## Delta 2: partitionToolCalls (T18)

### New Algorithm

```go
// partitionToolCalls mirrors clawcode toolOrchestration.ts:84-118.
func partitionToolCalls(calls []ToolCall, surfaces SurfaceLookup) []Batch {
    var batches []Batch
    for i, call := range calls {
        safe := IsConcurrencySafeForTool(call.Name, json.RawMessage(call.Input), surfaces, concurrency)
        if safe && len(batches) > 0 && batches[len(batches)-1].IsConcurrencySafe {
            batches[len(batches)-1].Calls = append(batches[len(batches)-1].Calls, call)
            continue
        }
        batches = append(batches, Batch{
            IsConcurrencySafe: safe,
            Calls:             []llmgateway.ToolCall{call},
            Indices:           []int{i},
        })
    }
    return batches
}
```

### Invariants (AC15-AC21)

| AC | Invariant | Test |
|----|-----------|------|
| AC15 | N:N + 保序 + id 1:1 | TestPartition_Integrity |
| AC16 | 交错保序 (batch 间不交换) | TestPartition_Interleaving |
| AC17 | read-only 部分失败 (单 batch 内失败不影响其它) | TestPartition_ReadOnlyPartialFailure |
| AC18 | read_file 大/小 input 均 true (8K 回归锁) | TestReadFile_IsConcurrencySafe_BothSizes |
| AC19 | panic 隔离 (单个 call panic 不影响 batch 其它) | TestPartition_PanicIsolation |
| AC20 | errgroup.SetLimit 限流 | TestPartition_ConcurrencyLimit |
| AC21 | ctx 取消 goleak | TestPartition_CtxCancel |

## Delta 3: toCompactBlock JSONL 序列化 (T20)

### New Function

```go
func toCompactBlock(
    block TranscriptBlock,
    role string,
    surfaceLookup map[string]ToolSurface,
) string {
    if block.Type == "tool_use" {
        s, ok := surfaceLookup[block.Name]
        if !ok { return "" }
        encoded, err := safeToAutoClassifierInput(s, block.Input)
        if err != nil {
            metrics.AutoModeMalformedToolInput(block.Name).Inc()
            encoded = string(block.Input) // fail-open
        }
        line, _ := json.Marshal(map[string]string{block.Name: encoded})
        return string(line)
    }
    text := extractTextContent(block)
    line, _ := json.Marshal(map[string]string{role: text})
    return string(line)
}
```

### 6 Test Cases

| Case | Input | Expected |
|------|-------|----------|
| tool_use_ok | bash {"command":"ls"} | `{"bash":"ls"}` |
| user_text | "hello" | `{"user":"hello"}` |
| malformed_input | bash {invalid json} | fail-open → raw input + metric |
| empty | bash {"command":""} | `{"bash":""}` |
| escape_attack | bash {"command":"rm -rf /"} | `{"bash":"rm -rf /"}` (escaped properly) |
| unknown_tool | unknown_tool {...} | empty string (skipped) |

## Delta 4: GrowthBook Override 1 Flag (T25')

### New Flag

```yaml
# internal/layers/observability/instrument/growthbook/registry.go
flags:
  bash_readonly_threshold_bytes:
    default: 30000    # 30K
    override: 50000   # 50K (ops 调优需求)
    description: "Bash readonly threshold for large command result"
```

### Production-Safety Constraints

- 默认全关: 启动时 `seedFeatureFlags` 走 secure default (空 map = 全关)
- flag 未开启时, override 返回 defaultVal, **0 行为变化**
- flag 运行时变更通过 GrowthBook SDK 推送, 不需要重启 devrix

### Deferred Flags (P2)

- bash readonly canary (5% → 50%): 等 bash 30K→50K 实际调优后再立 flag
- classifier 5% canary: 等 T22'-T23' 升 P1 实施时一并立

## Delta 5: inputsEquivalent (T28)

### New Function

```go
// inputsEquivalent 判断两个 input 是否语义等价 (per-tool 默认).
// 默认实现: 比较每个字段 (depth-first), 字段顺序不影响结果.
// 19 工具 override: bash (parse AST) / read_file/write_file/edit_file (path-only) / 等.
func inputsEquivalent(toolName string, a, b json.RawMessage) bool
```

### 57 Unit Tests

19 工具 × 3 case = 57 单测 (相同 / 字段顺序不同 / 完全不同)

### ContentReplacementState Bridge

`inputsEquivalent` 跟 `ContentReplacementState` 联动:
- 当 file 内容变化时, 调用 `inputsEquivalent(toolName, oldInput, newInput)` 判断 cache 是否需要 invalidation
- 19 工具 per-tool 默认, 避免 framework 误判

## tech-debt Closed

| tech-debt | Closed-by | Files |
|-----------|-----------|-------|
| TD-STE-01 | PR-B partitionToolCalls | `internal/bootstrap/partition_tool_calls.go` |
| TD-STE-06 | PR-A ToolSurface v4 + 19 工具 default helpers | `internal/layers/contextengine/enforce/tools/surface/orthogonal_flags_v2.go` |

## Cross-Reference

- d7-spec-delta: D7 Execute 节点 partition + Bash sibling abort + Discard
- d5-spec-delta: LTL-Lite L4-L6 终止不变量 cross-check 配套