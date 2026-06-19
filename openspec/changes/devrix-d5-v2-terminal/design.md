# Design: D5 Observability v2.1 终态重构

**Change ID:** devrix-d5-v2-terminal  
**Demand ID:** DM-20260619-006  
**Status:** S3_Draft  
**Methodology:** DSAFT Refactoring Playbook §6 双锚点对齐

---

## 1. 架构目标

在 **不改变 North Star、不破坏 T ID 契约** 的前提下，完成 D5 终态闭合：

1. **规格锚点** — `spec.md` / `d5-domain.md` 以 S21–S24 为 Canonical 主叙事
2. **物理锚点** — 删除 bridge；根目录文件归位 scenario 目录
3. **语义锚点** — S23 子承诺 C3a–C3e 写入 A 层；错位 T↔A 用 canonical 列校正
4. **跨域锚点** — `d5-boundary.md` 对称 D2/D7 boundary 文档

---

## 2. Decision 记录

### Decision: 重构范围（Q1）

| 方案 | 优点 | 缺点 |
|------|------|------|
| A: 仅文档 | 零风险 | bridge 与 S23 语义债残留 |
| B: 文档 + S23 注册表 | 闭合语义 | bridge 仍阻碍 v2.0 宣告完成 |
| **C: 文档 + 语义 + bridge 清债** | 一次闭合终态 | 2 PR；需全量测试 |

**选择:** C  
**理由:** v2.0 物理迁移已完成；剩余债务均为「半终态」问题；D6 已证明 bridge 可安全删除  
**影响:** Phase A docs + Phase B code；不重做 git mv

### Decision: S23 不增 S25（Q2）

| 方案 | 优点 | 缺点 |
|------|------|------|
| A: 新增 S25 TerminalTools | 目录 1:1 | BREAKING S 清单；与 D7「T 不变」冲突 |
| **B: S23 子承诺 C3a–C3e** | T ID 不变；承诺可验收 | S23 A 层条目增多 |

**选择:** B  
**理由:** Tracker/Doctor/FaultInject 均属「帮用户排查问题」同一价值流；Playbook 原则 2 用 A 编排差异  
**影响:** a-registry S23 扩至 A01–A10；承诺 C3 文档化子表

### Decision: DebugFilter → S21（Q3）

| 方案 | 优点 | 缺点 |
|------|------|------|
| **A: S21-A14 FilterDebugLog** | 与物理路径 `instrument/logger/debugfilter/` 一致 | T ID D5-S23-A08 需 canonical_s 改 S21 |
| B: 留 S23 | T 与 A 同 S | 物理在 Instrument 包，双锚点断裂 |

**选择:** A  
**理由:** DebugFilter 是日志管道过滤，不是独立诊断报告；与 LogRecord 同包  
**影响:** 新增 D5-S21-A14；`t-registry` D5-S23-A08-T* canonical_s → S21

### Decision: SessionBridge → S0（Q4）

| 方案 | 优点 | 缺点 |
|------|------|------|
| **A: S0-A03 TrackActiveSessions** | Bridge 横切语义正确 | T canonical_s 改 S0 |
| B: 留 S23 | 无 T 表改动 | Health/Session 语义混杂 |

**选择:** A  
**理由:** `SessionBridge` 在 `bridge.go`，与 CreateBridge 同属 Facade 集成面  
**影响:** 新增 D5-S0-A03；D5-S23-A06-T01 canonical_s → S0

### Decision: bridge 本轮删除（Q5）

| 方案 | 优点 | 缺点 |
|------|------|------|
| A: 再保留 1 release | 外部兼容 | Facade 内部仍走 Deprecated；v2.0 永不闭环 |
| **B: 本轮删除** | 对齐 D6 v2.0.1；import 单一 | 需 grep 确认无外部 bridge 依赖 |

**选择:** B  
**理由:** grep 显示 bridge import 仅 D5 内部 5 处；外部已用 `instrument/*`  
**影响:** 删除 9 目录 bridge.go；更新 `internal/layers/observability/bridge.go`

### Decision: T 层策略

**选择:** T ID **不变**；校正 `canonical_s` / 增 `canonical_a` 列  
**理由:** 39 IMPLEMENTED T 是硬约束（Playbook 原则 3）  
**影响:** Doctor T（D5-S23-A03-T01/T02）canonical_a → A10 RunDoctorChecks

### Decision: Span 运行时命名规范（Q7）

