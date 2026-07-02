Connection lost, reconnecting to https://api2.cursor.sh (attempt 1)...
Retry attempt 1...
Connection lost, reconnecting to https://api2.cursor.sh (attempt 2)...
Retry attempt 2...
Connection lost, reconnecting to https://api2.cursor.sh (attempt 3)...
Retry attempt 3...
# 独立博弈论分析 — DM-20260702-009

## 总判断
我的独立立场是：**治本方向应接受，但要严格区分“必须现在收口的内核问题”和“可后移的运维/缓存精修问题”**。  
5 个博弈点里，我支持 `per-input` 并发判定、支持引入 `auto-mode classifier` 但仅作为默认关闭的中间防线、支持 **4 项 tech-debt 在同一 change 内收口**，但**不支持把所有尾项都压成同等 P0 强度**。在拆 PR 上，我偏向 **5 PR** 而不是 6 PR；在 clawcode 借鉴上，我认为 **GrowthBook 值得保留，`inputsEquivalent` 不值得在本 change 强推**。

---

## 博弈点 1: `per-input` 函数 vs 字段化 metadata

**我的立场:** 选 **`per-input` 函数**，不要继续停留在字段化 metadata。

**理由**
1. devrix 当前并发判定就是静态 `ToolSpec.ConcurrencySafe`，并由 `ExecuteRound` 直接按 tool name 查表做并发/串行分流；这天然只能表达 “这个工具通常能否并发”，表达不了 “这次输入是否安全并发”。
2. 当前真值表还把 `bash` 静态标成并发安全，这说明静态 metadata 在 Bash 这种“同工具、异输入、风险差异极大”的工具上，语义已经不够精确。
3. `BuiltinSurface.CheckPermission()` 已经证明 devrix 接受“按输入解析后再决策”的模式：它会把 Bash 的 JSON 输入反序列化、抽出 `command`，再交给 `bashAST.Check()`。并发安全本质上也是同类问题，应该沿这个接口风格推进，而不是新增另一层 declarative 枚举。

**源码依据**
```295:343:internal/bootstrap/turn_adapter.go
func (a *contextEngineAdapter) ExecuteRound(ctx context.Context, req sessionorchestrator.ToolRoundRequest) (sessionorchestrator.ToolRoundResult, error) {
	// ...
	concSafe := a.concurrencyMap()
	// ...
	for i, tc := range req.ToolCalls {
		if results[i].Error != "" {
			continue
		}
		if concSafe[tc.Name] {
			parallelIdx = append(parallelIdx, i)
			continue
		}
		results[i] = a.executeOne(toolCtx, req.SessionID, tc)
	}
	// ...
}
```

```39:42:internal/shared/contracts/tool_surface.go
// ConcurrencySafe: multiple invocations of the same tool may run in parallel
// without mutual interference (e.g. read_file on different paths).
ConcurrencySafe bool
```

```44:53:internal/layers/contextengine/enforce/tools/surface/orthogonal_flags.go
func OrthogonalFlagFor(toolName string) (readOnly, destructive, openWorld, concurrencySafe bool) {
	switch toolName {
	case "read_file":
		return true, false, false, true
	// ...
	case "bash":
		return false, true, false, true
```

```129:143:internal/layers/contextengine/enforce/tools/surface/builtin_surface.go
func (s *BuiltinSurface) CheckPermission(_ context.Context, spec contracts.ToolSpec, input json.RawMessage) contracts.Decision {
	if spec.Name != "bash" {
		return contracts.DecisionAllow
	}
	// ...
	var in struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return contracts.DecisionAsk
	}
	decision, _ := s.bashAST.Check(in.Command)
	return decision
}
```

**反方意见**
- 字段化 metadata 更符合 devrix 一贯的 spec-first 设计，评审更容易，测试门禁也更集中。
- `per-input` 函数把策略逻辑塞回代码，会稀释 `orthogonal_flags.go` 那种“表驱动可审查性”。

**我的回应**
- 对于 `ReadOnly/OpenWorld/Destructive` 这类“静态能力描述”，表驱动是对的。
- 但 `ConcurrencySafe` 对 Bash、未来 MCP、甚至大文件 read 场景已经不是“静态能力描述”，而是**输入相关判定**。继续字段化，只会制造一个看起来声明式、实则不断打补丁的伪抽象。

