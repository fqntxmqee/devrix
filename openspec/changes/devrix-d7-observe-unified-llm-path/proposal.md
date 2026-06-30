# Proposal: D7 Observe 统一 LLM 入口

**Change ID:** `devrix-d7-observe-unified-llm-path`  
**Demand ID:** DM-20260630-001  
**Status:** Accepted  
**Created:** 2026-06-30

---

## Problem Statement

`DM-20260628-002` T35 在 Observe 节点引入 `LLMObservationProposer`，**裸调 D3**（跳过 D2），形成与 Clawcode QueryLoop 不一致的路径：

| 路径 | 触发点 | D2 | Tools | i18n |
|------|--------|-----|-------|------|
| **Observe（T35，待修复）** | 首条消息 Observe | ❌ | ❌ | ❌ 英文写死 prompt |
| **Execute（主链路）** | WorkItemExecutor ReAct | ✅ | ✅ | ✅ zh-CN 默认 |

Jaeger 首条消息先看到 Observe 的英文 D3 调用，用户误以为系统未加载中文上下文。

## Proposed Solution

**Observe 可以调 D3，但必须先走 D2 获取上下文，再走 D3。**

1. 重写 `LLMObservationProposer`：`ContextPreparer.Prepare` → 拼接本地化 Obs 附录 → `LLMInvoker.InvokeStream`
2. Bootstrap wired `NewLLMObservationProposer(llm, ctx, locale)`
3. 保留 `ValidateObservationProposals` 规则门控 + fail-safe
4. 规格层 A74 SUPERSEDED，新增 A75「Observe D2→D3」

## Alternatives Considered

| 方案 | 结论 |
|------|------|
| A. 仅改 i18n，仍裸调 D3 | ❌ 未走 D2，Jaeger 无 `D2_Context_Process` |
| B. 完全移除 Observe LLM | ❌ 失去 LLM 辅助 Obs 提案能力 |
| **C. D2 Prepare → D3 Obs 提案（选用）** | ✅ 保留能力 + 对齐 D2 i18n |

## Capabilities

| Capability | L1 | Change |
|------------|-----|--------|
| d7-orchestration | D7 | MODIFY Observe LLM path; ADD A75 D2→D3 |

## Impact

| Component | Change |
|-----------|--------|
| `llm_observation_proposer.go` | REWRITE (D2→D3) |
| `wire_item_pipeline.go` | wired proposer + locale |
| `llm_observation_proposer_test.go` | ADD D2-before-D3 tests |
| `spec.md` / `t-registry.md` | A74 SUPERSEDED, A75 ADDED |

## Success Criteria

- [x] Observe D3 前出现 `D2_Context_Process`
- [x] System prompt = D2 基座 + 本地化 Obs 附录
- [x] 单测全绿
- [x] SoT spec 已同步

## Risks & Mitigations

| Risk | Mitigation |
|------|------------|
| Observe + Execute 双 D2 调用 | 语义分离：Obs 提案 vs ReAct；可接受 |
| LLM 失败 | fail-safe rules-only Observe |
