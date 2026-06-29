# Spec Delta: devrix-d3-dsaft-restructuring (DM-20260629-003)

**Change ID:** devrix-d3-dsaft-restructuring
**Demand ID:** DM-20260629-003
**Status:** S2_Proposal → S3_Design → S4_Implemented → S5_Accepted → S7_Archived

---

## §1 Modified Specs

| Spec | Version | Status |
|------|---------|--------|
| `d3-domain.md` | v1.0.0 → **v1.5.0** | 修订 |
| `a-registry.md` | v3.1.0 → **v3.2.0** | 修订 (ValueFlow Alias) |
| `f-registry.md` | v3.1.0 → **v3.2.0** | 修订 (ValueFlow Alias + 4 F path 修正) |
| `t-registry.md` | v3.3.1 → **v3.4.0** | 修订 (ValueFlow Alias + Span Evidence 列) |
| `span-registry.md` | v3.1.0 → **v3.2.0** | 5 active ops 保持 (R1 Q3 runtime 字面量) |
| `observability-guide.md` | v1.0.0 → **v2.0.0** | T-Without-Span Tracker + Coverage Guard |
| `layer-delta.md` | v1.0.0 → **v1.1.0** | §Canonical S → ValueFlow Alias 表 |
| `d3-boundary.md` | v1.0.0 → **v1.1.0** | 同步 0 boundary-debt 决策 |

---

## §2 Delta — d3-domain.md §North Star

**ADDED** ValueFlow Alias 列 (5 S):

| 可验证承诺 | Canonical S | ValueFlow Alias (用户感知) |
|-----------|-------------|------------------------------|
| C1 模型路由 | D3-S1 RouteModel | **D3_Model_Routing** |
| C2 流式 SSE chunk 流 | D3-S2 StreamChat | **D3_Stream_Chat_Completion** |
| C3 Provider 故障不阻塞用户 | D3-S3 ProtectCall | **D3_Circuit_Breaker_And_Retry** |
| C4 Token 超预算截断/报错 | D3-S4 BudgetTokens | **D3_Token_Budget_Control** |
| C5 危险 prompt 拒绝/告警 | D3-S5 GuardContent | **D3_Content_Safety_Filter** |
| 配置加载与启动 fail-fast | D3-S6 ConfigureGateway | (横切, 启动期) |

## §3 Delta — d3-domain.md §Boundary Debt Decisions

**ADDED** §Boundary Debt Decisions 章节 (PR-7):

| Boundary ID | 状态 | 内容 | 重新评估 |
|-------------|------|------|----------|
| `boundary-debt:d3-no-debt-v1.0` | ✅ RESOLVED (v1.5.0) | D3 当前 0 boundary-debt (R1+R2+R3 决议 + DM-20260628-001 APIErrorCode 7 类) | — |

> 治理常量在 `internal/layers/llmgateway/orchtypes/boundary_decision.go`。
> 3 单元测试: `internal/layers/llmgateway/orchtypes/boundary_decision_test.go`。

## §4 Delta — f-registry.md F 路径修正 (4 处)

**CHANGED**:

| F ID | 旧路径 | 新路径 |
|------|--------|--------|
| D3-S4-A01-F01..F05 | `token/counter.go` | `budget/counter.go` |
| D3-S5-A01-F01..F03 | `safety/filter.go` | `guard/filter.go` |
| D3-S6-A01-F01 | `config/loader.go` | `configure/loader.go` |
| D3-S6-A01-F02 | `shared/config/llmgateway.go` | `configure/shared_config.go` |

## §5 Delta — t-registry.md Span Evidence 列

**ADDED** Span Evidence 列 (12/14 canonical T 映射 → ≥80%):

- D3-S1-A01-T01..T05 → `D3_LLM_Provider_Route`
- D3-S2-A01-T01..T06 → `D3_LLM_Stream`
- D3-S3-A01-T01..T17 → `D3_LLM_CircuitBreaker` / `D3_LLM_Retry` / `D3_LLM_Adapter_Stream`
- D3-S4-A01-T01..T03 → (注入)
- D3-S5-A01-T01..T03 → (注入)
- D3-S6-A01-T01..T02 → (启动期, 不进 trace)

**ADDED** §T-Without-Span Tracker (PR-6):

- D3-S4 BudgetTokens 注入 (3 entry)
- D3-S5 GuardContent 注入 (3 entry)
- D3-S6 ConfigureGateway 启动期 (2 entry)

## §6 Delta — a/f/t-registry + layer-delta.md ValueFlow Alias

**ADDED** ValueFlow Alias block to:
- `a-registry.md` — 5 S section header
- `f-registry.md` — 5 S 段
- `t-registry.md` — §Canonical T 映射块
- `layer-delta.md` — §Canonical S → ValueFlow Alias 表

## §7 Delta — span-registry.md

**UNCHANGED** 5 active ops (R1 Q3 决议, runtime 字面量稳定性):
- `D3_LLM_Stream` (CLIENT, llm_gateway)
- `D3_LLM_Provider_Route` (INTERNAL, llm_gateway)
- `D3_LLM_CircuitBreaker` (INTERNAL, llm_gateway)
- `D3_LLM_Retry` (INTERNAL, llm_gateway)
- `D3_LLM_Adapter_Stream` (CLIENT, llm_adapter)

**ADDED** T↔Span Evidence 映射表 (PR-6 + t-registry 一致性).

## §8 Delta — observability-guide.md

**ADDED** §T-Without-Span Tracker (2-3 entry) + §Coverage Guard v1.5.0 follow-up.

**ADDED** CI script `scripts/d3-span-coverage.sh` 守门 ≥80%.

## §9 修订记录

| Version | Date | Changes |
|---------|------|---------|
| v1.5.0 | 2026-06-29 | **DM-20260629-003 S7_Archived 收口**: (1) v1.0.0 → v1.5.0 域版本对齐 D2 v9.0.0 + D7 v2.6.0；(2) 7 PR / 6 子 Change / 40 T 全部 IMPLEMENTED；(3) Span Evidence 覆盖率 ≥80%；(4) 死代码 0；(5) god fn 拆 6 文件（stream/pipeline + protected + instrument + configure/defaults + errorclass/regex_rules）；(6) 4 F path 修正（token→budget / safety→guard / config→configure / shared/config→configure/shared_config）；(7) 5 S ValueFlow Alias 加 D3_ 前缀；(8) 0 boundary-debt 决策 + 治理常量 + 3 单元测试 PASS |