---

## 博弈点 2: auto-mode classifier 是否必要

**我的立场:** **必要，但只能是默认关闭的第二道防线，且不能凌驾于 VerifyContract 与前置权限检查之上。**

**理由**
1. devrix 当前已经有两层相关机制，但都不覆盖“执行前、静态规则之后”的空窗：
   - `surface.CheckPermission()` / `IPermissionGate.CheckPermission()` 是**前置静态规则**。
   - `VerifyContract` 是**事后审计**，关注 deliverable/evidence/source quality，不是执行前安全判定。
2. 这意味着如果某个输入穿透了静态规则，执行已经发生；Verify 再强，也只能在事后说“这不对”，无法撤销副作用。
3. 同时，devrix 已经有 **SubQuery** 基础设施，说明“拉起独立副查询、带独立上下文预算执行一个次级判断”在架构上是现成路径，不是凭空造系统。
4. 但现阶段必须 **默认关闭**，因为现有 `decisionplanning/classifier.go` 仍是 rule-only、deterministic、hot-path 导向；把 LLM classifier 直接常开，会和当前“快路径、确定性、低延迟”原则冲突。

**源码依据**
```349:399:internal/bootstrap/turn_adapter.go
func (a *contextEngineAdapter) checkPermission(toolCtx context.Context, sessionID string, tc llmgateway.ToolCall) (sessionorchestrator.ToolResult, bool) {
	// ...
	decision := surf.CheckPermission(toolCtx, specVal, json.RawMessage(tc.Input))
	if decision == contracts.DecisionAllow {
		return sessionorchestrator.ToolResult{}, false
	}
	if decision == contracts.DecisionAsk && a.perm != nil {
		decision = a.perm.CheckPermission(toolCtx, specVal)
	}
	// ...
}
```

```54:66:internal/layers/orchestration/executionflow/verify/verify_contract.go
// VerifyContract is the 4-tuple input contract that the Verify node
// MUST validate before issuing a Verdict
//
// The 4 tuples are:
//  1. expected_class
//  2. deliverable_text
//  3. evidence
//  4. source_uncertainty
type VerifyContract struct {
```

```205:269:internal/layers/orchestration/executionflow/verify/verify_contract.go
func (c VerifyContract) Verify(input VerifyInput) (*Verdict, error) {
	// Check 1: deliverable_text
	// Check 2: evidence
	// Check 3: calibrated_confidence
	// All checks pass -> VerdictPass
}
```

```68:82:internal/layers/contextengine/enforce/subquery.go
func Run(ctx context.Context, deps SubQueryDeps, params SubQueryParams) (*SubQueryResult, error) {
	// ...
	tools := params.Tools
	if params.ReadOnlyTools {
		tools = filterReadOnlyTools(tools)
	}
```

```18:24:internal/layers/orchestration/decisionplanning/classifier.go
// The classifier must:
//   - Be safe for the FastPath hot path: no allocations beyond the result;
//     sub-millisecond on the rule set.
//   - Be deterministic: same input → same output
//   - Honor CommandFirst
```

**反方意见**
- VerifyContract 已经是 ground truth，再加 classifier 是重复防线。
- 5s timeout + 默认关闭，容易变成“实现了但不生效”的死代码。
- LLM classifier 可能幻觉，反而引入新的 false allow / false deny。

**我的回应**
- 它不是替代 VerifyContract，而是补 `CheckPermission -> Execute -> Verify` 之间的空洞。
- 但我同意：**如果不把触发条件、失败策略、可观测性写死，这会滑向死代码**。因此只能接受“默认关闭 + 5s 硬超时 + fail-safe + telemetry + VerifyContract 不降权”的版本。

---

## 博弈点 3: tech-debt 收口策略

**我的立场:** **4 项一起收，但必须分“主线治本”与“尾债封口”两层推进，而不是平均用力。**

**理由**
1. `TD-STE-01` 和 `TD-STE-06` 与本 change 的两大根因直接同源：一个是分批并发，一个是并发安全判定粒度，应该与主线一起收。
2. `TD-STE-02` 和 `TD-STE-03` 不是根因，但它们都属于同一执行器语义闭环：并发 sibling 取消、fallback discard。如果主线改了批次执行，却把这两项继续挂着，新的执行语义仍然不完整。
3. tech-debt 文档本身已经把 4 项都明确标成由本 change 关闭，说明 OpenSpec 叙事上已经合流；这时再拆成多个 change，收益不大，反而增加 SoT 漂移。

