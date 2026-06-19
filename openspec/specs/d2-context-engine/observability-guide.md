# D2 Context Engine — 可观测性与验收指南

**Capability:** d2-context-engine
**Status:** Active
**Version:** 2.0.0
**Last Updated:** 2026-06-19
**Parent:** `d2-domain.md` · `span-registry.md` · `t-registry.md`
**Complements:** `terminal-state-guide.md` · `../d7-orchestration/observability-guide.md`

---

## 0. 文档定位

| 本文 | SoT |
|------|-----|
| Span ↔ S15–S18 绑定 | `span-registry.md` |
| DM-020 Trace 树（Follower 侧） | `telemetry/names.go` |
| T 摘要 + P0 Runbook | `t-registry.md`（60 条，59 IMPLEMENTED） |

> **Harness span（5 ops）已随 S20 移除**；登记表保留追溯，生产路径不应出现。

---

## 1. Canonical Span ↔ S 绑定

| Operation | S | 备注 |
|-----------|---|------|
| `context.process` | S15–S17 | Prepare+Persist 聚合 |
| `context.snapshot.load` | S15 | |
| `context.compression.run` | S15 | |
| `context.longterm.recall` | S15 | |
| `context.longterm.store` | S17 | |
| `context.tools.register` | S18 | |
| `context.memory.snapshot.save` | S17 | |
| ~~`query.loop.run` / `turn`~~ | ~~S16~~ | **REMOVED** — 见 D7 `orchestration.turn.*` |
| ~~`query.loop.llm.call`~~ | ~~S16~~ | **REMOVED** — LLM span 在 D7→D3 |
| `tool.execute.single` | S18 | D7→D2 ToolRound |
| `tool.execute.permission` | S18 | |

**DM-020 规则：** 被 D7 调用时 D2 span 应带 `context.caller=d7`（见 D7 observability-guide §2.1）。

---

## 2. Trace 树

### 2.1 DM-020 Follower（挂在 D7 Turn 下）

```text
D7_Orchestration_Turn_Run
├── D2_Context_Process          (S15 Prepare)
│   ├── context.snapshot.load
│   ├── context.compression.run
│   └── context.system_prompt.load
├── D7_Orchestration_LLM_Invoke → D3   ← 非 D2
└── D7_Orchestration_Turn_Iteration
    └── D2_Tool_Execute_Single  (S18)
        └── tool.execute.permission
```

### 2.2 Legacy engine.Process（追溯）

```text
context.process
└── query.loop.run
    └── query.loop.llm.call → D3   ← 旧路径，ingress 已退役
```

---

## 3. T 层验收矩阵（按 S 摘要）

| S | 覆盖重点 | 代表 T |
|---|----------|--------|
| S15 | 压缩、Prompt、RepairToolChain | D2-S13-A01-T01 → S15 |
| S16 | ~~Thin Loop~~ → D7 RunTurn | D7-S2-A06-T09, `queryloop_removed_test.go` |
| S17 | Snapshot、Transcript | D2-S6 映射 T |
| S18 | Permission、PlanMode 拒绝、BGTask | D2-S10-A01-T37, D2-S9-A01-T16..T18 |
| 契约 | D2→D3 ban、D7 ingress | D2-THIN-T01, path_regression |

### P0 必跑清单

```bash
# QueryLoop removal guard + D7 turn path
go test ./internal/layers/contextengine/ -run 'QueryLoopRemoved|NoQueryLoop|PathRegression' -v
go test ./internal/layers/orchestration/turn/ -run RunTurn -v

# D2 thin boundary + D2→D3 import ban
go test ./internal/lint/layer/ -run 'D2Thin|D2_D3' -v

# Permission / PlanMode
go test ./internal/layers/contextengine/enforce/permission/ -v

# Compression / Prompt
go test ./internal/layers/contextengine/prepare/ -v

# Background / Subquery
go test ./internal/layers/contextengine/enforce/ -run 'Background|Subquery' -v

# 路径回归（无 harness 生产路径）
go test ./internal/layers/contextengine/ -run PathRegression -v
```

---

## 4. 生产检查清单

| 检查项 | 期望 |
|--------|------|
| Fast 路径无 `query.loop.llm.call` 在 D2 下独立出现 | LLM 应在 D7 Invoke 子树 |
| `context.caller=d7` | D7 调度时 Prepare/Tool span 必有 |
| 无 harness bootstrap span | S20 已移除 |
| PlanMode Write 拒绝 | permission 测试绿 |

---

## 5. 已知缺口

| 缺口 | 建议 |
|------|------|
| S15–S18 统一 OTel 常量名 | 与 `D2_*` 命名对齐 span-registry |
| Task/Plan span 仍在 D2 登记 | 迁 D7 workmodel span 登记 |

---

## 6. 相关文档

| 文档 | 关系 |
|------|------|
| `span-registry.md` | operation 全表 |
| `t-registry.md` | T 全表 |
| `terminal-state-guide.md` | 拆面时序 |