| 方案 | 优点 | 缺点 |
|------|------|------|
| A: `D{N}_S{N}_{场景}_{动作}_{细节}` | DSAFT S 编号显式可见 | S 编号与场景语义名重复（`D1_S13_Capture_...` → `S13` = `Capture`）；Jaeger 中冗余 |
| **B: `D{N}_{场景}_{动作}_{细节}`** | 简洁；场景名已唯一标识 S；一眼可读 | Go 常量名需保留 S 编号用于 DSAFT 追溯 |

**选择:** B  
**理由:** 场景语义名（`Capture`/`Orchestration`/`Context`）本身已唯一映射到 S 层，运行时字符串无需重复编号；Jaeger 展示更清晰  
**影响:** `names.go` 中 Go 常量名格式不变（`OpD1_S13_Capture_Message_Receive`），运行时字符串值采用 `D1_Capture_Message_Receive`；`spans-registry.md` 同步更新格式描述

### Decision: 主路径 Trace 树 SoT

**选择:** D7 Turn span 族为 Canonical；`query.loop.*` 仅 RETIRED 登记  
**理由:** DM-20260618-010 代码已删 QueryLoop span  
**影响:** spec/design/span-registry/coverage 统一刷新

---

## 3. North Star 与 Out of Scope

### North Star

> **作为全链路可观测性基础设施（裁判域），为任意域操作提供可追踪、可度量、可诊断的遥测数据，并保证 Jaeger / Prometheus / Health / Incident Export 可交叉验证。**

### 可验证承诺 ↔ S 映射

| 承诺 | S | E2E 验收证据 |
|------|---|-------------|
| C1 遥测生成 | S21 | `tracer_test` + `names_test` + obs integration trace propagation |
| C2 遥测导出 | S22 | OTLP event test + orchestrator span test |
| C3 诊断辅助 | S23 | coverage_test + doctor_test + tracker_test + export_test |
| C4 配置管理 | S24 | runtime_metric_test + config validate |
| C0 Facade | S0 | bootstrap observability_test + SessionBridge integration |

### Out of Scope

| 能力 | 归属 |
|------|------|
| Turn 主循环 / LLM 调用编排 | D7 |
| 工具执行 / Prepare | D2 |
| IM ingress / 用户展示 | D1 |
| LLM 路由 / 熔断 | D3 |
| Agent 生命周期 | D4 |
| 评测 / Guard 决策 | D6 |

---

## 4. 目标目录树（终态）

```text
internal/layers/observability/
├── observability.go          # D5-S0-A01 InitObservability
├── bridge.go                 # D5-S0-A02 CreateBridge + A03 SessionBridge
├── config.go / load.go       # D5-S24（配置加载入口）
├── health.go                 # D5-S23-A06 HealthCheck
├── bench_test.go
│
├── instrument/               # D5-S21 Instrument
│   ├── tracer/
│   ├── metrics/
│   │   └── genai_tokens.go   # 自根目录迁入
│   ├── logger/
│   │   ├── slog_bridge.go
│   │   └── debugfilter/      # FilterDebugLog (A14)
│   └── telemetry/
│       └── names.go
│
├── export/                   # D5-S22 Export
│   ├── console.go / otlp.go / memory.go / factory.go
│
├── diagnose/                 # D5-S23 Diagnose
│   ├── coverage/             # C3a
│   ├── incident/
│   │   └── llm_log.go        # 自根目录迁入（或 incident/llm_log.go）
│   ├── doctor/               # C3c
│   ├── tracker/              # C3d
│   └── faultinject/          # C3e
│
└── configure/                # D5-S24 Configure
    ├── settings/
    └── runtime/

# 删除（v2.1）:
# tracer/ metrics/ logger/ exporter/ coverage/ settings/ runtime/
# telemetry/ incident/  — 各仅含 bridge.go
# 根目录: genai_tokens.go llm_log.go slog_bridge.go（迁入或合并后删除）
```

---

## 5. S23 子承诺与 A 层终态

### C3 总承诺

**提供诊断辅助：Coverage 对账、Incident 导出、运行时健康检查、文件变更追踪、测试故障注入。**

### A 层登记表（v4.0 增量）

#### D5-S21 Instrument（新增）

| A ID | Name | Code Location | 备注 |
|------|------|---------------|------|
| D5-S21-A14 | FilterDebugLog | `instrument/logger/debugfilter/filter.go` | 自 S23-A08 语义迁入 |

#### D5-S0 Facade（新增）

