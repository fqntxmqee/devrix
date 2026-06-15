# Proposal: D6 Evolution S/A 重切 — 消除占位 S + 解决命名冲突

**Change ID:** devrix-d6-sa-refine  
**Demand ID:** DM-20260615-002  
**Status:** S2_Proposal  
**Phase Scope:** D + S（A/F 编排在 design.md；本文件含 S 层切法论证）

---

## 1. Background

D6 Evolution 自 Pilot（2026-06-10）至 Phase 4（2026-06-10）共迭代 5 个 change、24 条 T（19 IMPLEMENTED + 5 PLANNED，P0 6 条），已形成 Eval 引擎 + Runtime Guard 双能力。但 D6 的 S 层存在三个结构性问题：

| 域 | 价值流 S 数 | 价值流化状态 |
|----|------------|------------|
| D1 Communication | 6 (S13–S18) | ✅ v2.0 |
| D2 Context Engine | 6 (S15–S20) | ✅ v2.0 |
| D3 LLM Gateway | 6 (S1–S6) | ✅ v1.0 Registry |
| D4 Multi-Agent | 6 (S11–S16) | ✅ v1.0 Registry |
| D5 Observability | 0/9 | ❌ 并行 change |
| **D6 Evolution** | **0 / 4** | ❌ **本 change** |
| D7 Orchestration | 5 (S1–S5) | ✅ v1.0 |

本 change 延续 SA Refine 模式，修复 D6 S 层的三个结构性问题。

---

## 2. Problem Statement

### 2.1 S1/S2 是 PLANNED 占位符

D6-S1 Version（DetectVersion）和 D6-S2 Config（HotReload）各只有 1 个 PLANNED A + 1 个 PLANNED T，**无任何实现代码**。它们是早期架构预留的占位 S，但在 v1.0 Registry 中作为独立 Scenario 存在，稀释了 D6 的实际能力表达。

### 2.2 S4 "Orchestration" 与 D7 命名冲突

D6-S4 名为 "Orchestration"，但 D7 的全称就是 "Orchestration Domain"。两个域共用同一词汇，含义不同：
- D7 Orchestration = Session 编排、Turn 状态机、Wave 调度
- D6-S4 Orchestration = Runtime Validation + Intervention + Agent Observation

`code-layout.md §2` 明确要求 L2 scenario-slug **跨域唯一语义化**。当前冲突违反该规则。

### 2.3 S3 Eval 承载过重

D6-S3 Eval 包含 5 个 A（RunEval / JudgeResult / CompareDelta / GenerateTune / ManageDataset）+ 22 条 T（17 IMPLEMENTED + 5 PLANNED v1.1），占 D6 全部 IMPLEMENTED T 的 89%。S3 内部的 A 已经形成清晰的子价值流（评测执行 vs 数据集管理），但 S 层未反映。

---

## 3. Proposed Solution

### 3.1 D 层（不变）

**D6 Evolution** 保持支撑域身份，提供自演化评测 + 运行时守护能力，**不调整 D 层职责边界**。

### 3.2 S 层 — Canonical（4 价值流，2 IMPLEMENTED + 2 PLANNED）

```
D6（Evolution / 支撑域）
├── D6-S11 RunEvaluation        # C1：执行评测探针 + Judge + Delta + Tune
├── D6-S12 GuardRuntime         # C2：运行时校验 + 干预 + Agent 观测（原 S4 Orchestration）
├── D6-S13 TrackVersion         # C3：版本检测与报告（PLANNED）
└── D6-S14 ReloadConfig         # C4：配置热更新（PLANNED）
```

**S ↔ 承诺 1:1 对应表：**

| S ID | Scenario | 对应承诺 | 消费者可验证 WHAT | 旧 S 归属（冻结追溯） |
|------|----------|---------|-------------------|---------------------|
| D6-S11 | RunEvaluation | C1 评测执行 | 给定 dataset + probes，返回 eval_report + delta + tune_suggestions | D6-S3 Eval（5 A） |
| D6-S12 | GuardRuntime | C2 运行时守护 | Agent 异常行为被 Observer 捕获 → Validator 判定 → Intervention 执行 | D6-S4 Orchestration（3 A） |
| D6-S13 | TrackVersion | C3 版本追踪 | 构建信息 → 版本报告 | D6-S1 Version（PLANNED） |
| D6-S14 | ReloadConfig | C4 配置热更新 | 监控配置文件变更 → 热加载 | D6-S2 Config（PLANNED） |

### 3.3 关键设计决策

**D1: S4 "Orchestration" → S12 "GuardRuntime"**

| 候选 | 优点 | 缺点 |
|------|------|------|
| **GuardRuntime** ✅ | 与 D7 Orchestration 完全区分；动词+名词语义化；slug `guard` 简洁 | 略新颖（但无现有冲突） |
| ValidateRuntime | 强调 ValidateDecision A | 忽略 Intervention + Observer 两个 A |
| RuntimeGuard | 名词+名词（欠动作性） | slug `runtimeguard` 过长 |

**架构师理由（推荐 GuardRuntime）：**

1. **消除 D7 冲突**：D6 不再使用 "Orchestration" 词汇，D7 独占编排语义。
2. **覆盖完整**：ValidateDecision + ExecuteIntervention + ObserveAgent 三者统一于「守护」概念。
3. **对称命名**：D6-S11 `RunEvaluation`（主动评测）vs D6-S12 `GuardRuntime`（被动守护）——评测+守护形成 D6 的双能力定位。

