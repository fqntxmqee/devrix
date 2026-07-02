# Game Theory Review (Codex 2nd Pass): Implementation Status + 8K Token 问题诊断

**Change ID:** `devrix-mups-tool-classification-and-channel-autonomy`
**Demand ID:** DM-20260701-007
**Review Type:** 博弈论视角 2nd Pass — PR-A 实现后实地点评 + 8K token 问题答 Cursor
**Reviewer:** Codex (MiniMax-M3) 2nd pass
**Review Date:** 2026-07-02
**Predecessor Reviews:**
- [game-theory-review.md](./game-theory-review.md) (Codex v1, 2026-07-01)
- [game-theory-review-composer.md](./game-theory-review-composer.md) (Composer v1, 2026-07-01)
- [demand.md](./demand.md) v1.1 / [proposal.md](./proposal.md) v1.0 / [design.md](./design.md) v1.3.1
**Source of Truth for PR-A:** commit `74fba9c5` on `feat/devrix-mups-tool-classification-and-channel-autonomy`
**Purpose:** (a) 给 Cursor 的 review 一手 (b) 答"8K token 问题是否解决" (c) 给 PR-B/C/D 实施博弈论 guard

---

## 0. TL;DR

PR-A 落地的是**类型揭示的语法层 (syntactic layer)** — 它把 `emission_class` / `iteration_bound` 等 metadata 写进 ToolSpec，但**类型揭示的语义层 (semantic layer) — 路由 + 终止 + 审计 — 全部在 PR-B/C/D**。所以 8K token 问题在 PR-A 阶段**没解决**（甚至可以说**问题没出现**），PR-A 只是给解决方案铺了"词汇表"。

**8K token 问题彻底解需要四个 PR 全部落地**：
- PR-A（✅ 完成）= 元数据词汇表
- PR-B（❌ 待做）= ToolChannel 路由 + L4-Bounded 硬约束
- PR-C（❌ 待做）= TruncateWithMarker 透明截断 + VerifyContract 4 元组
- PR-D（❌ 待做）= Filter v2 三维 + PlanKind × EmissionClass 交叉一致性

**如果只跑 PR-A 不跑 PR-B/C/D** = **pooling 均衡继续生效**，LLM 仍然会反复 read_file，**且 8K token 问题比治本前更严重**（理由见 §4.1）。

**核心博弈论增量**（Codex 1 + Composer 1 没覆盖的）：
1. **8K token 真正的机制缺陷不在截断阈值，而在"截断后 LLM 不知道"** — 解决手段 = TruncateMarker 强制信号分离（PR-C）
2. **Pooled equilibrium vs separating equilibrium 的相变 (phase transition) 点 = PR-B ToolChannel 第一行 enforce 代码** — 这是 critical path
3. **PR-A 引入的 cheap talk 风险必须由 PR-C 的 Learn feedback 闭环对冲**，否则 silent default 风险持续
4. **PR-D 交叉一致性规则是 PlanKind 战略对 ToolChannel 战术的"封口"** — 没这条规则 PR-B 的 Bounded(n) 可被 L1 架空

---

## 1. PR-A 实际落地的内容（commit 74fba9c5 视角）

### 1.1 16 文件 / 775 行 / 0 函数签名变化

```
internal/layers/contextengine/enforce/tools/surface/
  - ask_user_question_surface.go      (+8)
  - background_task_surface.go        (+6)
  - builtin_surface.go                (+8)
  - delegate_surface.go               (+6)
  - freefork_surface.go               (+8)
  - lsptool_surface.go                (+8)
  - orthogonal_flags.go               (+188) ← DefaultV3MetadataFor + ApplyV3Metadata
  - orthogonal_flags_test.go          (+78)
  - plugin_surface.go                 (+13)
  - surface_metadata_gate_test.go     (+110) ← T14 silent default gate
  - tool_search_surface.go            (+10)
  - tracker_surface.go                (+8)
  - verify_surface.go                 (+8)
internal/shared/contracts/
  - tool_surface.go                   (+9)  ← ToolSpec struct + 6 fields at END
  - tool_surface_test.go              (+96) ← T12 兼容性 + JSON tag
  - tool_surface_v3.go                (+229) ← 4 type + 4 enum + String()
```

