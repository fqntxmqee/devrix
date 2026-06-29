# Demand: devrix-d3-dsaft-restructuring (DM-20260629-003)

**Demand ID:** DM-20260629-003
**Status:** S1_Demand
**Priority:** P0 (深度架构重构)
**Created:** 2026-06-29
**Change ID:** devrix-d3-dsaft-restructuring
**Triggered By:** D3 域整体 DSAFT 方法论 Review（2026-06-29 会话）+ 对齐 D7 v6.0.x → v7.0 (DM-20260629-001) + D2 DSAFT (DM-20260629-002) 模板
**Related:**
- `devrix-d7-dsaft-restructuring` (DM-20260629-001) — D7 6 子 Change 联动模板
- `devrix-d2-dsaft-restructuring` (DM-20260629-002) — D2 6 子 Change 联动 (S7_Archived 2026-06-29)
- `devrix-d3-sa-refine` (R1+R2+R3) — D3 v3.0.0→v3.1.0 早期 refine
- `devrix-api-error-classification` (DM-20260628-001) — D3 v3.3.1 APIErrorCode 7 类
- `devrix-llm-gateway-user-override-v2.x` (DM-20260619-006) — Go 代码无模型名硬编码
- `docs/methodology/dsaft-methodology.md` v4.0.0 — 6 原则
- `docs/methodology/dsaft-refactoring-playbook.md` v1.0.0 — 4 轴 / 6 阶段

---

## §1 背景

D3 v1.0.0（2026-06-16 d3-domain.md）+ v3.3.1 t-registry（2026-06-28 DM-20260628-001）已达"5+1 S 全部 IMPLEMENTED + 5 active span ops + 8 A / 35 F / 51 T"的稳定 SoT 状态。但 2026-06-29 DSAFT Review 暴露 **5 类深度架构债**，对齐 D7 v6.0.x → v7.0 演进节奏 + D2 DSAFT (DM-20260629-002) 联动模板需要 6 子 Change 联动 refactoring。

### 1.1 1 个 god function 跨 2 个 S（P0）

| 文件:函数 | LOC | S | 风险等级 |
|---|---|---|---|
| `stream/gateway.go::Gateway.Stream()` | 235 | S2/S3 | High（路由 + 熔断 + retry + adapter + 7 step span） |

其他 god fn 中等：
- `configure/shared_config.go::BuildLLMGatewayConfig()` 100 LOC（S6，Mid）
- `configure/merge.go::Merge*()` 50+ LOC（S6，Mid）
- `protect/errorclass/classifier.go::registerRegexRules()` 60+ LOC（S3，Low）

### 1.2 F 路径漂移 4 处（P0）

- `D3-S4-A01-F01..F05` 标 `token/counter.go` → 实际 `budget/counter.go`（v2.x 包迁移）
- `D3-S5-A01-F01..F03` 标 `safety/filter.go` → 实际 `guard/filter.go`（v2.x 包迁移）
- `D3-S6-A01-F01` 标 `config/loader.go` → 实际 `configure/loader.go`（v2.x 包迁移）
- `D3-S6-A01-F02` 标 `shared/config/llmgateway.go` → 实际 `configure/shared_config.go`（v2.x 包迁移）

### 1.3 ValueFlow Alias 缺失（P1）

`d3-domain.md §North Star` 表**缺 ValueFlow Alias 列**，对齐 D2 v9.0.0 + D7 v2.6.0 §North Star ValueFlow Alias 模式。

5 canonical S 需用户感知层命名：
- S1 RouteModel → ?
- S2 StreamChat → ?
- S3 ProtectCall → ?
- S4 BudgetTokens → ?
- S5 GuardContent → ?

### 1.4 T↔Span Evidence 列缺失（P1）

