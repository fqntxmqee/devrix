# Devrix 文档索引

Devrix 文档分为 **架构（DSAFT）**、**域级详细设计**、**OpenSpec 规格** 三层。改代码前先看架构；写验收先看 OpenSpec。

---

## 架构文档（推荐入口）

| 文档 | 说明 |
|------|------|
| [**architecture/README.md**](./architecture/README.md) | 架构文档地图与 Onboarding 路径 |
| [architecture/dsaft-overview.md](./architecture/dsaft-overview.md) | D-S-A-F-T 编号、七域 + ORCH |
| [architecture/code-map.md](./architecture/code-map.md) | 包路径 ↔ D{N}-S{M} 对照 |
| [architecture/request-flow.md](./architecture/request-flow.md) | 用户消息端到端时序（QueryLoop） |
| [architecture/contracts-and-boundaries.md](./architecture/contracts-and-boundaries.md) | 跨层契约、依赖红线 |

**OpenSpec 权威规格（与上表互补）：**

| 路径 | 内容 |
|------|------|
| `openspec/specs/architecture/layering.md` | DSAFT 注册表 v3.1 |
| `openspec/specs/architecture/code-atlas.md` | 机器可读模块索引 |
| `openspec/a-registry.md` / `f-registry.md` / `t-registry.md` | A / F / T 层 |
| `openspec/project.md` | 项目元数据 |

---

## 域级详细设计（六段式）

框架说明：[detail design framework.md](./detail%20design%20framework.md)

| 文档 | 域 | 状态 |
|------|-----|------|
| [context-engine-design.md](./context-engine-design.md) | D2 | **Historical** — 含 PEV 章节；现行见 architecture/request-flow.md |
| [llm-gateway-design.md](./llm-gateway-design.md) | D3 | S7 已归档，层边界仍参考 |
| [multi-agent-design.md](./multi-agent-design.md) | D4 | Design Review |
| [observability-design.md](./observability-design.md) | D5 | 分析文档 |

---

## 专题与运维

| 文档 | 说明 |
|------|------|
| [config.md](./config.md) | 配置项说明 |
| [prompt-system-design.md](./prompt-system-design.md) | Prompt / Harness 装配 |
| [task-planning-design.md](./task-planning-design.md) | Task 工具与 plan mode |
| [llm-gateway-model-resolution-trace.md](./llm-gateway-model-resolution-trace.md) | 模型 Tier 解析 |
| [coverage.md](./coverage.md) | Operation 染色与覆盖率 |
| [development-workflow.md](./development-workflow.md) | OpenSpec S1–S7 + GitHub Flow |

---

## 新增模块流程

1. 在 `openspec/changes/{slug}/` 创建 demand → proposal → design → tasks（S1–S3）
2. 在 `openspec/t-registry.md` 登记 T 测试点
3. 按 [architecture/code-map.md](./architecture/code-map.md) 确定包路径
4. 可选：按六段式补充 `docs/{module}-design.md`
5. S4 开发 → S5 验收 → S7 归档

---

## 已移除 / 归档

| 文档 | 说明 |
|------|------|
| ~~llm-gateway-pev-message-flow.md~~ | PEV 已退役；由 architecture/request-flow.md 替代 |
