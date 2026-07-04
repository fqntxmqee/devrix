---
demand-id: DM-20260704-004
title: "MUPS prompttags framework — centralized tag Wrap/ExtractOne"
priority: P0
status: S5_Acceptance
dsaft_domain: shared, context-engine, orchestration
created: 2026-07-04
related:
  - internal/shared/prompttags/
  - internal/layers/contextengine/materialize/phase_prompts.go
  - internal/layers/orchestration/workmodel/deliverable_contract.go
  - internal/layers/orchestration/workmodel/scope_contract_parse.go
---

# MUPS prompttags framework

## 1. 背景

MUPS 机器可读 prompt tag（`<scope_contract>`、`<deliverable_contract>`、`<deliverable_schema>` 等）在 D2 materialize 与 D7 workmodel 中各自手写序列化/正则解析，导致：

- `scopeContractBlock` 手工拼接 JSON，`out_of_scope` 被错误写成逗号分隔字符串而非 JSON 数组
- 同一 tag 格式在多处重复 regex / string concat
- 新增 tag 无统一注册表，难以做 phase 级 ExtractAll

## 2. 目标

建立 `internal/shared/prompttags/` 包，提供 TagSpec 注册表与泛型 `Wrap` / `ExtractOne` / `ExtractAll` / `ParseWholeBody` / `DocBlock` API；迁移 envelope tag 写读路径及 Observe/Plan/Execute 提示词集成。

## 3. 验收标准

| ID | 标准 | 优先级 | 状态 |
|----|------|--------|------|
| AC1 | `prompttags` 包含 MUPSRegistry（scope_contract、deliverable_contract、deliverable_schema、prior_verify_reason、open_questions） | P0 | PASS |
| AC2 | `Wrap`→`ExtractOne` round-trip golden test 覆盖各 envelope tag | P0 | PASS |
| AC3 | `phase_prompts.go` scope_contract 使用 `json.Marshal`（修复 out_of_scope 数组） | P0 | PASS |
| AC4 | workmodel 公开 API（DeliverableContractTag、ParseScopeContractBlock 等）保持向后兼容 thin wrapper | P0 | PASS |
| AC5 | Observe/Plan user prompt 经 `BuildLineFrame` + `ObserveUserFrame`/`PlanUserFrame` | P1 | PASS |
| AC6 | `DocBlock` / `ExecuteOutputTagDoc` 提供机器 tag 语法；i18n 保留 locale 散文规则 | P2 | PASS |
| AC7 | Observe/Plan appendix 注入 `DocBlockObserveSchema` / `DocBlockPlanSchema` | P2 | PASS |
| AC8 | `parseObservationProposalsJSON` / `parseStrategicPlanJSON` 使用 `ParseWholeBody` | P3 | PASS |
| AC9 | `ExtractAll` 支持 MUPS phase 过滤 | P3 | PASS |
| AC10 | `go test` prompttags + materialize + workmodel + sessionorchestrator 相关包 PASS（除已知 pre-existing failure） | P0 | PASS |

## 4. 范围

### P0
- 新建 `internal/shared/prompttags/`（registry、envelope、wholebody）
- OpenSpec change 包
- 迁移 phase_prompts + deliverable_contract + expected_return + scope_contract_parse

### P1
- Observe/Plan user prompt builders（linefield frames）

### P2
- i18n appendix DocBlock 集成

### P3
- wholebody 全面替换 deliverable JSON 解析（proposers + findings fast path）

## 5. 约束

- 不引入新依赖
- orchestration Go 不含战术散文
- DocBlock 仅机器 tag 语法，locale 规则留 i18n