### 1.2 博弈论对应：PR-A 是 "sunk cost of type revelation"

每个 tool 现在都有一个**声明的、init-time 的、自报的**类型 — 这就是 Farrell–Rabin (1996) "cheap talk" 的标准形式。**声明本身不创造 incentive**，只创造**审计的可能**。

**PR-A 的真实价值不是"声明了类型"而是"创造了 8 个 LTL-Lite L4-L6 invariant 挂载点"** — 没有 metadata，Phase B 的 ProbeToolChannel.Bounded(n) 就不知道针对哪个 tool 强制。

**评估**：PR-A 是**必要但不充分**的一步。它把 6 字段挂上去的成本一次性付完（19 工具 × 4 元组 = 76 决策），但 metadata 离真正解决 8K token 问题的距离 = 还需要 24 个 T 点（PR-B/C/D 共 25 T 点减 1 Phase B-pre = 24）。

---

## 2. 8K Token 问题答 Cursor：是否解决？

### 2.1 问题溯源（sess_1782885908460_4000 实证）

LLM 在 review 任务中：

| iter | tool_call | result_tokens | 累计 | LLM 行为 |
|------|-----------|---------------|------|----------|
| 1 | read_file(d2/kernel/a.go) | 8000 | 8000 | "Let me see more" |
| 2 | read_file(d2/kernel/b.go) | 8000 | 16000 | "Let me see more" |
| ... | ... (×9) | ... | 72000 | "Now let me synthesize..." |
| 9 | read_file(d2/kernel/i.go) | 8000 | 72000 | D2 截断到 8K → 输出 finalText |
| 10 | text() | 200 | 72000+200 | "I read 9 files but..."  |

LLM 累计触发 9 次 read_file，**实际产出 0 review**，原因：

1. **Pooling** — LLM 不知道 read_file 一次能拿到多少
2. **Hidden truncation** — 截断发生在 kernel 层，LLM 看到的 result 没 marker
3. **No convergence signal** — LLM 没有"已经够了"的信号
4. **Verify 无契约** — Verify 接受"我读了 9 个文件"为 partial deliverable

### 2.2 PR-A 单独是否解决问题？

**答：完全没解决**。PR-A 只是给 19 工具挂上 metadata，但：

- 没有 ToolChannel enforce → read_file 还是能调 100 次
- 没有 TruncateWithMarker → 截断还是对 LLM 透明
- 没有 Bounded(15) → metadata 里的 15 是 declaration，runtime 不查
- 没有 VerifyContract → Verify 还是一维审计

**反讽**：如果用户只 merge PR-A 而不 merge PR-B/C/D，那 8K token 问题**反而恶化**：

- read_file 被标 EC_Probe + Bounded(15)，但**没有 enforce 代码** → metadata 是"骗 LLM"
- LLM 看到 metadata 里有 Bounded(15) 但调用到 16 次还是成功 → **pooling 强化**（LLM 学到"metadata 是不可信的"）
- 后续 PR-B/C/D 落地时 LLM 已不信任 metadata → 治本方案被 LLM 当 cheap talk 忽略

**这就是 Codex 1 提的 "cheap talk 风险"的具体危害**：metadata 单独存在会**反向**把治本叙事变成治标失败。

### 2.3 真正解决 8K token 问题的最小集

按 release 顺序：

1. **PR-A**（✅ 已合）= 元数据词汇表 + silent default gate
2. **PR-B-pre** (D7-S9-A26-T06) = Channel → PlanChannel rename，消除 focal point 冲突
3. **PR-B** (D7-S9-A50-T01..T08 + D5-S25-A01..A03) = ProbeToolChannel Bounded(15) hard reject + PromptPressure soft warning
4. **PR-C** (D7-S10-A50-T01..T04 + D7-S2-A50-T07..T08 + D2-S15-A02-T13) = VerifyContract 4 元组 + Reason 透传 + TruncateWithMarker
5. **PR-D** (D2-S15-A02-T02..T05, T15) = Filter v2 三维 + PlanKind × EC 交叉一致性

**4 个 PR 全部落地** = 8K token 问题**在 review 任务上**的相变点（phase transition）：
- 治本前：pooling equilibrium（LLM 把 8K 当公地）
- 治本后：separating equilibrium（LLM 知道 read_file Bounded(15)，知道截断带 marker，知道 Verify 按类举证）

