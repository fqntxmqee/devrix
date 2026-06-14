---
demand-id: DM-20260614-005
title: D7-S5-T03 + T06 ClassifyIntent 规则置信度 + Command-first 端到端测试闭环
source: devrix-d7-orchestration-domain R2 决议 v1.0 P0 PLANNED
priority: P0
status: S1_Requirement
dsaft_domain: D7
created: 2026-06-14
last-updated: 2026-06-14
---

# D7-S5-T03 + T06 ClassifyIntent 规则置信度 + Command-first 端到端闭环

## 1. 原始描述

`devrix-d7-orchestration-domain` Change 归档时（DM-20260614-001），D7-S5-T03 与 D7-S5-T06 在 `openspec/specs/d7-orchestration/t-registry.md` 标注为 **v1.0 P0 PLANNED**：

| T ID | 描述 | Status |
|------|------|--------|
| D7-S5-T03 | ClassifyIntent 规则高置信 → simple | PLANNED (v1.0) P0 |
| D7-S5-T06 | Command-first：`/plan` 不触发 LLM Classify | PLANNED (v1.0) P0 |

**现状核查**：

- `internal/layers/d7/classifier_test.go` 已存在 5 个测试，注释明确标 `T: D7-S5-T03`：
  - `TestRuleClassifier_Classify_Empty` — empty → IntentSkip + Confidence 100
  - `TestRuleClassifier_Classify_CommandFirst` — `/plan` `/stop` `/task` `/help` → IntentCommand + Confidence 100
  - `TestRuleClassifier_Classify_FastPath` — fast pattern → IntentFast + Confidence ≥ 70
  - `TestRuleClassifier_Classify_Orchestrate` — 复杂消息 → IntentOrchestrate
  - `TestRuleClassifier_Classify_ShortDefaultsFast` — 短消息 → IntentFast + Confidence 70
- `internal/layers/d7/shadow_classifier_test.go` 已覆盖 `TestShadowClassifier_TailOnly_NotCalledOnCommand`：证明 shadow 包装下 `/plan` 不触发 LLM
- `internal/layers/d7/orchestrator_test.go` 已覆盖 `TestSessionOrchestrator_ProcessMessage_Command`：证明 orchestrator 端到端命令路径
- `internal/layers/d7/orchestrator.go` ProcessMessage 在 `o.shadowClassifier != nil` 时调用 shadow.Classify；shadow 内部 `tail-only` 已确保命令路径不调用 LLM

**Gap**：

1. **T-registry 状态未同步** — t-registry.md 仍标 PLANNED；按 testing.md 规范应同步为 IMPLEMENTED
2. **端到端缺失** — orchestrator 层「WithShadowClassifier 启用 + 命令路径 + LLM stub 未被调用」的整合测试缺失。当前 shadow_classifier_test 仅断言 shadow 包装层；orchestrator_test 仅断言基础命令路径，未覆盖「shadow 启用 + 命令路径短路 LLM」的端到端组合
3. **Command-first 默认关闭路径无回归** — 若 `CommandFirst=false`，`/plan` 应 fall through 到 fast 或 orchestrate；当前测试未覆盖

**目标**：以 1 个端到端测试 + 2 个 T-registry 同步更新，闭环 v1.0 最后 2 个 D7 P0 项。

## 2. 验收标准

| ID | 标准 | 优先级 |
|----|------|--------|
| AC1 | 新增 `TestSessionOrchestrator_CommandFirst_ShadowNotCalled`：orchestrator + ShadowClassifier 启用时，`/plan` 命令路径**不调用 stubLLM** | **P0** |
| AC2 | 新增 `TestRuleClassifier_Classify_CommandFirst_Disabled`：`CommandFirst=false` 时，`/plan add auth` 不再匹配 `IntentCommand`（fall through 到 fast 或 orchestrate） | **P0** |
| AC3 | `openspec/specs/d7-orchestration/t-registry.md` D7-S5-T03 / D7-S5-T06 状态由 PLANNED → IMPLEMENTED；Test 位置补全 | **P0** |
| AC4 | `openspec/t-registry.md` 域统计同步（PLANNED -2 / IMPLEMENTED +2） | **P0** |
| AC5 | `classifier_test.go` / `orchestrator_test.go` `go test -race` 全绿 | **P0** |
| AC6 | d7 包覆盖率 ≥ 80% 阈值（与归档基线对齐） | **P0** |
| AC7 | acceptance-report.md 列出 T03 / T06 测试位置 + 通过证据 | P1 |

## 3. 范围

### 3.1 新增

- `internal/layers/d7/orchestrator_test.go`：
  - `TestSessionOrchestrator_CommandFirst_ShadowNotCalled` — 端到端 AC1
- `internal/layers/d7/classifier_test.go`：
  - `TestRuleClassifier_Classify_CommandFirst_Disabled` — 配置回归 AC2

### 3.2 修改

- `openspec/specs/d7-orchestration/t-registry.md`：
  - D7-S5-T03 PLANNED → IMPLEMENTED，Test 位置 `internal/layers/d7/classifier_test.go`
  - D7-S5-T06 PLANNED → IMPLEMENTED，Test 位置 `internal/layers/d7/classifier_test.go` + `shadow_classifier_test.go` + `orchestrator_test.go`
  - 统计：Total 46 不变；IMPLEMENTED 37 → 39；PLANNED 7 → 5
- `openspec/t-registry.md`：
  - D7 行：IMPLEMENTED 37 → 39；PLANNED 7 → 5
  - 总计：IMPLEMENTED 256 → 258；PLANNED 13 → 11

### 3.3 不变更

- `internal/layers/d7/classifier.go` — Rule 实现不动
- `internal/layers/d7/shadow_classifier.go` — Shadow 实现不动
- `internal/layers/d7/orchestrator.go` — Orchestrator 实现不动
- `internal/layers/d7/config.go` — Config 字段不动
- D7-S2 路由矩阵（规则 + command-first 仍为 v1.0 权威决策）

## 4. 依赖与约束

| 类型 | 内容 |
|------|------|
| 依赖 | DM-20260614-001 (D7 Orchestration Domain) 已归档 |
| 依赖 | DM-20260614-004 (S5-P2 Shadow Classifier) 已归档（提供 ShadowClassifier + stubLLM 模式） |
| 约束 | 不修改 RuleClassifier / ShadowClassifier / SessionOrchestrator 实现 |
| 约束 | 端到端测试不引入真实 LLM 调用（仍用 stubLLM） |
| 约束 | go test -race -count=10 不出现 flake |

## 5. 风险评估

| 风险 | 影响 | 缓解 |
|------|------|------|
| Shadow 异步路径在测试中存在 race | 中 | 复用 `waitForCalls` 模式 + `time.Sleep(30ms)` 兜底（已在 shadow_classifier_test.go 验证） |
| CommandFirst=false 时下游 fast pattern 误匹配 `/plan` | 低 | 仅断言 `Kind != IntentCommand`，不限定回退到具体 Kind |
| t-registry 数值统计漂移 | 低 | 双重检查 D7 行 + 总计行 |

## 6. 后续 (v1.1+)

- D7-S5-T04 / T05 (SynthesizeTaskGraph, SelectExecutor) — 列入 v1.1
- D7-D4-T01 / D7-THIN-T01 / T02 (D2 loop 瘦身) — 按 R2 §4.3 决议 C 保留到 v1.1
