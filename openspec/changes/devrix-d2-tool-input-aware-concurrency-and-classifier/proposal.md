# Proposal: D2 Tool Input-Aware Concurrency + Auto-Mode Security Classifier

**Change ID:** `devrix-d2-tool-input-aware-concurrency-and-classifier`
**Demand ID:** DM-20260702-009
**Status:** S2_Proposal (待 S3-Gate review)
**Created:** 2026-07-02
**Parent Demand:** `demand.md`

---

## 0. Synthesis Lineage

本文档基于 2026-07-02 对 DM-20260702-008 P1 延期 + 复盘清单 6 项审计 + 借鉴关系 10 项的复盘, 主要输入:

**clawcode 真实源码** (10 处):

- `/Users/fukai/workspace/clawcode/src/Tool.ts:402` (`isConcurrencySafe` interface)
- `/Users/fukai/workspace/clawcode/src/Tool.ts:556` (`toAutoClassifierInput` interface)
- `/Users/fukai/workspace/clawcode/src/Tool.ts:759,767` (TOOL_DEFAULTS — fail-closed)
- `/Users/fukai/workspace/clawcode/src/Tool.ts:402,556,712-714` (`inputsEquivalent` 35 字段之一)
- `/Users/fukai/workspace/clawcode/src/services/tools/toolOrchestration.ts:84-118` (`partitionToolCalls`)
- `/Users/fukai/workspace/clawcode/src/services/tools/toolOrchestration.ts:26-32` (batch consume)
- `/Users/fukai/workspace/clawcode/src/utils/permissions/yoloClassifier.ts:378-410` (`toCompactBlock`)
- `/Users/fukai/workspace/clawcode/src/utils/permissions/yoloClassifier.ts:1485-1493` (sideQuery)
- `/Users/fukai/workspace/clawcode/src/tools/BashTool/BashTool.tsx:434-442` (per-input IsConcurrencySafe + ToAutoClassifierInput 实例)
- `/Users/fukai/workspace/clawcode/src/services/tools/StreamingToolExecutor.ts` (siblingAbortController + discard)

**devrix 现状** (5 处):

- `internal/shared/contracts/tool_surface.go:39-43` (devrix 静态 `ConcurrencySafe bool` 现状)
- `internal/bootstrap/turn_adapter.go:277` (devrix `ExecuteRound` 现状)
- `internal/layers/contextengine/prepare/compression/persist.go` (T01 PersistToFile — 联动 GrowthBook)
- `internal/layers/contextengine/persist/content_replacement_state.go` (T04 ContentReplacementState — 联动 GrowthBook)
- `internal/layers/contextengine/enforce/tools/bash/bash_runner.go` (BashTool runner — 集成 sibling abort)

**复盘文档** (4 处):

- `openspec/tech-debt/streaming-tool-executor-v2.md` (TD-STE-01~06, 4 项被本 change 关闭)
- `openspec/tech-debt/queryloop-error-recovery.md` (TD-QL-03 已 CLOSED, TD-QL-07 联动)
- `/Users/fukai/brain/01知识探索/项目/20260620-certain-architecture/core-concepts/53-clawcode-tools-design.md` (35 字段参考)
- `openspec/changes/devrix-token-design-v2/{demand,proposal,design}.md` (借鉴关系 10 项)

---

## 1. 提案动机 (RC-1 + RC-2)

### 1.1 RC-1: `ConcurrencySafe` 静态 bool 是治标

devrix 现状 (`internal/shared/contracts/tool_surface.go:39-43`):

```go
// ConcurrencySafe: multiple invocations of the same tool may run in parallel
// without mutual interference (e.g. read_file on different paths).
// turn_adapter.ExecuteRound uses this to decide parallel vs sequential dispatch.
ConcurrencySafe bool
```

**问题**: 静态 bool, **per-tool**, 不知道具体 input

