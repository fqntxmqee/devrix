# Tasks: devrix-d3-dsaft-restructuring (DM-20260629-003)

**Change ID:** `devrix-d3-dsaft-restructuring`
**Demand ID:** DM-20260629-003
**Status:** S2_Tasks
**Total Tasks:** 40
**Total AC:** 40
**Template:** `devrix-d2-dsaft-restructuring` tasks.md (DM-20260629-002 S7_Archived)

---

## §0 任务索引（40 T / 7 子 Change）

| 子 Change | PR | T 范围 | 工作量 |
|---|---|---|---|
| **#0** legacy-cleanup | PR-1 | T01-T08 | 1 PR / 1 天 |
| **#1** god-fn-split pt1 | PR-2 | T09-T13 | 1 PR / 1 天 |
| **#1** god-fn-split pt2 | PR-3 | T14-T18 | 1 PR / 1 天 |
| **#2** registry-sync | PR-4 | T19-T24 | 1 PR / 1 天 |
| **#3** value-flow-rename | PR-5 | T25-T28 | 1 PR / 1 天 |
| **#4** span-coverage | PR-6 | T29-T33 | 1 PR / 1 天 |
| **#5** boundary-decision | PR-7 | T34-T37 | 1 PR / 1 天 |
| **S7_Archive** | PR-8 | T38-T40 | 1 PR / 1 天 |
| **总计** | 8 PR | **40 T** | **7-9 天** |

---

## §1 子 Change #0 legacy-cleanup（PR-1, T01-T08）

### T01 审计未使用 exported function

- `grep -rE "^func [A-Z]" internal/layers/llmgateway/ --include="*.go" -n` 全量列出 exported
- 验证每个 exported 是否被引用

### T02 审计未使用 exported type

- `grep -rE "^type [A-Z]" internal/layers/llmgateway/ --include="*.go" -n` 全量
- 验证每个 exported type 是否被引用

### T03 删死代码

- 删 spans.go 死代码 (if unused)
- 删 unused param / unused var
- 验证: `go vet ./internal/layers/llmgateway/...` 0 warning

### T04-T08 5 dead span ops 删 (如存在)

- D3 5 active ops 保持 (R1 Q3 runtime 字面量)
- 验证: 5 ops 全部在 stream/gateway.go emit
- AC: dead ops 0

---

## §2 子 Change #1 god-fn-split pt1（PR-2, T09-T13）

### T09 拆 `stream/gateway.go::Gateway.Stream()` 235 LOC

- 拆为 `stream/pipeline.go` + `stream/protected.go` + `stream/instrument.go`
- Stream() public API 保持不变
- 7 step 顺序保持

### T10 stream/pipeline.go (routing + 入口)

- 包含 route + chunk fanout + recordSuccess/Error (outcome phase)
- <400 LOC

### T11 stream/protected.go (breaker + retry + adapter)

- 包含 breaker.Allow + retry.wrap + adapter.stream
- <400 LOC

### T12 stream/instrument.go (span/recorder helpers)

- 包含 startSpan + recordSuccess/Error + summarizeToolsForTrace + buildStreamRequestInfo
- <300 LOC

### T13 验证 t-registry T 编号归属

- t-registry D3-S1-A01-T01..T05 / D3-S2-A01-T01..T06 / D3-S3-A01-T01..T17 全部归属到正确新文件
- 拆前后 t-registry 编号一致

---

## §3 子 Change #1 god-fn-split pt2（PR-3, T14-T18）

### T14 拆 `configure/shared_config.go::BuildLLMGatewayConfig()` 100 LOC

- 拆为 `configure/defaults.go` + `configure/shared_config.go` (简化)
- DefaultLLMGatewayConfig() 100 LOC → defaults.go
- BuildLLMGatewayConfig() 简化到 50 LOC

### T15 拆 `configure/merge.go::Merge*()` 50 LOC

- 拆为 `configure/merge.go` (2-way) + `configure/merge_user.go` (3-way user-override)
- 简化后 <300 LOC each

### T16 拆 `protect/errorclass/classifier.go::registerRegexRules()` 60 LOC

