# Devrix 架构文档（DSAFT）

本目录是 **Devrix 现行架构** 的可读文档入口，与 OpenSpec 规格互为补充：

| 类型 | 权威来源 | 本文档角色 |
|------|----------|------------|
| 分层编号、目录映射 | `openspec/specs/architecture/layering.md` | 提炼 + 结合代码解读 |
| 代码落盘索引 | `openspec/specs/architecture/code-atlas.md` | 人类可读版 `code-map.md` |
| A/F/T 注册表 | `openspec/a-registry.md` / `f-registry.md` / `t-registry.md` | 引用，不重复维护 |
| 验收规格 | `openspec/specs/*/spec.md` | Gherkin Scenario → T 层 |

**最后同步代码基线：** 2026-06-13（QueryLoop 生产路径、PEV 退役、toolrunner 子包、契约分层）

---

## 文档地图

| 文档 | 内容 |
|------|------|
| [dsaft-overview.md](./dsaft-overview.md) | DSAFT 五层编号、七域 + ORCH、Scenario 全景 |
| [code-map.md](./code-map.md) | `internal/layers/` 包路径 ↔ D{N}-S{M} 对照表 |
| [request-flow.md](./request-flow.md) | 用户消息 → Gateway → Process → QueryLoop → LLM 端到端时序 |
| [contracts-and-boundaries.md](./contracts-and-boundaries.md) | 跨层契约、依赖方向、禁止 import 规则 |

---

## 域级详细设计（六段式）

遵循 `docs/detail design framework.md` 的模块级深描，部分章节为历史版本（PEV 时代），阅读时注意文首状态标记。

| 域 | 文档 | 状态 |
|----|------|------|
| D2 Context Engine | [../context-engine-design.md](../context-engine-design.md) | **Historical** — 现行路径见 [request-flow.md](./request-flow.md) |
| D3 LLM Gateway | [../llm-gateway-design.md](../llm-gateway-design.md) | Archived S7，层边界仍有效 |
| D4 Multi-Agent | [../multi-agent-design.md](../multi-agent-design.md) | Design Review，Delegate/Worker 见 code-map |
| D5 Observability | [../observability-design.md](../observability-design.md) | 分析文档，span 族以 telemetry/names.go 为准 |

---

## 专题文档

| 文档 | 说明 |
|------|------|
| [../config.md](../config.md) | 配置项与热重载 |
| [../prompt-system-design.md](../prompt-system-design.md) | System Prompt 与 Harness 装配 |
| [../task-planning-design.md](../task-planning-design.md) | Task 磁盘工具与 plan mode |
| [../llm-gateway-model-resolution-trace.md](../llm-gateway-model-resolution-trace.md) | 模型/Tier 解析链路 |
| [../coverage.md](../coverage.md) | Operation 级代码染色 |
| [../development-workflow.md](../development-workflow.md) | OpenSpec S1–S7 + GitHub Flow |

---

## 阅读顺序（Onboarding）

1. [dsaft-overview.md](./dsaft-overview.md) — 建立 D-S-A-F-T 词汇
2. [request-flow.md](./request-flow.md) — 理解一次用户消息的完整路径
3. [code-map.md](./code-map.md) — 改代码前先查 D-S 归属
4. [contracts-and-boundaries.md](./contracts-and-boundaries.md) — 跨层改动时的红线
5. 按需深入域级 `*-design.md` 或 OpenSpec `spec.md`
