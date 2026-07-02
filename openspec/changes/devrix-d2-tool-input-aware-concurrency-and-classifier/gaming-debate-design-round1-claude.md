# 博弈论辩论 Design Round 1 — Claude 强论证稿

**日期:** 2026-07-02
**作者:** Claude (辩论者, 不是综合者)
**任务:** 强论证 4 个 design 阶段差异点, 让 codex + cursor 在 Design Round 2 答辩
**输入:** design.md 草稿 + devrix 源码 + 复盘记录

> **范围**: S2 阶段 D1-D4 已 Round 3 收敛, 本 design Round 仅针对 design.md 浮现的 D5-D8 4 个新差异点.

---

## 0. 辩论方法

每个差异点用**辩论赛格式**:
1. 立场表 (设计倾向 + 反方倾向)
2. **最强论据** (具体证据 + 仿照对象)
3. **核心反问** (让 codex + cursor 答辩)

---

## D5: `IsConcurrencySafe(input)` 参数类型 — `[]byte` vs `json.RawMessage`

**立场表:**
- Claude (Round 1): **`[]byte`** (raw input, 跟 clawcode `unknown` 一致)
- Codex: 待答辩
- Cursor: 待答辩

**我的强论证:**

1. **接口语义边界**: per-input 方法的语义是 "**拿到原始输入, 决定 outcome**", 不是 "**解析 JSON 后做语义决策**"。
   - 多数工具 surface (Bash / read_file) 内部**已有解析层** (`parseInput` 在 partitionToolCalls 调用 IsConcurrencySafe 之前), 把解析责任放在 partition 层, surface 只做"看着输入判安全"
   - 仿 clawcode `Tool.ts:402`: `isConcurrencySafe(input?: unknown)` — TypeScript 角度是 "any, 你自己 narrow", Go 角度对应 `[]byte` (raw, 自己 parse)

2. **类型稳定**: `json.RawMessage` 强迫所有 surface 都 JSON-serializable, 但部分工具 (e.g. bash 接收自由文本, 不是 JSON-encoded) 不适合
   - bash command 可能是 `rm -rf /tmp/* | tee log`, 不是 JSON
   - read_file input 是 `{"path": "/foo.txt"}` JSON
   - **统一 `[]byte` 让 caller 决定怎么解析** (partitionToolCalls 调 `parseInput` 抽出 input 字段)

3. **ToolSurface 既有 Execute 签名** (tool_surface.go:135): `Execute(ctx, name, input, workDir string)` — **`input` 是 `string`, 不是 `json.RawMessage`**
   - 既有约定: input 字符串, surface 内部解析
   - 跟既有约定对齐, 避免类型不一致

4. **fail-safe 优势**: `[]byte` 比 `json.RawMessage` 更宽容
   - `json.RawMessage` 强校验 JSON 合法性, parse failure 直接 fail
   - `[]byte` 允许 surface 自己决定 parse 策略 (e.g. bash: `string(input)`, read_file: `json.Unmarshal`)

**反方 (`json.RawMessage`) 反驳: "类型安全, 强迫解析"**
- 反驳的反驳: clawcode 35 字段设计里, `input` 类型从不是 `JSON.parse` 后的对象, 而是 raw (e.g. BashTool 用 `command: string` 而不是 `command: object`). **Go 类型系统不必镜像 TypeScript 强校验**.

**核心反问 codex + cursor:**
- **R2-D5-Q1**: 给出 `bash` 工具 `IsConcurrencySafe` 函数体, 如果参数是 `json.RawMessage`, 怎么处理 bash command 不是 JSON-encoded 的情况? (e.g. `rm -rf /tmp/* | tee log`)
- **R2-D5-Q2**: tool_surface.go:135 既有 `Execute(ctx, name, input, workDir string)` 用 `string` 类型, IsConcurrencySafe 用 `json.RawMessage` 是否违反既有约定?
- **R2-D5-Q3**: 如果未来某个工具 (e.g. mcp_*) 接收 protobuf / messagepack 二进制 input, `json.RawMessage` 怎么处理?