- 拆为 `protect/errorclass/classifier.go` (核心) + `protect/errorclass/regex_rules.go`
- classifier.go 简化到 <300 LOC
- regex_rules.go <250 LOC

### T17 验证 configure + errorclass packages -race

- `go test ./internal/layers/llmgateway/configure/... ./protect/... -race -count=1` PASS
- 文件 <800 行

### T18 验证 t-registry T 编号归属

- D3-S6-A01-T01..T02 归属到 configure/
- D3-S3-A01-T17 归属到 errorclass/

---

## §4 子 Change #2 registry-sync（PR-4, T19-T24）

### T19 F 路径 D3-S4 (token → budget) 修正

- 5 个 F (D3-S4-A01-F01..F05) 路径从 `token/counter.go` 改为 `budget/counter.go`
- f-registry.md 同步

### T20 F 路径 D3-S5 (safety → guard) 修正

- 3 个 F (D3-S5-A01-F01..F03) 路径从 `safety/filter.go` 改为 `guard/filter.go`
- f-registry.md 同步

### T21 F 路径 D3-S6 (config → configure) 修正

- 2 个 F (D3-S6-A01-F01..F02) 路径从 `config/loader.go` / `shared/config/llmgateway.go` 改为 `configure/loader.go` / `configure/shared_config.go`
- f-registry.md 同步

### T22 验证 F 路径 100% 对齐

- `grep -rE "D3-S[0-9]-A[0-9]+-F[0-9]+" openspec/specs/d3-llm-gateway/f-registry.md | awk -F'|' '{print $6}'` 全量检查
- 0 漂移

### T23 d3-domain.md 物理路径表与 code 100% 对齐

- d3-domain.md §登记规模表 + §DSAFT 资产 + §Boundary Decision 同步
- Last Updated 2026-06-29

### T24 同步 `d3-boundary.md` 中 D2→D3 边界描述

- d3-boundary.md 反映 DM-020 v2.0-d CI 硬阻断
- 同步 D3 跨域 emit 状态

---

## §5 子 Change #3 value-flow-rename（PR-5, T25-T28）

### T25 d3-domain.md §North Star 加 ValueFlow Alias 列

- 5 S + 用户感知层
- D3_Model_Routing / D3_Stream_Chat_Completion / D3_Circuit_Breaker_And_Retry / D3_Token_Budget_Control / D3_Content_Safety_Filter

### T26 a-registry 加 ValueFlow Alias

- 5 S section header 加 `> **ValueFlow Alias (用户感知):** D3_*` 行

### T27 f-registry 加 ValueFlow Alias

- 5 S 段加 Alias block

### T28 t-registry + layer-delta.md §Canonical S 加别名

- t-registry §Canonical T 映射块加 Alias
- layer-delta.md §Canonical S 加 ValueFlow Alias 表

---

## §6 子 Change #4 span-coverage（PR-6, T29-T33）

### T29 验证 5 active D3 span ops runtime 字面量保持

- D3_LLM_Stream / D3_LLM_Provider_Route / D3_LLM_CircuitBreaker / D3_LLM_Retry / D3_LLM_Adapter_Stream
- R1 Q3 决议: 不允许改名为 route.model / stream.chat 等
- 验证 grep + telemetry/names.go

### T30 t-registry 51+ T 行加 Span Evidence 列

- 12/14 T 映射 (D2 模板 88% 目标)
- D3-S1-A01-T01..T05 → D3_LLM_Provider_Route
- D3-S2-A01-T01..T06 → D3_LLM_Stream
- D3-S3-A01-T01..T17 → D3_LLM_CircuitBreaker / D3_LLM_Retry / D3_LLM_Adapter_Stream

### T31 增 T-Without-Span Tracker

- D3-S4-A01-T01..T03 (BudgetTokens 注入)
- D3-S5-A01-T01..T03 (GuardContent 注入)
- D3-S6-A01-T01..T02 (启动期 config load)
- 显式标 `—`

### T32 scripts/d3-span-coverage.sh (NEW, ~80 lines)

