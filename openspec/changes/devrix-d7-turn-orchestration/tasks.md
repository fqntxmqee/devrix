# Tasks: D7 Turn 编排上移

**Change ID:** devrix-d7-turn-orchestration  
**Demand ID:** DM-20260614-020  
**Status:** S5_Accepted（v2.0 Structure 完成）

> v1.0 Registry + v2.0 Structure 全部完成。

---

## Phase A — 澄清（S1 → S2 → S3）

| ID | Task | 状态 |
|----|------|------|
| A1 | 创建 change 包 | ✅ |
| A2 | `demand.md` | ✅ |
| A3 | R1 决议 OQ1–OQ4 | ✅ |
| A4 | `proposal.md` | ✅ |
| A5 | `design.md` | ✅ |
| A6 | `gaming-analysis.md` | ✅ |
| A7 | `tasks.md` | ✅ |
| A8 | S3-Gate `review-s3.md` | ✅ |
| A9 | demand → S3_Gate_Approved | ✅ |

---

## Phase B — v1.0 Registry

| ID | Task | 产出 |
|----|------|------|
| B1 | 新建 `d7-orchestration/d3-boundary.md` | D7↔D3 SoT | ✅ |
| B2 | 修订 `d7-domain.md`（Turn Leader + 删「D3 不交互」） | domain | ✅ |
| B3 | 修订 `d2-domain.md`（Context Follower + S16 Legacy） | domain | ✅ |
| B4 | 修订 `d2-context-engine/d7-boundary.md` 调用链 + 禁 D2→D3 | boundary | ✅ |
| B5 | 修订 `cross-domain-boundaries.md` §2.1 + §D7↔D3 | 全局 | ✅ |
| B6 | `d7-orchestration/a-registry.md` 增 A06/A07 | A | ✅ |
| B7 | `d2-context-engine/a-registry.md` S16 Legacy + S18 扩展 | A | ✅ |
| B8 | `d7/d2 t-registry.md` §Legacy Archive | T | ✅ |
| B9 | `layering.md` §D2/D7 Turn 双轨 | layering | ✅ |
| B10 | `code-layout.md` §4.3/§4.6 bootstrap 注释 | layout | ✅ |
| B11 | `d3-llm-gateway` contracts 注释消费方→D7 | D3 spec | ✅ |

---

## Phase C — v1.0 验证

| ID | Task |
|----|------|
| C1 | Legacy T 映射 100% | ✅ |
| C2 | grep D2-domain 无「D2 执行 LLM 循环」SoT | ✅ |
| C3 | `go test` 全绿（零变更） | ✅ |
| C4 | `acceptance-report（v1.0）` | ✅ |
| C5 | `demand-archive-index` 状态更新 | ✅ |

---

## Phase D — v2.0 Structure（slice a–f）

| Slice | Task | T | 状态 |
|-------|------|---|------|
| D-a | `orchestration/turn/` 骨架 | — | ✅ |
| D-b | bootstrap WireContextLLM → D7 | A07 | ✅ |
| D-c | FastPath → TurnOrchestrator | A06 P0 | ✅ |
| D-d | D2 移除 ILLMGateway + import lint | THIN-T01 | ✅ |
| D-e | Autocompact D7→D3 | S15-T10 | ✅ |
| D-f | Legacy adapter + 全量 T 绿 | P0 19+ | ✅ |

---

## 依赖图

```text
A → B (v1.0 Registry) → C (验收)
         ↓
       D-a → D-b → D-c
         ├→ D-d → D-e
         └→ D-f
```

**与 DM-018：** D-c 完成后可并行 hubspoke slice-a。

---

**Revision History**

| 版本 | 日期 | 变更 |
|------|------|------|
| 0.1 | 2026-06-14 | 初稿 |
| 0.2 | 2026-06-15 | Phase D (a–f) 全部完成 |