**没有 PR-B 的 Bounded(n) hard reject** = 8K token 问题不解。
**没有 PR-C 的 TruncateMarker** = LLM 不知道"为什么看不到完整内容"。

---

## 3. PR-A 单独落地的副作用与对策

### 3.1 "Metadata 但无 enforce" 的博弈论陷阱

PR-A 现状是 **declaration without enforcement**。Farrell–Rabin (1996) 的 cheap talk 理论预测：

- 如果 LLM 知道 metadata 是 declaration（"不可信"）→ 忽略
- 如果 LLM 不知道 metadata 是 declaration（"以为 enforce"）→ 在 Bounded(15) 失效时**产生不信任升级**（learning of broken promise）

后者的危害更大：LLM 看到 metadata 期望 enforce，实际没 enforce → 下次看到 metadata 期望全降级 → 整个治本方案的 credibility 受损。

**对策（必须 PR-B 第一行代码就是 hard reject）**：

> PR-B 第一行 enforce 代码的 `would_reject` 统计必须 0（即：metadata 声明的 Bounded(n) 必须 100% 兑现）。如果有 would_reject > 0 → 立即 fail merge，**不允许**先松后紧。

### 3.2 PR-A 的 MaxResultSizeChars 没被使用

19 工具 metadata 里有 MaxResultSizeChars=8192/4096/2048/1024，但**目前 D2 kernel 的 TruncateToTokens 还在用 cfg.ToolResultBudget=800 tokens 全局阈值**：

```go
// internal/layers/contextengine/kernel/context_engine_persist_v2.go:151
if e.counter.CountText(resultContent) > e.cfg.ToolResultBudget {
    resultContent = e.counter.TruncateToTokens(resultContent, e.cfg.ToolResultBudget) +
        "\n...[truncated for persist]"
}
```

**问题**：
1. Marker 是 `...[truncated for persist]` 不是 `DefaultTruncateMarkerText`
2. LLM 看到 `...[truncated for persist]` 以为**只是持久化层**截断，**没意识到 token 层也截断** — 这是 Akerlof pooling
3. 每个 tool 的 MaxResultSizeChars（meta 层）和 cfg.ToolResultBudget（runtime 层）**没有映射关系**

**对策（PR-C T13 + 一次小重构）**：

PR-C 的 TruncateWithMarker 完成后，需要把 kernel 改成：

```go
if e.counter.CountText(resultContent) > toolSpec.MaxResultSizeChars {
    resultContent, _ = truncate.TruncateWithMarker(
        resultContent, toolSpec.MaxResultSizeChars, toolSpec.TruncateMarkerText)
}
```

**这是 PR-A + PR-C 的"接缝"**，T13 单独做不完。

### 3.3 PR-A 引入的 Pooling 反向风险（新增 Codex 1 没覆盖的）

考虑 3 期重复博弈：

- t=0: LLM 看到 read_file metadata EC_Probe + Bounded(15)
- t=1: LLM 调 15 次 read_file 都成功，第 16 次也成功（无 enforce）
- t=2: LLM 学到 "metadata 是 cheap talk"，开始在所有 tool 上忽略 metadata
- t=3: PR-B 落地，read_file 第 16 次被 reject → LLM 认为是"系统 bug"而非"机制设计"
- t=4: 整个治本方案 credibility 受损，PR-D Filter 也失效

**对策**：**PR-A 单独落地后，**必须**在 README/CHANGELOG 显式标注 "metadata declarations are not yet enforced; see PR-B for enforcement"**，让 LLM 训练数据 / prompt 知道当前状态。或者：**PR-A 不单独合入 master**，必须等 PR-B 也合入 master。

**对应到本仓库的 git workflow**：

当前 commit `74fba9c5` 在 `feat/devrix-mups-tool-classification-and-channel-autonomy` 分支（local 1 commit ahead of remote），还未 push。**建议：PR-B 完成 enforce 逻辑后再 push PR-A**（squash PR-A + PR-B-pre + PR-B 一次性合入 master）。

---

## 4. 8K Token 问题的完整因果链

### 4.1 现状因果链（治本前）