| 工具 | 现状 (v2 bool) | 应该的 per-input 决策 |
|------|----------------|---------------------|
| `bash` | `false` (永远串行) | `git status` → true, `rm -rf` → false |
| `read_file` | `true` (永远并发) | 大文件 (>8K) → 串行 (1 batch), 小文件 → true |
| `write_file` | `false` | 永远 false (写并发会乱序) |
| `edit_file` | `false` | 永远 false |
| `grep` | `true` | true (read-only) |
| `glob` | `true` | true (read-only) |
| `lsp_*` | `true` | true (read-only) |
| `web_fetch` | `true` | 永远 false (per-host rate-limit) |
| `verify_*` | `true` | 永远 false (重资源) |
| `free_fork` | `false` | 永远 false (spawn 副作用) |
| `mcp_*` | `true` | 跟具体 mcp server 协议有关, 默认 false (保守) |

**vs clawcode** (`src/Tool.ts:402`):

```typescript
isConcurrencySafe(input: z.infer<Input>): boolean
```

**per-input 函数**, 19 工具 surface 自己决定。`BashTool` 实际实现 (`src/tools/BashTool/BashTool.tsx:434-437`):

```typescript
isConcurrencySafe(input) {
  return this.isReadOnly?.(input) ?? false;
}
```

**consequence**: 9 个 `git status` 在 devrix 当前全串行 (9×1s = 9s), 在 clawcode 1 batch 并发 (1×1s = 1s). **9× speedup** for typical read-only batches.

### 1.2 RC-2: 无 auto-mode 安全分类器, 缺中间层

devrix 当前安全栈 (3 道):

1. **事前静态规则** — `surface.CheckPermission` (VerifyContract 4 元组 Burden × Class × Discipline × Outcome, DM-20260701-007)
2. **执行** — tool runner
3. **事后验证** — `executionflow/verify/` Verify 节点

**缺中间层** (执行后, Verify 节点前). 静态规则漏掉的攻击直接执行, 后果不可逆.

**vs clawcode** (`src/utils/permissions/yoloClassifier.ts`):

- **事前投影**: `Tool.toAutoClassifierInput(input)` → 紧凑 string (e.g. `ls -la` for Bash, `/tmp/x: new content` for Edit)
- **transcript 序列化**: `toCompactBlock` → `{"Bash":"ls"}` JSONL 喂独立 LLM (SideQuery)
- **LLM 判 allow/deny**: 5s timeout, fail-open (网络挂了默认 allow + log metric)
- **失败 telemetry**: `tengu_auto_mode_malformed_tool_input` 事件 + `tengu_auto_mode_classifier_unavailable` 事件
- **复用 ToolUseContext**: sideQuery 复用 main loop 的 LLM gateway, 不另起 connection

**对比**:

| 层 | devrix | clawcode |
|----|--------|----------|
| L0 静态规则 | ✅ VerifyContract 4 元组 | ✅ checkPermissions |
| L1 SideQuery 中间层 | ❌ 无 | ✅ yoloClassifier |
| L2 运行时沙箱 | ✅ Bash AST analyzer (W4 AC10) | ✅ bashClassifier |
| L3 事后 Verify | ✅ executionflow/verify | ✅ TaskVerify (post) |

本 change 加 L1 SideQuery 中间层, 跟现有 L0/L2/L3 互补.

---

## 2. 核心机制 (M1-M5)

### 2.1 M1 — Per-Input `IsConcurrencySafe`

**接口** (`internal/shared/contracts/tool_surface_v4.go` 新建):

```go
// IsConcurrencySafe reports whether THIS SPECIFIC INVOCATION (with this
// specific input) may run concurrently with other concurrency-safe tool
// calls in the same batch. The decision may depend on input — e.g. bash
// with a read-only command is concurrency-safe, but bash with `rm -rf`
// is not.
//
// Default implementation: return ToolSpec.ConcurrencySafe (v2 static
// bool) for back-compat. Tools that need per-input logic MUST override.
//
// Fail-safe: implementations MUST NOT panic; on parse failure, return
// false (treat as not concurrency-safe). Emits telemetry
// `tool.is_concurrency_safe.failed` on parse failure for observability.
//
// Mirrors clawcode Tool.ts:402 + Tool.ts:759 (`(_input?: unknown) => false`
// default — fail-closed).
type ToolSurface interface {
    // ... existing 9 + 6 v3 methods ...
    
    // IsConcurrencySafe(input) is the v4 per-input decision.
    IsConcurrencySafe(input []byte) bool
    
    // ToAutoClassifierInput(input) is the v4 auto-mode classifier projection.
    ToAutoClassifierInput(input []byte) string
}
```