**源码依据**
```22:34:openspec/tech-debt/streaming-tool-executor-v2.md
Devrix v1 `StreamingToolExecutor` 仅在 **整批工具全部 concurrency-safe** 时才并行。
clawcode 支持 **混合批次**（只读工具并行 + 写工具独占）、并行 Bash 兄弟取消、fallback discard、执行中 progress 流式输出。

| 能力 | Devrix v1 | clawcode | 目标 / 状态 |
|------|-----------|----------|------|
| 混合批次并发 | 全 safe 才并行 | safe 可与 safe 并行；unsafe 独占 | **TD-STE-01 → CLOSED T18** |
| Bash 并行 sibling abort | 无 | `siblingAbortController` | **TD-STE-02 → CLOSED T26** |
| fallback 时 discard 在途工具 | 无 | `discard()` + synthetic error | **TD-STE-03 → CLOSED T27** |
| per-tool `isConcurrencySafe` | 硬编码 switch | 工具定义回调 | **TD-STE-06 → CLOSED T16-T17** |
```

```456:472:internal/bootstrap/turn_adapter.go
func (a *contextEngineAdapter) concurrencyMap() map[string]bool {
	m := make(map[string]bool, 32)
	for _, s := range a.surfaces {
		// ...
		for _, sp := range s.Tools(context.Background(), "", "") {
			m[sp.Name] = sp.ConcurrencySafe
		}
	}
	return m
}
```

**反方意见**
- 一次收 4 项会把 change 变厚，验证矩阵放大。
- `TD-STE-02/03` 更像 executor 语义细节，完全可以后续独立做。

**我的回应**
- 如果是实现层，我会主张分批；但从博弈论看，这个 change 的叙事核心就是“把 streaming tool executor 从 v1 粗粒度修到可长期站住的 v2”。  
- 所以 **同 change 收口是对的，分 PR 落地即可**。

---

## 博弈点 4: PR 拆分粒度

**我的立场:** **5 PR 优于 6 PR**，应合并 `PR-D/PR-E`。

**理由**
1. `PR-D`（classifier + toCompactBlock + 集成）与 `PR-E`（classifier tests + telemetry + e2e）技术上是同一能力的两个半边，拆开会出现一个“系统已接线但验证还没合上”的中间态。
2. devrix 当前已有明确的 no-silent-default / gate 型测试文化；对于 classifier 这种高风险变更，**实现与回归应同 PR 原子出现**，否则 reviewer 很难给出稳定结论。
3. 相反，`PR-A/B/C/F` 的边界是清楚的：接口/并发调度/投影/classifier闭环以外的尾项。

**源码依据**
```3:19:internal/shared/contracts/tool_surface_v3.go
// v2 ToolSpec has 9 fields; v3 adds 6 control plane fields at the END
// of the struct to guarantee position-struct-literal backward compat.
// ...
// T14 gate forbids this fallback for any registered tool
```

```225:229:internal/layers/contextengine/enforce/tools/surface/orthogonal_flags_test.go
// T: D2-S15-A02-T08 — DefaultV3MetadataFor returns the 6 control plane
// fields per the 19-tool truth table. Each registered tool MUST have an
// explicit entry; the gate (T14) fails the build if any registered surface
// returns a spec whose v3 fields are the zero defaults.
```

**反方意见**
- 6 PR review 面更窄，风险更可控。
- `PR-E` 独立能让 telemetry/e2e 单独评审，不被 classifier 实现噪音淹没。

**我的回应**
- 如果 `PR-E` 是横跨多模块的补测试 PR，我会同意拆；但这里它主要服务 `PR-D`，拆开后的“半成品时间窗”比 review 便利更危险。  
- 所以我赞成 **5 PR：A / B / C / D(+E) / F**。

---

## 博弈点 5: GrowthBook (P0 AC11) + `inputsEquivalent` (P2 AC14) 是否过度工程

**我的立场:** **GrowthBook 值得借鉴，但 `inputsEquivalent` 在本 change 里属于过度工程。**

