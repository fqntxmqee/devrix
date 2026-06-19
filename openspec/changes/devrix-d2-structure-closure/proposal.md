# Proposal: D2 v2.2 Structure 终态

**Change ID:** devrix-d2-structure-closure  
**Demand ID:** DM-20260619-007  
**Status:** S3_Approved  
**S3-Gate:** P1-a Approved 2026-06-19（owner 确认；P1-b/c 推进中）  
**Methodology:** `docs/methodology/dsaft-refactoring-playbook.md` §3–§6

---

## 1. Summary

将 D2 从「**目录已 scenario 化、调用链仍 Legacy**」推进到 DSAFT v2.2 Structure 终态：

1. **Scenario orchestrator 成为生产 SoT**（消除 facade 双轨）
2. **Tools / Memory 按 S15/S17/S18 价值流归位**
3. **根目录仅保留 re-export**；测试与 mock 迁出域
4. **规格双锚点闭合**（`demand.md` §4 终态树 → `d2-domain.md` / `a-registry`）

## 2. Why Now

- D7 DM-20260619-005 已 Demonstrated 终态闭合模式
- D2 #104 完成 facade 分包，但 **未 wired orchestrator** — 是当前最大 Structure 债务
- Owner 已确认 tools→S18、memory 读写分离、根目录清零目标

## 3. Approach（6 Phase）

| Phase | 交付 | PR 约估 |
|-------|------|---------|
| P1 | 编排收敛 | 2 |
| P2 | 根目录清零 | 1 |
| P3 | enforce/tools + sandbox 归位 | 2 |
| P4 | memory 读写分离 | 1 |
| P5 | legacy Process 退役 | 1 |
| P6 | specs 回写 | 1 |

详见 `demand.md` §4 终态物理目录 + §7 AC。

## 4. Out of Scope

D7 路径、QueryLoop、Anthropic client、跨域 contract 签名变更。

## 5. Success Criteria

- 生产 Prepare/Persist/ToolRound 均经 scenario orchestrator
- 根目录生产文件 = 2
- `TestD2_*` + layer-lint + D7 integration 全绿
- `openspec/specs/d2-context-engine/` 与仓库目录 grep 一致