51+ T 行未关联 Span Evidence；D3 5 active span ops（`D3_LLM_Stream` / `D3_LLM_Provider_Route` / `D3_LLM_CircuitBreaker` / `D3_LLM_Retry` / `D3_LLM_Adapter_Stream`）覆盖率未量化。

### 1.5 死代码债（Low）

D3 域**无 legacy/ 目录**（对齐 D2 PR-1 删除模式后），但需要审计：
- 未使用的 exported function/type（如 classify_helpers.go 的 helper 是否还有用）
- 死代码 brace （如 spans.go 是否还有用）

### 1.6 跨域 boundary 债（Low）

D3 现状：
- D2→D3 import ban（DM-020）— 已 CI 硬阻断
- D3 emit `flow.breaker.*` → D1 展示 — 已 wire
- D3 emit `llm.*` span/metric → D5 — 已 wire
- **Potential debt**: D3 内部有没有跨域 capability 临时持有的？

---

## §2 重构价值

| 维度 | Before | After | 收益 |
|------|--------|-------|------|
| god fn | 1 High + 3 Mid/Low | 0 | 文件 <800 行 |
| F 路径对齐 | 4 处漂移 | 100% 对齐 | grep 0 漂移 |
| ValueFlow | 缺 Alias 列 | 5 S 全部 `D3_*` Alias | 用户感知一致 |
| T↔Span Evidence | 0% | ≥80% | 覆盖率可量化 |
| 死代码 | 待审计 | 0 | 复用性提升 |
| 跨域 debt | 待评估 | N 个 decision | 治理颗粒度 |
| Span 数量 | 5 active | 5 active（runtime 字面量保持 R1 Q3） | 命名契约稳定 |

---

## §3 范围

6 子 Change:
1. **#0 legacy-cleanup** — D3 死代码/未使用 export 审计 + 删
2. **#1 god-fn-split** — 1 High + 3 Mid/Low god fn 拆 8+ 文件
3. **#2 registry-sync** — 4 F path 修正 + Historical appendix 补
4. **#3 value-flow-rename** — 5 S + 用户感知层
5. **#4 span-coverage** — T↔Span Evidence 列 + 覆盖率 CI guard
6. **#5 boundary-decision** — D3 跨域 debt 治理

**Total**: 7 PR + 1 S7_Archive = 8 PR / ~30-40 T / 7-9 天

---

## §4 验收

### 4.1 量化指标

- 死代码 LOC: 0
- god fn: 0（拆后每个 <800 行）
- F 路径对齐: 4/4 正确
- ValueFlow Alias: 5/5 S
- T↔Span 覆盖率: ≥80%
- 跨域 Decision: N/N
- D3 packages -race: 全 PASS

### 4.2 验证命令

```bash
# D3 全量 -race
go test ./internal/layers/llmgateway/... -race -count=1

# T↔Span 覆盖率
./scripts/d3-span-coverage.sh  # ≥80%

# 跨域 import 门禁
go test ./internal/lint/layer/... -v  # D2→D3 ban 0 命中

# verify-archive 12/12
./scripts/verify-archive.sh devrix-d3-dsaft-restructuring
```

---

## §5 关键决策

1. **god fn 拆分策略**: 1 High (Stream 235 LOC) 优先拆为 3 文件（routing / retry+breaker / adapter）；其他 Mid/Low god fn 拆为 1-2 文件
2. **F 路径修正**: 4 处包名漂移（token→budget / safety→guard / config→configure / shared/config→configure/shared_config）全部 1:1 字符串替换；不动语义
3. **ValueFlow Alias 命名**: 加 `D3_` 前缀避免与 D2/D7 冲突
4. **Span 覆盖率目标**: ≥80%（对齐 D2/D7 实际）
5. **跨域 debt**: D3 域无明显 boundary debt 风险，PR-7 主要确认 0 债务 + 治理常量预留
6. **D3 spec 版本**: v1.0.0 → v1.5.0（次版本对齐 D2 v9.0.0 + D7 v2.6.0 演化）
