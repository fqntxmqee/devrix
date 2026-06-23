---
demand-id: DM-20260623-001
title: D7 MUPS v4 Phase 2 PR-A1 Design Review 反馈修复
priority: P0
status: S1_Proposal
dsaft_domain: orchestration
created: 2026-06-23
---

# D7 MUPS v4 Phase 2 PR-A1 Design Review 反馈修复

## 1. 背景

`devrix-d7-mups-v4-phase2-observe-plan` Change 的 S3 阶段 design.md 已完成（2026-06-23），未进 S3-Gate。Agent 自 review 发现 3 Critical + 8 Warning + 8 Info 项需修复。本需求把这些修复作为 PR-A1 落地前的最后一道关卡，确保进入 S4 实现阶段时 design 与实现零偏差。

## 2. 问题陈述

PR-A1 涉及 4 个核心文件（observation.go / uncertainty_report.go / uncertainty_coord.go / errors.go）+ 3 个测试文件。Agent 5 维度 review 发现：

### Critical（block S4 进入）
- **C1** `QuantizedIntent.Kind` 为 `string` 而非 `IntentKind`，PR-A2 IntentQuantizer 落地时需新增翻译层
- **C2** `UncertaintyReport.Observations` + `BusinessObservations` 双字段 + `MatchKind([]Observation)` 签名让 Plan 调用方易误用
- **C3** `FromVerifier` 对未知 verdict 静默兜底，但 `NewUncertaintyCoordInvalidVerdictKindError` 错误码永不会触发

### Warning（应在 S4 修）
- **W1** Observation.MarshalJSON wire format 设计稿未明确
- **W2** `validateFact` 无 `fmt.Errorf("%w")` 包装，与其他 Payload.Validate 风格不一致
- **W3** `clamp01` vs `clamp01Coord` 重复（唯一区别：NaN → 0.5）
- **W6** + **I8** `Partition` 末尾重算 Overall 时未 clamp（NaN 风险）
- **W8** `MatchKind(observations []Observation)` 静默忽略 CatSystem obs，调用契约不清

### Info（延后下个 PR）
- I1-I7 风格/边界微调

## 3. 验收标准

| ID | 标准 | 优先级 |
|----|------|--------|
| **AC1** | `QuantizedIntent.Kind` 改为 `IntentKind` 类型，原有 `QuantizedIntent` 调用方零修改 | P0 |
| **AC2** | `UncertaintyReport` 删除冗余 `Observations` 字段（或在 design.md §6.2 改 MatchKind 签名为 `(*UncertaintyReport)`），由 reviewer 在 S3-Gate 决议 | P0 |
| **AC3** | `FromVerifier` 对未知 verdict 返回错误（不静默兜底），`NewUncertaintyCoordInvalidVerdictKindError` 错误码正常触发；`TestUncertaintyCoord_FromVerifier_UnknownKind` 通过 | P0 |
| **AC4** | design.md §2.1 增 wire format 示例（payload 嵌套对象） | P1 |
| **AC5** | `validateFact` 改 `fmt.Errorf("orchtypes: FactPayload.Statement empty: %w", ErrObservationPayloadInvalid)` 与其他 Payload.Validate 风格统一 | P1 |
| **AC6** | `clamp01` + `clamp01Coord` 合并为 `clamp01Float(v float64, onNaN float64) float64` 单函数 | P1 |
| **AC7** | `Partition` 末尾 `r.Overall = clamp01Float(r.ComputeOverallStrength(), 0.5)`（NaN 兜底） | P1 |
| **AC8** | design.md §6.2 `MatchKind` 签名从 `(observations []Observation)` 改为 `(*UncertaintyReport)`（或保留 + 加注释，AC2 决议） | P0 |
| **AC9** | 所有 P0 验收标准对应 6 个新测试用例全部 PASS：`go test -race ./internal/layers/orchestration/orchtypes/` | P0 |
| **AC10** | `go vet` 0 issue | P0 |
| **AC11** | 覆盖率不低于 PR-A1 现状 72.2% | P0 |

## 4. 依赖与约束

| 类型 | 内容 |
|------|------|
| **依赖** | PR-A1 当前 working tree（observation.go / uncertainty_report.go / uncertainty_coord.go / errors.go 已写但未 commit） |
| **依赖** | `internal/shared/errors/` SentinelError 模式（已落地） |
| **依赖** | `orchtypes/intent.go` 已有 `IntentKind` 枚举（fast/command/orchestrate/skip） |
| **约束** | 不修改 `internal/shared/errors/`（跨域） |
| **约束** | 不动 Phase 1 既有文件（config.go / process.go / intent.go / routing.go 主体） |
| **约束** | 不修改 PR-A1 范围之外的文件（决策点、observe/、plan/、decisionplanning/intent_quantizer.go 留给后续 PR） |

## 5. 变更范围

