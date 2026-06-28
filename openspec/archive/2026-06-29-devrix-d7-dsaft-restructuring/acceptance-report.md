# Acceptance Report: devrix-d7-dsaft-restructuring (DM-20260629-001)

**Change ID:** devrix-d7-dsaft-restructuring
**Demand ID:** DM-20260629-001
**Status:** S7_Archived
**Acceptance Date:** 2026-06-29
**Total PRs:** 10 (PR-1 + PR-2 + PR-3a + PR-3-extended + PR-4 + PR-5 + PR-6 + PR-7 + PR-8 + PR-9)
**Total Tasks:** 55

---

## 1. Phase 交付摘要

| Phase | PR | 范围 | 状态 | T 范围 |
|-------|----|----|------|------|
| **#0 dead-code-cleanup** | PR-1 (#280) | ~775 LOC 死代码 + 老链路 + 4 处 doc drift | ✅ MERGED | 11/11 |
| **#1 turn-fn-split（part 1）** | PR-2 (#281) | turn_orchestrator.go 拆 turn_loop.go (T15) | ✅ MERGED | 1/1 |
| **#1 turn-fn-split（part 2）** | PR-3a (#282) | 拆 turn_invoke.go (T16) + turn_recovery.go (T17) | ✅ MERGED | 2/2 |
| **#1 WorkTree 上行反馈治理** | PR-3-extended (#283) | RollupReport struct + deterministic root + 3 governance T | ✅ MERGED | 4/4 (T52-T55) |
| **#2 registry-sync** | PR-4 (#284) | 6 F paths + t-registry Statistics + pipeline-arch v1.2.0 | ✅ MERGED | 10/10 (T20-T29) |
| **#3 value-flow-rename** | PR-5 (#285) | ValueFlow Semantic 列 + 用户感知层 | ✅ MERGED | 5/5 (T30-T34) |
| **#4 t-span-coverage (part 1)** | PR-6 (#286) | 5 ops span + 5 emit points | ✅ MERGED | 8/8 (T35-T42) |
| **#4 t-span-coverage (part 2)** | PR-7 (#287) | 4 acceptance tests LP-1/LP-2/LP-5/ResumeSession | ✅ MERGED | 1/1 (T43) + 12/12 acceptance |
| **#4 t-span-coverage (part 3)** | PR-8 (#288) | t-registry Span Evidence 列填充 + observability T-Without-Span Tracker | ✅ MERGED | 1/1 (T44) + 248 T 行 / 94% 覆盖率 |
| **#5 boundary-decision** | PR-9 (#289) | 3 boundary debt Decision + orchtypes governance | ✅ MERGED | 4/4 (T45-T48) |

---

## 2. Goals 验收（15 项量化指标）

| Goal | Metric | Target | Actual | 状态 |
|------|--------|--------|--------|------|
| **G1** | 死代码 LOC | 0 | 0（~775 LOC 全删） | ✅ PASS |
| **G2** | turn_orchestrator.go 拆 | <800 行/文件 | 4 文件 / 全部 <800 | ✅ PASS |
| **G3** | Registry 路径 | 6/6 正确 | 6/6 | ✅ PASS |
| **G4** | ValueFlow Alias | 6/6 S | 6/6 | ✅ PASS |
| **G5** | T↔Span 覆盖率 | ≥80% | 94%（235/248） | ✅ PASS |
| **G6** | 跨域 Decision | 3/3 | 3/3 | ✅ PASS |
| **G7** | Legacy ghost | 0 | 0（68 F = deprecated 2 + canonical 66） | ✅ PASS |
| **G8** | pipeline-arch v1.2.0 | ✅ | ✅ | ✅ PASS |
| **G9** | orchestration packages -race | 22/22 | 22/22 | ✅ PASS |
| **G10** | verify-archive.sh | 12/12 | 12/12 | ✅ PASS |
| **G11** | P0 T 100% | 193/193 | 193/193 | ✅ PASS |
| **G12** | god function T 重映射 | 36/36 | 36/36 | ✅ PASS |
| **G13** | RollupReport typed | 4/4 | 4/4 | ✅ PASS |
| **G14** | sessionRootGoal 确定性 | 1/1 | 1/1 | ✅ PASS |
| **G15** | D7-S0-A07 3 T | 3/3 | 3/3 | ✅ PASS |

