# Change 文档索引 — devrix-d5-v2-terminal

**Demand ID:** DM-20260619-006  
**阶段:** S3 设计完成（S4 未启动）

---

## 研发流程产物

| 阶段 | 文件 | 说明 |
|------|------|------|
| S1 | `demand.md` | 需求 + AC + L1–L5 + 依赖 |
| S2 | `proposal.md` | 方案 + Out of Scope |
| S3 | `design.md` | Decision ×6 + 回归/回滚 |
| S3 | `tasks.md` | Phase A/B/C 任务 |
| S3-Gate | `design-review.md` | 审查清单 |
| S3 | `specs/d5-observability/spec.md` | Gherkin delta |
| S5 | `acceptance-report.md` | **待 S5** |
| S7 | `openspec/archive/...` | **待归档** |

---

## 博弈论对焦（给 Claude）

| 优先级 | 文件 | 读什么 |
|--------|------|--------|
| **P0** | `gaming-analysis.md` | 玩家、错配、Commitment、OQ-1~6 |
| P0 | `d5-requirements-clarifications.md` | Grill Review §6 六个问题 |
| P1 | `specs/d5-boundary.md` | 跨域激励（D7 Turn vs D5 Referee） |
| P1 | `specs/d5-domain.md` | 博弈角色表 |
| P2 | `design.md` §2 Decision | 六项已拍板决议 |

---

## 规格草案（S7 → `openspec/specs/d5-observability/`）

| 文件 | 目标路径 |
|------|----------|
| `specs/d5-domain.md` | `d5-domain.md` |
| `specs/d5-boundary.md` | `d5-boundary.md` |
| `specs/observability-guide.md` | `observability-guide.md` |
| `specs/terminal-state-guide.md` | `terminal-state-guide.md` |
| `specs/dsaft-architecture.md` | `dsaft-architecture.md` |
| `specs/a-registry.md` | `a-registry.md` v4.0 |
| `specs/f-registry.md` | `f-registry.md` v3.0 |
| `specs/layer-delta.md` | 合并入 `layer-delta.md` |
| `specs/d5-observability_delta.md` | 归档参考 |
| `specs/cross-domain-boundaries-d5-delta.md` | 合并入 `cross-domain-boundaries.md` |

---

## 关联归档 Change

| Change | DM | 关系 |
|--------|-----|------|
| devrix-d5-sa-refine | DM-20260615-001 | v1.0 Registry 父 change |
| devrix-d5-d6-sa-refine-v2.0 | DM-20260615-003 | v2.0 物理迁移 |
| devrix-diagnostic-tools-* | DM-20260616~18 | S23 能力来源 |
| devrix-d7-v2-structure | DM-20260619-005 | 终态闭合参考模式 |