### 修改（in scope）
- `internal/layers/orchestration/orchtypes/observation.go`（W2 validateFact + W3 clamp01）
- `internal/layers/orchestration/orchtypes/uncertainty_report.go`（C1 QuantizedIntent + C2 + W3 + AC7 Overall clamp）
- `internal/layers/orchestration/orchtypes/uncertainty_coord.go`（C3 FromVerifier 改强类型 + W3 clamp01Coord 合并）
- `internal/layers/orchestration/orchtypes/errors.go`（C3 错误码保留）
- `internal/layers/orchestration/orchtypes/observation_test.go`（AC5/AC6/AC7 新增测试）
- `internal/layers/orchestration/orchtypes/uncertainty_report_test.go`（C1/C2/AC7/AC8 新增测试）
- `internal/layers/orchestration/orchtypes/uncertainty_coord_test.go`（C3 新增 UnknownKind 测试）
- `openspec/changes/devrix-d7-mups-v4-phase2-observe-plan/design.md`（W1 wire format + C2/C3/W8 设计对齐 + AC2/AC8 决议）
- `openspec/changes/devrix-d7-mups-v4-phase2-observe-plan/proposal.md`（DM ID 修正 + C2/C3 风险更新）
- `openspec/changes/devrix-d7-mups-v4-phase2-observe-plan/tasks.md`（DM ID 修正 + 新增 review fix 任务项）

### 新增
- `openspec/changes/devrix-d7-mups-v4-phase2-observe-plan/.openspec.yaml`（缺失的 S2 必须文件）
- `openspec/changes/devrix-d7-mups-v4-phase2-observe-plan/acceptance-report.md`（S5 产出）

### 不变更（out of scope）
- `orchtypes/observation.go` 的 4 类 Payload sealed interface 设计（已落地）
- `orchtypes/uncertainty_report.go` 的 Partition 末尾重算 Overall 逻辑（已落地）
- `orchtypes/uncertainty_coord.go` 的 IsColdStart/Equal/With* 不可变方法（已落地）
- 4 个错误码 7001-7004 分配（已落地）
- 72.2% 覆盖率现状（不增加新测试覆盖率红线）
- 后续 PR-A2 / PR-A3 / PR-A4 / PR-B1 / PR-B2 / PR-B3 范围（PR-A1 是 Phase 2 第一刀）
- Phase 3 / Phase 4 / Phase 5 任何节点

## 6. 风险评估

| 风险 | 影响 | 缓解 |
|------|------|------|
| **C1 类型替换破坏 JSON wire format** | 旧 JSON 含 `kind: "fast"` → 反序列化 `IntentKind("fast")` OK；但新 JSON marshal 行为可能变化 | 保留 `IntentKind` 底层 `type ... string`，wire format 不变 |
| **C2 删 `Observations` 字段破坏调用方** | PR-A2/A3/B1 即将消费此字段 | 选 "不改字段，design.md §6.2 改 MatchKind 签名" 方案 |
| **C3 改 `FromVerifier` 为强类型** | 未知 verdict 报错 = 兜底变 fail-fast，可能引发更多错误 | 仅 unknown verdict 报错；4 种已知 verdict 行为不变 |
| **W3 合并 clamp01 影响 NaN 处理** | 其他模块调用 clamp01 时 NaN 行为可能变化 | 合并后函数签名 `clamp01Float(v, onNaN)`，默认 onNaN=0.5，调用方需显式传入 |
| **W8 改 MatchKind 签名** | PR-B1 调用方需更新 | 同步改 design.md + tasks.md PR-B1 段 |
| **测试覆盖率下降** | 合并/重构可能降低覆盖率 | 跑 go test -cover，对比前后数值；新加 6 个测试用例覆盖重构路径 |
| **DM ID 修正引发追溯问题** | `DM-20260624-001` → `DM-20260623-001` 需全仓替换 | 改 proposal.md / tasks.md / design.md 4 处引用即可，无外部 commit |

## 7. Out of Scope

明确**不在本需求**内的事项：

| 任务 | 落点 |
|------|------|
| `Observation.Validate` 进一步校验（Payload.Validate 二次调用） | 已落地（AC1 验证） |
| `W4 NewObservationWithID` 工厂方法 | 后续 PR（trace 增强需求） |
| `W5 unmarshalPayload graceful degrade` | 后续 PR（forward-compat 需求） |
| `W7 QuantizedIntent.Source 类型` | PR-A2 决策 |
| `I1-I7` 全部 Info 项 | 后续 PR（风格/边界微调） |
| PR-A2 IntentQuantizer 落地 | devrix-d7-mups-v4-phase2-observe-plan 后续 PR |
| PR-A3 AnomalyDetector 4 实现 | devrix-d7-mups-v4-phase2-observe-plan 后续 PR |
| PR-A4 ObserveNode + ProcessMessage wiring | devrix-d7-mups-v4-phase2-observe-plan 后续 PR |
| PR-B1/B2/B3 Plan 节点 | devrix-d7-mups-v4-phase2-observe-plan 后续 PR |
| Phase 3-5 任何节点 | devrix-d7-mups-v4-phase3-execute / phase4-verify-promotion / phase5-learn |

## 8. Cross-references

- Design Review 报告：对话上下文（Agent 5 维度自 review）
- 上游 PR-A1 状态：S3_Design，未 commit，未进 S3-Gate
- 下游 PR-A2 计划：IntentQuantizer 3 轮收敛
- Phase 1 前置：`devrix-d7-mups-v4-phase1-foundation`（DM-20260623-001 原始编号）
- Phase 3 后续：`devrix-d7-mups-v4-phase3-execute`
