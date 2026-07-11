# Proposal: D7 Observe 节点全协议修订 + 实现债闭环

**Change ID:** `devrix-d7-observe-node-spec`
**Demand ID:** DM-20260711-001
**Priority:** P1
**Status:** S3_Design
**Parent:** `devrix-d7-observe-llm-protocol-doc` (DM-20260708-003)

---

## 1. Problem Statement

DM-20260708-003 将 Observe↔LLM 子协议文档化，但三方对照（文档 / trace test / 生产代码）暴露四类问题：

| # | 问题 | 影响 |
|---|------|------|
| P-doc | 旧 spec 仅描述 LLM 子路径，忽略 Go 机械轨 | fast-path / Partition 行为不可预测 |
| P-scenario | 旧 §5 场景 2–4 与 trace test 输入不一致 | 文档不能作验收 SoT |
| P-fastpath | `pickHighStrengthBusinessFact` 无 source 优先级 | prior≥0.85 时可能 emit directive 原文 |
| P-catsystem | CatSystem 提升未实现，场景 4 依赖测试 hack | Anomalies 路由在生产不可达 |

**谁受影响**：维护 Observe 协议的开发者、做 trace 验证的用户、消费 `UncertaintyReport` 的 Plan/fast-path 链路。

## 2. Proposed Solution

### 2.1 文档层

新增 **`observe-node-spec.md`** 作为 Observe **全节点 SoT**：

- 双轨架构（Go 机械 + LLM 分类）
- 三层输入（struct / LLM frame / signal 词汇表）
- 证据剖面 OBS-E0–E7 + 用例 OBS-U01–U12 + OBS-O/G/P/I 穷举
- 实现债 P1–P5 与分波次修复计划

旧 `d7-observe-llm-io-protocol-spec.md`：**保留 §3–§4、§7**，§5 标注 superseded。

### 2.2 代码层（三波 PR）

| 波次 | 内容 | 优先级 |
|------|------|--------|
| **Wave 1** | P1 fast-path source 过滤 + trace 回归 | P0 |
| **Wave 2** | P2 CatSystem promote + P3 scope 去重 | P1 |
| **Wave 3** | P4 signal 注册表 + P5 字段 SoT 统一 | P1 |

## 3. Capabilities

| Capability | L1/L2 | 类型 | 说明 |
|------------|-------|------|------|
| **observe-node-protocol** | D7 / D7-S5 | **MODIFIED** | 全节点 spec；**输入类型无关** + 证据剖面 OBS-E* |
| `observe-llm-classifier` | D7 / D7-S5 | UNCHANGED | 6 字段 + 4 kind（旧 spec §3–§4 仍有效） |
| `observe-fastpath-pick` | D7 / D7-S5 | **MODIFIED** | source 感知选题 |
| `observe-category-promote` | D7 / D7-S5 | **ADDED** | Go CatSystem 提升 |
| `observe-signal-registry` | D7 / D7-S5 | **ADDED** | 注册表化 signal 前缀 |

## 4. Alternatives Considered

| 方案 | 优点 | 缺点 | 决策 |
|------|------|------|------|
| A. 仅修文档 | 零风险 | 不闭合 P1/P2 生产缺陷 | ❌ |
| B. 全节点 spec + P1 only | 快速闭合 fast-path 风险 | CatSystem 仍虚构 | ✅ Wave 1 |
| C. 一次性 P1–P5 | 一次到位 | PR 超 400 行红线 | ❌ 分波次 |
| D. LLM emit CatSystem | 模型直接分类 | 与封闭式分类器定位冲突；已有 Go 兜底模式 | ❌ |

**推荐**：B → Wave 2 → Wave 3。

## 5. Impact Analysis

| 组件 | 变更 | 详情 |
|------|------|------|
| `deliverable_execute.go` | Yes | `pickHighStrengthBusinessFact` source 过滤 |
| `observe_category_promote.go` | NEW | CatSystem 提升 |
| `observation_proposer.go` | Yes | signal 注册表；scope 去重 wiring |
| `llm_observation_proposer.go` | Maybe | P5 统一 field map |
| `observe_trace_e2e_test.go` | Yes | 场景对齐 + prior≥0.85 回归 |
| OpenSpec | Yes | observe-node-spec + delta spec + t-registry |
| API / DB | No | 无对外 API 变更 |

## 6. Scope

### In Scope

- S3 四件套 + delta spec
- Wave 1 代码（P1）可在 S4 立即启动
- t-registry D7-S5-A121 预登记

### Out of Scope

- 新 ObservationKind
- Plan/Execute 协议变更
- `known_gaps` 实算（Phase 3+）

## 7. Goals (SLO)

| 指标 | 目标 |
|------|------|
| 证据剖面覆盖率 | OBS-E0–E7 + OBS-U01–U12 + OBS-I01–I07 trace 绑定 ≥ 90% |
| fast-path 正确率 | prior∈{0.625,0.90} × 确定性 Q&A → 均 emit LLM 答案 |
| 回归 | `go test -race ./internal/layers/orchestration/sessionorchestrator/...` 100% PASS |
| 文档漂移 | §7.7 偏离表 0 新增行（S5 后） |

## 8. Risks & Mitigations

| 风险 | 概率 | 影响 | 缓解 |
|------|------|------|------|
| source 过滤误伤合法 item_pipeline fact | Med | Med | 仅排除 `statement==directive` echo |
| scope 去重后 LLM 分类质量下降 | Low | Med | Go 机械层已注入；保留 scope_goal |
| CatSystem 规则误 promote | Med | High | 保守规则 + 独立 trace；可 feature flag |
| PR 超 400 行 | Med | Low | 三波拆分 |

## 9. 关联

- DM-20260708-003 — 父 LLM 子协议
- DM-20260706-011 — fast-path 四闸门
- DM-20260705-009 — 封闭式分类器
- `d7-observational-fastpath-spec.md` — Gate 定义
