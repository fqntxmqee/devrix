# Tasks: D4 Multi-Agent S/A 重切 + Hub-Spoke 归 D7

**Change ID:** devrix-d4-sa-refine  
**Demand ID:** DM-20260614-018  
**Status:** S5_Accepted（v1.0 Registry 验收通过；v1.1/v2.0 待后续 Phase）  
**Phases:** v1.0 Registry（B–C）→ v1.1 Traceability（D–E）→ v2.0 Structure（F，含 D7 hubspoke + D2 flow_report）

> **不估时**。任务标注关联 Canonical S / T。v1.0 **零 Go 变更**。

---

## Phase A — 文档澄清（S1 → S2 → S3，进行中）

| ID | Task | 依赖 | 状态 |
|----|------|------|------|
| A1 | 创建 `openspec/changes/devrix-d4-sa-refine/` | — | ✅ |
| A2 | 写 `demand.md`（4 轴 Review + Hub-Spoke 讨论） | — | ✅ |
| A3 | Owner R1 决议（D7-1 / D2 迁 / ExecuteWorker / v2.0 并入） | A2 | ✅ |
| A4 | 更新 `demand.md` → S2_Clarified | A3 | ✅ |
| A5 | 写 `proposal.md`（D + S + R1 合入） | A4 | ✅ |
| A6 | 写 `design.md`（A/F + Gherkin + v2.0 slice） | A5 | ✅ |
| A7 | 写 `gaming-analysis.md` | A5 | ✅ |
| A8 | 写 `tasks.md`（本文件） | A6 | ✅ |
| A9 | S3-Gate Review（Gherkin sad path + 跨域 T 重归属） | A6–A8 | ✅ |
| A10 | `demand.md` 推进到 S3_Gate_Approved | A9 | ✅ |

---

## Phase B — v1.0 Registry 重排

| ID | Task | 依赖 | 产出 | L4/T |
|----|------|------|------|------|
| B1 | 新建 `openspec/specs/d4-multi-agent/d4-domain.md` | A10 | North Star + Out of Scope | — | ✅ |
| B2 | 新建 `openspec/specs/d4-multi-agent/d7-boundary.md` | B1 | D4↔D7 双向引用 | — | ✅ |
| B3 | 重排 `d4-multi-agent/spec.md`（S11–S16 + 删除 S10 Hub-Spoke 叙述） | A10 | spec.md v3.0.0 | S11–S16 | ✅ |
| B4 | 重排 `a-registry.md` Canonical 列 + Legacy | B3 | a-registry v3.0.0 | — | ✅ |
| B5 | 重排 `f-registry.md` | B4 | f-registry v3.0.0 | — | ✅ |
| B6 | 重排 `t-registry.md` + `§Legacy Archive`（38 条） | B5 | t-registry v3.0.0 | 38 T | ✅ |
| B7 | Hub-Spoke T 重归属至 D7 列（spec 层） | B6 | t-registry §D7 canonical | D7-S2/S4 | ✅ |
| B8 | 更新 `span-registry.md`（S8 迁 D5 声明） | B6 | span-registry v3.0.0 | — | ✅ |
| B9 | 同步 `layering.md` §D4 双轨 + §D7 Hub-Spoke A 草案 | B4 | layering | — | ✅ |
| B10 | 补 `code-layout.md §4.5` D4 scenario-slug | B3 | code-layout | — | ✅ |
| B11 | 扩展 `cross-domain-boundaries.md` §D4↔D7 Hub-Spoke | B2 | cross-domain | — | ✅ |
| B12 | 同步 `d7-orchestration/a-registry.md` 增量（S2/S4 新 A） | B2 | D7 a-registry delta | D7 | ✅ |
| B13 | 同步 `d2-context-engine/d7-boundary.md`（SubQuery Flow 迁出） | B2 | D2 boundary patch | D2-S19 | ✅ |
| B14 | `go test` 全量（v1.0 无代码变更，应保持绿） | B3–B13 | 38/38 T 绿 | P0 19 | ✅ |

---

## Phase C — v1.0 验证

| ID | Task | 依赖 | 产出 |
|----|------|------|------|
| C1 | Legacy→Canonical 追溯表 100% 覆盖（38 T） | B6 | 校验报告 | ✅ |
| C2 | Hub-Spoke T 重归属清单与 D7 registry 一致 | B7 | 无悬空 ID | ✅ |
| C3 | `grep` D4 spec 无 S10 Hub-Spoke SoT 表述 | B3 | 边界闭合 | ✅ |
| C4 | `demand-archive-index.md` 追加 D4 入口 | B3 | index | ✅ |
| C5 | 写 `acceptance-report（v1.0）` | C1–C4 | ACCEPTED | ✅ |

---

## Phase D — v1.1 可追溯

| ID | Task | 依赖 | 产出 | T |
|----|------|------|------|---|
| D1 | Span 归属：D4 agent.* 注册归 D5；D4 仅 emit | C5 | span-registry + D5 | — |
| D2 | 统一 Spoke 写侧 span 族 `orchestration.flow.*` | D1 | D7 span-registry | — |
| D3 | D6 增 probe：Delegate 成功率 / fallback 率 | C5 | d6-evolution | — |
| D4 | D6 增 probe：Fork COW 污染检出 | C5 | d6-evolution | D4-S13-T05 |
| D5 | D6 增 probe：PermissionGate 超时率 | C5 | d6-evolution | D4-S12 |
| D6 | import lint：`multiagent` 不 import `orchestration`（目标态） | C5 | lint 规则草案 | D4 Thin |
| D7 | 写 `acceptance-report（v1.1）` | D1–D6 | ACCEPTED | — |