**19 工具默认实现** (`orthogonal_flags_v2.go` 新建):

```go
// BuiltinSurface 6 工具: bash/write/edit/read/grep/glob
func (s *BuiltinSurface) IsConcurrencySafe(input []byte) bool {
    var p struct {
        Command string `json:"command"`
    }
    if err := json.Unmarshal(input, &p); err != nil {
        // fail-safe: 保守 false
        return false
    }
    switch s.toolName {
    case "read_file", "grep", "glob":
        return true
    case "bash":
        // bash 走 isReadOnly 决策 (跟 clawcode BashTool.tsx:434-437 一致)
        return isReadOnlyBashCommand(p.Command)
    case "write_file", "edit_file":
        return false
    }
    return false
}
```

### 2.2 M2 — `partitionToolCalls` Batch 改造

**位置**: `internal/bootstrap/turn_adapter.go:277` 改造

**当前实现** (per-tool 静态 bool):

```go
concMap := a.concurrencyMap() // toolName → bool
// 9 个 read_file 全部 false? 不, read_file true → 全部塞 errgroup 并发
// 9 个 bash 全部 false → 全部串行
```

**改造后** (per-input 函数 + partition):

```go
// partitionToolCalls mirrors clawcode toolOrchestration.ts:84-118.
// Consecutive concurrency-safe tool calls go into the same batch;
// the next non-safe call starts a new batch. Each batch runs
// concurrently (errgroup); batches run sequentially to preserve
// LLM-issued ordering within non-safe regions.
func (a *contextEngineAdapter) partitionToolCalls(
    calls []ToolCall,
    surfaces map[string]ToolSurface,
) []Batch {
    batches := []Batch{}
    for _, call := range calls {
        s := surfaces[call.Name]
        input := parseInput(call.Input)
        safe := s.IsConcurrencySafe(input)
        if safe && len(batches) > 0 && batches[len(batches)-1].isConcurrencySafe {
            batches[len(batches)-1].calls = append(batches[len(batches)-1].calls, call)
        } else {
            batches = append(batches, Batch{isConcurrencySafe: safe, calls: []ToolCall{call}})
        }
    }
    return batches
}
```

**预期提速**: 9 个 `git status` (read-only) → 1 batch 并发, 1s 而非 9s. 50 文件 review (9 并发 read_file) → ~10 batch × 1s = ~10s 而非 ~50s 串行.

### 2.3 M3 — Per-Input `ToAutoClassifierInput`

**接口**:

```go
// ToAutoClassifierInput returns a compact, security-relevant
// representation of this tool invocation for the auto-mode security
// classifier. Examples:
//
//   Bash("ls -la")               → `"ls -la"`
//   Edit("/tmp/x", "new content") → `"/tmp/x: new content"`
//   Read("/etc/passwd")          → `""`  (no security relevance, skip)
//
// Return '' to skip this tool in the classifier transcript
// (e.g. tools with no security relevance like TodoWrite). Mirrors
// clawcode Tool.ts:556 + Tool.ts:767 (`(_input?: unknown) => ''`
// default).
//
// Fail-safe: on parse failure or panic, return the raw input string
// and emit `auto_mode.malformed_tool_input` metric. NEVER panic.
type ToolSurface interface {
    ToAutoClassifierInput(input []byte) string
}
```

**19 工具默认实现**:

```go
func (s *BuiltinSurface) ToAutoClassifierInput(input []byte) string {
    var p struct {
        Command string `json:"command"`
        Path    string `json:"path"`
    }
    if err := json.Unmarshal(input, &p); err != nil {
        metric.AutoModeMalformedToolInput(s.toolName).Inc()
        return string(input) // fail-open: 落 raw input
    }
    switch s.toolName {
    case "bash":
        return p.Command
    case "read_file", "grep", "glob":
        return "" // 无安全相关性, 跳过
    case "edit_file", "write_file":
        return fmt.Sprintf("%s: %s", p.Path, truncate(p.Content, 200))
    }
    return ""
}
```

