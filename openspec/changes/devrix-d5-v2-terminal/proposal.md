# Proposal: D5 Observability v2.1 终态重构

**Change ID:** devrix-d5-v2-terminal  
**Demand ID:** DM-20260619-006  
**Status:** S3_Design  
**Methodology:** `docs/methodology/dsaft-refactoring-playbook.md` §3–§6

---

## 1. Background

D5 是 Devrix **横向裁判域**（Tracing / Metrics / Logging / Coverage / Diagnose），所有 D1–D7 依赖其 Bridge 与 Operation Registry。2026-06-15 完成价值流切法（S21–S24）与物理迁移后，**规格与代码进入「半终态」**：

- 代码已在 `instrument/` `export/` `diagnose/` `configure/`
- 规格仍以 `spec.md` 的 D5-S1–S9 为主叙事
- 9 个 Deprecated bridge 包阻碍 v2.0 Structure 宣告完成
- 诊断工具（DM-20260616~18）使 S23 语义超出原承诺 C3，但未扩 A/F

D7 在 DM-20260619-005 Demonstrated **终态闭合模式**：文档栈 + 物理对齐 + shim 删除。本 change 将 D5 对齐同一终态标准，**不重做已完成的 git mv**。

---

## 2. Problem Statement（摘要）

详见 `demand.md` P1–P6。核心根因：

> **v1.0 Registry 与 v2.0 Structure 已落地，但 v2.1 Terminal（规格 SoT + 语义闭合 + bridge 清债）被跳过。**

叠加诊断工具「实现先于注册表」，S23 从「Coverage + Incident + Health」膨胀为「诊断全家桶」，违背 Playbook 原则 4（不让 S 层膨胀为万能场景）——但通过 **子承诺 C3a–C3e** 可在不增 S 号的前提下闭合。

---

## 3. Proposed Solution

### 3.1 North Star（领域根本目标）

**为全链路请求提供可追踪、可度量、可诊断的遥测基础设施，使任意域的操作在 Jaeger / Prometheus / Health / Incident Export 中可交叉验证。**

| 可验证承诺 | Canonical S | 消费者可验证 WHAT |
|-----------|-------------|-------------------|
| C1 遥测生成 | D5-S21 Instrument | 任意 operation 可创建 Span、记录 Metric、写入 Log，属性含 layer/component |
| C2 遥测导出 | D5-S22 Export | Span 到达 OTLP/Console；Metric 到达 Prometheus |
| C3 诊断辅助 | D5-S23 Diagnose | Coverage 对账、Incident bundle、Doctor、Tracker、FaultInject（测试） |
| C4 配置管理 | D5-S24 Configure | yaml 切换 exporter/采样；runtime path 计数准确 |
| C0 横切 Facade | D5-S0 | Init/Shutdown/Bridge/SessionGauge 零侵入集成 |

### 3.2 S 层 — 不变（Canonical 冻结）

```
D5 Observability
├── D5-S21 Instrument     # C1
├── D5-S22 Export         # C2
├── D5-S23 Diagnose       # C3（子承诺 C3a–C3e，见 design.md §5）
├── D5-S24 Configure      # C4
└── D5-S0 Facade          # C0
```

**不新增 S25。** 诊断工具归入 S23 子承诺，DebugFilter 归入 S21。

### 3.3 S23 子承诺（语义闭合，不增 S 号）

| 子承诺 | 范围 | 物理路径 |
|--------|------|----------|
| C3a Coverage 对账 | Registry + Hit + DailyReport | `diagnose/coverage/` |
| C3b Incident 导出 | Bundle + LLM JSONL | `diagnose/incident/` + `llm_log` |
| C3c 运行时健康 | HealthCheck + Doctor | `health.go` + `diagnose/doctor/` |
| C3d 文件诊断追踪 | Tracker LRU/Diff/Linter | `diagnose/tracker/` |
| C3e 测试故障注入 | FaultInject testbuild | `diagnose/faultinject/` |

### 3.4 文档栈（对标 D7）

| 新建/刷新 | 用途 |
|-----------|------|
| `d5-domain.md` | 领域 SoT（对齐 d2/d6/d7-domain.md） |
| `spec.md` v3.0 | Gherkin Requirements，Canonical S 主表 |
| `observability-guide.md` | Span↔T + D7 Turn Trace + P0 Runbook |
| `terminal-state-guide.md` | 终态叠合 + 文档索引 |
| `dsaft-architecture.md` | 五层计数 Stub |
| `d5-boundary.md` | D5↔D* 契约表 |

### 3.5 代码清债

1. 删除 9 bridge 包；Facade 直连 `instrument/*`
2. 根目录归位：`genai_tokens` → S21；`llm_log` → S23
3. `slog_bridge.go` 改 import `instrument/logger`（删除对 legacy logger bridge 的依赖）

---

## 4. Out of Scope

- 新业务可观测能力
- Operation 56 条变更
- D2/D7 代码修改
- S 号段重编

---

## 5. 风险与缓解

| 风险 | 缓解 |
|------|------|
| bridge 删除破坏外部 import | grep 全仓；bridge 仅 D5 内部残留 |
| T ID 与 A 编号错位 | T ID 冻结；`canonical_s` + `canonical_a` 列校正 |
| PR 面宽 | Phase A docs-only PR 先行，Phase B 代码 PR 跟进 |
| 文档与 D7 span 不一致 | observability-guide 引用 `span-registry.md` + D7 guide 交叉链接 |

---

## 6. 成功标准

- [ ] `openspec/specs/d5-observability/` 6 产物齐全，spec v3.0 Canonical 主叙事
- [ ] 0 个 `observability/tracer` 等 bridge import（全仓）
- [ ] 41 T 验收矩阵闭合；P0 Runbook 可执行
- [ ] S7 归档回写 + `demand-archive-index.md` 登记

---

## 7. 参考

| 文档 | 用途 |
|------|------|
| `openspec/archive/2026-06-15-devrix-d5-sa-refine/` | v1.0 Registry 决策 |
| `openspec/archive/2026-06-15-devrix-d5-d6-sa-refine-v2.0/` | v2.0 物理迁移 |
| `openspec/archive/2026-06-19-devrix-d7-v2-structure/` | 终态闭合参考模式 |
| `openspec/specs/d6-evolution/d6-domain.md` | d5-domain 结构模板 |