**理由**
1. **GrowthBook 不算过度工程**：devrix 已经在持久化阈值上引入了可运行时覆写的 GrowthBook override，并且明确采用“硬编码 baseline + flag 渐进放量”的模式。这说明项目已经接受“高风险行为先基线、再 flag 调优”的治理方式。
2. 但 **`inputsEquivalent` 价值不足**：当前 `ContentReplacementState` 的冻结单位是 `toolUseID`，核心目标是“同一结果在后续 turn 保持 byte-identical”，并不是“判断两个不同调用输入在语义上是否等价”。也就是说，devrix 现有缓存稳定性机制并不依赖输入等价判定。
3. 把 `inputsEquivalent` 硬塞进 19 个 tool surface，会引入大量表层测试与判定歧义，但没有像 `IsConcurrencySafe(input)` 那样直连当前根因。

**源码依据**
```1:9:internal/layers/contextengine/persist/growthbook_override.go
// Per-tool persistence threshold override.
// Use case: roll out the 100K per-tool thresholds progressively by
// changing the override map for the 5% canary first, then 25%, 100%.
// The hardcoded per-tool values in orthogonal_flags.go stay as the
// "consensus" baseline; GB can shift individual tools up or down.
```

```57:89:internal/layers/contextengine/persist/growthbook_override.go
func GetPersistenceThreshold(
	toolName string,
	declaredMaxResultSizeChars int,
	override *ThresholdOverride,
) int {
	// ...
	if override != nil {
		if v, ok := override.values[toolName]; ok && v > 0 {
			return v
		}
	}
	return declaredMaxResultSizeChars
}
```

```14:23:internal/layers/contextengine/persist/content_replacement_state.go
// ContentReplacementState tracks per-conversation-thread decisions about
// whether a given tool result was persisted and what preview string
// replaced it.
// ...
// Re-application is a map lookup — no file I/O, byte-identical, cannot fail.
```

```81:118:internal/layers/contextengine/persist/content_replacement_state.go
func (s *ContentReplacementState) MarkSeen(toolUseID string) { ... }
func (s *ContentReplacementState) Lookup(toolUseID string) (string, bool) { ... }
func (s *ContentReplacementState) RecordReplacement(toolUseID, replacement string) { ... }
```

**反方意见**
- `inputsEquivalent` 能让未来缓存失效、去重、retry 判定更优雅，先占位后续会省事。
- GrowthBook 默认全关也可能沦为死代码。

**我的回应**
- GrowthBook 已有同类先例，且主要是运维治理接口，保留是合理的。
- `inputsEquivalent` 则没有同等强度的“现有架构缺口”证据。它更像 clawcode 工具框架里的通用便利字段，不是当前 devrix 这次 change 的核心矛盾。  
- 我的建议是：**AC11 保留，AC14 降级为后续 P2/P3 候选，不在本次主线强绑定。**

---

## 共识诉求（与 Claude 一致）
1. **必须从静态 `ConcurrencySafe bool` 走向 `per-input` 判定**，否则无法真正解决 Bash/混合批次问题。
2. **auto-mode classifier 不能替代 VerifyContract 4 元组**；Verify 仍是 ground truth。
3. **4 项 tech-debt 应在本 change 范围内收口**，至少在 OpenSpec 叙事上合并处理。
4. **`inputsEquivalent` 的优先级明显低于并发判定和 classifier 主链。**

---

## 争议点（与 Claude 不一致）
1. **PR 粒度**：Claude 倾向 5 PR，我也倾向 5 PR，但我的理由更强烈，不是“足够紧凑”，而是 **D/E 分拆会制造不稳定中间态**。
2. **GrowthBook 价值判断**：Claude 认为默认全关导致其偏“死代码”，建议降 P2；我不同意。  
   现有 `persist/growthbook_override.go` 已证明 devrix 接受这类“baseline + runtime override”机制，所以 **AC11 不是外来异物，而是既有治理模式的横向复用**。
3. **auto-mode classifier 的必要性强度**：Claude 是“可做但默认关”；我更强一点，认为它是**结构上必要**，因为它补的是当前执行前防线空洞，而不只是可选增强。

---

## 关键风险评估（高 → 低）