**D2: S1/S2 PLANNED 占位符保留但重命名**

保留 D6-S13 TrackVersion 和 D6-S14 ReloadConfig 为 PLANNED Scenario，但：
- 不再使用「Version」「Config」等泛化技术词
- 使用动词+名词语义化名称
- T 注册表中明确标注 PLANNED，待有实际实现后再补充 A 层

**D3: S3 Eval → S11 RunEvaluation（仅重命名，不拆分）**

D6-S3 的 5 个 A 内聚性高（都围绕评测生命周期），不强制拆分。重命名为 `RunEvaluation` 与 D4-S12 `RunAgentLoop`、D2-S16 `RunQueryLoop` 保持「动词+名词」命名一致。

### 3.4 scenario-slug 注册表（草案）

| S ID | scenario-slug | v2.0 目标目录 | 当前路径 |
|------|---------------|-------------|---------|
| D6-S11 | `evaluate` | `evolution/evaluate/` | `evolution/eval/` |
| D6-S12 | `guard` | `evolution/guard/` | `evolution/orchestration/` |
| D6-S13 | `version` | `evolution/version/` | `evolution/version/`（PLANNED） |
| D6-S14 | `reload` | `evolution/reload/` | `evolution/config/`（PLANNED） |

**关键路径变更（v2.0）：**

| 现行路径 | 目标路径 | 原因 |
|---------|---------|------|
| `evolution/orchestration/` | `evolution/guard/` | 消除与 D7 的命名冲突 |
| `evolution/eval/` | `evolution/evaluate/` | scenario-slug 语义化（`eval` → `evaluate`，与 S 名 RunEvaluation 一致） |

### 3.5 T 层迁移策略

24 条 T（19 IMPLEMENTED + 5 PLANNED）**不改测试代码**。`t-registry.md` 增 `canonical_s` 列：

| 旧 S | T 数 | 新 Canonical S | 备注 |
|------|------|---------------|------|
| D6-S1 Version | 1 (PLANNED) | D6-S13 TrackVersion | 保持 PLANNED |
| D6-S2 Config | 1 (PLANNED) | D6-S14 ReloadConfig | 保持 PLANNED |
| D6-S3 Eval (A01) | 19 (17 IMPL + 2 PLANNED) | D6-S11 RunEvaluation | 全部 RunEval 子树 |
| D6-S3 Eval (A02) | 1 (IMPL) | D6-S11 RunEvaluation | JudgeResult |
| D6-S3 Eval (A05) | 1 (IMPL) | D6-S11 RunEvaluation | ManageDataset |
| D6-S4 Orchestration | 1 (IMPL) | D6-S12 GuardRuntime | ValidateDecision |

**v1.1 新增 T（T20/T21/T22）** 已在 D6-S3-A01 下 → Canonical D6-S11-A01。

---

## 4. Success Metrics

| 指标 | 基线 | v1.0 目标 |
|------|------|----------|
| D6 价值流 S 数 | 0/4 | 4（2 IMPL + 2 PLANNED） |
| S 名语义化 | 0/4（含 PLANNED 占位） | 4/4（动词+名词） |
| D7 命名冲突 | 1（S4 "Orchestration"） | **0** |
| P0 T 全绿 | 6 | 6（保持） |
| T 总数 | 24 | 24 + canonical 列 |

---

## 5. Implementation Plan

### Phase A — S1→S2 澄清

- `demand.md`
- `proposal.md`（本文件）

### Phase B — v1.0 Registry（纯文档，零代码变更）

- D6 `a-registry.md` Canonical 重排（4 S 层）
- D6 `t-registry.md` 增 `canonical_s` 列 + Legacy 双轨
- `layering.md` §D6 双轨
- `code-layout.md §4` 补 D6 scenario-slug 注册表
- `cross-domain-boundaries.md` §D6 扩展

### Phase C — S3 design + S3-Gate

- `design.md`：A/F 编排 + Decision 表
- S3-Gate review

### Phase D — v1.0 验收

- 24 T 追溯表 100% 覆盖
- `acceptance-report.md`
- **零 Go 变更**

### Phase E — v2.0（后续 change）

- `evolution/orchestration/` → `evolution/guard/`（消除 D7 冲突）
- `evolution/eval/` → `evolution/evaluate/`（slug 语义化）
- re-export bridge 1 周期后清理

---

## 6. Out of Scope（本 change v1.0）

- Go 代码移动
- 修改已有 `// T:` 测试注释
- 物理路径迁移（v2.0）
- D6-S13/S14 PLANNED → IMPLEMENTED（独立 change）

---

## 7. 相关文档

| 文档 | 用途 |
|------|------|
| `docs/methodology/dsaft-refactoring-playbook.md` | 方法论 SoT |
| `openspec/archive/2026-06-14-devrix-d3-sa-refine/proposal.md` | 首案样板 |
| `openspec/archive/2026-06-15-devrix-d4-sa-refine/proposal.md` | 同型参考 |
| `openspec/specs/d6-evolution/a-registry.md` | 现行 A 注册表 |
| `openspec/specs/d6-evolution/t-registry.md` | 现行 T 注册表 |