| A ID | Name | Code Location | 备注 |
|------|------|---------------|------|
| D5-S0-A03 | TrackActiveSessions | `bridge.go` (SessionBridge) | 自 S23-A06-T 语义迁入 |

#### D5-S23 Diagnose（扩展）

| A ID | Name | 子承诺 | Code Location | Legacy T 锚点 |
|------|------|--------|---------------|---------------|
| D5-S23-A01 | RecordOperationHit | C3a | `diagnose/coverage/` + `instrument/tracer/` | — |
| D5-S23-A02 | AssessCoverage | C3a | `diagnose/coverage/coverage.go` | — |
| D5-S23-A03 | GenerateDailyReport | C3a | `diagnose/coverage/reporter.go` | — |
| D5-S23-A04 | ExportSessionBundle | C3b | `diagnose/incident/export.go` | — |
| D5-S23-A05 | RecordLLMPayload | C3b | `diagnose/incident/llm_log.go` | — |
| D5-S23-A06 | HealthCheck | C3c | `health.go`, `observability.go` | — |
| D5-S23-A07 | TrackFileDiagnostics | C3d | `diagnose/tracker/tracker.go` | D5-S23-A07-T* |
| D5-S23-A08 | ~~FilterDebugLog~~ | — | **RETIRED → S21-A14** | D5-S23-A08-T* canonical_s=S21 |
| D5-S23-A09 | InjectFault | C3e | `diagnose/faultinject/` | D5-S23-A09-T* |
| D5-S23-A10 | RunDoctorChecks | C3c | `diagnose/doctor/doctor.go` | D5-S23-A03-T* canonical_a=A10 |

> **T↔A 错位说明:** `D5-S23-A03-T01/T02`（Doctor）创建时 A03 已被 GenerateDailyReport 占用。终态方案：**T ID 冻结**，`canonical_a` 列指向 A10，不在 T ID 中改号。

### Statistics（终态目标）

| Scenarios | Activities | F | T |
|-----------|------------|---|---|
| S0 + S21–S24 | 30 (+2 新增) | ~45 (+诊断 F) | 41 (2 PLANNED 闭合) |

---

## 6. Canonical Trace 树（SoT）

主路径（2026-06-18 起）：

```text
gateway.message.receive                    [SERVER, D1]
└── orchestration.turn.run               [INTERNAL, D7]
    └── orchestration.turn.iteration     [per iteration]
        ├── orchestration.llm.invoke     [CLIENT, D7]
        │   └── llm.stream               [CLIENT, D3]
        │       └── llm.adapter.stream   [CLIENT, D3]
        └── tool.execute.single          [INTERNAL, D7→D2]
            └── context.process          [INTERNAL, D2 Prepare caller=d7]
```

条件路径：

- `context.compression.run` + `context.compression.step.*`（D2 压缩）
- `orchestration.wave.*` / `orchestration.flow.*`（D7 Wave）
- `agent.run` / `agent.tool.call`（D4）
- `context.harness.*`（仅 harness.enabled，Legacy）

**RETIRED:** `query.loop.*`, `context.pev.*`

---

## 7. 跨域契约（d5-boundary.md 摘要）

| 方向 | 契约 | D5 提供 | 对端义务 |
|------|------|---------|----------|
| D*→D5 | Bridge 注入 | `observability.Bridge` | 通过 Bridge 创建 span/metric/log；禁止直接 new Tracer |
| D*→D5 | Operation 命名 | `telemetry.Op*` 常量 | Span name 必须使用 Registry 内 canonical op |
| D5→D* | RecordHit | `Tracer.Start` 无条件计数 | 所有 instrumented path 必须调 Start |
| D2→D5 | Tracker 只读 | `diagnose/tracker.Recent()` | D2 TrackerSurface 只读；写入仅在 D5 |
| D1→D5 | Root span | Gateway 创建 `gateway.message.receive` | 传播 W3C + Baggage |
| D7→D5 | Turn spans | `orchestration.turn.*` | D7 负责创建 Turn 族 span |
| CLI→D5 | Incident export | `devrix debug export` | schema v1 bundle |

---

## 8. 文档产物规格

### 8.1 d5-domain.md（新建）

结构对齐 `d6-domain.md`：

- Metadata（Depends On / Depended By）
- North Star + 承诺表
- Out of Scope
- DSAFT 双轨（Canonical S21–S24 + Legacy S1–S9）
- 物理路径映射表
- 跨域契约摘要
- v2.0 / v2.1 迁移记录

### 8.2 observability-guide.md（新建）

结构对齐 `d7-orchestration/observability-guide.md`：

