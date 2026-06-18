# 双边共识：终态任务架构博弈论分析

**Change ID:** devrix-unified-work-tree
**日期:** 2026-06-18
**参与方:** Claude (Opus 4.7) + Codex (MiniMax-M3)
**输入:**
- `gaming-analysis.md` (Claude 初始分析)
- `review-gametheory-worktree.md` (Codex 独立审查)
- `gaming-analysis-response.md` (Claude 对审查的观点)

---

## 共识摘要

| # | 议题 | 共识状态 | 行动 |
|---|------|---------|------|
| C1 | §4.2 Uncertainty ≠ Costly Signal | **完全共识** | 重写为 Cheap Talk + Uncertainty Anchor 机制 |
| C2 | 产权过渡期博弈缺失 | **完全共识** | 新增 T0→T1→T2 两阶段博弈分析 |
| C3 | 递归深度硬上限缺失 | **完全共识** | 新增 AC20-22 |
| C4 | §2 Coase 引用方向 | **共识（术语修正）** | 改为 Demsetz/Williamson，保留 Coasean umbrella |
| C5 | §3.1 Williamson 缺不确定性维度 | **完全共识** | 表格补充不确定性列 |
| C6 | §6 Stackelberg 承诺可信性 | **共识（术语修正）** | 改为 Hierarchical Game with Incomplete Information |
| C7 | §7.2 防御机制升级 | **共识（方案折中）** | CI 自动化为主 + CR 为辅 |
| C8 | Uncertainty Anchor 具体设计 | **共识（方向一致，方案折中）** | Claude 的 structured provenance + Codex 的 historical anchor 合并 |
| C9 | AC26: empty RunRef block | **保留分歧，Claude 方案先行** | v1.1 rate-limited warn；v1.2 hard dependency |
| C10 | 跨 Session WorkItem 访问 | **共识（优先级）** | 标记 Designed, deferred to v2.1 |

---

## C1: Uncertainty ≠ Costly Signal — 全文最大修正

**共识：** LLM 设置 `uncertainty` 对 LLM 无私人成本 → 不是 Spence costly signal → 是 Crawford & Sobel cheap talk。Separating equilibrium 论证需重建。

**修正方案（合并双方建议）：**

```
Uncertainty(wi) = α × historicalFailure(wi.Kind) + β × structuralComplexity(wi) + γ × LLM_claim

其中：
  α + β + γ = 1
  α 权重随样本量动态调整（冷启动 γ 高，充分锚定后 γ 低）
  structuralComplexity = f(BlockedBy 深度, FileScope 扩散度, 相似任务 terminal 率)

LLM_claim 必须附带 structured provenance：
  { "source": "tool_output" | "dependency_unknown" | "code_smell",
    "tool_call_id": "call_xxx",
    "snippet": "..." }
  → evidence 为空时 LLM_claim 权重强制为 0
```

**新增 AC27 (P0, v2.0):** Uncertainty Anchor 机制通过集成测试验证——LLM 空 evidence 时 uncertainty 回退到 historical + structural。

---

## C2: 产权过渡期博弈

**共识：** 原始分析假定了"D2/D4 会自愿退化为 Follower"，没有分析过渡期的对抗策略。

**修正方案：**

```markdown
### 新增 §X: 产权过渡期的合规博弈

T0 (现状)：D2 拥有 sc.Todos + BackgroundRegistry；D4 拥有 wave.TaskNode
T1 (过渡)：D7 WorkTree 集中产权，D2/D4 必须通过 WorkTree API
T2 (终态)：D2/D4 是纯 Follower

T1 阶段关键博弈：
  参与者：D2 开发者、D4 开发者、D7 架构师
  策略空间：合规 vs 阳奉 (绕过 WorkTree 直写本地状态)
  检测概率 p(CI) ≈ 0.7, p(CI+CR) ≈ 0.85
  均衡条件：p × F > R_defiant - R_compliant

当前 p 不足以保证合规均衡 → 需要：
  1. CI static analysis 提高 p（AC23）
  2. 自动 revert 提高 F
  3. sc.Todos 标 ReadProjection 降低 R_defiant
```

**新增 AC23 (P0, v1.0):** CI static analysis 检测 D2 直写 sc.Todos（非经 WorkTree）。

---

## C3: 递归深度硬上限

**共识：** 原分析没有限制 decompose 深度，LLM 可通过"无限分解"逃避责任（cheap talk 的递归放大）。

**修正方案：**