### 2.4 M4 — `toCompactBlock` JSONL Transcript

**位置**: `internal/layers/orchestration/decisionplanning/to_compact_block.go` 新建

**claude 真实代码** (`yoloClassifier.ts:378-410`):

```typescript
function toCompactBlock(block, role, lookup) {
  if (block.type === 'tool_use') {
    const tool = lookup.get(block.name)
    if (!tool) return ''
    const input = (block.input ?? {}) as Record<string, unknown>
    let encoded
    try {
      encoded = tool.toAutoClassifierInput(input) ?? input
    } catch (e) {
      logEvent('tengu_auto_mode_malformed_tool_input', { toolName: block.name })
      encoded = input
    }
    return JSON.stringify({ [block.name]: encoded })
  }
  return JSON.stringify({ [role]: extractTextContent(block) })
}
```

**devrix Go 翻译**:

```go
// toCompactBlock mirrors clawcode yoloClassifier.ts:378-410.
// Serializes one transcript block as a JSON dict line: `{"Bash":"ls"}`
// for tool calls, `{"user":"text"}` for user text. The tool value is
// the per-tool ToAutoClassifierInput projection. JSON escaping means
// hostile content can't break out of its string context to forge a
// `{"user":...}` line — newlines become `\n` inside the value.
//
// Returns '' for tool_use blocks whose tool encodes to '' (e.g. read_file
// has no security relevance).
func toCompactBlock(
    block TranscriptBlock,
    role string,
    surfaceLookup map[string]ToolSurface,
) string {
    if block.Type == "tool_use" {
        s, ok := surfaceLookup[block.Name]
        if !ok {
            return "" // unknown tool, skip
        }
        encoded, err := safeToAutoClassifierInput(s, block.Input)
        if err != nil {
            // fail-safe: 落 raw input + emit metric
            metrics.AutoModeMalformedToolInput(block.Name).Inc()
            encoded = string(block.Input)
        }
        line, _ := json.Marshal(map[string]string{block.Name: encoded})
        return string(line)
    }
    text := extractTextContent(block)
    line, _ := json.Marshal(map[string]string{role: text})
    return string(line)
}

// safeToClassifier wraps surface.ToAutoClassifierInput with panic recovery.
// Mirror clawcode yoloClassifier.ts:393-404 fail-safe semantics.
func safeToAutoClassifierInput(s ToolSurface, input []byte) (string, error) {
    var result string
    var err error
    func() {
        defer func() {
            if r := recover(); r != nil {
                err = fmt.Errorf("panic: %v", r)
            }
        }()
        result = s.ToAutoClassifierInput(input)
    }()
    return result, err
}
```

### 2.5 M5 — Auto-Mode Classifier (SideQuery LLM)

**位置**: `internal/layers/orchestration/decisionplanning/auto_classifier.go` 新建

**接口**:

```go
// AutoModeClassifier runs the LLM-driven security classifier on a
// compact transcript. Returns Allow | Deny + reason.
//
// 5s timeout hard cap (mirrors clawcode yoloClassifier.ts:5s policy).
// Fail-open on LLM unavailable (return Allow + emit
// `auto_mode.classifier_unavailable` metric).
//
// The classifier NEVER replaces VerifyContract 4 元组 (第一道安全);
// it is a complementary middle layer.
type AutoModeClassifier interface {
    ClassifyToolUse(ctx context.Context, transcript []TranscriptBlock) (YoloResult, error)
}

type YoloResult struct {
    Decision YoloDecision  // Allow | Deny
    Reason   string        // LLM 解释
    Source   string        // "anthropic" | "external" | "rule-fallback"
}

type YoloDecision int

const (
    YoloAllow YoloDecision = iota
    YoloDeny
)
```

**SideQuery 复用** (`yoloClassifier.ts:1485-1493`):

