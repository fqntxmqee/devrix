---
reviewer: oh-my-claudecode
review-date: 2026-06-25
round: 3
demand-id: DM-20260625-003
scope: demand.md + proposal.md + design.md + tasks.md + spec.md + codex-review.md
methodology: 5 维度（数据/逻辑/边界/调用/异常）
---

# Round 3 Review — devrix-d7-mups-v5-escape-engine (DM-20260625-003)

> **本评审独立于 codex review**：codex 评审见 `codex-review.md`（5 赞同 + 5 担心 + 3 建议）；
> 本轮聚焦"实施前 5 维度一致性"问题，识别 6 个新问题（1 P0 + 4 P1 + 1 P2）。

## 1. 总体判断

需求质量高，整体可推进到 S4 实现。本轮发现 **6 个新问题**，建议合并为 **V5.1 review fixes PR**（1.1 天工作量），前置到 V5.1 之前完成。

## 2. 6 个问题清单

### 🔴 P0: ISSUE-1 — MaxDepth 边界不一致

**位置**:
- `design.md §5.1` line 142-145
- `tasks.md L3-02` line 311
- `tasks.md L1-04` line 342

**问题**:
- `design.md §5.1` 明确：
  ```
  MaxDepth 判定规则：
  - depth < MaxDepth → EscapeContinue（继续回路）
  - depth >= MaxDepth → EscapeForceExit（强制退出）
  - 例：MaxDepth=3 时，depth=1/2 → Continue，depth=3 → ForceExit
  ```
- `tasks.md L3-02` 攻击者视角写：`同模式 3 次 → Continue, 4 次 → ForceExit`
- 两者对"第几次触发 ForceExit"定义不一致（depth=3 vs depth=4）

**影响**:
- V5.1 单元测试边界断言不明确
- V5.5 集成测试期望值不确定

**修复**:
- SoT 取 `design.md §5.1`（`depth >= MaxDepth` 语义）
- `tasks.md L3-02` 攻击者视角改为 `depth=3 → ForceExit`
- `tasks.md L1-04` 测试描述补充具体断言

**归属**: V5.1 review fixes（0.1 天）

---

### 🟡 P1: ISSUE-2 — ChainedArbitrator 实现细节缺失

**位置**: `design.md §5.3`（仅流程图）

**问题**:
- `LLMArbitrator.Arbitrate` 和 `HumanArbitrator.Arbitrate` 都给了 Go 骨架
- **`ChainedArbitrator.Arbitrate` 完整 Go 骨架缺失**——V5.3 核心调度函数
- 若 LLMExit→RuleRecoverable→Human 路径写错，可能导致 ChainedArbitrator 把 EscalateToRule 直接返回给 caller（违反 §6 processEscapeDecision 兜底逻辑）

**修复**:
- `design.md §5.3` 增加 `ChainedArbitrator.Arbitrate` 完整 Go 骨架（约 50 行）
- 明确 3 层调度顺序：LLM → Rule → Human，每层根据 Action 决定下一步

**归属**: V5.3 review fixes（0.3 天）

---

### 🟡 P1: ISSUE-3 — `applyResumeDecision` 实现缺失

**位置**: `design.md §6:736` + `design.md §5.3.2` 描述

**问题**:
- `design.md §6` 第 736 行：`ResumeSession 命中 → applyResumeDecision 恢复状态`
- `design.md §5.3.2` 描述了 user_choice=A/B/C 三种续跑语义
- **applyResumeDecision 函数未在 design 中定义**——T2 续跑机制关键函数

**修复**:
- `design.md §5.3.2` 末尾增加 `applyResumeDecision` 完整 Go 骨架（约 30 行）
- 明确 user_choice=A → 续跑回路，user_choice=B → ForceExit，user_choice=C → AbortWithAudit 三种语义
- 与 Phase 7 Auto-Close 的"同步返回+内部异步"差异（Auto-Close 不续跑）

**归属**: V5.5 review fixes（0.3 天）

---

### 🟡 P1: ISSUE-4 — `Notifier` interface 重复定义 + `SubmitOverrideCard` 归属歧义

**位置**: `design.md §5.3.1` line 353-355 和 416-423

**问题**:
- 第 353 行和第 420 行都定义了 `type Notifier interface { Notify(...) }`（重复声明）
- `SubmitOverrideCard` 不在 `Notifier` interface 中，作为 `OverrideCardNotifier` 子 interface
- `ChainedNotifier.SubmitOverrideCard` 类型断言失败时静默忽略，无 fallback 日志