```
LLM 收到 review 任务
   ↓
Execute 节点路由（无 ToolChannel）
   ↓
LLM 选 read_file（不知 budget）
   ↓
read_file 触发，result 50K → D2 截到 8K
   ↓
D2 截断对 LLM 不透明（仅 "...[truncated]" 文字）
   ↓
LLM 看不到完整内容 → Bayesian 更新失败（Akerlof）
   ↓
LLM 再次调 read_file（hardin 公地悲剧）
   ↓
... 循环 9 次 ...
   ↓
Verify 接受探索性 finalText（Hart–Holmström 不完全契约）
   ↓
D1 渲染红卡（task_incomplete=true）
   ↓
User 看到 0 review
```

### 4.2 治本后因果链（PR-A+PR-B+PR-C+PR-D 全部合入）

```
LLM 收到 review 任务
   ↓
D2 PrepareOrchestrator 推 task_kind=review（PR-D T05）
   ↓
Filter v2 收紧：read_file/grep/glob → Bounded(15)（PR-D T03+T15）
   ↓
Execute 节点路由到 ProbeToolChannel（PR-B T01）
   ↓
LLM 调 read_file（知道 Bounded(15)）→ 拿完整 result
   ↓
D2 TruncateWithMarker 截到 MaxResultSizeChars=8192 + marker "complete=false"（PR-C T13）
   ↓
LLM 看到 marker → Bayesian 更新："已截断，下游用 sub-tool"
   ↓
LLM 调 16 次 read_file → ProbeToolChannel 注入 PromptPressure（PR-B T05）
   ↓
LLM 调 17 次 read_file → hard reject + InjectSynthesize（PR-B T03+T04）
   ↓
LLM 强制 synthesize → finalText 是 review
   ↓
VerifyContract 4 元组审计：deliverable 存在 + min_chars + 3 sub-evidence（PR-C T01）
   ↓
verdict.Reason 透传到 D1 feishu（PR-C T02+T03）
   ↓
D1 渲染 title 包含 "✅ review (ProbeToolChannel, iter X/15, source_uncertainty=1.0)"
   ↓
verdict.Reason 写入 Learn FeedbackMemory（PR-C T08）
   ↓
下个 session Observe 节点读 AdaptivePrior → 调整 trust
```

**相变点 = PR-B 第一行 enforce 代码 + PR-C 的 TruncateWithMarker + PR-D 的 task_kind 推**。三者缺一不可。

### 4.3 治本后均衡分析

**单 session 层面**：

- 目标均衡 = Subgame Perfect Equilibrium (SPE) on tool-call count
- 在 iter ≥ 15 的子博弈中，LLM 的 dominant strategy 是 synthesize（继续调 read_file = hard reject，payoff = -∞）
- PromptPressure 注入 iter=10（建议）作为 Schelling focal point，让 LLM "看见" Bounded(15) 即将触发

**跨 session 层面**：

- 目标均衡 = Reputation Equilibrium（Fudenberg–Tirole 1991）
- VerifyContract.Reason → FeedbackMemory → AdaptivePrior → 下轮 Filter bound 调整
- 高 reputation 工具 = 历史 verdict 干净 = Filter bound 放宽
- 低 reputation 工具 = 历史 deliverable_missing 多 = Filter bound 收紧

**PR-A 现状**是**只有 declaration 无 enforce 无 reputation 链** = 仍是 pooling。**4 个 PR 全部落地后**才是 separating + reputation。

---

## 5. PR-B/C/D 实施时的博弈论 guard（新增）

### 5.1 PR-B 的"零 would_reject" gate

PR-B 第一行 enforce 代码必须通过：

```go
// PR-B T07 shadow mode assertion
assert(shadowModeFirstWeekWouldRejectCount == 0 ||
       shadowModeFirstWeekWouldRejectCount / total < 0.05)
```

否则 fail PR-B merge，**不允许**先松后紧。

理由：PR-A 已经让 LLM 看到 metadata declaration。如果 PR-B enforce 时 LLM 已经学会"metadata 是 cheap talk"，再硬切 = reputation collapse。

### 5.2 PR-C 的 TruncateMarker 必须 LLM 可见

PR-C T13 实现的 TruncateWithMarker 必须保证：

