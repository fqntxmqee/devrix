# D5 Observability — Cross-Domain Boundary

**Capability:** observability
**Status:** S3 Design Draft
**Version:** 1.0.0
**Last Updated:** 2026-06-19
**Parent:** `d5-domain.md`
**Symmetric:** `../d2-context-engine/d7-boundary.md`（D7 侧视角）

> S7 归档后迁入 `openspec/specs/d5-observability/d5-boundary.md`。

---

## 1. 边界原则

| 原则 | 说明 |
|------|------|
| D5 是裁判，不是玩家 | D5 不编排 Turn、不执行 Tool、不路由 LLM |
| Bridge 零侵入 | 各域注入 `*observability.Bridge`；禁止 `new Tracer` |
| Operation 命名统一 | Span name = Registry canonical op = `telemetry.Op*` |
| Coverage 独立于采样 | `Tracer.Start` 无条件 `RecordHit` |
| 诊断写主权在 D5 | Tracker LRU 写入仅 D5/D2 约定钩子；D2 Surface 只读 |

---

## 2. 契约表

### 2.1 各域 → D5（消费可观测能力）

| 对端 | 契约入口 | D5 提供 | 对端义务 | T 锚点 |
|------|----------|---------|----------|--------|
| D1 | `capture/gateway.go` | Root span `gateway.message.receive` | W3C inject + Baggage `session.id` | D5-S22-A01-T03 |
| D2 | `observability.Bridge` | Tracer/Meter/Logger | Prepare/Tool span 用 `telemetry.Op*` | D5-S21-A01 |
| D2 | `diagnose/tracker` | Tracker LRU/Diff | TrackerSurface **只读** `Recent()` | D5-S23-A07-T01 |
| D3 | Bridge + gateway | `llm.stream` 子树 | adapter 继承 trace_id | D5-S22-A01-T03 |
| D4 | Bridge | `agent.*` spans | Fork policy metrics | D5-S21-A05 |
| D6 | OpenTelemetry sink | Counter/Histogram 写入 | 不高基数 label | D5-S21-A05 |
| D7 | Bridge + turn tracing | Turn span 族由 **D7 创建** | 使用 D5 Op 常量 | D5-S22-A01-T02 |
| Bootstrap | `bootstrap/observability.go` | `observability.New` | 加载 yaml config | D5-S24-A01 |
| CLI | `devrix debug export` | Incident bundle schema v1 | — | D5-S23-A04 |

### 2.2 D5 → 外部系统（导出）

| 目标 | 协议 | D5 场景 | 配置键 |
|------|------|---------|--------|
| Jaeger / OTLP Collector | OTLP HTTP | S22 Export | `observability.tracing.otlp` |
| Prometheus | `/metrics` scrape | S21 + S22 | `observability.metrics` |
| 本地 JSONL | LLM log dir | S23 C3b | `observability.llm.log_dir` |
| ~/.devrix/coverage/ | Daily JSON | S23 C3a | coverage persistence |

---

## 3. D2 ↔ D5 Tracker 专项边界

```
D2 TrackerSurface (enforce/toolrunner/surface/tracker_surface.go)
    │  read-only: tracker.Recent(), formatting for LLM
    ▼
D5 diagnose/tracker (LRU + Diff + Async Linter)
    │  write: SnapshotBefore, OnEditComplete (from tool specs / hooks)
    ▼
  下一回合 system reminder（非阻塞）
```

| 规则 | 说明 |
|------|------|
| D2 禁止复制 Tracker 实现 | 不得再建 `toolrunner/tracker` 写模型 SoT |
| D5 Tracker 不调用 D2 | 无 import contextengine |
| Linter 异步 | 主路径非阻塞（W7 T） |

---

## 4. D7 ↔ D5 Turn Span 边界

**Canonical 主路径（2026-06-18 起）：**

```text
D1 gateway.message.receive
└── D7 orchestration.turn.run          ← D7 创建
    └── orchestration.turn.iteration
        ├── orchestration.llm.invoke   ← D7 创建
        │   └── D3 llm.stream          ← D3 创建
        └── tool.execute.single        ← D7 触发
            └── D2 context.process   ← D2 创建 (caller=d7)
```

| 责任 | 域 |
|------|-----|
| Turn / LLM invoke span 创建 | D7 |
| `llm.stream` 子树 | D3 |
| `context.process` Prepare | D2 |
| Op 常量 + layer/component 属性 | D5 `instrument/telemetry/names.go` |
| Registry 对账 | D5 `diagnose/coverage/` |

**RETIRED:** D2 `query.loop.*` span — 不得再创建。

---

## 5. 禁止依赖

| 禁止 | 原因 |
|------|------|
| D5 → D2/D7 业务包 | 公共域不得依赖核心域实现 |
| D5 → D3 gateway 直接调 LLM | 仅 metrics/token 辅助 |
| 外部 import `observability/tracer` bridge | v2.1 删除；用 `instrument/tracer` |
| session_id 作为 metric label | 基数爆炸（blocklist） |

---

## 6. 修订记录

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-19 | 初稿：devrix-d5-v2-terminal S3 |
