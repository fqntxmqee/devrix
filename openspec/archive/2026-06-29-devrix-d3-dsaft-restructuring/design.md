# Design: devrix-d3-dsaft-restructuring (DM-20260629-003)

**Change ID:** devrix-d3-dsaft-restructuring
**Demand ID:** DM-20260629-003
**Status:** S3_Design
**Created:** 2026-06-29
**Template:** `devrix-d2-dsaft-restructuring` design.md (DM-20260629-002)

---

## §1 整体结构

```
devrix-d3-dsaft-restructuring (DM-20260629-003)
├── PR-1 #0 legacy-cleanup           (T01-T08, ~200 LOC)
├── PR-2 #1 god-fn-split pt1         (T09-T13: stream.Stream 235→3)
├── PR-3 #1 god-fn-split pt2         (T14-T18: configure + errorclass)
├── PR-4 #2 registry-sync            (T19-T24: 4 F path + Historical)
├── PR-5 #3 value-flow-rename        (T25-T28: 5 S + Alias)
├── PR-6 #4 span-coverage            (T29-T33: T↔Span Evidence 80%)
├── PR-7 #5 boundary-decision        (T34-T37: 0 debt + 治理常量)
└── PR-8 S7_Archive                  (T38-T40: 6 artifacts + verify-archive)

Total: 8 PR / ~40 T / 7-9 天
```

---

## §2 PR-1 #0 legacy-cleanup

### 2.1 范围

- 审计 57 Go files / 3995 LOC
- 删死代码/未使用 export
- 同步 5 active span ops (D3 t-registry 反映)

### 2.2 关键文件

- `internal/layers/llmgateway/**/*.go` 全部审计
- `internal/layers/llmgateway/spans.go` (20 LOC) 检查是否还需要

### 2.3 验证

```bash
grep -rE "^func [A-Z]" internal/layers/llmgateway/ --include="*.go" | xargs -I{} grep -rE "func_name" --include="*.go" | head
go test ./internal/layers/llmgateway/... -race -count=1
```

---

## §3 PR-2 #1 god-fn-split pt1 (stream.Stream 235→3 文件)

### 3.1 拆后结构

| 原文件:函数 | 拆后文件 | 范围 |
|---|---|---|
| `stream/gateway.go::Stream()` 235 LOC | `stream/pipeline.go` (routing + 入口) | 路由 + 入口 + chunk fanout |
| | `stream/protected.go` (breaker + retry + adapter) | breaker.allow + retry + adapter.stream |
| | `stream/instrument.go` (span/recorder helpers) | startSpan + recordSuccess/Error + summarizeToolsForTrace |

### 3.2 关键设计决策

- Stream() public API 保持不变（`func (g *Gateway) Stream(ctx, req) (<-chan Chunk, error)`）
- 7 step 顺序保持: route → breaker.allow → retry.wrap → adapter.stream → chunk fanout → recordSuccess/Error → return
- 内部 helper 拆为 unexported method (`g.streamPipeline` / `g.streamProtected` / `g.recordOutcome`)

### 3.3 t-registry 关联

- D3-S1-A01-T01 / T02 / T03 / T04 / T05 (RouteModel)
- D3-S2-A01-T01..T06 (StreamChat)
- D3-S3-A01-T01..T17 (ProtectCall)

---

## §4 PR-3 #1 god-fn-split pt2

### 4.1 拆后结构

| 原文件:函数 | 拆后文件 | 范围 |
|---|---|---|
| `configure/shared_config.go::BuildLLMGatewayConfig()` 100 LOC | `configure/defaults.go` | DefaultLLMGatewayConfig + 5 const |
| | `configure/shared_config.go` | BuildLLMGatewayConfig (简化) |
| `configure/merge.go::Merge*()` 50 LOC | `configure/merge.go` (核心) | 2-way merge |
| | `configure/merge_user.go` | 3-way user-override merge |
| `protect/errorclass/classifier.go::registerRegexRules()` 60 LOC | `protect/errorclass/classifier.go` (核心) | Classify + sentinelMatches + classByHTTPStatus |
| | `protect/errorclass/regex_rules.go` | registerRegexRules + rules table |

### 4.2 关键文件

- `internal/layers/llmgateway/configure/shared_config.go` → 拆 +1
- `internal/layers/llmgateway/configure/merge.go` → 拆 +1
- `internal/layers/llmgateway/protect/errorclass/classifier.go` → 拆 +1

---

## §5 PR-4 #2 registry-sync

### 5.1 F 路径修正 (4 处)

