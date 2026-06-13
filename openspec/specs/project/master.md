# Devrix 项目研发规范

**版本:** 1.1.0
**状态:** Active
**最后更新:** 2026-06-12

---

## 1. 概述

本规范是 Devrix 项目研发活动的顶层入口。它定义研发流程的各个阶段，并指定每个阶段应遵循的子规范。

### 1.1 核心原则

- **OpenSpec 强制** — 所有需求到归档的变更管理必须遵守 OpenSpec S1-S6 六阶段流程
- **Git 强制** — 分支策略采用 GitHub Flow，提交遵循 Conventional Commits
- **规范路由** — Agent 或开发者进入任一阶段时，根据路由表加载对应子规范

### 1.2 规范索引

| 子规范 | 文件 | 用途 |
|--------|------|------|
| 需求规范 | `requirements.md` | DM ID 分配、demand.md 模板、验收标准格式 |
| DSAFT 方法论 | `../../docs/methodology/dsaft-methodology.md` | 五层架构体系、领域类型、ID 格式、追溯规则（所有阶段基础） |
| 架构设计规范 | `architecture-design.md` | .openspec.yaml、proposal/design/spec 模板与设计原则 |
| 设计 Review 规范 | `review-design.md` | S3 门禁：设计审查清单与通过标准 |
| 编码规范 | `coding.md` | Go 编码规则（引全局规则 + 项目补充） |
| 测试规范 | `testing.md` | 测试金字塔、T 层追溯、覆盖率要求（引 testing-framework/spec.md） |
| 代码 Review 规范 | `review-code.md` | S4 门禁：代码审查清单与通过标准 |
| 归档规范 | `archiving.md` | S6 归档检查清单与操作流程 |
| 配置与环境规范 | `config-environment.md` | 配置层级、环境变量、secret 管理 |

---

## 2. 研发流程

### 2.1 六阶段总览

```
S1 需求 → S2 提案 → S3 设计 → S4 实现 → S5 验收 → S6 归档
  │          │          │          │          │          │
Issue    分支       Draft PR   逐 commit    test-all    merge
         .yaml      design.md   实现代码    验收报告     archive
         proposal   specs/      tasks.md
```

### 2.2 阶段→规范路由表

Agent 或开发者进入任一阶段时，**必须**加载对应的子规范。这是规范体系的核心路由机制。

| 阶段 | 角色 | 必须遵循的规范 | 产出物 | 门禁 |
|------|------|---------------|--------|------|
| S1 需求 | 提出者 | `requirements.md` | `demand.md`（可选，轻量变更可免） | DM ID 合法 |
| S2 提案 | 架构师 | `requirements.md`、`dsaft-methodology.md`、`architecture-design.md` | `.openspec.yaml`、`proposal.md` | 文件完整性检查 |
| S3 设计 | 架构师 | `dsaft-methodology.md`、`architecture-design.md` | `design.md`、`specs/*/spec.md` | — |
| **S3-Gate** | **Reviewer** | **`review-design.md`** | **Review 结论** | **设计审查通过** |
| S4 实现 | 开发者 | `coding.md`、`testing.md` | 代码 + 测试 + `tasks.md` | `go vet` + `test-unit` 通过 |
| **S4-Gate** | **Reviewer** | **`review-code.md`** | **Review 结论** | **代码审查通过** |
| S5 验收 | QA | `testing.md` | `acceptance-report.md` | P0 T 层 100% PASS、覆盖率 ≥ 80% |
| S6 归档 | 维护者 | `archiving.md` | `archive/` 目录 + 域文档同步（如需要） | 归档检查清单 + 域文档同步评估通过 |

### 2.3 如何使用

**Agent 模式：** 当 Agent 需要执行某个阶段的任务时，先读 `master.md` 找到该阶段的规范列表，再加载对应子规范。例如：
- S2 提案阶段 → 加载 `requirements.md` + `dsaft-methodology.md` + `architecture-design.md`
- S4 实现阶段 → 加载 `coding.md` + `testing.md`
- S3-Gate → 加载 `review-design.md`

**人类开发者：** 在每个阶段的开始，查阅对应子规范了解必须遵守的规则和产出模板。

---

## 3. Git 规范

### 3.1 分支策略

采用 **GitHub Flow**。`main` 始终保持可部署。

```
main
  └── feat/<change-id>     # 功能分支
  └── fix/<change-id>      # 修复分支
```

规则：
- 一个 Change 一个分支
- 分支从 `main` 拉出，合并回 `main`
- 合并前 CI 必须通过
- 禁止 force push 到 `main`

### 3.2 提交信息格式

```
<type>: <简短描述>

关联: D{X}-S{X}-A{XX}-T{XX}
Change: <change-id>
```

类型：`feat`、`fix`、`refactor`、`test`、`docs`、`chore`、`perf`、`ci`、`proposal`、`design`、`acceptance`、`archive`

### 3.3 PR 规范

- PR 标题：`<change-id>: <简短描述（50 字以内）>`
- S3 设计完成时创建 Draft PR
- S4 所有任务完成后标记 PR 就绪（`gh pr ready`）
- 合并使用 squash merge（`gh pr merge --squash --delete-branch`）

---

## 4. OpenSpec 规范

### 4.1 Change 目录结构

```
openspec/changes/<change-id>/
  ├── .openspec.yaml         # 元数据（必须）
  ├── demand.md              # 需求（可选，轻量变更可免）
  ├── proposal.md            # 提案（必须）
  ├── design.md              # 技术设计（必须）
  ├── tasks.md               # 任务拆解（必须）
  ├── acceptance-report.md   # 验收报告（S5 生成）
  └── specs/                 # Gherkin 规格（必须）
      └── <module>/
          └── spec.md
```

### 4.2 已完成 Change 归档

```
openspec/archive/<YYYY-MM-DD>-<change-id>/
```

### 4.3 T 层测试点注册

所有能力变更必须在 T 层注册表预登记测试点。根索引：`openspec/t-registry.md`；各域注册表：`openspec/specs/d{N}-*/t-registry.md`。编号格式：`D{X}-S{X}-A{XX}-T{XX}`（T 归属 A）或 `D{X}-S{X}-A{XX}-F{XX}-T{XX}`（T 归属 F）。

---

## 5. 文档 SoT 约定

| 目录 | 角色 | 读者 |
|------|------|------|
| `openspec/specs/project/` | 项目规范（本目录）— 权威 | Agent + 人类 |
| `openspec/specs/d{N}-*/` | 域架构归档（spec.md + A/F/T 注册表）— 权威 | Agent + 人类 |
| `docs/` | 架构概述、设计说明 — 辅助阅读 | 人类 |
| `internal/` | 代码 — 唯一可执行事实 | 编译器 |

**规则：** 当 `docs/` 与 `openspec/specs/` 冲突时，openspec/specs 胜出。