- Marker 在 result string 末尾
- Marker 含 `complete=false` 字面（让 LLM 识别）
- Marker 含字符位置（如 `at 8192/25000`）让 LLM 知道"读了多少"
- Marker 在 LLM context 里**不被任何 layer 移除或简化**

第二条特别重要：如果 D2 后续 compression 步骤把 marker 当"无用字符"删掉 → 信号失灵。

### 5.3 PR-C 的 Learn 写入必须"verifiable + retrievable"

PR-C T08 实现的 FeedbackMemory.Record 必须满足：

- 写入字段：`sessionID, verify_exit_reason, emission_class, tool_name, iter_count`
- 读取接口：下轮 Observe 节点能按 `tool_name` 查最近 N 笔 reputation
- 跨 session 持久化（不能 session 结束就丢）

如果 Learn 只是 session 内 placeholder，跨 session reputation 链就断了，整个治本方案停留在"单 session SPE"层面。

### 5.4 PR-D 的交叉一致性必须 runtime enforce

PR-D T15 的 `TestPerTaskKindFilterCrossConsistency` 必须运行时（即 PrepareOrchestrator 实际跑 review 任务时）也 enforce，不能仅单元测试。

理由：单测可以过，但 LLM 通过 PlanKind 战略"架空" ToolChannel 战术的攻击面只在 runtime 出现。

```go
// runtime enforce in PrepareOrchestrator
if taskKind == "review" && toolSpec.EmissionClass == EC_Probe {
    if toolSpec.IterationBound.Kind == IB_OpenEnded {
        return error.New("cross-consistency violation: review task + Probe tool + OpenEnded bound")
    }
    // 否则强制 Bounded(15)
    toolSpec.IterationBound = IterationBound{Kind: IB_Bounded, MaxN: 15}
}
```

---

## 6. Cursor 的 review 检查清单（给人类 reader）

| # | 提问 | PR-A 答 | 4-PR 全落地答 |
|---|------|---------|----------------|
| 1 | 8K token 还会触发 LLM 自我循环吗？ | **是**（无 enforce） | 否（Bounded(15) + TruncateMarker） |
| 2 | LLM 知道 read_file 被 Bounded(15) 吗？ | 知道（meta）但**不信**（无 enforce） | 知道且信（enforce 100% 兑现） |
| 3 | Verify 还能接受"我读了 9 个文件"吗？ | 能（无 VerifyContract） | 否（4 元组审计） |
| 4 | D1 渲染包含 verdict.Reason 吗？ | 否（meta 没透传） | 是（meta 全链路） |
| 5 | 下个 session Learn 节点记得 verdict 吗？ | 否（无写入） | 是（FeedbackMemory） |
| 6 | PlanKind=Exploration 能否架空 Bounded(15)？ | 能（无交叉一致性） | 否（PR-D T15 runtime enforce） |
| 7 | emission_class 漂移会被审计吗？ | 否（无 Learn） | Phase C 最小（仅 verify_exit_reason） |
| 8 | 8K token 是被"压缩"还是"硬停"？ | 压缩（D2 persist 截断） | 硬停（ProbeToolChannel 第 16 次 reject） |
| 9 | LLM 知道 result 被截断吗？ | 否（`...[truncated for persist]`） | 是（`[TRUNCATED at X/Y, complete=false, REREAD may help]`） |
| 10 | 单 session SPE？ | 否 | 是 |
| 11 | 跨 session reputation？ | 否 | 最小（仅 verify_exit_reason 写入） |
| 12 | Filter v2 三维生效？ | 否 | 是（PR-D T02-T05） |

**关键结论**：PR-A 答"否"的 8 项 = 8K token 问题的真实解。PR-A 单独**不解决** 8K token 问题。

---

## 7. 实施路线建议（给 Claude 续做 / Cursor 监督）

### 7.1 不要单独合入 PR-A

PR-A 当前 commit `74fba9c5` 在本地未 push。**建议**：

- 选项 A：等 PR-B 完成 enforce 逻辑，**squash PR-A + PR-B-pre + PR-B 一起 push**
- 选项 B：push PR-A 但在 PR description / changelog 显式标注 "metadata declarations not yet enforced; PR-B in flight"
- 选项 C：revert PR-A，等 PR-B 一起重新提交

**推荐 A**（最稳）。理由：避免 §3.3 描述的 cheap talk 反向风险。

