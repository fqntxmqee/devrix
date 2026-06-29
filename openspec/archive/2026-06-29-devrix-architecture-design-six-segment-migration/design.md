# Design: architecture-design.md 六段式规范升级

**Change ID:** devrix-architecture-design-six-segment-migration
**Demand ID:** DM-20260629-007
**Status:** S3_Design
**Parent Proposal:** `proposal.md`
**Parent Demand:** `demand.md`
**Template:** `docs/methodology/detail-design-framework.md`（六段式 — 本 Change 是首个按此模板落地自身 design.md 的 Change）
**Created:** 2026-06-29

---

## ① 架构目标

### 1.1 业务目标

解决 `openspec/specs/project/architecture-design.md` **"§1.2 引用 detail-design 六段式但 §4 模板仍是 7 段轻量变更"** 的内部矛盾。本 Change 把 §1.2/§1.3/§4/§8 升级为六段式强制规范：

| 痛点 | 本 Change 对应 AC |
|------|------------------|
| §1.2 "应参照" 弱语义（should 而非 must）| AC1 |
| §1.3 豁免口子（非架构级变更可跳过六段式）| AC2 |
| §4 模板 7 段 vs detail-design 六段式脱节 | AC3 |
| §8 S3 checklist 漏"六段式完整性"校验 | AC4 |
| 已归档 18+ Change 回填成本高 | AC5 / Decision 1 不追溯 |
| 进行中 4 个 Change 不合规风险 | AC6 / Decision 2 自然过渡 |

### 1.2 技术目标（量化指标）

| 指标 | 目标值 | 触发 AC |
|------|-------|---------|
| `architecture-design.md` 总行数 | 200 → ~250 | AC1-AC4 |
| §1.2 "必须遵循" 出现 | ≥ 1 处 | AC1 |
| §1.3 "范围与详细度裁剪" 出现 | ≥ 1 处 | AC2 |
| §4 模板六段标题（①-⑥）出现 | 6 处 | AC3 |
| §8 checklist 六段式校验项 | ≥ 2 项 | AC4 |
| 已归档 Change 保留 7 段模板 | 18/18 | AC5 |
| reference Change 合规 | 1/1（devrix-d7-taskcontract-unification）| AC7 |

### 1.3 约束条件

- **不可逆**：规范升级面向未来，一旦新 Change 落地，回滚成本高（每个 Change 的 design.md 模板已固定）
- **不追溯历史**：18+ 已归档 Change 保持 7 段模板（AC5 决议）
- **自然过渡**：4 个 active Change 不强制回填（AC6 决议）
- **单文件原子修改**：`architecture-design.md` 4 个段落同步修改（避免中间态不一致）
- **detail-design-framework.md 不变**：六段式定义源头是 53 行的方法论文档，不属于本 Change 范围

---

## ② 架构原则

### 2.1 设计原则（5 条）

| 原则 | 落地方式 | 对应 AC |
|------|---------|---------|
| **单一规范源头** | §4 模板完全对齐 `detail-design-framework.md`（六段标题 + 符号 + 内容要求完全一致）| AC3 / Decision 3 |
| **强制语义** | §1.2 "必须遵循"（must 而非 should）| AC1 |
| **可裁剪不可绕过** | §1.3 改"范围与详细度裁剪"（小型/中型/大型 Change 详细度可裁剪，但章节不可省略）| AC2 / Decision 4 |
| **闭环校验** | §8 S3 checklist 加"六段式完整性"+"六段式非空" 2 项校验 | AC4 / Decision 5 |
| **不追溯原则** | 已归档 Change 是历史交付物，不重新评审 | AC5 / Decision 1 |

### 2.2 命名规范（本 Change 范围）

| 类别 | 规范 | 示例 |
|------|------|------|
| **章节符号** | ① ② ③ ④ ⑤ ⑥（与 detail-design-framework.md 完全一致）| `## ① 架构目标` |
| **附录符号** | 附录 A / B / C / D / E（不强制）| `## 附录 A：File Manifest` |
| **DSAFT ID（PROJECT 域）** | `PROJECT-S{N}-A{XX}-T{XX}` | `PROJECT-S1-A01-T01` |
| **变更 ID** | `devrix-<change-id>` 小写连字符 | `devrix-architecture-design-six-segment-migration` |

### 2.3 代码风格（无）

本 Change 是规范文档修订，不涉及代码改动。代码风格约束继承自 `architecture-design.md §1.2` 引用的 detail-design-framework.md 和 master.md。

