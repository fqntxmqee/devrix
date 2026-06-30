---
demand-id: DM-20260630-001
title: D7 Observe 统一 LLM 入口 — 退役 Observe 独立 D3 链路
priority: P1
status: S5_Accepted
created: 2026-06-30
reporter: Jaeger 实测 + Clawcode QueryLoop 对齐分析
supersedes_partial: DM-20260628-002  # 仅 A74/T35 LLM ObservationProposer 部分
related:
  - DM-20260628-002 (Layer SubContext Phase 3 — 引入 LLMObservationProposer)
  - DM-20260627-003 (Layer SubContext Phase 1+2 — R-OBS rules mapping)
  - DM-20260610-012 (QueryLoop 对齐 Clawcode Harness)
---

# Demand: D7 Observe 统一 LLM 入口

## 1. 原始诉求

用户发送首条消息（如「你好」）时，Jaeger trace 显示：

1. **Observe 阶段**出现独立 `D3_LLM_Stream`，`system_prompt` 为写死的英文 Observation 提案 prompt；
2. **未出现** `D2_Context_Process` / `D2_Context_Materialize`；
3. 与 Clawcode QueryLoop 设计不一致——Clawcode 首次 LLM 请求即携带完整 system prompt（含 i18n）、tools 与 userContext。

根因：`DM-20260628-002` Phase 3 T35 在 Observe 节点增加了 `LLMObservationProposer`，绕过 D2 直接调 D3，形成与 Execute 主链路并行的「轻量 LLM 入口」。

## 2. 业务目标

| ID | 目标 | 可验证承诺 |
|----|------|------------|
| **OU-1** | Observe 调 D3 前必须先走 D2 | Jaeger Observe 子树：`D2_Context_Process` 在 `D3_LLM_Stream` 之前 |
| **OU-2** | System prompt 含 D2 i18n 基座 | 中文环境 Obs system prompt 含 D2 模板 + 中文 Obs 附录 |
| **OU-3** | 禁止裸调 D3 | 无独立英文写死 system prompt |
| **OU-4** | R-OBS + fail-safe 保留 | LLM 失败时 rules-only Observe 仍产出 |

## 3. 澄清记录

| Q | A |
|---|---|
| Observe 能否调 D3？ | **可以**（可选 `LLMObservationProposer`），但**必须先** `ContextPreparer.Prepare`（D2） |
| 是否完全删除 ObservationProposer？ | **否**。生产 wired `LLMObservationProposer(llm, ctx, locale)` |
| SubTurn/Wave Materialize（T33/T34）是否回退？ | **否**。本需求仅退役 A74 独立 Observe LLM 路径 |
| 是否需要新 feature flag？ | **否**。直接移除 bootstrap 默认 wiring；0 环境变量 |

## 4. 澄清范围

- **L1 领域**：D7 Orchestration
- **L2 场景**：D7-S16 Layer SubContext / ItemPipeline MUPS
- **L3 活动**：Observe（rules-only）、Execute（WorkItemExecutor ReAct）
- **L4 功能点**：
  - 退役：`LLMObservationProposer`、`llm_observation_proposer.go`
  - 保留：`observeWorkItem` R-OBS mapping、`ValidateObservationProposals`
- **L5 测试点（草案）**：
  - T01：bootstrap `WireItemPipeline` 不注入 ObservationProposer
  - T02：`llm_observation_proposer.go` 已删除
  - T03：Observe rules-only 单测仍绿
  - T04：spec + t-registry 同步 A74→DEPRECATED、A75 ADDED

## 5. In Scope / Out of Scope

### In Scope

- 移除 `NewLLMObservationProposer` bootstrap wiring
- 删除 `llm_observation_proposer.go`
- OpenSpec change 包 + SoT spec/t-registry 同步

### Out of Scope

- Execute 阶段 Materialize 策略变更（L0 legacy Prepare 不变）
- Observe 新增 LLM 能力（未来若需要，必须走 D2 Prepare + D3，不得裸调 D3）
- SubTurn / Wave Materialize（DM-20260628-002 T33/T34）

## 6. Demand 级验收标准

- [x] **P0** Observe 阶段无独立 D3 LLM 调用（bootstrap ObservationProposer=nil）
- [x] **P0** `llm_observation_proposer.go` 已删除
- [x] **P0** `go test ./internal/bootstrap/... ./internal/layers/orchestration/sessionorchestrator/...` PASS
- [x] **P1** `openspec/specs/d7-orchestration/spec.md` A74 DEPRECATED + A75 ADDED
- [x] **P1** t-registry D7-S16-A75 T 点登记