---

## Phase E — v2.0 Structure（并入本 change，slice a–e）

### Slice a — D7 hubspoke 骨架

| ID | Task | 依赖 | 产出 | T |
|----|------|------|------|---|
| E-a1 | 定义 `hubspoke.SpokeBridge` 接口 | C5 | `orchestration/hubspoke/bridge.go` | — |
| E-a2 | 定义 `hubspoke.Dispatcher` + Spoke 枚举 | E-a1 | `hubspoke/dispatch.go` 骨架 | — |
| E-a3 | bootstrap 接线 Dispatcher deps | E-a2 | `bootstrap/hubspoke.go` | — |

### Slice b — 迁 D4 bridge + dispatch

| ID | Task | 依赖 | 产出 | T |
|----|------|------|------|---|
| E-b1 | 迁 `delegate/bridge.go` → `hubspoke/agent_bridge.go` | E-a1 | D7 | D7-S4 |
| E-b2 | 迁 `DelegateOrFallback` → `hubspoke/dispatch.go` | E-a2 | D7 | D7-S2 |
| E-b3 | 瘦身 `delegate/service.go` → `multiagent/execute/worker.go` | E-b2 | D4-S14 | D4-S14 T |
| E-b4 | `delegatetools` 改调 Dispatcher（非 D4 Service 直连） | E-b2 | D7 | D4-S10 legacy T |
| E-b5 | 删除 D4 对 `flow.GlobalHub` 直接依赖 | E-b1 | import lint 绿 | D6 |

### Slice c — 迁 D2 flow_report

| ID | Task | 依赖 | 产出 | T |
|----|------|------|------|---|
| E-c1 | 迁 `nested/flow_report.go` → `hubspoke/subquery_bridge.go` | E-a1 | D7 | D2-S19 |
| E-c2 | 删除 `SubQueryParams.FlowHub`；D7 包装 SubQuery | E-c1 | D2 nested | D2 SubQuery T |
| E-c3 | 更新 `d2-context-engine/d7-boundary.md` + `d7-boundary.md` | E-c2 | specs | — |

### Slice d — D4 物理路径

| ID | Task | 依赖 | 产出 | T |
|----|------|------|------|---|
| E-d1 | `provision/` ← factory + collaboration + builtin | E-b3 | multiagent | S11 |
| E-d2 | `run/` ← lifecycle + perm + state | E-d1 | multiagent | S12 |
| E-d3 | `isolate/` ← forkjoin + sessionview + worker_engine | E-d2 | multiagent | S13 |
| E-d4 | `execute/` ← worker.go | E-b3 | multiagent | S14 |
| E-d5 | `external/` ← tool/ | E-d4 | multiagent | S15 |
| E-d6 | `kernel/` ← contracts + observer | E-d5 | multiagent | kernel |
| E-d7 | 根 `multiagent/` re-export 1 周期 | E-d6 | compat | — |

### Slice e — 验证与清理

| ID | Task | 依赖 | 产出 | T |
|----|------|------|------|---|
| E-e1 | 38 T + Hub-Spoke 跨域 T 全绿 | E-b–E-d | test report | P0 19 |
| E-e2 | `-race` agent 测试 | E-e1 | D4-S0-T01/T02 | P0 |
| E-e3 | E2E agent_fork + delegate 回归 | E-e1 | D4-S0-T04 | P0 |
| E-e4 | 删除旧路径 dead code + re-export | E-e1 | cleanup PR | — |
| E-e5 | 更新 `layering.md` Domain Layout + code-layout §4.4 状态 | E-d7 | specs | — |
| E-e6 | 写 `acceptance-report（v2.0）` | E-e1–E-e5 | ACCEPTED | — |
| E-e7 | S7 归档 + `demand-archive-index.md` | E-e6 | archive | — |

---

## Phase 依赖图

```text
A (澄清) → B (v1.0 Registry) → C (v1.0 验收)
                ↓
              D (v1.1 可追溯)
                ↓
         E-a (hubspoke 骨架)
           ├→ E-b (D4 迁 D7)
           └→ E-c (D2 迁 D7)  [可与 E-b 并行，依赖 E-a]
                ↓
              E-d (D4 物理路径) [可与 E-b/c 并行]
                ↓
              E-e (验证归档)
```

---

## 关联 L5/T 测试点（开发时标注）

| Phase | 必跑 T |
|-------|--------|
| B/C v1.0 | 全量 38 T（无代码变更，回归基线） |
| E-b | D4-S14-* + D7-S2/S4 Hub-Spoke T |
| E-c | D2-S19 SubQuery T + D4-S10-A01-T07（fallback，canonical D7） |
| E-e P0 | 19 条 P0 + D4-S0 cross T |

---

**Revision History**

| 版本 | 日期 | 变更 |
|------|------|------|
| 0.2 | 2026-06-14 | Phase B–C 完成；S3-Gate + v1.0 ACCEPTED；DM ID 修正为 018 |