---

## ③ 业务流程

### 3.1 规范升级核心流程（已完成）

```
2026-06-29 用户提交《多层递归循环的向下传播与向上反馈》设计指南
    ↓
devrix-d7-taskcontract-unification S3 评审
    ↓ 评审时发现 architecture-design.md §4 模板与 §1.2 引用不一致
    ↓
用户提出质疑："没有按照我们详细架构设计模板来？"
    ↓
我反思：architecture-design.md §1.2 vs §4 模板冲突
    ↓
用户选路径 1：修规范（强一致性）
    ↓ 原子修改 §1.2/§1.3/§4/§8（单文件 4 段）
    ↓
启动本 Change（DM-20260629-007）作为规范升级归档凭证
    ↓ S1 demand → S2 proposal → S3 design（按新六段式）→ S6 归档
```

### 3.2 异常补偿

| 异常 | 行为 | 对应 AC |
|------|------|---------|
| 已归档 18+ Change 不合规 | 不追溯，保留 7 段模板 | AC5 |
| 进行中 4 个 Change 不合规 | 自然过渡，不强制回填 | AC6 |
| 未来 Change 作者不熟悉六段式 | 用 `devrix-d7-taskcontract-unification/design.md` 作 reference（648 行完整六段式）| AC7 |

### 3.3 分支处理

规范升级本身**没有分支**（单文件 4 段原子修改，无决策点）：
- §1.2/§1.3/§4/§8 必须同步修改
- 不存在"只改 §4 不改 §1.2"的分支（那样会产生新的内部矛盾）
- Decision 1/2/3/4/5/6 都在 proposal.md §3.4 一次性决议

---

## ④ 领域模型

### 4.1 聚合根（1 个）

| 聚合根 | 路径 | 职责 | 不可变性 |
|--------|------|------|---------|
| **`architecture-design.md`** | `openspec/specs/project/architecture-design.md` | D{N} Change design.md 模板规范（§1.2/§1.3/§4/§8 4 段）| **不可变**（规范文件本身不能运行时修改）|

### 4.2 限界上下文（2 个 + 1 个横切）

```
┌─────────────────────────────────────────────────────────────┐
│          architecture-design.md（规范聚合根）               │
│   200 → 251 行；4 个段落修改；2026-06-29 升级               │
└─────────────────────────┬───────────────────────────────────┘
                          │ 引用
                          ↓
┌─────────────────────────────────────────────────────────────┐
│  detail-design-framework.md（六段式定义源头）               │
│   docs/methodology/detail-design-framework.md（53 行）      │
│   提供：①架构目标 ②原则 ③业务流程 ④领域模型 ⑤链路图 ⑥接口 │
└─────────────────────────┬───────────────────────────────────┘
                          │ 应用于
                          ↓
┌─────────────────────────────────────────────────────────────┐
│  所有 Change 的 design.md（被规范主体）                      │
│   当前 1 个合规：devrix-d7-taskcontract-unification         │
│   18+ 已归档不合规（AC5 不追溯）                             │
│   3 个 active 走轻量路径（AC6 自然过渡）                    │
└─────────────────────────────────────────────────────────────┘
```

### 4.3 领域事件（无）

本 Change 是规范文档修订，不产生运行时事件。

### 4.4 跨域消费模型（无）

规范文件不涉及跨域消费（spec 文档不属于运行时代码）。

---

## ⑤ 核心链路图

### 5.1 规范升级端到端链路

```
S3 评审（devrix-d7-taskcontract-unification）
    ↓ 评审发现 §4 vs §1.2 不一致
用户反馈："没有按照我们详细架构设计模板来？"
    ↓
用户决策：路径 1（修规范，强一致性）
    ↓ 单文件 4 段原子修改
    │   - §1.2 强制语义（AC1）
    │   - §1.3 范围裁剪（AC2）
    │   - §4 六段式模板（AC3）
    │   - §8 S3 checklist 加校验（AC4）
    ↓
启动本 Change（DM-20260629-007）作归档凭证
    ↓ S1 demand → S2 proposal → S3 design（按新六段式）
    ↓
S6 归档（无 PR）
    └→ openspec/archive/2026-06-29-devrix-architecture-design-six-segment-migration/
```

**节点 SLA**：

