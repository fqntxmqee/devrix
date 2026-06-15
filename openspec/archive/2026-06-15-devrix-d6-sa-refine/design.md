# D6 Evolution — S 层重构 Design

**Change ID:** devrix-d6-sa-refine  
**Demand ID:** DM-20260615-002  
**阶段:** S3 Design  
**版本:** v1.0  
**状态:** Draft  
**关联:** `proposal.md`

---

## 1. 概述

### 1.1 设计目标

| 目标 | 描述 |
|------|------|
| S 切法 | 消除 PLANNED 占位 S + 解决 S4 "Orchestration" 与 D7 命名冲突 |
| Legacy 双轨 | S1–S4 冻结为 Legacy；S11–S14 Canonical |
| 零代码变更 | v1.0 仅重排注册表 + 增 canonical_s 列，不改任何 Go 文件 |
| 语义化 | 4 个 S 全部动词+名词，与 D4 S11–S16、D2 S15–S20 风格一致 |

### 1.2 版本范围

| 版本 | 范围 |
|------|------|
| v1.0 | Registry 重排 + Legacy 双轨 + canonical_s 列 |
| v2.0 | 物理路径迁移：`orchestration/` → `guard/`、`eval/` → `evaluate/`（后续 change） |

---

## 2. Decision 记录

### Decision 1: S4 "Orchestration" 重命名

| 方案 | 优点 | 缺点 |
|------|------|------|
| A: GuardRuntime | 完全区分 D7；动词+名词；覆盖 Validate+Intervene+Observe | 新词汇 |
| B: ValidateRuntime | 强调 ValidateDecision A | 忽略 Intervention + Observer |
| C: 保留 Orchestration | 零变更 | **违反 code-layout.md §2**；与 D7 持续冲突 |

**选择:** A — GuardRuntime  
**理由:** 消除 D7 命名冲突是硬需求；GuardRuntime 覆盖三个 A 的完整语义

### Decision 2: S1/S2 PLANNED 占位符处理

| 方案 | 选择 | 理由 |
|------|------|------|
| 删除 S1/S2 | 拒绝 | 架构预留仍有价值（HotReload 有独立 change `feat-config-hot-reload`） |
| 保留但重命名 | **采用** | TrackVersion + ReloadConfig 语义化；标注 PLANNED |

### Decision 3: S3 Eval 拆分

| 方案 | 选择 | 理由 |
|------|------|------|
| 拆为 RunEval + ManageDataset 两个 S | 拒绝 | 5 个 A 内聚性高（都围绕评测生命周期）；A05 ManageDataset 只有 1 个 T |
| 整体重命名为 RunEvaluation | **采用** | 动词+名词；与 D4 RunAgentLoop、D2 RunQueryLoop 对称 |

### Decision 4: S 编号

| 方案 | 选择 | 理由 |
|------|------|------|
| 重编 S1–S4 | 拒绝 | BREAKING T ID |
| 新号段 S11–S14 | **采用** | 与 D4 S11–S16 同段（Evolution 紧邻 Multi-Agent） |

---

## 3. S 层定义（Canonical）

### D6-S11: RunEvaluation

| 属性 | 值 |
|------|---|
| 承诺 | C1：给定 dataset + probes，返回 eval_report + delta + tune_suggestions |
| 消费者 | CLI (`devrix eval run`) / CI pipeline |
| 涉及 Legacy | D6-S3 Eval（全部 5 A） |

**包含 A：**

| A ID | Name | Legacy |
|------|------|--------|
| D6-S11-A01 | RunEval | S3-A01 |
| D6-S11-A02 | JudgeResult | S3-A02 |
| D6-S11-A03 | CompareDelta | S3-A03 |
| D6-S11-A04 | GenerateTune | S3-A04 |
| D6-S11-A05 | ManageDataset | S3-A05 |

### D6-S12: GuardRuntime

| 属性 | 值 |
|------|---|
| 承诺 | C2：Agent 异常行为被 Observer 捕获 → Validator 判定 → Intervention 执行 |
| 消费者 | D7 Orchestration（运行时挂钩） |
| 涉及 Legacy | D6-S4 Orchestration（全部 3 A） |

**包含 A：**

| A ID | Name | Legacy |
|------|------|--------|
| D6-S12-A01 | ValidateDecision | S4-A01 |
| D6-S12-A02 | ExecuteIntervention | S4-A02 |
| D6-S12-A03 | ObserveAgent | S4-A03 |

### D6-S13: TrackVersion（PLANNED）

| 属性 | 值 |
|------|---|
| 承诺 | C3：构建信息 → 版本报告 |
| 涉及 Legacy | D6-S1 Version |

**包含 A：**

| A ID | Name | Legacy | Status |
|------|------|--------|--------|
| D6-S13-A01 | DetectVersion | S1-A01 | PLANNED |

### D6-S14: ReloadConfig（PLANNED）

| 属性 | 值 |
|------|---|
| 承诺 | C4：监控配置文件变更 → 热加载 |
| 涉及 Legacy | D6-S2 Config |