| AC | 内容 | Phase |
|----|------|-------|
| AC20 | 单 WorkItem 递归 decompose 深度 ≤ 3（可配置 `work_tree.max_decompose_depth`） | v2.0 |
| AC21 | 深度超限 fallback inline execute（保留 LLM 对 leaf task 的直接责任） | v2.0 |
| AC22 | 同 Session 24h 内同 Kind decompose 次数 > 5 → 触发 `task_await` 人工 review | v2.0 |

**补充（Claude 建议）：** 单层 decompose 子任务数 ≤ 7（`max_children_per_decompose`），防止宽度爆炸。

---

## C4-C6: 术语精度修正

| 原术语 | 修正为 | 理由 |
|--------|--------|------|
| Coase 问题 (§2) | Demsetz 产权理论 + Williamson TCE | Coase 定理是"初始产权不重要"，本文是"产权需要重新分配" |
| Costly Signal (§4.2) | Cheap Talk + Bayesian Persuasion + Anchor | C1 已详细说明 |
| Stackelberg 均衡 (§6) | Hierarchical Game with Incomplete Information (Harsanyi) | D7/D4/D2 是层级信息不对称，不是一次性 leader-follower |

这些不影响论证核心方向，仅提升学术精度。

---

## C7: 防御机制升级

**共识：** 纯代码审查不可靠。CI 自动化 + CR 互补。

**修正方案：**

| AC | 内容 | Phase |
|----|------|-------|
| AC23 | CI static analysis 检测新增 `*Registry / *Manager` 类 + sc.Todos 直写 | v1.0 |
| AC24 | Code Owner Bot 自动 @ D7 架构师（新增 task-related 实体时） | v1.1 |
| AC25 | 季度 Property Rights Audit — 扫描游离 WorkTree 外的 task 实体 | v1.1+ |

CR 规则保留为 supplemental defense，不作为 primary。

---

## C8: Uncertainty Anchor 具体设计

**合并方案：**

```go
// 双方建议的合并
type UncertaintyEvidence struct {
    Source      string // tool_output | dependency_unknown | code_smell
    ToolCallID  string // 指向具体 tool call
    Snippet     string // 引用输出片段
}

func ComputeUncertainty(wi *WorkItem, llmClaim float64, evidence *UncertaintyEvidence) float64 {
    histFail := reputation.FailureRate(wi.Kind)
    structComp := structuralComplexity(wi)
    
    // 权重动态调整
    sampleSize := reputation.SampleSize(wi.Kind)
    llmWeight := lerp(0.5, 0.15, min(sampleSize/100.0, 1.0)) // 冷启动 0.5 → 充分 0.15
    
    // evidence 为空 → LLM claim 权重归零
    if evidence == nil || evidence.ToolCallID == "" {
        llmWeight = 0
    }
    
    return llmWeight*llmClaim + (1-llmWeight)*(0.6*histFail + 0.4*structComp)
}
```

---

## C9: AC26 分歧 — empty RunRef 处理

**Codex 立场：** v1.1 empty RunRef → block spawn（防止 signal dilution）。

**Claude 立场：** v1.1 empty RunRef → rate-limited warn + dashboard 计数器；v1.2 hard dependency。

**决议：** Claude 方案先行。理由：
1. Phase 0-2 的独立价值不依赖 RunRegistry
2. empty RunRef 不代表零观测——Legacy BackgroundRegistry 提供基本观测
3. 如果 DM-011 确实拖延，dashboard 计数器会触发升级

**监控：** `worktree_spawn_without_runref_total` 计数器，告警阈值 > 10/hour。

---

## C10: 跨 Session WorkItem 访问

**共识：** v1.x Out of Scope 是正确的阶段决策。v2.1 再设计 lock/propose-modify/arbitration 协议。

**v2.1 待设计：**
- 历史 Session WorkItem 只读查询（`QueryWorkPlan(historical_session_id)`）
- Mutable 引用需创建新 Session + arbitration 协议
- DM-011 RunRegistry terminal 状态 = lock 信号

---

## 行动清单

### 立即 (本轮)

- [ ] `gaming-analysis.md` v2 — 按 C1-C8 修正
- [ ] `tasks.md` — 新增 AC20-22, AC23-25, AC27
- [ ] `design.md` §10 风险 — 新增产权过渡期风险

### S3-Gate

- [ ] 双模型确认修正版通过
- [ ] `gaming-analysis-bilateral-consensus.md` 标为 FINAL

### 实施阶段

- [ ] v1.0: AC23 CI static analysis 上线
- [ ] v2.0: AC20-22 递归深度上限 + AC27 Uncertainty Anchor
- [ ] v2.1: 跨 Session 协议设计

---

**本文件状态:** DRAFT — 待双方最终确认后标为 FINAL。
