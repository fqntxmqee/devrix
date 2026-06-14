---
change-id: devrix-d7-turn-orchestration
demand-id: DM-20260614-020
phase: v1.0 Registry (Phase B+C)
verdict: PASS
date: 2026-06-15
---

# Acceptance Report — D7 Turn 编排上移 v1.0 Registry

## 1. 验收范围

| 维度 | 范围 |
|------|------|
| Change | `devrix-d7-turn-orchestration`（DM-20260614-020） |
| Phase | **v1.0 Registry（B + C）** |
| 约束 | 零 Go 变更；仅规格/文档/注册表更新 |

## 2. Phase B — v1.0 Registry 产出

| ID | Task | 产出文件 | 状态 |
|----|------|---------|------|
| B1 | 新建 `d7-orchestration/d3-boundary.md` | `openspec/specs/d7-orchestration/d3-boundary.md` | ✅ |
| B2 | 修订 `d7-domain.md` | v2.4.0→2.5.0：Turn Leader 角色、A06/A07 登记、D2 Follower 拆面契约、D7→D3 边界 | ✅ |
| B3 | 修订 `d2-domain.md` | D2-S16 Legacy Freeze、D2→D3 禁止、North Star 更新、调用链 D7→D3 | ✅ |
| B4 | 修订 `d2-context-engine/d7-boundary.md` | v1.0.0→1.1.0：调用链 DM-020 修订、D2→D3 禁止、LLM 调用权归 D7 | ✅ |
| B5 | 修订 `cross-domain-boundaries.md` | 已有 DM-020 修订（§2.1 + §2.4 + §3.6），v1.3.0 无需额外变更 | ✅ |
| B6 | 修订 `d7-orchestration/a-registry.md` | v2.1.0→3.1.0：D7-S2-A06 RunTurnLoop + D7-S2-A07 InvokeLLM 登记 | ✅ |
| B7 | 修订 `d2-context-engine/a-registry.md` | v2.0.0→2.1.0：D2-S16 Legacy Freeze + D2-S18-A04 ExecuteToolRound 新增 | ✅ |
| B8 | 修订 `d7-orchestration/t-registry.md` | v2.3.0→2.5.0：D7-S2-A06/A07 T 点（6 PLANNED）+ Legacy T 映射表 | ✅ |
| B8 | 修订 `d2-context-engine/t-registry.md` | v2.0.0→2.1.0：DM-020 Legacy T 映射（11 行）+ D2-S15-A01-T10 | ✅ |
| B9 | 修订 `layering.md` | v4.0.0→4.1.0：D2/D7 Turn 双轨声明、Stackelberg 博弈角色、Legacy 兼容策略 | ✅ |
| B10 | 修订 `code-layout.md` | v1.7.0→1.8.0：D7-S2-A06/A07 turn/ 目录、D2-S16 Legacy Freeze、bootstrap 接线注释 | ✅ |
| B11 | 修订 D3 contracts | spec.md v3.1.0→3.2.0：消费方 D2→D7；design.md ILLMGateway 注释更新；a-registry.md D3-X-A03 AdaptToOrchestrator 新增 | ✅ |

## 3. Phase C — v1.0 验证

| ID | Task | 结果 |
|----|------|------|
| C1 | Legacy T 映射 100% | ✅ 8 条 Legacy→Canonical 映射完整覆盖（design.md §6） |
| C2 | grep D2-domain 无「D2 执行 LLM 循环」SoT | ✅ 0 匹配（North Star 已更新为 Context Follower） |
| C3 | `go test ./...` 全绿（零 Go 变更） | ✅ 71 包全 PASS，0 FAIL |
| C4 | `acceptance-report（v1.0）` | ✅ 本文件 |
| C5 | `demand-archive-index` 状态更新 | ✅ |

## 4. 关键设计决策落地验证

| 决策 | design.md | v1.0 Registry 落地点 | 状态 |
|------|-----------|---------------------|------|
| D1: Turn SoT 归 D7-S2-A06 | §1 | `d7-domain.md` + `a-registry.md`（B2/B6） | ✅ |
| D2: D2-S16 Legacy 冻结 | §1 | `d2-domain.md` + `a-registry.md`（B3/B7） | ✅ |
| D4: ILLMGateway 消费方 D7 | §1 | `d3-llm-gateway/spec.md` + `design.md`（B11） | ✅ |
| D6: Autocompact D7→D3 | §1 | `d3-boundary.md` §6.1（B1） | ✅ |
| D2→D3 禁止 | §2.4 | `d7-boundary.md` §4.1 + `cross-domain-boundaries.md` §2.1（B4/B5） | ✅ |
| Follower 对称性 | G-02 | `layering.md` + `cross-domain-boundaries.md` §3.6（B9/B5） | ✅ |

## 5. 未覆盖项（v2.0 范围）

| 项 | 归属 Slice | 状态 |
|----|-----------|------|
| `orchestration/turn/` 骨架代码 | D-a | ⬜ v2.0 |
| `WireContextLLM` bootstrap D2→D7 | D-b | ⬜ v2.0 |
| FastPath 改 `TurnOrchestrator` | D-c | ⬜ v2.0 |
| D2 移除 `ILLMGateway` | D-d | ⬜ v2.0 |
| import lint D2→D3 CI 硬阻断 | D-d | ⬜ v2.0 |
| Autocompact D7→D3 实施 | D-e | ⬜ v2.0 |
| Legacy adapter + 全量 T 绿 | D-f | ⬜ v2.0 |

## 6. 裁决

**Verdict: PASS — v1.0 Registry 验收通过**

- ✅ 11 项 B 任务全部完成（1 新建 + 10 修订）
- ✅ 5 项 C 任务全部通过
- ✅ 零 Go 代码变更（纯规格/文档/注册表）
- ✅ 6 项关键设计决策全部闭环落地
- ✅ 71 包测试全绿

可进入 **Phase D v2.0 Structure（slice a–f）**。

---

**Revision History**

| 版本 | 日期 | 变更 |
|------|------|------|
| 1.0 | 2026-06-15 | v1.0 Registry 验收通过 |