**包含 A：**

| A ID | Name | Legacy | Status |
|------|------|--------|--------|
| D6-S14-A01 | HotReload | S2-A01 | PLANNED |

---

## 4. T 层 Legacy → Canonical 映射

| Legacy T ID | Canonical T ID | Canonical S | 备注 |
|-------------|----------------|-------------|------|
| D6-S3-A01-T01 | D6-S11-A01-T01 | S11 | EvalRun 编排 |
| D6-S3-A01-T03 | D6-S11-A01-T03 | S11 | Compression Recall Probe |
| D6-S3-A01-T04 | D6-S11-A01-T04 | S11 | Delta 报告对比 |
| D6-S3-A01-T06 | D6-S11-A01-T06 | S11 | Tool 选择准确率 |
| D6-S3-A01-T07 | D6-S11-A01-T07 | S11 | eval.enabled=false |
| D6-S3-A01-T09 | D6-S11-A01-T09 | S11 | Provider 质量对比 |
| D6-S3-A01-T10 | D6-S11-A01-T10 | S11 | Agent Fork/Join 质量 |
| D6-S3-A01-T11 | D6-S11-A01-T11 | S11 | eval run 子命令 |
| D6-S3-A01-T12 | D6-S11-A04-T01 | S11 | 调优建议生成 |
| D6-S3-A01-T13 | D6-S11-A01-T13 | S11 | 真实 Judge 接入 |
| D6-S3-A01-T14 | D6-S11-A01-T14 | S11 | CI delta gate |
| D6-S3-A01-T15 | D6-S11-A01-T15 | S11 | run-eval.sh CI |
| D6-S3-A01-T16 | D6-S11-A01-T16 | S11 | Path Regression Probe |
| D6-S3-A01-T17 | D6-S11-A01-T17 | S11 | Layer Violation Probe |
| D6-S3-A01-T18 | D6-S11-A01-T18 | S11 | Session Isolation Probe |
| D6-S3-A01-T19 | D6-S11-A01-T19 | S11 | Probe 辅助函数 |
| D6-S3-A01-T20 | D6-S11-A01-T20 | S11 | Tier Resolution Probe |
| D6-S3-A01-T21 | D6-S11-A01-T21 | S11 | Breaker Anomaly Probe |
| D6-S3-A01-T22 | D6-S11-A01-T22 | S11 | Safety Filter Latency Probe |
| D6-S3-A02-T02 | D6-S11-A02-T02 | S11 | LLM-as-Judge Cohen's kappa |
| D6-S3-A05-T01 | D6-S11-A05-T01 | S11 | Dataset 加载/抽样 |
| D6-S4-A01-T01 | D6-S12-A01-T01 | S12 | Validator + Judge + Intervention |
| D6-S1-A01-T01 | D6-S13-A01-T01 | S13 | 版本检测（PLANNED） |
| D6-S2-A01-T01 | D6-S14-A01-T01 | S14 | 配置热更新（PLANNED） |

---

## 5. Legacy Module Index（D6-S1–S4）

| S ID | Module | Status | Canonical |
|------|--------|--------|-----------|
| D6-S1 | Version | Legacy（PLANNED） | → S13 |
| D6-S2 | Config | Legacy（PLANNED） | → S14 |
| D6-S3 | Eval | Legacy | → S11 |
| D6-S4 | Orchestration | Legacy | → S12 GuardRuntime（**消除 D7 冲突**） |

---

## 6. 物理路径

| Canonical S | scenario-slug | v1.0 当前 | v2.0 目标 |
|-------------|---------------|----------|-----------|
| S11 | `evaluate` | `evolution/eval/` | `evolution/evaluate/` |
| S12 | `guard` | `evolution/orchestration/` | `evolution/guard/` |
| S13 | `version` | `evolution/version/`（PLANNED） | 保持 |
| S14 | `reload` | `evolution/config/`（PLANNED） | `evolution/reload/` |

**关键 v2.0 迁移：**

| 现行路径 | 目标路径 | 原因 |
|---------|---------|------|
| `evolution/orchestration/` | `evolution/guard/` | 消除与 D7 的命名冲突 |
| `evolution/eval/` | `evolution/evaluate/` | slug 语义化（eval→evaluate，与 RunEvaluation 一致） |

---

## 7. 统计

| 指标 | 旧 | 新 |
|------|-----|-----|
| S 数 | 4（2 IMPL + 2 PLANNED） | 4（2 IMPL + 2 PLANNED） |
| A 数 | 10（8 IMPL + 2 PLANNED） | 10（0 变更） |
| T 数 | 24 | 24（0 变更） |
| P0 | 6 | 6（保持） |
| D7 命名冲突 | 1（S4 "Orchestration"） | **0** |

---

**Revision History**

| 版本 | 日期 | 变更 |
|------|------|------|
| 0.1 | 2026-06-15 | 初稿：S11–S14 + Legacy 双轨 + T 映射 |