| F ID | 旧路径 | 新路径 |
|---|---|---|
| D3-S4-A01-F01..F05 | `token/counter.go` | `budget/counter.go` |
| D3-S5-A01-F01..F03 | `safety/filter.go` | `guard/filter.go` |
| D3-S6-A01-F01 | `config/loader.go` | `configure/loader.go` |
| D3-S6-A01-F02 | `shared/config/llmgateway.go` | `configure/shared_config.go` |

### 5.2 Historical appendix 补

D3 域 6 S 全部 ACTIVE，无 RETIRED/REMOVED 状态变更。

### 5.3 t-registry Span Evidence 列填充

- D3-S1-A01-T01..T05 → `D3_LLM_Provider_Route`
- D3-S2-A01-T01..T06 → `D3_LLM_Stream`
- D3-S3-A01-T01..T17 → `D3_LLM_CircuitBreaker` / `D3_LLM_Retry` / `D3_LLM_Adapter_Stream`
- D3-S4-A01-T01..T03 → (注入 `D3_LLM_Stream`)
- D3-S5-A01-T01..T03 → (注入 `D3_LLM_Stream`)
- D3-S6-A01-T01..T02 → (启动期，不进 trace)

---

## §6 PR-5 #3 value-flow-rename

### 6.1 ValueFlow Alias

| S ID | Scenario | ValueFlow Alias (用户感知) |
|------|----------|------------------------------|
| D3-S1 | RouteModel | **D3_Model_Routing** |
| D3-S2 | StreamChat | **D3_Stream_Chat_Completion** |
| D3-S3 | ProtectCall | **D3_Circuit_Breaker_And_Retry** |
| D3-S4 | BudgetTokens | **D3_Token_Budget_Control** |
| D3-S5 | GuardContent | **D3_Content_Safety_Filter** |
| D3-S6 | ConfigureGateway | (横切，启动期) |

### 6.2 命名空间隔离

- `D3_` 前缀避免与 D2 (`D2_*`)/D7 (`D7_*`) 冲突
- 5 S 全部 IMPLEMENTED, ValueFlow Alias 立即启用

---

## §7 PR-6 #4 span-coverage

### 7.1 Span Evidence 列填充 (PR-4 + PR-6 合并)

- 51+ T 行加 Span Evidence 列
- 5 active ops 100% 保留 (R1 Q3 runtime 字面量稳定性)
- T-Without-Span Tracker (compile-time invariant + 启动期 config load)

### 7.2 CI Guard

- `scripts/d3-span-coverage.sh` awk 解析 t-registry §Canonical T 映射
- 守门 ≥80% (D2 88% / D7 94% 实际)
- 复用 D2 模板

### 7.3 期望覆盖率

- 5 active ops + 51 T → ≥80% 覆盖率
- T-Without-Span Tracker: 2-3 entry (compile-time + config load)

---

## §8 PR-7 #5 boundary-decision

### 8.1 D3 跨域 boundary 现状

| Boundary | 状态 | 决策 |
|---|---|---|
| D2→D3 import ban | ✅ 已 CI 硬阻断 (DM-020) | RESOLVED |
| D3 emit `flow.breaker.*` → D1 | ✅ WireD3 已实现 | RESOLVED |
| D3 emit `llm.*` span/metric → D5 | ✅ telemetry 已 wire | RESOLVED |
| D3 → shared/errors APIError | ✅ v3.3.1 (DM-20260628-001) | RESOLVED |
| D3 → shared/types Request/Chunk | ✅ stable | RESOLVED |

**当前 0 boundary-debt** — 也是 valid decision (类比 D2 跨域 fixture 待定项)。

### 8.2 治理常量

`internal/layers/llmgateway/orchtypes/boundary_decision.go` (D2/D7 命名空间统一):

```go
package orchtypes

const (
    // D3 当前 0 boundary-debt
    BoundaryD3NoDebt = "boundary-debt:d3-no-debt-v1.0"
)
```

---

## §9 S7_Archive

### 9.1 6 artifacts

- `openspec/archive/2026-06-29-devrix-d3-dsaft-restructuring/`
  - `.openspec.yaml` (status: s7_archived)
  - `acceptance-report.md` (ACCEPTED)
  - `demand.md`
  - `design.md`
  - `proposal.md`
  - `tasks.md`
  - `specs/d3-llm-gateway/spec.md`

### 9.2 verify-archive 12/12 PASS

复用 D2 PR 模板, 加 S2_Proposal/S3_Design/S4_Implemented/S5_Accepted/S7_Archived 全闭环。

### 9.3 spec 同步

- d3-domain.md v1.0.0 → **v1.5.0**
- 修订记录 v1.5.0 row

---

## §10 总结

D3 DSAFT (DM-20260629-003) — 8 PR / 6 子 Change / ~40 T / 7-9 天, 对齐 D2 v9.0.0 + D7 v2.6.0 模板。