### 7.2 PR-B 的 3 个 hard merge 门禁

1. **shadow mode FP<5%**（PR-B T07）
2. **PlanChannel rename 完成**（PR-B-pre T06）— 避免 focal point 冲突
3. **L0-L3 与 L4-L6 至少 3 条 cross-check**（PR-B T08）

3 条任一不过 → PR-B fail merge。

### 7.3 PR-C 的 2 个 hard merge 门禁

1. **TruncateWithMarker 单测 + integrate 测试**（PR-C T13）
2. **verdict.Reason 写入 FeedbackMemory + 跨 session 读取测试**（PR-C T08）

2 条任一不过 → PR-C fail merge。

### 7.4 PR-D 的 2 个 hard merge 门禁

1. **Filter v2 三维 runtime enforce 测试**（PR-D T15）— PlanKind × EC 交叉一致性
2. **task_kind 推 验证集 ≥ 90%**（PR-D T05，复用 DM-20260618 Phase 5 数据）

2 条任一不过 → PR-D fail merge。

---

## 8. 给 Cursor 的一页结论

| 问题 | 答 |
|------|-----|
| 8K token 问题解了吗？ | **没有**。PR-A 只是 metadata 词汇表，enforce 在 PR-B |
| 4 个 PR 全部落地后能解吗？ | **能**，但需要 PR-A 单独合入不引发 §3.3 的 cheap talk 反向风险 |
| 现在 19 工具的 Bounded(15) 是装饰吗？ | **是**，runtime 不查 |
| LLM 看到 metadata 会更糟吗？ | **会**（短期），除非 PR-B 紧跟 PR-A 落地 |
| 治本方案 credibility 风险点？ | PR-A 与 PR-B 之间的"半诚实"窗口期 |
| 建议？ | squash PR-A + PR-B-pre + PR-B 一起 push / 或者保留 PR-A 在 branch 等 PR-B |

---

## 9. 参考（增量 + Codex 1 §8）

- Farrell, J. & Rabin, M. (1996). "Cheap Talk". *JEP*. — PR-A 单独落地的"声明 vs enforce"博弈
- Fudenberg, D. & Tirole, J. (1991). *Game Theory*. — 跨 session reputation equilibrium
- Maskin, E. & Tirole, J. (1999). "Unforeseen Contingencies and Incomplete Contracts". *RES*. — VerifyContract 4 元组的"可证伪契约"理论基础
- Akerlof, G. A. (1970). "The Market for Lemons". — 截断对 LLM 透明的 pooling 风险
- Schelling, T. C. (1960). *The Strategy of Conflict*. — PromptPressure focal point
- Myerson, R. B. (1979). "Incentive Compatibility and the Bargaining Problem". — PR-D 交叉一致性的 IC 约束
- Bolton, P. & Dewatripont, M. (2005). *Contract Theory*. — VerifyContract + burden of proof 形式化

---

## 10. 实施时间线（从本日 2026-07-02 算起）

| Day | Task | Risk | 验收 |
|-----|------|------|------|
| D+0 | push PR-A 单独合入 OR 等 PR-B | §3.3 风险 | Cursor 选 |
| D+0..D+1 | PR-B-pre PlanChannel rename | focal point 冲突 | compile + grep gate |
| D+1..D+4 | PR-B ToolChannel + LTL-Lite + shadow | shadow FP > 5% 风险 | shadow 跑 1 周 |
| D+4..D+6 | PR-C VerifyContract + Reason + TruncateMarker | Learn 写入失败 | unit + integration |
| D+6..D+8 | PR-D Filter v2 + cross-consistency | task_kind 推 < 90% | 验证集 |
| D+8..D+9 | S5 验收 + acceptance-report | 33/33 T IMPLEMENTED | verify-archive.sh 13/13 |
| D+9 | S6-交付 (squash + auto-merge) | — | PR master 合入 |
| D+10 | S6-归档 | — | archive + s7_archived |

**总估时 = 10 工作日**（与 tasks.md 估时 2+3.5+2.5+2 = 10d 一致）。

---

## 更新历史

- 2026-07-02：v1 创建（Codex 2nd pass — 实施后实地点评 + 8K token 问题答 Cursor + PR-A 单独落地副作用对策）