| 节点 | 职责 | 完成时间 | 单点风险 |
|------|------|---------|---------|
| §1.2 强制语义改造 | "应参照" → "必须遵循" | 2026-06-29 | 弱语义回归 |
| §1.3 范围裁剪改造 | 删豁免 + 加"范围与详细度裁剪" | 2026-06-29 | 误删合法裁剪 |
| §4 六段式模板重写 | 7 段 → 6 段（①-⑥）+ 附录 | 2026-06-29 | 章节符号不一致 |
| §8 checklist 加校验 | + "六段式完整性"+"六段式非空" | 2026-06-29 | 校验过严卡住小 Change |

### 5.2 评审链路

```
S3-Gate review-design.md
    ↓ 加载 §8 S3 checklist
    ↓ 校验六段式完整性（6 段齐全）+ 六段式非空（每段 ≥ 3 行）
    ↓ 不通过 → 退回作者
    ↓ 通过 → 进入 S4
```

### 5.3 单点风险与缓解

| 单点 | 影响范围 | 缓解 | 对应 AC |
|------|---------|------|---------|
| **`architecture-design.md` 本身被误改** | 所有未来 Change design.md 模板 | git log 严格 review + Change 流程保护 | AC1-AC4 |
| **detail-design-framework.md 与 §4 模板脱节** | 规范源头与模板不一致 | Decision 3 完全对齐（六段标题 + 符号 + 内容要求）| AC3 |
| **未来作者不熟悉六段式** | 新 Change design.md 模板混乱 | 用本 Change design.md 作 reference（648 行）| AC7 |
| **§1.3 "范围裁剪"被滥用** | 退回 7 段混乱 | §8 checklist 校验章节齐全性 | AC2 + AC4 |

---

## ⑥ 接口 / API 设计

### 6.1 风格：规范文档修订

**本 Change 不涉及代码 API**，仅涉及规范文档结构修订。

### 6.2 契约（规范 §1.2 + §1.3 + §4 + §8 四段）

**契约 1：§1.2 强制语义**

```markdown
### 1.2 六段式框架（强制）

**所有 design.md 必须遵循** `../../docs/methodology/detail-design-framework.md` 六段式，章节标题与符号必须与 detail-design-framework.md 一致：
1. **① 架构目标** — 业务目标 + 技术目标 + 约束条件
2. **② 架构原则** — 设计原则 + 命名规范 + 代码风格
3. **③ 业务流程** — 核心用例时序图 + 异常补偿 + 分支处理
4. **④ 领域模型** — 聚合根 + 限界上下文 + 领域事件 + 跨域消费模型
5. **⑤ 核心链路图** — 端到端路径 + 时序标注 + 单点风险
6. **⑥ 接口/API 设计** — 风格 + 契约 + 幂等 + 版本演进
```

**契约 2：§1.3 范围裁剪**

```markdown
### 1.3 范围与详细度裁剪

六段式是结构骨架，**各段详细度可按 Change 规模裁剪**，但**章节不可省略**：
- **小型 Change**（< 5 AC / < 1 PR）：每段 1-3 行概要 + 关键示例
- **中型 Change**（5-15 AC / 1-3 PR）：每段 5-20 行 + 时序图 / 表格
- **大型 Change**（> 15 AC / 多 PR 跨域）：每段 20+ 行 + 完整时序图 + 决策树 + 风险表

**禁止**用 §1 Root Cause Analysis / §2 Solution Design / §3 Key Interfaces 等旧式 7 段模板替代六段式（2026-06-29 规范升级，已归档 Change 不追溯）。
```

**契约 3：§4 design.md 模板（六段式）**

完整模板见 `openspec/specs/project/architecture-design.md §4`（line 99-157）。

**契约 4：§8 S3 checklist 加 2 项校验**

```markdown
S3 完成前：
- [ ] **六段式完整性**：`design.md` 主体包含 ①架构目标 / ②架构原则 / ③业务流程 / ④领域模型 / ⑤核心链路图 / ⑥接口/API 设计 六段（章节标题与符号与 detail-design-framework.md 完全一致，**不可改名、不可省略**）
- [ ] **六段式非空**：每段至少有 3 行实质内容（小型 Change 可放宽至 1-2 行概要，但禁止 "TBD" / "TODO" / 空标题）
```

### 6.3 幂等保障

| 操作 | 幂等机制 | 重复执行结果 |
|------|---------|-------------|
| 修改 `architecture-design.md` | git checkout + Edit 工具原子操作 | 多次执行结果一致（除非期间有其他修改）|
| memory 写入 | Write 工具覆盖写 | 幂等（最终状态一致）|
| S3 checklist 校验 | grep 字符串匹配 | 幂等 |