**修复**:
- 删除 `design.md §5.3.1` 第 420-423 行重复定义
- 明确 OverrideCardNotifier 为可选 interface
- ChainedNotifier 类型断言失败时增加 `slog.Warn` 降级日志

**归属**: V5.3 review fixes（0.2 天）

---

### 🟡 P1: ISSUE-5 — Observe 失败后 Continue 路径模糊

**位置**: `design.md §6:643-644`

**问题**:
```go
// 接线点 0: Observe 失败
decision := o.escapeEngine.Evaluate(loopCtx)
if terminate, derr := o.processEscapeDecision(decision, err); terminate {
    return derr
}
// Observe 失败但未到上限，observe 仍可用，继续回路
```

- observe == nil 或 observe.Observations == [] 时，注释"仍可用"模糊
- 实际两种语义不同：observe==nil 进 Plan 失败路径（1a 触发），observe 空列表走默认 Plan

**修复**:
- `design.md §6` 接线点 0 注释明确化两种 case
- `tasks.md L1-81` 测试增加 "observe==nil 输入" sub-case

**归属**: V5.5 review fixes（0.1 天）

---

### 🟢 P2: ISSUE-6 — `L2-07` 4 IntentKind × 5 节点矩阵复杂度

**位置**: `tasks.md:625` L2-07

**问题**:
- 4 IntentKind × 5 节点 = 20 个 case，作为单测过于庞大
- 实际意图应该是"4 IntentKind 都能正确触发 EscapeEngine 接线点"

**修复**:
- `tasks.md L2-07` 测试描述明确化为表驱动（4 IntentKind × 关键节点）
- 明确 Skip IntentKind 应跳过多数节点，仅 1 次 Evaluate

**归属**: V5.5 review fixes（0.1 天）

---

## 3. 工作量估算

| ISSUE | 严重性 | 工作量 | 归属 PR |
|-------|--------|-------|---------|
| ISSUE-1 MaxDepth 边界 | 🔴 P0 | 0.1 天 | V5.1 review fixes |
| ISSUE-2 ChainedArbitrator 骨架 | 🟡 P1 | 0.3 天 | V5.3 review fixes |
| ISSUE-3 applyResumeDecision 骨架 | 🟡 P1 | 0.3 天 | V5.5 review fixes |
| ISSUE-4 Notifier interface 清理 | 🟡 P1 | 0.2 天 | V5.3 review fixes |
| ISSUE-5 Observe 失败 Continue 注释 | 🟡 P1 | 0.1 天 | V5.5 review fixes |
| ISSUE-6 L2-07 表驱动明确化 | 🟢 P2 | 0.1 天 | V5.5 review fixes |
| **合计** | | **1.1 天** | |

## 4. 与之前 round 对比

| 维度 | Round 1 (PR #196) | Round 2 (PR #197) | Round 3 (本轮) |
|------|-------------------|-------------------|----------------|
| 数据 | LoopContext 11→7 字段 ✓ | — | ISSUE-1 MaxDepth 边界 |
| 逻辑 | 1a 短路不调 1b ✓ | — | ISSUE-3 applyResumeDecision 缺实现 |
| 边界 | — | 6 类 EscapeAction 分层 ✓ | ISSUE-5 Observe 失败 Continue 模糊 |
| 调用 | — | — | ISSUE-2 ChainedArbitrator 骨架 / ISSUE-4 Notifier 重复 |
| 异常 | — | 13 类失败降级矩阵 ✓ | — |

**趋势**: 数据和异常维度已收敛；**调用和边界维度在 V5.3/V5.5 实施细节上有空缺**。

## 5. 5 句话总结

1. **需求质量高，可推进 S4**——6 个 ISSUE 都是工程可落地的小颗粒度修复
2. **🔴 ISSUE-1 是必须前置的**——MaxDepth 边界不统一会导致 V5.1 单元测试失败
3. **🟡 ISSUE-2/4 集中在 V5.3**——ChainedArbitrator 和 Notifier 是 V5.3 实施的关键
4. **🟡 ISSUE-3/5 + 🟢 ISSUE-6 集中在 V5.5**——5 节点接线和 T2 续跑细节补全
5. **1.1 天修复合并为 V5.1 review fixes PR**，不影响 V5.1-V5.5 主流程（6.5 天 + 2.2 天 gap tests）

## 6. References

- `demand.md`
- `proposal.md`
- `design.md`
- `tasks.md`
- `codex-review.md` (Round 1-2 review)
- `brain/.../core-concepts/38-mature-uncertainty-methodology.md §21` (v5 SoT)
- `brain/.../core-concepts/38-mature-uncertainty-methodology.md §22` (独立 codex review 完整版)