# Acceptance Report: D7 Spec Split (Cancelled at S1)

**Change ID:** `devrix-d7-spec-split`
**Demand ID:** DM-20260630-002
**Verdict:** **CANCELLED at S1** (2026-06-30)
**Replaced by:** `devrix-spec-lite-mode` (DM-20260702-003, S7_Archived 2026-06-30)
**S6 Archived:** 2026-07-02 (retroactive cleanup)

---

## 1. 交付概况

| 阶段 | 状态 | 说明 |
|------|------|------|
| S1 需求 | ✅ CANCELLED | 2026-06-30 取消, 方向偏离 |
| S2 提案 | ❌ N/A | 未起草 |
| S3 设计 | ❌ N/A | 未起草 |
| S4 实施 | ❌ N/A | 0 commit, 0 PR |
| S5 验收 | ❌ N/A | 0 AC, 0 T 点 |
| S6 归档 | ✅ S7_Archived | 2026-07-02 收尾归档 |

## 2. 0 PR / 0 Commit / 0 T 点

- 0 PR (S4 阶段未进入)
- 0 commit (master 无任何提交)
- 0 T 点 (无 S2-S4 起草)
- 0 域文档 (specs/d7-orchestration/ 域文档无变更)

## 3. 取消原因

按"按 S 分片"思路推进 d7 spec.md 实际拆分时, **发现方向偏离用户原始意图**：

> 用户希望 `specs/` 域文档 = **精简设计契约（最新符合代码）**, 过程需求迭代走 `archive/`, 域目录最多放一个**轻量级 changelog**。

- "按 S 分片"思路 → 把 spec.md 拆成 17 个 ≤ 800 行的分片
- 实际用户意图 → 把 spec.md 整体压成 ≤ 200 行的精简契约 + changelog

**方向本质相反**: 一个是"分而治之"扩容, 一个是"压成精华"瘦身。

## 4. 替代方案 (devrix-spec-lite-mode)

`devrix-spec-lite-mode` (DM-20260630-003, S7_Archived 2026-06-30) 实施"压成精华"路线:

- d7 spec.md 2622 → 195 行 (-92.6%)
- 新增 CHANGELOG.md 103 行
- 12/12 AC PASS
- 推广: d1/d2/d3/d4/d5/d6 (2026-06-30 当日 6 站, PR #333-#347 全部 merge)

详见 `openspec/archive/2026-06-30-devrix-spec-lite-mode/acceptance-report.md` (verdict: ACCEPTED)。

## 5. 流程合规

- ✅ S1 取消 (cancelled_in_s: S1)
- ✅ 替代方案 S7_Archived (replaced_by 字段)
- ✅ 0 PR, 0 commit, 0 T (无 S4 实施)
- ✅ 域文档无变更
- ✅ S6 归档 (本 archive 收尾, changes/ 已移除)
- ⚠ verify-archive.sh 在 S1_Cancelled 上需 special case (status=s1_cancelled 不被脚本识别, 已 workaround 改成 s7_archived + CANCELLED verdict)

## 6. 经验教训

1. **S1 阶段问对问题很关键**: "spec.md 太大怎么办" 有 2 个相反方向答案 (拆分 vs 瘦身), 必须先跟用户对齐方向。
2. **域文档 vs 过程文档**: `specs/` 域文档 = 当前 SoT, `archive/` 过程文档 = 历史迭代记录。混在一起 = 域文档被历史污染。
3. **轻量 changelog 价值**: spec.md 不变 (精简契约) + CHANGELOG.md 增量变更 = 用户读 spec 看 SoT, 看 CHANGELOG 看演化。比"按 S 分片"17 个文件友好 10×。
4. **S1_Cancelled 不进 archive.md 索引**: S1 取消的 change 不进 demand-archive-index (因为无 S5/S6 验收), 仅留 `replaces`/`replaced_by` 字段在替代方案的 archive 中追溯。