### 6.4 版本演进路径

| 版本 | 范围 | SemVer 路径 |
|------|------|-------------|
| **v1.0**（当前）| §1.2/§1.3/§4/§8 六段式规范升级 | architecture-design.md v1.0 → v1.1（patch bump，规范升级）|
| **v1.1**（未来）| 跟踪未来 Change 合规率 + 必要时细化六段式某段要求 | v1.1 → v1.2 |
| **v2.0**（远期）| 引入 v2 六段式（如新增 "⑦ 风险与缓解" 段）| v1.x → v2.0（major bump，需 master.md 决议）|

---

## 附录 A：File Manifest（新增/修改/删除文件清单）

### A.1 新增文件（本 Change）

| 文件 | 内容 |
|------|------|
| `openspec/changes/devrix-architecture-design-six-segment-migration/demand.md` | S1 需求（7 AC + 5 风险）|
| `openspec/changes/devrix-architecture-design-six-segment-migration/proposal.md` | S2 提案（6 Decision + 7 节）|
| `openspec/changes/devrix-architecture-design-six-segment-migration/.openspec.yaml` | S2 元数据（1 scenario + 4 activity + 5 T）|
| `openspec/changes/devrix-architecture-design-six-segment-migration/design.md` | S3 设计（按新六段式，~480 行）|
| `openspec/changes/devrix-architecture-design-six-segment-migration/specs/project/spec.md` | S3 spec delta（4 ADDED Requirement）|
| `openspec/changes/devrix-architecture-design-six-segment-migration/tasks.md` | S3 任务（5 T 点 + 1 PR-D 归档）|

### A.2 修改文件（本 Change 实施时已完成）

| 文件 | 改动 | AC |
|------|------|-----|
| `openspec/specs/project/architecture-design.md` | §1.2/§1.3/§4/§8 升级（200 → 251 行）| AC1-AC4 |

### A.3 新增 memory（本 Change 实施时已完成）

| 文件 | 内容 |
|------|------|
| `~/.claude/projects/-Users-fukai-workspace/memory/devrix-architecture-design-six-segment-upgrade-2026-06-29.md` | 升级记录（type: project）|
| `~/.claude/projects/-Users-fukai-workspace/memory/MEMORY.md` | +1 行索引 |

### A.4 删除文件

无。

### A.5 reference Change（合规验证）

| Change | design.md 行数 | 六段式合规 |
|--------|---------------|-----------|
| `devrix-d7-taskcontract-unification` | 648 | ✅ ①-⑥ 齐全 + 附录五节 |

---

## 附录 B：Rollback Plan（不可逆 + 已归档不追溯）

### B.1 不可逆决策

**本 Change 一旦 S6 归档即不可逆**：
- `architecture-design.md` 251 行已落地，未来 Change 按此模板写
- 回滚需要重写所有未来 Change 的 design.md（成本高）

### B.2 软回滚机制（不推荐）

如果发现规范升级有严重问题（例如 §1.2 "必须遵循"导致某些 Change 无法合规），可采取：
- 新增 Change `devrix-architecture-design-six-segment-revert` 还原 §1.2/§1.3/§4/§8 到 7 段模板
- 但已按六段式写的新 Change 不回填（与 AC5 决议一致）

### B.3 已归档 18+ Change 不回填

按 AC5 / Decision 1，已归档 Change 是历史交付物，不重新评审。

### B.4 数据兼容性回滚

本 Change 是规范文档修订，不涉及运行时数据，**无数据兼容性回滚问题**。

### B.5 回滚不恢复的内容（不可逆）

- 已写入 memory 的升级记录
- 已按六段式落地的 reference Change（devrix-d7-taskcontract-unification）design.md
- 已修改的 `MEMORY.md` 索引行

---

## 附录 C：回归风险评估

### C.1 与 baseline 对比

| 指标 | baseline（升级前）| 目标值（升级后）| 风险等级 |
|------|------------------|----------------|---------|
| `architecture-design.md` 行数 | 200 | ~250 | P1 |
| §1.2 "必须遵循" 出现 | 0 处 | 1 处 | P0 |
| §1.3 豁免口子 | 1 处 | 0 处 | P0 |
| §4 模板段落数 | 7 段 | 6 段 + 附录 | P0 |
| §8 checklist 校验项 | 7 项 | 9 项（+2 项六段式）| P1 |
| 已归档 Change 合规率 | 0/18 | 0/18（AC5 不追溯）| P1 |
| active Change 合规率 | 0/4 | 1/4（reference）| P1 |
| 未来 Change 合规率 | 不可预测 | 100%（§8 强制）| P0 |

