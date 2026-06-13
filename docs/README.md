# Devrix 文档索引

Devrix 文档分为 **稳定态文档（docs/）** 和 **演进态架构归档（openspec/specs/）** 两层。

- **docs/** — 配置说明、使用手册、方法论、可读版架构入口（不涉及具体架构设计，偏稳定态）
- **openspec/specs/** — 按域划分的架构归档：领域架构方案、可观测性方案、可测试性方案、A/F/T 注册表（随代码演进持续更新）

---

## 稳定态文档

### 方法论

| 文档 | 说明 |
|------|------|
| [methodology/dsaft-methodology.md](./methodology/dsaft-methodology.md) | DSAFT 五层架构方法论 v4.0 |
| [methodology/detail-design-framework.md](./methodology/detail-design-framework.md) | 详细架构设计框架（六段式模板） |

### 配置与运维

| 文档 | 说明 |
|------|------|
| [config.md](./config.md) | 配置项说明与热重载 |
| [development-workflow.md](./development-workflow.md) | OpenSpec S1–S7 + GitHub Flow |

### 架构入口（可读版，权威来源见 openspec/specs/）

| 文档 | 说明 |
|------|------|
| [architecture/README.md](./architecture/README.md) | 架构文档地图与 Onboarding 路径 |
| [architecture/dsaft-overview.md](./architecture/dsaft-overview.md) | DSAFT 五层编号、七域 + ORCH |
| [architecture/code-map.md](./architecture/code-map.md) | 包路径 ↔ D{N}-S{M} 对照 |
| [architecture/request-flow.md](./architecture/request-flow.md) | 用户消息端到端时序（QueryLoop） |
| [architecture/contracts-and-boundaries.md](./architecture/contracts-and-boundaries.md) | 跨层契约、依赖红线 |

---

## 演进态架构归档（openspec/specs/）

### 横切架构

| 路径 | 内容 |
|------|------|
| `openspec/specs/architecture/layering.md` | DSAFT 注册表权威来源 |
| `openspec/specs/architecture/code-atlas.md` | 机器可读模块索引 |
| `openspec/a-registry.md` | A 层索引入口 → 各域 a-registry.md |
| `openspec/f-registry.md` | F 层索引入口 → 各域 f-registry.md |
| `openspec/t-registry.md` | T 层索引入口 → 各域 t-registry.md |

### 流程规范

| 路径 | 内容 |
|------|------|
| `openspec/specs/project/master.md` | 研发流程主规范 |
| `openspec/specs/project/requirements.md` | 需求规范 |
| `openspec/specs/project/architecture-design.md` | 架构设计规范 |
| `openspec/specs/project/coding.md` | 编码规范 |
| `openspec/specs/project/testing.md` | 测试规范 |
| `openspec/specs/project/review-design.md` | 设计审查规范 |
| `openspec/specs/project/review-code.md` | 代码审查规范 |
| `openspec/specs/project/archiving.md` | 归档规范 |

### 域架构归档

| 域 | 目录 | 内容 |
|----|------|------|
| D1 Communication | `openspec/specs/d1-communication/` | spec.md · A/F/T 注册表 · 飞书任务验证 |
| D2 Context Engine | `openspec/specs/d2-context-engine/` | spec.md · A/F/T 注册表 · design.md · prompt-system |
| D3 LLM Gateway | `openspec/specs/d3-llm-gateway/` | spec.md · A/F/T 注册表 · design.md · model-resolution-trace |
| D4 Multi-Agent | `openspec/specs/d4-multi-agent/` | spec.md · A/F/T 注册表 · design.md |
| D5 Observability | `openspec/specs/d5-observability/` | spec.md · A/F/T 注册表 · design.md · coverage |
| D6 Evolution | `openspec/specs/d6-evolution/` | spec.md · A/F/T 注册表 |
| D7 Orchestration | `openspec/specs/d7-orchestration/` | spec.md · d7-domain.md · A/F/T 注册表 · task-planning |

### 跨域规范

| 路径 | 内容 |
|------|------|
| `openspec/specs/testing-framework/` | 测试框架与分层规范 |
| `openspec/specs/testing-quality/` | 测试质量门禁 |
| `openspec/specs/tool-security/` | 工具安全规范 |

---

## 新增模块流程

1. 在 `openspec/changes/{slug}/` 创建 demand → proposal → design → tasks（S1–S3）
2. 在对应域 `openspec/specs/d{N}-*/t-registry.md` 登记 T 测试点
3. 按 `docs/architecture/code-map.md` 确定包路径
4. 更新对应域的 A/F 注册表
5. S4 开发 → S5 验收 → S6 归档