```go
// sideQuery invokes a one-shot LLM completion reusing the main loop's
// gateway. The classifier prompt + transcript is sent; the response is
// parsed for {decision, reason}.
func (c *AutoModeClassifierImpl) sideQuery(
    ctx context.Context,
    prompt string,
    transcript string,
) (YoloResult, error) {
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()
    
    resp, err := c.gateway.Complete(ctx, llmgateway.Request{
        Model:    c.config.ClassifierModel,
        System:   prompt,
        Messages: []Message{{Role: "user", Content: transcript}},
    })
    if err != nil {
        // fail-open + metric
        metrics.AutoModeClassifierUnavailable().Inc()
        return YoloResult{Decision: YoloAllow, Source: "rule-fallback"}, nil
    }
    
    return parseYoloResult(resp)
}
```

**ChannelRouter 集成** (T23): `ExecuteRound` 在 `partitionToolCalls` 之后, 每个 batch 跑之前调 `AutoModeClassifier.ClassifyToolUse`. Deny → 整个 batch skip + emit `auto_mode.denied` metric.

---

## 3. 架构图 (M1-M5 关系)

```
LLM turn
   │
   ▼
ExecuteRound (turn_adapter.go:277)
   │
   ├─ partitionToolCalls (per-input IsConcurrencySafe)
   │     │
   │     └─→ Batch[0] (9 read_file)         Batch[1] (1 write_file)    Batch[2] (1 bash)
   │            │                                   │                           │
   │            │ auto-mode classifier             │ classifier                 │ classifier
   │            │ ClassifyToolUse                  │                            │
   │            │ (5s timeout, fail-open)          ▼                            ▼
   │            ▼                              skip (write to plan)         run (or deny)
   │       Allow → errgroup (concurrent)
   │
   └─ per-batch: result merge
```

**3 道安全栈** (本 change 强化 L1 中间层):

| 层 | 机制 | 兜底 |
|----|------|------|
| L0 事前静态 | VerifyContract 4 元组 (DM-20260701-007) | surface.CheckPermission |
| **L1 中间 SideQuery (本 change)** | **AutoModeClassifier + toCompactBlock** | **5s timeout fail-open** |
| L2 运行时沙箱 | Bash AST analyzer (W4 AC10) | 静态 allowlist |
| L3 事后 Verify | executionflow/verify | 重新跑 |

---

## 4. 博弈论共识 (H13-H17, 新增)

| ID | 设计承诺 | 落地 T 点 |
|----|----------|----------|
| **H13** | **per-input 并发决策, 不过度保守** | **T16-T19 (IsConcurrencySafe + partitionToolCalls)** |
| **H14** | **3 道安全栈 (L0/L1/L2/L3) 互补, L1 中间层不替换 L0** | **T20-T24 (auto-mode classifier)** |
| **H15** | **Fail-safe 默认 (抛错 → 不并发 / 落 raw input)** | **T16/T20 (interface 默认实现)** |
| **H16** | **Transcript 投影 + JSONL 序列化, 不暴露整个 transcript 给 LLM** | **T20-T21 (toCompactBlock)** |
| **H17** | **Telemetry 完整 (malformed_input + classifier_unavailable + denied)** | **T22-T24 (metric 集成)** |

---

## 5. T 点划分 (9 T, P1 → P0 验收)