- awk 解析 t-registry §Canonical T 映射
- 守门 ≥80%, FAIL exit 1
- D2 模板复用

### T33 Coverage Guard ≥80% PASS

- MAPPED count / Total ≥ 0.8
- 实际预计 ~85% (45/51+ canonical T)

---

## §7 子 Change #5 boundary-decision（PR-7, T34-T37）

### T34 审计 D3 跨域 boundary

- D2→D3 import ban (DM-020 v2.0-d CI 硬阻断) — RESOLVED
- D3 emit `flow.breaker.*` → D1 — RESOLVED
- D3 emit `llm.*` span/metric → D5 — RESOLVED
- D3 → shared/errors APIError — RESOLVED
- D3 → shared/types Request/Chunk — RESOLVED
- 决策: 当前 0 boundary-debt (valid decision 类比 D2 跨域 fixture 待定项)

### T35 internal/layers/llmgateway/orchtypes/boundary_decision.go (NEW)

```go
package orchtypes

const (
    // D3 当前 0 boundary-debt (R1+R2+R3 决议 + DM-20260628-001 APIErrorCode 7 类)
    BoundaryD3NoDebt = "boundary-debt:d3-no-debt-v1.0"
)
```

### T36 internal/layers/llmgateway/orchtypes/boundary_decision_test.go (NEW)

- 3 unit tests:
  - TestBoundaryDecisionConstants_Exist
  - TestBoundaryDecisionConstants_VersionFormat
  - TestBoundaryDecisionConstants_Unique

### T37 d3-domain.md §Boundary Debt Decisions 章节

- 1 row: 0 debt 决策
- 治理常量 in `orchtypes/boundary_decision.go`

---

## §8 S7_Archive（PR-8, T38-T40）

### T38 6 artifacts 复制

- `openspec/archive/2026-06-29-devrix-d3-dsaft-restructuring/`
  - `.openspec.yaml`
  - `acceptance-report.md`
  - `demand.md`
  - `design.md`
  - `proposal.md`
  - `tasks.md`
  - `specs/d3-llm-gateway/spec.md`

### T39 verify-archive.sh 12/12 PASS

- 8 文件完整性 + 状态一致性 + 索引更新 + 域文档同步

### T40 d3-domain.md v1.0.0 → v1.5.0

- 修订记录 v1.5.0 row
- 8 PR / 6 子 Change / 40 T 总结

---

## §9 子 Change 总览

### 9.1 8 PR 概览

| PR | 子 Change | 范围 | AC |
|---|---|---|---|
| PR-1 | #0 legacy-cleanup | 死代码 + unused export | D3 全量 -race PASS |
| PR-2 | #1 god-fn pt1 | stream.Stream 235→3 | 文件 <800 行 |
| PR-3 | #1 god-fn pt2 | configure + errorclass | 文件 <800 行 |
| PR-4 | #2 registry-sync | 4 F path + Historical | t-registry 对齐 |
| PR-5 | #3 value-flow-rename | 5 S + Alias | d3-domain v1.5.0 |
| PR-6 | #4 span-coverage | T↔Span Evidence 80% | coverage check ≥80% |
| PR-7 | #5 boundary-decision | 0 debt + 治理常量 | 3 unit tests PASS |
| PR-8 | S7_Archive | 6 artifacts + verify-archive | spec v1.5.0 |

### 9.2 量化指标

- 死代码 LOC: 0
- god fn: 0（拆后每个 <800 行）
- F 路径对齐: 4/4 正确 (D3-S4/D3-S5/D3-S6-A01-F01/F02)
- ValueFlow Alias: 5/5 S
- T↔Span 覆盖率: ≥80%
- 跨域 Decision: 0/0 debt (valid decision)
- D3 packages -race: 全 PASS

---

**§10 S7_Archived 闭环条件**

- 8 PR 全部 squash auto-merge
- 6 artifacts 复制到 archive/ 目录
- verify-archive.sh 12/12 PASS
- d3-domain.md v1.0.0 → v1.5.0
- demand-archive-index.md 加 DM-20260629-003 row
- t-registry.md §Header 加 devrix-d3-dsaft-restructuring Change 注释
