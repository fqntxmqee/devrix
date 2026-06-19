# Cross-Domain Boundaries — D5 Delta (devrix-d5-v2-terminal)

**Change ID:** devrix-d5-v2-terminal  
**Demand ID:** DM-20260619-006  
**Base:** `openspec/specs/architecture/cross-domain-boundaries.md`  
**Status:** S3 Draft — 合并目标 §2.x D5 Observability

> 完整契约见 `specs/d5-boundary.md`；本文件为 cross-domain-boundaries 增补草案。

---

## MODIFIED: D5 Observability Section

原 D5 段为「调用方列表」，终态升级为 **契约表 + 禁止项**。

### 契约摘要（合并后）

| 方向 | 契约 | 违反后果 |
|------|------|----------|
| D*→D5 | 经 `Bridge` 埋点；op 用 `telemetry.Op*` | unknown op WARN；Registry 对账失败 |
| D5→外部 | OTLP / Prometheus / JSONL / coverage 目录 | 观测面不可用 |
| D2→D5 Tracker | 只读 `Recent()` | 双 SoT 诊断不一致 |
| D7→D5 | D7 创建 Turn span；D5 提供命名 | Trace 树断裂 |
| D5 禁止 | 业务编排 import；bridge 包 import | 架构违规 / 编译失败（v2.1） |

### Span 主权矩阵（新增）

| Operation 族 | 创建域 | D5 职责 |
|--------------|--------|---------|
| `gateway.message.*` | D1 | Registry + attrs |
| `orchestration.turn.*`, `orchestration.llm.invoke` | D7 | Registry + attrs |
| `context.process`, `tool.execute.*` | D2（D7 触发） | Registry + attrs |
| `llm.stream.*` | D3 | Registry + attrs |
| `agent.*` | D4 | Registry + attrs |
| ~~`query.loop.*`~~ | **REMOVED** | 文档仅 RETIRED |

### Graceful Degradation（显式登记）

- Observability 初始化失败或 disabled → 业务路径 `NewNoOp()` / nil Bridge
- Health `degraded` ≠ Process 失败

---

## 修订记录

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-19 | D5 delta for v2.1 terminal |
