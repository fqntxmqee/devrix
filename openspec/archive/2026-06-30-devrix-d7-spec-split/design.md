# Design: D7 Spec Split (Cancelled at S1)

**Change ID:** `devrix-d7-spec-split`
**Demand ID:** DM-20260630-002
**Status:** **S1_Cancelled** (2026-06-30, 替代方案: devrix-spec-lite-mode DM-20260630-003)

---

## 0. 为什么没有 design.md

本 change 在 **S1 阶段取消** — 未进入 S2 (proposal) / S3 (design) / S4 (实现)。

**取消决策点**: 2026-06-30 推进 d7 spec.md 实际拆分时, 发现方向偏离用户原始意图。

详见 `proposal.md` §2 取消原因。

## 1. 取消时考虑的 2 个相反方向

| 方向 | 思路 | 结果 |
|------|------|------|
| A: 按 S 分片 (本 change 原方案) | d7 spec.md 拆 17 个 ≤ 800 行分片 | 偏离用户意图, 扩容方向 |
| B: 压成精华 (替代方案) | d7 spec.md 整体压 ≤ 200 行精简契约 + CHANGELOG.md | ✅ devrix-spec-lite-mode 实施 |

**结论**: B 方向符合"specs/ = 精简设计契约（最新符合代码）"原则。

## 2. 替代方案实施摘要

`devrix-spec-lite-mode` (DM-20260630-003, S7_Archived 2026-06-30):

- d7 spec.md 2622 → 195 行 (-92.6%)
- 新增 CHANGELOG.md 103 行
- 12/12 AC PASS
- 推广: d1/d2/d3/d4/d5/d6 (2026-06-30 当日 6 站, PR #333-#347)

详见 `openspec/archive/2026-06-30-devrix-spec-lite-mode/acceptance-report.md`。