---

## D6: partition batch 边界规则 — 连续 safe 合并 vs 同 tool 合并

**立场表:**
- Claude (Round 1): **连续 safe 合并** (clawcode toolOrchestration.ts:84-118)
- Codex: 待答辩
- Cursor: 待答辩

**我的强论证:**

1. **clawcode 实战验证**: `toolOrchestration.ts:84-118` 是 clawcode 已经生产验证的 partition 规则:
   ```typescript
   // clawcode toolOrchestration.ts:84
   for (const toolCall of toolCalls) {
     const isConcurrencySafe = tools.find(t => t.name === toolCall.name)?.isConcurrencySafe(toolCall.input) ?? false
     if (isConcurrencySafe && lastBatch?.isConcurrencySafe) {
       lastBatch.calls.push(toolCall)  // 连续 safe 合并
     } else {
       batches.push(newBatch(toolCall, isConcurrencySafe))  // 否则新 batch
     }
   }
   ```
   - 实战验证 = 1 论据胜 10 论证

2. **语义边界**: "连续 safe" 是 LLM 输出顺序的天然分段, 不依赖工具种类
   - LLM 输出 `[read(A), read(B), bash(rm), read(C)]` → `[safe×2, unsafe×1, safe×1]` → 3 batches
   - "连续 safe" 保留 LLM 顺序, "同 tool 合并" 会破坏 LLM 顺序 (e.g. `[read(A), glob(X), read(B)]` 在"同 tool 合并"下合并成 `[read(A), read(B)]`, 跳过 glob 顺序)

3. **errgroup 安全**: errgroup 内并发安全 = 不共享状态, 跟工具种类无关, 只跟"是否 safe"有关
   - 连续 safe 合并: batch 内 N 个工具并发, 都 safe → 不会冲突
   - 同 tool 合并: `[read(A), read(B), read(C)]` 合并成 1 batch → batch 内 3 read 并发 → OK, 但 `[read(A), glob(X), read(B)]` 同 tool 不会合并 → 3 batch 串行 → **不必要的串行化**

4. **5s timeout (P2 stub) 友好**: 连续 safe 合并后, 1 个 batch 走 1 次 ClassifyToolUse 调用 (P2 升 P1 后), 同 tool 合并会绕过 classifier 减少 1 次调用
   - 但这是 secondary, 不是主论据

**反方 (同 tool 合并) 反驳: "语义清晰, 工具种类是 natural partition key"**
- 反驳的反驳: "语义清晰" 是过度抽象. **LLM 输出顺序才是 partition 的真实语义** (devrix 编排层需要保留 LLM 顺序在 unsafe 区段, F06/F07 既有架构).

**核心反问 codex + cursor:**
- **R2-D6-Q1**: 如果 LLM 输出 `[read(A), glob(X), read(B), grep(Y), glob(Z)]` (5 个工具), 两种规则下 partition 结果分别是? 哪个跟 LLM 意图更对齐?
- **R2-D6-Q2**: clawcode 实战用 "连续 safe 合并" 跑通, 我们用 "同 tool 合并" 偏离实战, 偏离的代价是什么?
- **R2-D6-Q3**: "同 tool 合并" 下, `[read(A), glob(X), read(B)]` 拆成 3 batch 串行, 总耗时从 ~50ms (3 并发) → ~150ms (3 串行), 这是 partition 升级还是降级?

---

## D7: AutoModeClassifier 接口命名 — `YoloResult` (clawcode) vs `ClassifierResult` (语义化)

**立场表:**
- Claude (Round 1): **`ClassifierResult`** (语义化, 跟 devrix Naming Policy 一致)
- Codex: 待答辩
- Cursor: 待答辩

**我的强论证:**