| T | DSAFT | 优先级 | 内容 | 关闭/引用 |
|---|-------|--------|------|------|
| T16 | D7-S9-A50-T16 | P0 | ToolSurface interface v4 加 `IsConcurrencySafe(input) bool` + `ToAutoClassifierInput(input) string` + `inputsEquivalent(a, b []byte) bool` | **TD-STE-06 partial** |
| T17 | D7-S9-A50-T17 | P0 | 19 工具 surface 默认实现 (BuiltinSurface 6 + LSPToolSurface 5 + FreeFork/Tracker/Verify/AskUser/BackgroundTask/ToolSearch 8) | **TD-STE-06 closed-by** |
| T18 | D7-S9-A50-T18 | P0 | `turn_adapter.ExecuteRound` 改造为 `partitionToolCalls` batch | **TD-STE-01 closed-by** |
| T19 | D7-S9-A50-T19 | P0 | 50 文件 e2e 并发版 + 9 并发 read_file batch test | — |
| T20 | D7-S10-A50-T20 | P0 | `toCompactBlock` JSONL 序列化 (claude yoloClassifier.ts:378-410 翻译) | — |
| T21 | D7-S10-A50-T21 | P0 | 19 工具 `ToAutoClassifierInput` 默认实现 (Bash=command, Edit="path: content", Read/grep/glob="" skip) | — |
| T22 | D7-S10-A50-T22 | P0 | AutoModeClassifier 实现 (5s timeout SideQuery + fail-open) | — |
| T23 | D7-S10-A50-T23 | P0 | ChannelRouter 集成 (ExecuteRound 每个 batch 前调 ClassifyToolUse) | — |
| T24 | D7-S10-A50-T24 | P0 | Classifier 7 单测 (allow/deny/timeout/throw/malformed_input/empty_transcript/policy_violation) + AC5/AC6/AC10 e2e | — |
| T25 | D5-S25-A04-T01 (new) | P0 | **GrowthBook runtime override** — 19 工具 per-tool 阈值 + Classifier enable + ConcurrencySafe 全部可走 GrowthBook feature flag, 默认全关 | DM-20260702-008 借鉴 #8 |
| T26 | D7-S9-A50-T25 (new) | P1 | **Bash sibling abort** — BashTool 集成 `siblingAbortController`, 并行 Bash 中一个失败 abort 兄弟, 返 synthetic `Cancelled: parallel tool call errored` | **TD-STE-02 closed-by** |
| T27 | D7-S9-A50-T26 (new) | P1 | **Discard on fallback** — `StreamingToolExecutor.Discard()` + QueryLoop fallback 路径 wiring, 在途/queued 工具注入 `streaming_fallback` synthetic result | **TD-STE-03 closed-by** (TD-QL-03 CLOSED) |
| T28 | D2-S15-A02-T29 (new) | P2 | **inputsEquivalent(a, b)** — 19 工具 surface 加 `inputsEquivalent(a, b []byte) bool` 默认实现, 配合 ContentReplacementState (T04) 实现 cache invalidation 收口 | clawcode Tool.ts:712-714 |

---

## 6. 兼容性 (0 业务代码 out-of-scope diff)

- **ToolSpec v3 struct 0 字段变更** — 0 break (15 字段 → 15 字段)
- **ToolSurface interface 0 字段删除** — additive (v4 加 2 方法, 已有 surface 通过 `v3 → v4` 升级)
- **ExecuteRound 行为升级** — 旧 9 read_file 串行 (假) → 新 9 read_file 1 batch 并发 (真), 实际提速
- **无 surface 改语义** — 19 surface 默认 `IsConcurrencySafe` 行为跟 v2 `ConcurrencySafe bool` 一致 (per-input 函数 fallback 到 bool, AC1)
- **Classifier 默认关闭** — ChannelRouter 集成在 Shadow mode (log-only), 跟 DM-20260701-007 PromptPressure shadow 模式一致, 默认 enable 走 GrowthBook flag (T25)
- **GrowthBook 默认全关** — T25 默认所有 flag 全关, Production-Safety: 必须在 GrowthBook 后台显式 enable 才生效, 单元测试覆盖 "未 flag 开启时不改变行为"
- **Bash sibling abort 边界** — T26 只 abort 同 batch 并行 Bash 兄弟, 不 abort 父 QueryLoop turn, 不影响非 Bash 工具, 单测覆盖边界
- **Discard 只在 fallback 触发** — T27 只在 QueryLoop 切换 fallback model 前调 Discard(), 正常路径不触发, 单测覆盖"无 fallback 时无 discard 行为"
- **inputsEquivalent 默认按字段比较** — T28 19 工具 surface 默认按 JSON unmarshal 后逐字段比较, 跟 clawcode `inputsEquivalent` 默认行为一致, 不引入新机制

---

## 7. 测试策略 (T19 + T24 端到端)

### 7.1 单元测试 (T16-T18, T20-T23)

