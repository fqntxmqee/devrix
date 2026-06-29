# Proposal: devrix-d3-dsaft-restructuring (DM-20260629-003)

**Change ID:** devrix-d3-dsaft-restructuring
**Demand ID:** DM-20260629-003
**Status:** S2_Proposal
**Created:** 2026-06-29
**Template:** `devrix-d2-dsaft-restructuring` proposal.md (DM-20260629-002 S7_Archived)

---

## 1. 目标

D3 域 6 子 Change 联动 refactoring, 对齐 D7 v6.0.x → v7.0 演进 + D2 v9.0.0 模板。

**Single DM 范围**: 6 子 Change 全部纳入, 9 PR 落地。

---

## 2. 6 子 Change 概览

### #0 legacy-cleanup (PR-1)

- 审计 57 Go files / 3995 LOC 中未使用 exported 类型/函数
- 删死代码 + 删 unused param + 删 dead span 引用
- 验证: `go test -race ./internal/layers/llmgateway/...` PASS

### #1 god-fn-split (PR-2, PR-3)

- **PR-2**: `stream.Gateway.Stream()` 235 LOC 拆 3 文件
  - `stream_pipeline.go` (routing + 入口)
  - `stream_protected.go` (breaker + retry + adapter)
  - `stream_instrument.go` (span/recorder helpers)
- **PR-3**: configure + errorclass 拆 4 文件
  - `shared_config.go` 拆 `defaults.go` + `shared_config.go`
  - `merge.go` 拆 `merge.go` + `merge_user.go`
  - `errorclass/classifier.go` 拆 `classifier.go` + `regex_rules.go`

### #2 registry-sync (PR-4)

- 9 F 路径修正（4+ drift: token/counter → budget/counter, safety/filter → guard/filter, config/loader → configure/loader, shared/config/llmgateway → configure/shared_config）
- 6 S Historical appendix 补
- t-registry Span Evidence 列填充 (D2 模板)

### #3 value-flow-rename (PR-5)

- d3-domain.md §North Star 加 ValueFlow Alias 列:
  - S1 RouteModel → `D3_Model_Routing`
  - S2 StreamChat → `D3_Stream_Chat_Completion`
  - S3 ProtectCall → `D3_Circuit_Breaker_And_Retry`
  - S4 BudgetTokens → `D3_Token_Budget_Control`
  - S5 GuardContent → `D3_Content_Safety_Filter`
- a/f/t-registry + layer-delta.md 同步

### #4 span-coverage (PR-6)

- 5 active D3 span ops 保持 runtime 字面量 (R1 Q3 决议)
- t-registry 51+ T 行 Span Evidence 列填充
- CI guard `scripts/d3-span-coverage.sh` ≥80%
- T-Without-Span Tracker (compile-time invariant 等)

### #5 boundary-decision (PR-7)

- D3 跨域 boundary 审计 (D2→D3 ban / D3 emit D1 / D3 emit D5)
- 0 debt 决策: 当前 0 boundary-debt
- 治理常量 `internal/layers/llmgateway/orchtypes/boundary_decision.go` (D2/D7 命名空间对齐)
- 0 boundary-debt 也是 valid decision (类比 D2 待定项)

### S7_Archive (PR-8 + 后续 PR-9)

- 6 artifacts: .openspec.yaml + acceptance-report.md + demand.md + design.md + proposal.md + tasks.md
- verify-archive 12/12 PASS
- d3-domain v1.0.0 → v1.5.0 final

---

## 3. Spec 同步目标 (v1.0.0 → v1.5.0)

- `d3-domain.md`: Version 1.0.0 → **1.5.0** + Last Updated 2026-06-29 + v1.5.0 row
- `a/f/t-registry.md`: 同步 PR-4 (F path 修正) + PR-5 (ValueFlow) + PR-6 (Span Evidence)
- `span-registry.md`: 5 active ops 保持 + T↔Span Evidence 列
- `observability-guide.md`: T-Without-Span Tracker + Coverage Guard 守门

---

## 4. 风险

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Stream() 235 LOC 拆破坏 7 step 顺序 | Mid | High | 拆前后 -race PASS 守门 |
| F path 修正破坏 D3 跨包引用 | Low | Mid | grep 验证 + compile guard |
| 5 active span ops runtime 字面量变化 | Low | Critical | 严格保留 (R1 Q3 决议) |
| 跨域 debt 治理与 D2/D7 不一致 | Low | Low | 命名空间统一 `boundary-debt:` 前缀 |

---

## 5. PR 落地序列

| Day | PR | 范围 | 验收 |
|-----|----|----|------|
| 1 | PR-1 #0 legacy-cleanup | 死代码 + unused export 删 | D3 全量 -race PASS |
| 2 | PR-2 #1 god-fn pt1 | Stream 235 → 3 文件 | 文件 <800 行 |
| 3 | PR-3 #1 god-fn pt2 | configure + errorclass | 文件 <800 行 |
| 4 | PR-4 #2 registry-sync | 4 F path + Historical | t-registry 对齐 |
| 5 | PR-5 #3 value-flow-rename | 5 S + 用户感知层 | d3-domain v1.5.0 |
| 6 | PR-6 #4 span | T↔Span Evidence 80% | coverage check ≥80% |
| 7 | PR-7 #5 boundary-decision | 0 debt decision | 治理常量 |
| 8 | PR-8 S7_Archive | 6 artifacts + verify-archive 12/12 | spec v1.5.0 |

---

## 6. 总结

8 PR / ~30-40 T / 7-9 天，对齐 D2 DSAFT (DM-20260629-002) + D7 DSAFT (DM-20260629-001) 模板。
