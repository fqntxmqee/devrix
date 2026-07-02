# Proposal: D7 Spec Split (Cancelled at S1)

**Change ID:** `devrix-d7-spec-split`
**Demand ID:** DM-20260630-002
**Status:** **S7_Archived (S1_Cancelled)** (2026-06-30 取消, 2026-07-02 收尾归档)
**Replaced by:** `devrix-spec-lite-mode` (DM-20260630-003, S7_Archived 2026-06-30)

---

## 1. 原意图 (S1 提案)

按"按 S 分片"思路把 d7-orchestration/spec.md (2622 行) 拆成 ≤ 800 行片段（按 D7-S1/S3/S4/S5/S8/.../S22 切片），最后按需扩展至其他超大域文档（d2/d3/d4）。

### 1.1 拆分范围 (17 S 切片)

D7-S1 / S3 / S4 / S5 / S8 / S9 / S11 / S12 / S13 / S14 / S15 / S16 / S18 / S20 / S21 / S22

## 2. 取消原因 (复盘)

按"按 S 分片"思路推进 d7 spec.md 实际拆分时, **发现方向偏离用户原始意图**：

> 用户希望 `specs/` 域文档 = **精简设计契约（最新符合代码）**, 过程需求迭代走 `archive/`, 域目录最多放一个**轻量级 changelog**。

- "按 S 分片"思路 → 把 spec.md 拆成 17 个 ≤ 800 行的分片
- 实际用户意图 → 把 spec.md 整体压成 ≤ 200 行的精简契约 + changelog

**方向本质相反**: 一个是"分而治之"扩容, 一个是"压成精华"瘦身。

## 3. 替代方案 (Replaced by devrix-spec-lite-mode)

`devrix-spec-lite-mode` (DM-20260630-003) 实施"压成精华"路线：

- `openspec/specs/d7-orchestration/spec.md` 2622 → 195 行 (-92.6%)
- 新增 `CHANGELOG.md` 103 行
- 12/12 AC PASS
- 推广: d1/d2/d3/d4/d5/d6 全部 spec-lite 化（2026-06-30 当日 6 站 PR #333-#347 全部 merge）

详见 `openspec/archive/2026-06-30-devrix-spec-lite-mode/acceptance-report.md` (verdict: ACCEPTED)。

## 4. 经验教训 (Lessons Learned)

1. **S1 阶段问对问题很关键**："spec.md 太大怎么办" 有 2 个相反方向答案：
   - 拆分（按 S 分片, 扩容）
   - 瘦身（精简契约, 压成精华）
   - S1 阶段必须先跟用户对齐方向, 否则后续 S2-S5 全是浪费。
2. **域文档 vs 过程文档**：`specs/` 域文档 = 当前 SoT, `archive/` 过程文档 = 历史迭代记录。
   混在一起 = 域文档被历史污染（spec.md 越来越长直到失控）。
3. **轻量 changelog 价值**：spec.md 不变（精简契约）+ CHANGELOG.md 增量变更 = 用户读 spec 看 SoT, 看 CHANGELOG 看演化。
   比"按 S 分片"17 个文件友好 10×。

## 5. 归档完整性 (S6 archive metadata)

- ✅ `openspec/archive/2026-06-30-devrix-d7-spec-split/.openspec.yaml` (status s1_cancelled)
- ✅ `openspec/archive/2026-06-30-devrix-d7-spec-split/proposal.md` (本文件)
- ✅ `openspec/changes/devrix-d7-spec-split/` 已移除 (S6 收尾)
- ✅ `devrix-spec-lite-mode` (DM-20260630-003) `replaces: devrix-d7-spec-split` 字段已指明
- ✅ 0 PR (S1 阶段取消, 无 S4 实施)
- ✅ 0 T 点 (S1 阶段取消, 无 t-registry 增量)
