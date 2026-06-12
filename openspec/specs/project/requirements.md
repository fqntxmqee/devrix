# 需求规范

**版本:** 1.0.0
**状态:** Active
**所属阶段:** S1
**关联规范:** `architecture-design.md`

---

## 1. Demand ID 分配规则

### 1.1 格式

```
DM-YYYYMMDD-NNN
```

- `YYYYMMDD` — 需求创建日期
- `NNN` — 当日序号，从 001 开始

### 1.2 分配流程

1. 检查 `openspec/demand-archive-index.md` 确认当日已有最大序号
2. 按递增分配（001, 002, ...）
3. 不允许跳号或预占
4. 同一 Change 只能关联一个 DM ID；同一 DM ID 不能关联多个 Change

### 1.3 Change ID 规则

```
devrix-{module-name}
```

- 全小写，连字符分隔
- 与 `openspec/changes/<change-id>/` 目录名一致
- 与 `feat/<change-id>` 分支名一致

---

## 2. demand.md 模板

需求文档的核心要素：

```markdown
---
demand-id: DM-YYYYMMDD-NNN
title: <一句话描述>
priority: P0 | P1 | P2
status: S1_Proposal
dsaft_domain: architecture | communication | context-engine | llm-gateway | multi-agent | observability
created: YYYY-MM-DD
---

# <标题>

## 1. 背景
<为什么需要这个需求？>

## 2. 问题陈述
<具体描述要解决的问题>

## 3. 验收标准
| ID | 标准 | 优先级 |
|----|------|--------|
| AC1 | ... | P0 |
| AC2 | ... | P1 |

## 4. 依赖与约束
| 类型 | 内容 |
|------|------|
| 依赖 | <外部依赖> |
| 约束 | <不可突破的限制> |

## 5. 变更范围
### 新增 / 修改 / 不变更

## 6. 风险评估
| 风险 | 影响 | 缓解 |
|------|------|------|
```

---

## 3. 验收标准规范

- 每个需求至少 1 个 P0 验收标准
- 验收标准必须可测试、可度量
- AC ID 格式：`AC{N}`（AC1, AC2, ...）
- P0 标准对应 S5 验收的阻断条件

---

## 4. 范围定义规则

| 标记 | 含义 |
|------|------|
| **新增** | 当前不存在的文件、模块、功能 |
| **修改** | 已有文件的变更 |
| **不变更** | 明确声明不会改动的区域 |

必须明确声明 Out of Scope，防止范围蔓延。

---

## 5. 需求与 T 层映射

- S1 阶段不需要完成 T 层注册，但应在 demand.md 中标注关联的 DSAFT 域
- S2 阶段必须在 `openspec/t-registry.md` 预登记 T 层测试点（状态：PLANNED）

---

## 6. 禁止工时估算

demand.md **不得**包含工时估算。理由是：
- 需求阶段的估算极度不可靠，容易误导规划
- 工时估算属于实施阶段（S4 tasks.md）的职责
- 估算与方案绑定而非与需求绑定：同一需求可能有多个实现方案，方案不同工时不同

估算只在 `tasks.md` 中出现，且仅为参考值，不作为承诺。

---

## 7. 检查清单

S1 完成前确认：

- [ ] DM ID 已分配且无冲突（检查 `demand-archive-index.md`）
- [ ] demand.md 包含背景、问题、验收标准、范围
- [ ] 至少 1 个 P0 验收标准
- [ ] Out of Scope 已明确声明
- [ ] DSAFT 域标注正确
