# Design Review: devrix-d5-v2-terminal

**Change ID:** devrix-d5-v2-terminal  
**Demand ID:** DM-20260619-006  
**Review Type:** S3-Gate（设计审查）  
**Reviewer:** Architecture（Agent 自检 + 待 Owner 确认）  
**Date:** 2026-06-19  
**结论:** **Approved with Suggestions**（可进入 S4，建议项不阻塞）

---

## 1. 审查范围

| 产出物 | 路径 | 状态 |
|--------|------|------|
| demand.md | `openspec/changes/devrix-d5-v2-terminal/` | ✅ |
| proposal.md | 同上 | ✅ |
| design.md | 同上 | ✅ |
| tasks.md | 同上 | ✅ |
| specs/d5-observability/spec.md | Gherkin delta | ✅ |
| specs/d5-observability_delta.md | Delta 摘要 | ✅ |
| specs/d5-domain.md | 领域 SoT 设计稿 | ✅ |
| specs/d5-boundary.md | 跨域边界设计稿 | ✅ |
| specs/observability-guide.md | 指南设计稿 | ✅ |
| specs/terminal-state-guide.md | 终态指南 | ✅ |
| specs/dsaft-architecture.md | 五层计数 Stub | ✅ |
| specs/a-registry.md | A v4.0 草案 | ✅ |
| specs/f-registry.md | F v3.0 草案 | ✅ |
| specs/layer-delta.md | v2.1 delta | ✅ |
| specs/t-registry-canonical-draft.md | T canonical 校正 | ✅ |
| specs/cross-domain-boundaries-d5-delta.md | 跨域增补 | ✅ |
| gaming-analysis.md | 博弈论推导 | ✅ |
| d5-requirements-clarifications.md | Review 澄清 | ✅ |
| README.md | 文档索引 | ✅ |

---

## 2. 架构决策审查（§2.1 review-design.md）

| 检查项 | 结果 | 备注 |
|--------|------|------|
| 层归属正确 | ✅ | D5 公共域；无反向依赖 D2/D7 业务包 |
| 接口方向正确 | ✅ | Bridge 注入；Tracker D2 只读 |
| 不重复造轮子 | ✅ | 删除 bridge 复用 instrument 包 |
| 跨层依赖最小 | ✅ | 契约经 Bridge + boundary 文档 |
| Decision 记录 | ✅ | design.md §2 六项 Owner 决议 |

---

## 3. 需求完整性审查（§2.2）

| 检查项 | 结果 | 备注 |
|--------|------|------|
| demand → proposal → design → specs | ✅ | 链路完整 |
| P0 AC 有 Scenario | ✅ | demand AC-A* / AC-B* 映射到 spec.md |
| Out of Scope 明确 | ✅ | proposal §4 + d5-domain |
| DM ID 无冲突 | ✅ | DM-20260619-006 未在 archive-index 登记（新需求） |

---

## 4. 规格质量审查（§2.3）

| 检查项 | 结果 | 备注 |
|--------|------|------|
| Gherkin 格式 | ✅ | GIVEN/WHEN/THEN |
| Happy + sad path | ✅ | 降级、compile fail、FaultInject no-op |
| 并发场景 | ✅ | RecordHit 100 goroutine（引用既有 T） |
| T 层映射 | ✅ | spec 注释 `<!-- T: ... -->` |
| PLANNED T 闭合计划 | ✅ | design §11 + spec MODIFIED |

---

## 5. 风险审查（§2.4）

| 风险 | 等级 | 缓解 |
|------|------|------|
| bridge 删除编译破坏 | 中 | grep 仅 5 处内部；tasks B2.4 验收 |
| 文档与代码短期不一致 | 低 | Phase A 先于 B；A 合并后 spec 即权威 |
| Doctor T↔A 历史错位 | 低 | canonical_a 列；不改 T ID |
| PR 面宽 | 中 | 3 PR 拆分（A / B1 / B2） |

**回滚：** Phase B 可独立 revert；Phase A docs-only 无运行时影响。

---

## 6. Grill Review 记录

| 决策点 | 结论 |
|--------|------|
| 范围 C vs 仅文档 | **Agreed** — C |
| S23 不增 S25 | **Agreed** — C3a–C3e |
| DebugFilter → S21 | **Agreed** |
| SessionBridge → S0 | **Agreed** |
| bridge 本轮删 | **Agreed** |
| T ID 不变 | **Agreed** |
| query.loop 文档退役 | **Agreed** |

---

## 7. Suggestions（非阻塞）

1. **Owner + Claude 博弈论对焦** — 优先读 `gaming-analysis.md` §8 OQ-1~6 与 `d5-requirements-clarifications.md` §6
2. **Draft PR** — S3-Gate 确认后创建 `docs/d5-v2-terminal-spec`（仅合并 `openspec/specs/`）
3. **demand-archive-index** — S7 归档时登记 DM-20260619-006

---

## 8. 博弈论对焦准备（可选 S3 延伸）

| 议题 | 文档位置 |
|------|----------|
| Referee 是否应 fail 业务 | gaming §8 OQ-2 |
| Coverage 是否发布 gate | gaming §8 OQ-3 |
| S23 子承诺上限 | gaming §8 OQ-1 |
| FaultInject 安全边界 | gaming §8 OQ-4 |

---

## 9. 检查清单（review-design.md §5）

- [x] 架构决策有 Decision 记录
- [x] 跨域变更有 boundary 文档
- [x] P0 Scenario 齐全
- [x] T 层映射完整
- [x] 回归风险已评估
- [x] 无工时估算在 proposal/design（符合 architecture-design.md §5）

---

## 10. 后续行动

| 阶段 | 行动 | 阻塞 S4？ |
|------|------|-----------|
| S3 收尾 | Owner 确认本 Review 结论 | 建议确认 |
| S3 | 创建 Draft PR（Phase A 分支） | 否 |
| **S4** | **Owner 批准后**执行 tasks Phase A → Gate → Phase B | — |
| S5 | acceptance-report.md | — |
| S7 | 归档 + 回写 `openspec/specs/` | — |

**明确：当前会话不进入 S4 实现（按 Owner「先不开发」）。**
