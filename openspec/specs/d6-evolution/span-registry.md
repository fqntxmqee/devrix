# D6 Evolution Span 注册表

**Domain:** D6 Evolution
**Version:** 2.1.0
**Status:** Active (2026-06-14)
**Canonical Source:** `internal/layers/observability/instrument/telemetry/names.go` · `internal/layers/observability/diagnose/coverage/registry.go`

---

## Operations

### Runtime Validation（1 op）

| Operation | Kind | Component | Canonical S | Since | Key Attributes |
|-----------|------|-----------|-------------|-------|----------------|
| `D6_S4_Validation_Decision` | INTERNAL | validation | D6-S4-A01 | 2.1.0 | decision_id, category, risk_class, session_id, agent_id |

**Readable alias:** `evolution.decision.validate`

**Span events:** `prefilter_skip`, `judge_start`, `judge_complete`, `intervention_triggered`, `intervention_exec_start`, `intervention_exec_complete`

---

## Trace Tree

```
agent.tool.call
└── D6_S4_Validation_Decision
    ├── prefilter_skip (event, optional)
    ├── judge_start → judge_complete
    └── intervention_triggered → intervention_exec_* (optional)
```

---

## 命名规范

| 场景 | 格式 | 示例 |
|------|------|------|
| Span Operation | `D6_S{scenario}_{Activity}_{Function}` | `D6_S4_Validation_Decision` |
| Readable alias | `evolution.{module}.{action}` | `evolution.decision.validate` |

---

## 关联文档

- D5 全局 Trace Tree：`openspec/specs/d5-observability/span-registry.md`
- D6 A 层活动：`openspec/specs/d6-evolution/a-registry.md`（D6-S4-A01 ValidateDecision）
