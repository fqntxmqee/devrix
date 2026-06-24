# Demand: D7 MUPS v5 PR-V5.6 T2 ResumeSession 续跑入口 — Review 修复

**Demand ID:** DM-20260625-004
**Date:** 2026-06-25
**Origin:** PR-V5.6 (`532ebea`) 整体审查(4-agent 并行 review)
**Priority:** P0 (2 Critical t-registry 不一致 + 2 High 语义对齐 + 2 High 测试覆盖)

---

## 1. 来源

DM-20260625-003 PR-V5.6 (`532ebea`) 已于 2026-06-24 squash-merged,后续 PR-V5.6 doc sync #201 (`e729660`) 已同步 demand-archive-index.md。

对 PR-V5.6 进行了 4-agent 并行 review(数据 / 逻辑 / 边界 / 调用 / 异常 5 维度 + 测试质量 + 跨包契约 + spec 一致性),产出 **14 条发现**:
- 🔴 Critical 3 条(t-registry 内部数字脱节 + `runLoopWithResume` 不存在)
- 🟡 High 4 条(audit 注释脱节 + prior attrs 短路漏写 + span attr 测试 0 覆盖 + 5 节点未触发断言不全 + Metadata 4 字段漏断言)
- 🟢 Medium 4 条(空 sessionID 守卫 / fail-safe attr 对称 / dead code stubResume / V5.5 archive 描述)
- 🔵 Info 3 条

## 2. 修复范围

按 V4 review-fixes (DM-20260625-002) 同样采用 **hotfix 路径**(feedback-devrix-bugfix-skip-openspec):

### 范围(本 change 修复,7 条):

| ID | 维度 | 文件:行 | 工作量 |
|---|---|---|---|
| **C-1** | runLoopWithResume 不存在 | `openspec/specs/d7-orchestration/t-registry.md:567, 591` | 删 2 处描述 |
| **C-2** | Statistics 表 4 处数字未刷新 | `t-registry.md:434, 449, 450, 498-502` | 5 行更新 |
| **H-1** | audit 注释脱节(非功能 bug) | `escape_wiring.go:134-135, 189-190` | 改注释 4 行 |
| **H-2** | 短路早退时 prior attrs 全跳过 | `orchestrator.go:338-341` | +4 行 |
| **H-3** | 单元测试 0 验证 span attr 写入 | `orchestrator_resume_test.go` | +50 行新 test |
| **H-4** | 集成测试未断言 5 节点未触发 + Metadata 4 字段漏 | `orchestrator_resume_test.go` | +30 行增强 |
| **M-1** | 空 sessionID 静默 fail-safe | `escape_wiring.go:141 之前` | +3 行 |

### 不在范围(后续 cleanup change 处理):

- **M-2** fail-safe 2/3 span attr 对称 (低优先级,语义不影响)
- **M-3** dead code stubResume 删除(可顺手,合并入代码 PR)
- **M-4** V5.5 archive 描述(archive 不可改,放 V5.6 proposal 注释即可)
- **I-1..I-4** Info 级别(可延后)

## 3. 拆分 PR

| PR | 内容 | 行数估算 | 风险 |
|---|---|---|---|
| **Step 1 (docs-only)** | C-1 + C-2(t-registry 内部一致性) | 1 file, ~10 行 | 零(纯 docs) |
| **Step 2 (code + test)** | H-1 + H-2 + H-3 + H-4 + M-1 + M-3 | 4 files, ~80 行 | 低(纯增量 + 注释修正) |

## 4. 与已有 Change 的关系

- **不重复** `devrix-d7-mups-v5-escape-engine` (DM-20260625-003 PR #198) — 那是 V5.1..V5.5 主工作
- **不重复** `devrix-d7-mups-v5-escape-engine-v5-6` (DM-20260625-003 PR #200) — 那是 V5.6 主工作
- **姊妹篇** `devrix-d7-mups-v4-review-fixes` (DM-20260625-002 PR #192) — 同模式 hotfix 修复,V4 5 节点管道 review 修复 3C+10H+1doc
- **hotfix 路径依据** `feedback-devrix-bugfix-skip-openspec.md` — 跳过 S1-S3 完整立项,代码已落地 + 提案后置

## 5. 验收

- docs-only PR (Step 1) verify-archive.sh 12/12 PASS(0 regression)
- code PR (Step 2) 22/22 orchestration packages go test -race PASS + verify-archive.sh 12/12 PASS
- t-registry 数字与代码 100% 一致(`186 | 186 | 0 | 0 | 153` + D7-S14 18/18/0/0 + D7-S11 13/13/0/0)
- 14 → 7 修复(本 change),剩余 7 条(M/I 级别)留待后续 cleanup change