1. **devrix Naming Policy 反馈** (memory `feedback-devrix-naming-policy.md`):
   - "**D{N} 仅用于架构对齐, Go 标识符一律语义化**"
   - PR #33 落地, 反例表见 layering.md
   - `Yolo` 是 clawcode 内部命名 (跟 "you only look once" 模型同名, 但跟 classifier 语义无关), devrix 应避免直接照搬

2. **语义清晰度**:
   - `YoloResult`: 啥是 Yolo? 需要查 clawcode 才知道 = "auto-mode 决策"
   - `ClassifierResult`: 立刻明白 = "分类器结果"
   - devrix 内部 grep `YoloResult` 出现 0 次, `ClassifierResult` 出现 0 次 (无既有命名冲突)

3. **跟 clawcode 借鉴关系**: 需求 §2.2 列了"借鉴关系 10 项", 但**借鉴关系 ≠ 直接复制命名**
   - 借鉴的是 design pattern (SideQuery / 5s timeout / fail-safe), 不是命名
   - devrix 历史上借鉴 clawcode 时都做语义化改造 (e.g. ToolSurface 借鉴 clawcode Tool.ts 但命名 devrix 化)

4. **grep / 文档搜索友好**:
   - "classifier" 是 devrix spec.md / t-registry / t-registry.md 通用术语 (grep 出现 30+ 次)
   - "yolo" 在 devrix 出现 0 次 (除本 design 草稿)

**反方 (`YoloResult`) 反驳: "跟 clawcode 1:1 对齐, 减少认知负担"**
- 反驳的反驳: clawcode 1:1 对齐**只在 cross-project reference 场景**有价值. 但 devrix 是内部项目, 团队成员不需要同时读 clawcode 源码. **devrix 命名 = devrix 团队认知负担**, 跟 clawcode 一致不带来收益.

**核心反问 codex + cursor:**
- **R2-D7-Q1**: devrix Naming Policy 明确说 "Go 标识符一律语义化", `YoloResult` 是不是直接违反? 给具体反例场景
- **R2-D7-Q2**: 借鉴 clawcode 历史上, devrix 做过哪些语义化改造? (ToolSurface / BashTool 等案例)
- **R2-D7-Q3**: 如果未来某个新人加入 devrix, 看到 `YoloResult`, 他/她会先去查 clawcode 还是 devrix docs? 哪个路径更高效?

---

## D8: GrowthBook override 注入方式 — 复用 PERSIST 模式 vs 新增 CONCURRENCY

**立场表:**
- Claude (Round 1): **新增 CONCURRENCY** (语义化, 不混用)
- Codex: 待答辩
- Cursor: 待答辩

**我的强论证:**

