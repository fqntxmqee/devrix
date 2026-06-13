# D7 Orchestration Domain — Review R1 决议索引

**Review Date:** 2026-06-14
**Reviewer:** Architecture review（Cursor Agent）
**Status:** Incorporated into demand.md + d7-domain.md v2.2.0; R2 closed in review-r2.md
**Next Action:** 二次评审（Claude / 人工）— **仅文档，不开发**

---

## 1. Review 摘要

| 维度 | 原需求问题 | R1 决议 |
|------|-----------|---------|
| 域定位 | 「位于 D1 之上」易误解 | 改为横向**协调层**，D1 仍拥有 ingress |
| Task 模型 | 需求写「单一 SoT」，代码三套模型 | 三模型**职责分离** + 统一查询入口，v1.0 不合并存储 |
| 状态机 | created→assigned→running 与代码不符 | 沿用 pending/in_progress，附**别名映射表** |
| S2 vs S3 | 「串行 dispatch」与 Wave 并行矛盾 | **编排路由矩阵**明确分工 |
| S5 | PlanMode 与自动拆解未区分 | **S5-P1~P4 分阶段**，v1.0 仅 P1+P2 |
| 迁移 | 双入口共存行为未定义 | **Migration Coexistence Contract** + 4 组合矩阵 |
| 性能 | FastPath ≤2ms 不可测 | 拆为 T02a（proxy）+ T02b（classify） |
| 中断 | HandleInterrupt 范围模糊 | 拆 4 子能力，分步验收 |
| Background | 设计说要迁 D7，需求无 Scenario | 补 D7-S1-T07 + DM-011 对齐说明 |
| 配置 | store_dir 重复 | 单一 SoT：`context_engine.tasks.store_dir` |
| 交付 | 缺 demand/tasks | 补 demand.md、tasks.md |
| T 层 | D7-S5-T01 测试归属错误 | 修正为 plan_mode 测试 |

---

## 2. 未决议项（R2 已闭合 — 见 `review-r2.md` §3）

| ID | 问题 | 最终决议 |
|----|------|----------|
| OQ-1 | Wave 触发是否必须经 Plan approve gate？ | **A 强制 + 白名单** |
| OQ-2 | `d7_enabled` 默认何时翻 true？ | **B 内部 dogfood 先 true，对外默认 false** |
| OQ-3 | ClassifyIntent LLM 兜底是否 v1.0 必须？ | **B 改进版：tail-only shadow，P1** |
| OQ-4 | internal/layers/d7/ 是否 Phase B 就建？ | **A 先骨架 + re-export** |

---

## 3. 文档变更清单（R1）

| 文件 | 变更类型 |
|------|----------|
| `changes/.../demand.md` | 新增 |
| `changes/.../review-r1.md` | 新增（本文档） |
| `changes/.../tasks.md` | 新增 |
| `changes/.../proposal.md` | 修订 Phase 顺序 |
| `changes/.../.openspec.yaml` | 修订 version_scope、status |
| `specs/d7-orchestration/d7-domain.md` | v2.1.0 澄清章节 + 新 Requirement |
| `specs/d7-orchestration/spec.md` | 同步澄清摘要 |
| `specs/d7-orchestration/design.md` | 路由矩阵 + 三模型图 |
| `specs/d7-orchestration/t-registry.md` | T02 拆分、T07 新增、T01 修正 |

---

## 4. 二次 Review 检查清单

评审人请逐项确认：

- [ ] 三模型分离是否可接受，还是必须坚持 v1.0 合并？
- [ ] 编排路由矩阵是否覆盖所有现网路径（delegate_tools、/plan、wave）？
- [ ] S5-P2 only（无自动拆解）是否满足 v1.0 业务目标？
- [ ] Migration 4 组合矩阵是否足够？
- [ ] Phase 顺序修订是否合理？
- [ ] OQ-1 ~ OQ-4 的选择

---

**维护：** 二次 Review 结论写入 `review-r2.md` 或更新本文档 §2 决议项。
