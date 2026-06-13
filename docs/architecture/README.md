# Devrix 架构文档（DSAFT）

本目录是 **Devrix 现行架构** 的可读文档入口，与 OpenSpec 规格互为补充：

| 类型 | 权威来源 | 本文档角色 |
|------|----------|------------|
| 分层编号、目录映射 | `openspec/specs/architecture/layering.md` | 提炼 + 结合代码解读 |
| 代码落盘索引 | `openspec/specs/architecture/code-atlas.md` | 人类可读版 `code-map.md` |
| A/F/T 注册表 | `openspec/specs/d{N}-*/{a,f,t}-registry.md` | 引用，不重复维护 |
| 验收规格 | `openspec/specs/d{N}-*/spec.md` | Gherkin Scenario → T 层 |

**最后同步代码基线：** 2026-06-13（QueryLoop 生产路径、PEV 退役、D7 编排域）

---

## 文档地图

| 文档 | 内容 |
|------|------|
| [dsaft-overview.md](./dsaft-overview.md) | DSAFT 五层编号、七域 + ORCH、Scenario 全景 |
| [code-map.md](./code-map.md) | `internal/layers/` 包路径 ↔ D{N}-S{M} 对照表 |
| [request-flow.md](./request-flow.md) | 用户消息 → Gateway → Process → QueryLoop → LLM 端到端时序 |
| [contracts-and-boundaries.md](./contracts-and-boundaries.md) | 跨层契约、依赖方向、禁止 import 规则 |

---

## 域级架构归档

域级详细设计、A/F/T 注册表已迁移至 `openspec/specs/d{N}-*/`：

| 域 | 路径 |
|----|------|
| D1 Communication | `openspec/specs/d1-communication/` |
| D2 Context Engine | `openspec/specs/d2-context-engine/` |
| D3 LLM Gateway | `openspec/specs/d3-llm-gateway/` |
| D4 Multi-Agent | `openspec/specs/d4-multi-agent/` |
| D5 Observability | `openspec/specs/d5-observability/` |
| D6 Evolution | `openspec/specs/d6-evolution/` |
| D7 Orchestration | `openspec/specs/d7-orchestration/` |

---

## 专题文档（稳定态）

| 文档 | 说明 |
|------|------|
| [../config.md](../config.md) | 配置项与热重载 |
| [../development-workflow.md](../development-workflow.md) | OpenSpec S1–S7 + GitHub Flow |

---

## 阅读顺序（Onboarding）

1. [dsaft-overview.md](./dsaft-overview.md) — 建立 D-S-A-F-T 词汇
2. [request-flow.md](./request-flow.md) — 理解一次用户消息的完整路径
3. [code-map.md](./code-map.md) — 改代码前先查 D-S 归属
4. [contracts-and-boundaries.md](./contracts-and-boundaries.md) — 跨层改动时的红线
5. 按需深入 `openspec/specs/d{N}-*/` 的域 spec 和注册表