1. **domain boundary 守恒**: persist 和 concurrency 是两个不同 domain concern
   - `PersistThresholdOverride` (T04 ContentReplacementState): 持久化阈值 (e.g. 30K maxResultSizeChars)
   - `ConcurrencyOverride` (T25' bash 30K→50K): 并发决策阈值
   - **复用 PERSIST 模式** → 把不同 concern 塞进同一 struct → 违反 SRP

2. **类型系统收益**: `ConcurrencyOverride` 独立 struct 让 compile-time 区分:
   ```go
   // 方案 A: 复用 PERSIST 模式 (反例)
   threshold := GetConcurrencyThreshold("bash", 30_000, persistThresholdOverride)
   // 错把 PERSIST override 当 CONCURRENCY 用, 类型不报错

   // 方案 B: 新增 CONCURRENCY (正例)
   threshold := GetConcurrencyThreshold("bash", 30_000, concurrencyOverride)
   // 类型不一致编译失败
   ```

3. **跟 devrix GB 演进路径一致**: Cursor 提到未来 3 个 flag (bash threshold + bash readonly canary + classifier canary), 这些都是 concurrency / classifier concern, 不是 persist concern
   - persist concern 已有 T04 (CLOSED), 不会再加 flag
   - concurrency / classifier 是新 concern 域, 独立 struct 表达 "这是新 concern 域" 的边界

4. **测试 / debug 友好**:
   - `ConcurrencyOverride{values: {"bash": 50_000}}` 一眼看出是 concurrency 调优
   - 复用 `ThresholdOverride{values: {"bash": 50_000}}` 需要 grep 看是不是 persist override 被误用

**反方 (复用 PERSIST 模式) 反驳: "DRY, 同一种 GB 模式, 不需要新 struct"**
- 反驳的反驳: DRY ≠ 同一个 struct. **DRY 是说"不重复实现", 不是说"不区分类型"**. CONCURRENCY 模式可以引用 PERSIST 的实现细节 (`OverrideGetter` / `NewThresholdOverride` 等 utility 函数), 但 struct 类型应该独立.

**核心反问 codex + cursor:**
- **R2-D8-Q1**: 复用 PERSIST struct 下, 如果未来 ops 误把 persist override map (`{"bash": 100_000}`) 喂给 concurrency 调用, 类型系统能 catch 吗? 还是默默走错分支?
- **R2-D8-Q2**: Cursor 提到未来 3 flag, 这 3 flag 是不是都是 concurrency / classifier concern? 还是混了 persist / concurrency?
- **R2-D8-Q3**: devrix 历史上, 跨 concern 共用 struct 是不是有过 bad case? (e.g. emission_class + convergence_contract 早期共用)

---

## 总结: Round 1 倾向

| # | 决策项 | 我的倾向 | 关键反问 (3 个) |
|---|--------|---------|----------------|
| D5 | IsConcurrencySafe 参数类型 | **`[]byte`** (raw input, 跟 clawcode 一致) | Q1-Q3 |
| D6 | partition batch 边界 | **连续 safe 合并** (clawcode 实战验证) | Q1-Q3 |
| D7 | AutoModeClassifier 命名 | **`ClassifierResult`** (devrix Naming Policy 语义化) | Q1-Q3 |
| D8 | GrowthBook 注入方式 | **新增 CONCURRENCY** (domain boundary 守恒) | Q1-Q3 |

**请 codex + cursor 在 Design Round 2 答辩以上反问, 我将基于答辩做最终收敛.**

---

## 附录: 三方最尖锐的"互相"反驳预判

| 互驳预判 | 论据 | 评估 |
|---------|------|------|
| Codex 驳 Claude (D5) | "`json.RawMessage` 更类型安全, 编译时校验" | **弱论据** (clawcode 用 `unknown`, 不强校验) |
| Codex 驳 Claude (D6) | "同 tool 合并语义清晰" | **弱论据** (破坏 LLM 顺序) |
| Cursor 驳 Claude (D7) | "跟 clawcode 1:1 对齐" | **弱论据** (devrix Naming Policy 明确反对) |
| Cursor 驳 Claude (D8) | "DRY 不应该新 struct" | **中论据** (Cursor 可能引用 devrix 现有 struct 共用案例) |

**预期 Design Round 2 焦点**: D5 + D8 (D6 跟 clawcode 一致度最高, D7 跟 Naming Policy 一致度最高).

---

## Round 2 答辩要求 (写给 codex + cursor)

对每个 D, 给出:
- **我的回答** (针对 3 个 Q)
- **是否让步** (是 / 否 + 条件)
- **如果让步, 倾向哪个立场** (A 还是 B)

最后一段: **让步矩阵** (4 个 D 各自最终立场).

重点:
- **D5**: bash command 不是 JSON-encoded, `json.RawMessage` 怎么处理?
- **D6**: clawcode 实战 partition 规则, 我们偏离的代价?
- **D7**: devrix Naming Policy vs clawcode 1:1 对齐, 哪个优先?
- **D8**: 类型系统 vs DRY, 哪个更 devrix 文化?