- **P0: Bash 输入误判导致错误并发**  
  静态 `bash=true` 已经偏粗；若迁移到 `IsConcurrencySafe(input)` 但 read-only 判定不稳，会把真实写命令并发化，风险最高。证据见 `internal/layers/contextengine/enforce/tools/surface/orthogonal_flags.go:52-53` 与 `internal/layers/contextengine/enforce/tools/surface/builtin_surface.go:136-143`。

- **P0: classifier 定位错误，越权替代主权限链**  
  如果 auto-mode classifier 被实现成“最终裁决”，会破坏 `surface.CheckPermission -> IPermissionGate -> Execute -> Verify` 的层次。证据见 `internal/bootstrap/turn_adapter.go:355-399` 与 `internal/layers/orchestration/executionflow/verify/verify_contract.go:54-66`。

- **P1: `per-input` 逻辑落在各 surface 后，表驱动审查性下降**  
  当前很多 metadata 通过真值表和 gate 测试保障；新函数若散落实现，review 成本上升。证据见 `internal/layers/contextengine/enforce/tools/surface/orthogonal_flags.go:10-16` 与 `internal/layers/contextengine/enforce/tools/surface/orthogonal_flags_test.go:225-229`。

- **P1: 5s classifier timeout + 默认关闭导致“长期不实战”**  
  不是架构错误，而是产品化风险。若没有明确 rollout/telemetry，容易常年处于名义存在、实际无人依赖状态。

- **P2: `inputsEquivalent` 引入大量低信号测试与判定分歧**  
  现有 `ContentReplacementState` 没有输入等价依赖；硬加后测试数会放大，但收益不确定。证据见 `internal/layers/contextengine/persist/content_replacement_state.go:14-23`、`81-118`。

---

## clawcode 35 字段借鉴关系评分

### 高分借鉴（应采纳）
- **`isConcurrencySafe(input)`** — **9/10**  
  与 devrix 当前静态并发判定缺口直接对应，是本 change 的核心治本点。
- **`toAutoClassifierInput(input)`** — **8.5/10**  
  为 classifier 提供紧凑投影，避免把完整 transcript 粗暴送入 LLM，和 devrix 的中间防线需求强匹配。
- **fail-closed defaults / explicit gate 思路** — **8/10**  
  devrix 已有 `T14` 风格的 no-silent-default gate，和 clawcode 的默认值治理理念相容。
- **runtime override / feature-flag 治理思路** — **7.5/10**  
  devrix 已在 `persist/growthbook_override.go` 落地同类模式，扩到 classifier/concurrency 是自然延伸。

### 中性借鉴（可选，需裁剪）
- **Tool-level projection / compact block 序列化** — **7/10**  
  值得做，但要限定输入范围，只服务 classifier，不要演化成“把所有 transcript 智能重编码”的大工程。
- **sibling abort / discard 语义** — **7/10**  
  对执行器闭环有价值，但属于“把系统补完整”，不是 change 的最强主线。

### 应拒绝或后移
- **`inputsEquivalent(a, b)`** — **3.5/10**  
  在 clawcode 里是通用工具框架字段；在 devrix 当前架构里，没有对应的迫切缺口，也未被现有 `ContentReplacementState` 所需要。
- **大而全的 35 字段接口哲学** — **4/10**  
  clawcode 的强项是工具框架工程化，但 devrix 明显更偏 “少字段 + 明确 SoT + gate 测试”。一次性引入过多字段会稀释当前契约清晰度。
- **将 UI/render/search 等大量 tool chrome 元数据整体迁入 devrix** — **2/10**  
  这不是本 change 的问题空间，也不符合当前 devrix 的最小必要扩展原则。

---

## 结论
如果只给一句结论：**应采纳 clawcode 的“按输入判定并发 + classifier 输入投影 + 运行时 flag 治理”这三件事，但不能照搬它的大接口哲学。**

更具体地说：

- **支持**：`per-input IsConcurrencySafe`、默认关闭的 auto-mode classifier、4 项 tech-debt 同 change 收口、GrowthBook override。
- **反对强推**：把 `inputsEquivalent` 作为本 change 的正式主目标。
- **建议裁决**：实施上走 **5 PR**，并把 `classifier + tests + telemetry + e2e` 做成一个原子 PR。