---

## 3. 验证命令

```bash
# 22/22 orchestration packages -race PASS
go test ./internal/layers/orchestration/... -race -count=1

# 12/12 acceptance tests
go test ./tests/integration/d7/... -run 'LP-1|LP-2|LP-5|Resume' -v

# Span Evidence coverage
./scripts/span-coverage-check.sh  # 94% (235/248)

# verify-archive 12/12
./scripts/verify-archive.sh devrix-d7-dsaft-restructuring
```

---

## 4. 关键设计决策（governance 沉淀）

### 4.1 Hardening 横切（Discipline Keeper）

- `orchestration/hardening/` 5 .go 文件（doc + metrics + recovery + tests）
- `escape/circuit_breaker.go` 留 escape/（V5 EscapeEngine 核心机制，**不可迁移**）
- `sessionorchestrator/recovery.go` 保留 receiver methods（紧耦合 *DefaultOrchestrator）

### 4.2 3 Boundary Debt Decisions

| Item | Current D | Future re-evaluate trigger |
|------|-----------|----------------------------|
| ReputationEvidence | D7 (pragmatic) | v7.0 D6 advisory expansion |
| SystemAnomaly | D7 (certifier) | v7.0 D5/D6 partition |
| AdaptivePrior | D7 (LP-5 reverse trace) | v7.0 D4 federated memory |

统一治理常量在 `internal/layers/orchestration/orchtypes/boundary_decision.go`（v7.0 命名空间）。

### 4.3 WorkTree 上行反馈治理

- `RollupReport` struct：5 字段聚合（VerdictKind / ArtifactSummary / UncertaintyMean / SpawnPolicy / BubbleKind）
- 替换 4 处 `child.LastRound` 隐式 envelope
- `sessionRootGoal` 改为 `sort.Slice` 按 `item.ID` 排序保证确定性
- 3 governance T 点（D7-S0-A07-T01..T03）

---

## 5. Canonical Spec 同步

| Spec | Version | Changes |
|------|---------|---------|
| `d7-domain.md` | v2.5.1 → **v2.6.0** | §Out of Scope 3 boundary debt; §Cross-cutting Hardening 物理位置; §DSAFT 资产 186→230 T; §登记规模 Span 18→26 |
| `design.md` | v4.4.0 → **v4.5.0** | §⑧.5 Cross-Domain Boundary Decision 表 (3 items) |
| `a/f/t/span-registry.md` | v4.x → v4.12.0 | t-registry Span Evidence 列 + 94% 覆盖率 |
| `observability-guide.md` | v2.1.0 → **v2.2.0** | §8.1 T-Without-Span Tracker (13 unmapped T) + §8.2 Coverage Guard v7.0 follow-up |
| `pipeline-architecture.md` | v1.1.0 → **v1.2.0** | 同步 6 S + 1 横切 + LP/Trace 树 |
| `d7-domain.md 修订记录` | — | 新增 v2.6.0 row |

---

## 6. Follow-up（非本 change，v7.0 候选）

| Item | 触发条件 | 优先级 |
|------|----------|--------|
| 3 boundary debt 实际跨域迁移 | v7.0 D5/D6 partition | P0 |
| `D7_S1_Worktree_Op` op 覆盖扩展 | 新增 mutator | P2 |
| T-Without-Span 13 个 unmapped T 进一步映射 | 内层 span 接线 | P2 |
| Coverage Guard CI script v7.0 强制 | 提交 policy 决策 | P1 |
| god function 监控 guard (single-fn 36 T) | 防止新增 god | P2 |

---

## 7. 结论

**ACCEPTED — S7_Archived 2026-06-29**

10 PR / 55 T / 15 G 全部 PASS。D7 v6.0.x 维护阶段收官，v7.0 演进起点达成。

- Span Evidence 覆盖率 94%（目标 80%）
- 22/22 orchestration packages -race PASS
- verify-archive.sh 12/12 PASS
- 0 OPEN PR
- 0 函数签名变化（pure physical refactor + observability wiring + governance）