- §0 文档定位与 SoT 分工
- §1 Span↔T P0 绑定矩阵（按 S21–S24 分组）
- §2 Canonical Trace 树（D7 Turn）
- §3 按 S 分组的 T 验收摘要
- §4 P0 Runbook（Health zero_hit / debug export / Jaeger 检查清单）
- §5 与 D7 observability-guide 交叉引用

### 8.3 spec.md v3.0 变更要点

- Version 3.0.0；Last Updated 2026-06-19
- DSAFT 结构表 → S21–S24 + S0
- Scenarios 表 → Canonical
- Overview 主路径 → D7 Turn
- Legacy S1–S9 → 独立「冻结追溯」节
- Requirements 中 QueryLoop → REMOVED 标记

### 8.4 f-registry v3.0

- 增 `canonical_s` 列（与 t-registry 同模式）
- Code Location 更新为 `instrument/` 等真实路径
- 新增诊断 F 点（Tracker diff/LRU/linter、Doctor checks、FaultInject hook）

---

## 9. bridge 删除清单

| 删除路径 | 替代 import |
|----------|-------------|
| `observability/tracer/` | `observability/instrument/tracer` |
| `observability/metrics/` | `observability/instrument/metrics` |
| `observability/logger/` | `observability/instrument/logger` |
| `observability/telemetry/` | `observability/instrument/telemetry` |
| `observability/exporter/` | `observability/export` |
| `observability/coverage/` | `observability/diagnose/coverage` |
| `observability/incident/` | `observability/diagnose/incident` |
| `observability/settings/` | `observability/configure/settings` |
| `observability/runtime/` | `observability/configure/runtime` |

**内部需改文件（已 grep 确认）:**

- `observability/bridge.go`
- `observability/slog_bridge.go`
- `instrument/tracer/tracer_coverage_test.go`
- `instrument/telemetry/*_test.go`

---

## 10. Phase 划分与 PR 策略

| Phase | 范围 | PR | Gate |
|-------|------|-----|------|
| A | 规格终态（docs-only） | `docs/d5-v2-terminal-spec` | S3-Gate 文档一致性 |
| B | bridge 删除 + 根目录归位 | `chore/d5-bridge-cleanup` | 41 T + integration 全绿 |
| C | S7 归档回写 | archive PR | demand-archive-index |

**不建议合并 A+B：** docs PR 可并行 review；code PR 依赖 A 定稿的 canonical 路径。

---

## 11. PLANNED T 闭合计划

| T ID | 现状 | 终态动作 |
|------|------|----------|
| D5-S21-A05-T01 | PLANNED | 补 `instrument/tracer` 传播集成测试或标 IMPLEMENTED（若已有 obs_trace_propagation_test 覆盖） |
| D5-S21-A05-T02 | PLANNED | 补 Counter 单元测试或合并到 meter_test |
| D5-S23-A06-T02 | PLANNED | HealthCheck coverage 摘要 — 验证 `observability.HealthCheck` 已暴露 coverage 后标 IMPLEMENTED |

---

## 12. Grill Review 预判

| 挑战 | 回应 |
|------|------|
| S23 是否过大 | C3a–C3e 子承诺可独立验收；不增 S 号避免 T 震荡 |
| 为何不等 v2.2 删 bridge | D6 已删；内部-only import；拖延增加双路径维护成本 |
| Doctor T 挂 A03 | 历史债务；canonical_a 列校正，不改 T ID |
| D2 也有 tracker 目录 | D2 `toolrunner/tracker` 已收敛；仅 TrackerSurface 读 D5 |

---

## 修订记录

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-19 | 初稿：DM-20260619-006 最优终态方案 |

---

## 13. 回归风险评估

| 变更 | 风险 | 检测 |
|------|------|------|
| 删除 bridge 包 | 中 — 遗漏 import | `grep observability/tracer` + 全量 `go test` |
| 根目录 git mv | 低 — import 路径 | `go build ./...` |
| 仅文档 Phase A | 无运行时风险 | `grep query.loop` 文档 AC |
| T canonical 列校正 | 无测试行为变更 | t-registry review |

## 14. 回滚计划

| Phase | 回滚 |
|-------|------|
| Phase A | revert docs PR |
| Phase B1 | revert git mv + imports |
| Phase B2 | restore bridge 目录（git revert） |
| Phase B3 | revert t-registry status only |

不可逆点：bridge 删除后外部若新增 bridge import 需改 canonical — 当前无外部依赖。
