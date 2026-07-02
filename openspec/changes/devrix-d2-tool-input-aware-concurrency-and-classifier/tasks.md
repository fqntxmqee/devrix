# Tasks: D2 Tool Input-Aware Concurrency + Auto-Mode Security Classifier

**Change ID:** `devrix-d2-tool-input-aware-concurrency-and-classifier`
**Demand ID:** DM-20260702-009
**博弈论 Round 3 收敛:** 2026-07-02 (Claude + Codex + Cursor 三方共识)
**T 点总数:** 12 (T16-T21 P0 = 6, T22'-T23' P2 interface stubs = 2, T25'-T28 P0 = 4)
**AC 总数:** 21 (AC1-AC3/AC6-AC10 P0 + AC11 P0 缩减版 + AC12-AC13 P1 + AC4/AC5/AC14 P2 + **AC15-AC19 P0 并发不变量** + AC20-AC21 P1) — S3 设计阶段博弈论 AC 复核增补 (Claude+Codex 两方, cursor 后端宕机待补审)
**PR 收口:** **5** (PR-A / PR-B / PR-C / **PR-D+E 合并** / PR-F)
**tech-debt closed:** 4 (TD-STE-01/02/03/06, 引用见各 T 点)
**阶段:** 0 (决策) → 1-2 (interface + per-tool 默认) → 3 (partition + e2e) → 4 (投影 + 序列化) → 5 (classifier P2 interface stub) → 6 (验证 + GrowthBook 1 flag) → 6+ (sibling abort / discard / inputsEquivalent)

> **博弈论 Round 3 关键调整** (vs 需求原状):
> - **D1**: per-input 函数 = **分层混合** (interface 19 函数 + **4 工具 override** + 15 default)
> - **D2**: auto-mode classifier = **P2 interface only** (T22'-T23' 仅接口, 0 行实现, metric 触发升 P1)
> - **D3**: GrowthBook = **P0 部分保留 1 flag** (T25' bash 30K→50K; 其余推迟)
> - **D4**: **5 PR (PR-D+PR-E 合并)** vs 原状 6 PR

---

## 阶段 0: 决策 (本次, 0 T 点)

- [x] close devrix-token-design-v2 P1 延期 (T16-T24 走本 change)
- [x] 复盘吸收 6 项 (T25 GrowthBook + TD-STE-01/02/03/06 + inputsEquivalent)
- [x] 起草本 proposal / demand / tasks
- [x] 博弈论 Round 3 收敛 (Claude+Codex+Cursor 三方共识, 见 `gaming-debate-round3-convergence.md`)
- [x] proposal.md 更新到 S2_Proposal (反映 Round 3 收敛)
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
      IsConcurrencySafe(input json.RawMessage) bool

      // T16: per-tool auto-mode classifier projection.
      // Returns compact, security-relevant string (e.g. "ls -la" for Bash).
      // Returns '' to skip this tool in classifier transcript.
      // Fail-safe: on parse failure, return raw input + emit metric.
      ToAutoClassifierInput(input json.RawMessage) string
  }
  ```
- **仿:** clawcode `src/Tool.ts:402,556` interface + `src/Tool.ts:759,767` TOOL_DEFAULTS (`(_input?: unknown) => false` / `(_input?: unknown) => ''`)
- **AC:** 19 surface 全部加默认实现 (T17), 不破坏现有 19 工具的行为 (ConcurrencySafe bool 兼容)
- **tech-debt 引用:** TD-STE-06 partial (interface 框架, T17 完成 close)

### T17 — 19 工具 surface 默认实现 (分层混合)

- **DSAFT:** D7-S9-A50-T17
- **位置:** `internal/layers/contextengine/enforce/tools/surface/orthogonal_flags_v2.go` (新建) + 19 surface
- **19 surface 分布:** (同需求原状, 略)
- **4 工具 override + 15 工具 default 决策表** (博弈论 D1 收敛):

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

- **AC:** 19 surface 加默认实现, `surface_metadata_gate_test.go` 加 1 case (AC8: 0 silent default); `read_file` surface_test 断言大/小 input 均 `IsConcurrencySafe=true` (AC18: 8K anti-pattern 回归锁)
- **tech-debt 引用:** **TD-STE-06 closed-by** (per-tool `IsConcurrencySafe` 走 surface 元数据, 跟 clawcode `Tool` interface 一致)

---

## 阶段 3: partitionToolCalls + 50 文件 e2e (P0, 2 T 点) — PR-B 路线

### T18 — ChannelRouter partitionToolCalls 改造

- **DSAFT:** D7-S9-A50-T18
- **位置:** `internal/bootstrap/turn_adapter.go:277` 改造 + `partition_tool_calls.go` (新建 helper)
- **API:**
  ```go
  // partitionToolCalls mirrors clawcode toolOrchestration.ts:84-118.
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
          safe := s.IsConcurrencySafe(input)  // ← per-input 调用
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
- **AC:** 50 read_file 拆成 ~10 batch, 总时间 < 串行 / 3 (AC3)
- **并发不变量测试 (S3 AC 复核增补):** 新建 `partition_invariants_test.go` 覆盖 AC15 (完整性 N:N+保序+id 1:1) / AC16 (交错保序) / AC17 (read-only 部分失败) / AC19 (panic 隔离) / AC20 (并发上限 errgroup.SetLimit) / AC21 (ctx 取消 goleak) — 折进 T18, 不新增 T 编号
- **tech-debt 引用:** **TD-STE-01 closed-by** (混合批次: safe × N 并行 + unsafe 独占, 替换 v1 整批 all-or-nothing)

### T19 — 50 文件 e2e 并发版

- **DSAFT:** D7-S9-A50-T19
- **位置:** `internal/layers/contextengine/prepare/compression/review50_e2e_concurrent_test.go`
- **AC:** 50/50 完成, 总 wall time < 串行 / 3 (AC10)
- **老 e2e 保留做回归基线**

---

## 阶段 4: ToAutoClassifierInput + toCompactBlock (P0, 2 T 点) — PR-C 路线

### T20 — toCompactBlock JSONL 序列化

- **DSAFT:** D7-S10-A50-T20
- **位置:** `internal/layers/orchestration/decisionplanning/to_compact_block.go` (新建)
- **API:**
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
- **fail-safe wrapper:** panic recovery (同需求原状)
- **AC:** 6 case (tool_use_ok, user_text, malformed_input, empty, escape_attack, unknown_tool) PASS

### T21 — 19 工具 ToAutoClassifierInput 默认实现

- **DSAFT:** D7-S10-A50-T21
- **位置:** 19 surface 加 ToAutoClassifierInput 方法 (T17 同步落地)
- **fail-safe:** parse failure → 返回 raw input + emit `auto_mode.malformed_tool_input` metric
- **AC:** 19 surface 全部加 ToAutoClassifierInput 默认, 0 panic

---

## 阶段 5: Auto-Mode Classifier (P2 interface only, 2 T 点) — PR-D+E 路线

### T22' — AutoModeClassifier **interface stub** (P2, 0 行实现)

- **DSAFT:** D7-S10-A50-T22 (stub)
- **位置:** `internal/layers/orchestration/decisionplanning/auto_classifier.go` (新建)
- **API:**
  ```go
  // AutoModeClassifier — P2 interface only, 见 gaming-debate-round3-convergence.md
  // 触发升 P1 实施的条件: verify_contract.deny_rate > 5% (任意 7 天窗口)
  type AutoModeClassifier interface {
      ClassifyToolUse(ctx context.Context, transcript []TranscriptBlock) (YoloResult, error)
  }

  type YoloResult struct {
      Decision YoloDecision
      Reason   string
      Source   string  // "anthropic" | "external" | "rule-fallback"
  }
  ```
- **实现状态:** 仅接口 + 类型定义, `ClassifyToolUse` 方法 panic("P2 interface, not implemented; see gaming-debate-round3-convergence.md")
- **预期升级触发 metric:**
  - 主触发: `verify_contract.deny_rate` 7 天滑动 > 5%
  - 次触发: devrix 真实 incident 涉及 auto-mode 误判 (任意 1 次)
  - 任何触发 → 开 `devrix-d2-tool-input-aware-concurrency-and-classifier-pr-d-followup` Change 实施
- **AC:** 编译通过 + interface test 存在 (确认 panic 信息符合预期) PASS

### T23' — ChannelRouter 集成 stub (P2)

- **DSAFT:** D7-S10-A50-T23 (stub)
- **位置:** `internal/bootstrap/turn_adapter.go:277` ExecuteRound 集成 **interface-only 调用点**
- **行为:** partitionToolCalls 之后, batch 跑之前**预留调用点** `ClassifyToolUse`, 但**当前直接走 default allow** (TODO 注释 + metric 占位)
  ```go
  // TODO(gaming-debate-round3): 升 P1 时接入 AutoModeClassifier
  // 触发 metric: verify_contract.deny_rate 7d 滑动 > 5%
  // 触发 change: devrix-d2-tool-input-aware-concurrency-and-classifier-pr-d-followup
  ```
- **Shadow mode 默认:** interface 已存在, 但 ChannelRouter 当前不实例化任何实现 (P2 stub)
- **AC:** interface compile + 占位代码不破坏现有 ChannelRouter 行为 PASS

---

## 阶段 6: 验证 + GrowthBook 1 flag (P0, 2 T 点) — PR-D+E 路线 (D+E 合并)

### T24' — Classifier interface stub 单测 + e2e (PR-D+E 合并验证)

- **DSAFT:** D7-S10-A50-T24 (合并 PR-E 入 PR-D)
- **位置:** `internal/layers/orchestration/decisionplanning/auto_classifier_test.go` (新建) + `turn_adapter_partition_test.go`
- **4 单测 (P2 interface stub 范围):**
  1. `TestAutoModeClassifier_InterfaceExists` — interface 编译 + panic 信息合规
  2. `TestAutoModeClassifier_StubPanic` — 当前调用 panic 行为符合预期
  3. `TestPartition_NoClassifierNoRegression` — ChannelRouter 占位代码不破坏 partition 行为
  4. `TestChannelRouter_PlaceholderCallsite` — TODO 注释 + metric 占位存在
- **端到端 e2e (AC10):** 50 文件 review 用 9 并发 read_file batch (无 classifier, 但 partition 行为完整)
- **telemetry 验证 (AC5/AC6):** 占位 metric stub 编译存在 (不实际触发, 等 P1 实施后激活)

### T25' — GrowthBook override 1 flag (P0, bash 30K→50K)

- **DSAFT:** D5-S25-A04-T01 (new, 缩减到 1 flag)
- **位置:**
  - `internal/layers/observability/instrument/growthbook/registry.go` (新建) — flag 注册中心 + 默认全关
  - `internal/layers/observability/instrument/growthbook/concurrency_override.go` (T16-T17 IsConcurrencySafe 联动)
- **API:**
  ```go
  // registry 启动时 load GrowthBook feature flags; 默认全关
  func NewGrowthBookOverride(seedFeatureFlags map[string]bool) *Override

  // 唯一 P0 flag: bash 30K → 50K threshold override
  // Cursor 引用: devrix 内部 ops 配置, DM-20260630 用户验收复盘
  // 触发: devrix ops 调优需要, 不是推测性需求
  func (o *Override) BashReadOnlyThresholdBytes(defaultBytes int) int
  ```
- **Production-Safety 硬约束:**
  - 默认全关: 启动时 `seedFeatureFlags` 走 secure default (空 map = 全关)
  - flag 未开启时, override 返回 defaultVal, **0 行为变化**
  - flag 运行时变更通过 GrowthBook SDK 推送, 不需要重启 devrix
- **推迟的 2 flag (P2):**
  - bash readonly canary (5% → 50%): 等 bash 30K→50K 实际调优后再立 flag
  - classifier 5% canary: 等 T22'-T23' 升 P1 实施时一并立
- **AC:** AC11 (growthbook_override_test + bash threshold 1 flag PASS + Production-Safety 1 单测 PASS)
- **复盘吸收:** 项 #1 (DM-20260702-008 借鉴 #8, Cursor 引用 `persist/growthbook_override.go:1-9` 先例)

---

## 阶段 6+: Bash Sibling Abort + Discard + inputsEquivalent (P1/P1/P2, 3 T 点) — PR-F 路线 (W3 D1-D2)

### T26 — Bash sibling abort (P1)

- **DSAFT:** D7-S9-A50-T25 (new)
- **位置:**
  - `internal/layers/contextengine/enforce/tools/bash/sibling_abort.go` (新建)
  - `internal/layers/contextengine/enforce/tools/bash/bash_runner.go` 改造点
- **API:** (同需求原状)
- **AC:** AC12 PASS
- **tech-debt 引用:** **TD-STE-02 closed-by**

### T27 — StreamingToolExecutor.Discard() + fallback 路径 wiring (P1)

- **DSAFT:** D7-S9-A50-T26 (new)
- **位置:** `internal/bootstrap/streaming_executor.go` (新建) + `internal/bootstrap/discard_on_fallback.go`
- **API:** (同需求原状)
- **AC:** AC13 PASS
- **tech-debt 引用:** **TD-STE-03 closed-by** (依赖 TD-QL-03 已 CLOSED)

### T28 — inputsEquivalent(a, b) 19 工具默认实现 (P2)

- **DSAFT:** D2-S15-A02-T29 (new)
- **位置:** `internal/layers/contextengine/enforce/tools/surface/inputs_equivalent.go` (新建) + ContentReplacementState 联动
- **API:** (同需求原状)
- **AC:** AC14 (57 单测 PASS: 19 工具 × [相同 / 字段顺序不同 / 完全不同] 3 case)

---

## 验证状态 (本地 — 阶段 6 + 6+ 后)

- [ ] `go build ./...` 0 errors
- [ ] `go test -count=1 ./internal/layers/...` 全量 PASS
- [ ] `go test -race -count=1 ./...` 全量 PASS (master 预存失败 tools/ci-lint-invariant 除外)
- [ ] 12 T 全 IMPLEMENTED + 21 AC 全 PASS (P0 14 + P1 4 + P2 3)
- [ ] 50 文件 e2e 并发版 < 串行 / 3 (AC10)
- [ ] `partition_invariants_test` 全 PASS (AC15 完整性 / AC16 交错 / AC17 read部分失败 / AC19 panic 隔离 / AC20 限流 / AC21 goleak)
- [ ] read_file IsConcurrencySafe 大/小 input 均 true (AC18: 8K 回归锁)
- [ ] 50 文件 e2e 并发版 < 串行 / 3 (AC10)
- [ ] AutoModeClassifier interface stub panic 信息合规
- [ ] ChannelRouter 占位代码不破坏 partition 行为
- [ ] 4 tech-debt 关闭 (TD-STE-01/02/03/06) — 走 PR-F 同步标注
- [ ] GrowthBook bash threshold flag 单测 PASS (Production-Safety)

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
- OOS-NEW-11: metric emit 幂等 (同一 error 多次 emit 去重) — P1, Codex Round 1 B2, cursor 恢复后可翻案纳入 AC
- OOS-NEW-12: GrowthBook flag 运行时热切换一致性 (partition 途中 flag 变更防御) — P1, Codex Round 1 B3, "理论上不该发生" 边际收益低

> **OOS 编号变更说明:** 原 OOS-1~7 中 OOS-1 (GrowthBook) 部分走 T25' 已吸收, 重新编号 OOS-NEW-1~10.

---

## 5 PR 收口计划 (D+E 合并)

| PR | commit | 内容 | T 数 | AC | tech-debt closed |
|----|--------|------|------|----|------------------|
| **PR-A** | (TBD) | T16-T17 ToolSurface v4 + 19 工具 IsConcurrencySafe 默认 (4 override + 15 default) | 2 | AC1-AC2 + **AC18** | TD-STE-06 |
| **PR-B** | (TBD) | T18-T19 partitionToolCalls + `partition_invariants_test` + 50 文件 e2e 并发版 | 2 | AC3 + AC10 + **AC15-AC17 + AC19-AC21** | TD-STE-01 |
| **PR-C** | (TBD) | T20-T21 toCompactBlock + 19 工具 ToAutoClassifierInput 默认 | 2 | AC4-AC6 | — |
| **PR-D+E** | (TBD) | T22'-T23' AutoModeClassifier P2 interface stub + T24' 单测 + T25' GrowthBook bash threshold flag | 4 | AC7-AC11 | — |
| **PR-F** | (TBD) | T26 Bash sibling abort + T27 Discard on fallback + T28 inputsEquivalent | 3 | AC12-AC14 | TD-STE-02 + TD-STE-03 |
| **合计** | | | **13 T** | **21 AC** | **4 tech-debt** |

> **排期 (1W+3D, 总工期 8 天):**
> - W1 D1-D5: PR-A → PR-B → PR-C 顺序合入 (interface + per-input 决策 + 投影)
> - W2 D1-D2: PR-D+E 合入 (classifier P2 stub + GrowthBook 1 flag)
> - W2 D3: S3-Gate design review
> - W3 D1-D2: PR-F 合入 (sibling abort + discard + inputsEquivalent 收口)
> - W3 D3: S5 验收 + S6 归档
>
> **博弈论 D4 收敛理由** (vs 原 6 PR): devrix 文化是 "实现+测试+重启脚本让用户验收" 即时反馈模式, 6 PR 拆分类器实现+测试违反此文化; PR-D+E 合并让 reviewer 看到完整闭环.

---

## 博弈论决策记录 (审计追溯)

| 决策点 | 倾向 | 关键证据 | 反方让步理由 |
|--------|------|---------|--------------|
| **D1** per-input 实现 | **分层混合** (4 override + 15 default) | clawcode `Tool.ts:402,556` interface + `BashTool.tsx:434-442` 实例 | Claude+Cursor 让步: 15 工具 default table 不写 boilerplate |
| **D2** auto-mode classifier | **P2 interface only** | devrix 无相关 incident; VerifyContract 4 元组已够用 | Cursor 让步: 无 prod incident 证据, 接口先就位 |
| **D3** GrowthBook | **P0 部分保留 1 flag** (bash 30K→50K) | Cursor 引用 `persist/growthbook_override.go:1-9` devrix 内部 ops 调优先例 | Claude 让步: bash threshold 是真实 ops 需要, 不是推测; Codex 让步: 全删过于激进 |
| **D4** PR 数量 | **5 PR (D+E 合并)** | devrix hotfix 模式 + DM-20260702-008 延期教训 | Codex 让步: 6 PR 拆分违反即时反馈文化 |

完整辩论过程: `gaming-debate-round{1,2,3}-*.md` (Claude + Codex + Cursor 全文)