- **per-input decision**: 19 工具 × 2 方法 = 38 单测 (passes-fail matrix)
- **partitionToolCalls**: 6 case (all_safe, all_unsafe, mixed, empty, single, large_N)
- **toCompactBlock**: 6 case (tool_use_ok, user_text, malformed_input, empty, escape_attack, unknown_tool)
- **AutoModeClassifier**: 7 case (allow/deny/timeout/throw/malformed_input/empty_transcript/policy_violation) — T24

### 7.2 端到端 e2e (T19, AC10)

**复用 `internal/layers/contextengine/prepare/compression/review50_e2e_test.go`** 加并发版本:

- 50 文件 review, **9 并发 read_file batch** (per partitionToolCalls)
- 期望: 50/50 完成, 总时间 < 串行 / 3
- 老 e2e (T27) 保留做回归基线

### 7.3 集成测试 (T19 + T24)

- `turn_adapter_partition_test.go`: 100 个并发 read_file, 全部允许 + 实际并发
- `auto_classifier_integration_test.go`: 9 read_file + 1 write_file + 1 bash deny, 全 batch 行为符合 partition

---

## 8. 范围外 (OOS, 走 P2/P3 后续 change)

> 本 change 收纳了原 OOS-1 (GrowthBook 走 T25) + TD-STE-01/02/03/06 (4 项 tech-debt 关闭) + inputsEquivalent (走 T28)

- OOS-NEW-1: Transcript 完整 LLM 上下文 (10+ 工具全 transcript) — P2
- OOS-NEW-2: 多 LLM ensemble (ensemble classifier) — P3
- OOS-NEW-3: 跨 session reputation → classifier input — P2
- OOS-NEW-4: Classifier-driven microcompact (T13 PerMessageBudget 联动) — P2
- OOS-NEW-5: LLM SideQuery 模型选择 (Haiku vs Sonnet) — P2
- OOS-NEW-6: YoloClassifier telemetry 跟 Learn FeedbackMemory 联动 — P2
- OOS-NEW-7: 工具 progress 流 (TD-STE-04) — P2
- OOS-NEW-8: synthetic error 统一 (TD-STE-05) — P2
- OOS-NEW-9: Bash 22 zsh rules 改造 (DM-20260701-007 OOS-7 弱相关) — 域自治
- OOS-NEW-10: Filter v2 workspace 维 (DM-20260701-007 OOS-10) — 走 P1 独立 change

---

## 9. 验收 + 归档 (S5 + S6)

- **S5 验收**: 9 T 全 IMPLEMENTED + AC1-AC10 全 PASS + 50 文件 e2e 并发版 < 串行 / 3 + verify-archive.sh 12 PASS
- **S6 归档**: `openspec/archive/2026-07-02-devrix-d2-tool-input-aware-concurrency-and-classifier/` + 域文档同步 (D2 t-registry +9 T, D7 t-registry +9 T, root v5.15.0)

---

## 10. 时间表

| 周 | 活动 | 产出 |
|----|------|------|
| W1 D1-D2 | PR-A ToolSurface v4 + 19 工具 IsConcurrencySafe 默认 | T16-T17 | 关闭 TD-STE-06 |
| W1 D3-D5 | PR-B partitionToolCalls + 50 文件并发 e2e | T18-T19 | 关闭 TD-STE-01 |
| W2 D1-D2 | PR-C ToAutoClassifierInput + 19 工具默认 | T20-T21 | — |
| W2 D3-D4 | PR-D AutoModeClassifier + ChannelRouter 集成 | T22-T23 | — |
| W2 D5 | PR-E Classifier 测试 + telemetry + 端到端 e2e | T24 + AC1-AC10 | — |
| W3 D1-D2 | PR-F Tech-debt closure + GrowthBook (T25-T28) | T25-T28 | 关闭 TD-STE-02 + TD-STE-03 |
| W3 D3 | S3-Gate codex 复审 + S4-Gate | 13 T 全 IMPLEMENTED |
| W3 D4-D5 | S5 验收 + S6 归档 + PR squash auto-merge | ACCEPTED + 4 tech-debt closed |