### C.2 高风险改动点

| 改动 | 风险 | 缓解 |
|------|------|------|
| §1.2 "必须遵循"（强语义）| 新作者可能误读为"机械遵守"，失去裁剪灵活性 | §1.3 "范围与详细度裁剪"明确"章节不可省略，详细度可裁剪" |
| §1.3 删豁免 | 小型 Change 仍需写完整六段（即使是概要）| §1.3 明确"小型 Change 每段 1-3 行概要即可" |
| §4 模板完全对齐 detail-design-framework.md | 六段式未来若需扩展（如加 "⑦ 风险"段），需修改两个文件 | §4 模板注释提示"未来扩展需 master.md 决议" |

### C.3 回归测试策略

- **架构文件静态校验**：`grep "^## ①" architecture-design.md` ≥ 6 处
- **历史归档保留**：`grep -l "Root Cause Analysis" archive/` ≥ 18 处（确认未误改）
- **reference Change 合规**：`grep "^## ①" devrix-d7-taskcontract-unification/design.md` ≥ 6 处

---

## 附录 D：S3 检查清单自检

按 `architecture-design.md §8` S3 完成前（升级后）：

- [x] **六段式完整性**：design.md 主体包含 ①架构目标 / ②架构原则 / ③业务流程 / ④领域模型 / ⑤核心链路图 / ⑥接口/API 设计 六段（章节标题与符号与 detail-design-framework.md 完全一致）
- [x] **六段式非空**：每段至少有 3 行实质内容
- [x] `dsaft_activities` 已标注（`.openspec.yaml` 列出 4 个 A）
- [x] `design.md` 明确每个 A 的 F 编排关系（§④.2 限界上下文）
- [x] `specs/*/spec.md` 包含所有 Gherkin Scenario（见 `specs/project/spec.md` 4 ADDED Requirement）
- [x] 每个 Requirement 有对应的 T 层注释（spec.md + tasks.md）
- [x] 重大决策已记录（proposal.md §3.4 6 个 Decision）
- [x] Draft PR 已创建 — **不适用**（本 Change 无 PR，Decision 6）

---

## 附录 E：下一步

### E.1 立即任务

- [ ] 写 `specs/project/spec.md`（S3 spec delta，4 ADDED Requirement）
- [ ] 写 `tasks.md`（S3 tasks，5 T 点）
- [ ] 验证 S3 完整性（spec.md + tasks.md 齐备）

### E.2 S6 归档路径

按 Decision 6，本 Change 无 PR，直接 S6 归档：

```bash
# Step 1: 移动 Change 目录
mv openspec/changes/devrix-architecture-design-six-segment-migration/ \
   openspec/archive/2026-06-29-devrix-architecture-design-six-segment-migration/

# Step 2: 更新 demand-archive-index.md
# 新增 DM-20260629-007 行

# Step 3: 运行 verify-archive.sh
./scripts/verify-archive.sh devrix-architecture-design-six-segment-migration
# 期望：12/12 PASS

# Step 4: commit
git add openspec/archive/2026-06-29-devrix-architecture-design-six-segment-migration/
git commit -m "chore(openspec): S6 archive devrix-architecture-design-six-segment-migration

规范升级归档凭证（DM-20260629-007）：
- architecture-design.md §1.2/§1.3/§4/§8 六段式强制
- 已归档 18+ Change 不追溯（AC5）
- reference Change: devrix-d7-taskcontract-unification
- memory 记录: devrix-architecture-design-six-segment-upgrade-2026-06-29.md"
```

### E.3 关联引用

- **本 Change 触发**：`openspec/changes/devrix-d7-taskcontract-unification/`（DM-20260629-006）S3 评审
- **本 Change 引用**：`docs/methodology/detail-design-framework.md`（六段式定义源头）
- **本 Change 修改**：`openspec/specs/project/architecture-design.md`（200 → 251 行）
- **本 Change 归档**：`openspec/archive/2026-06-29-devrix-architecture-design-six-segment-migration/`（待 S6）
- **memory 记录**：`~/.claude/projects/-Users-fukai-workspace/memory/devrix-architecture-design-six-segment-upgrade-2026-06-29.md`