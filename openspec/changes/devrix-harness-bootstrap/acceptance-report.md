# Acceptance Report: Devrix Harness Bootstrap

**Change ID:** devrix-harness-bootstrap
**Demand ID:** DM-20260609-004
**Status:** S5 Ready（自动化验收通过，待合入 main）
**Review Round:** 2（2026-06-10）

---

## OpenSpec 合规检查（S3 准出）

| 检查项 | 要求 | 状态 |
|--------|------|------|
| 四件套 | demand / proposal / design / tasks | ✅ |
| Delta spec | `specs/{capability}/spec.md` + `## ADDED Requirements` | ✅ |
| `openspec validate` | 无 ERROR | ✅ |
| L3/L4 无悬空 ID | L3-BE-CTX-04（非 CTX-03） | ✅ |
| L5 注册 | `openspec/l5-registry.md` D2-S9 | ✅ |
| 架构登记 | `layering.md` + `project.md` D2-S9 | ✅ |

---

## S4 开发完成

| ID | Criteria | Status |
|----|----------|--------|
| S1.1 | `harness/` 包骨架 + contracts | ✅ |
| S1.2 | ToolPool simple_mode：`bash`/`read_file`/`write_file` | ✅ |
| S1.3 | messages-only 压缩 → Build → PEV | ✅ |
| S1.4 | PEV 消费 VisibleTools | ✅ |
| S1.5 | 6 harness Jaeger operations + registry | ✅ |

**自动化验证（2026-06-10）：**

```bash
./scripts/test-unit.sh                              # PASS
./scripts/test-domain.sh d2                         # PASS（unit + integration + acceptance + perf）
openspec validate devrix-harness-bootstrap            # PASS
```

---

## S5 验收 — L5 矩阵

| L5 ID | 描述 | Priority | 结果 | 证据 |
|-------|------|----------|------|------|
| L5-2-9-01 | harness.enabled 首次 Process bootstrap | P0 | **Pass** | `context_harness_bootstrap_test.go` |
| L5-2-9-02 | WorkspaceContext 扫描 | P1 | **Pass** | `workspace_test.go` |
| L5-2-9-03 | Bootstrap 幂等 | P1 | **Pass** | `context_harness_bootstrap_test.go` |
| L5-2-9-04 | trusted=false 跳过 deferred init | P0 | **Pass** | `ctx_harness_bootstrap_test.go` |
| L5-2-9-05 | ToolPool 过滤 | P0 | **Pass** | `toolpool_test.go` + acceptance |
| L5-2-9-06 | Routing advisory | P2 | **Pass** | `router_test.go` |
| L5-2-9-07 | Transcript 分离 + compact | P1 | **Pass** | `transcript_test.go` |
| L5-2-9-08 | disabled V4 回归 | P0 | **Pass** | integration bootstrap test |
| L5-2-9-09 | Preflight warn-only | P1 | **Pass** | `preflight_test.go` |
| L5-2-9-10 | System Prompt Assembly §十 | P0 | **Pass** | `system_prompt_assembler_test.go` |
| L5-2-9-11 | Jaeger span 树 | P0 | **Pass** | `context_harness_obs_test.go` |
| L5-2-9-12 | disabled 与 BuildLegacy 一致 | P0 | **Pass** | acceptance subtest |
| L5-2-9-13 | CompressedView system = Build | P0 | **Pass** | integration enabled flow |
| L5-2-9-14 | PEV tools ⊆ VisibleTools | P0 | **Pass** | acceptance tool capture |
| L5-2-9-15 | bootstrap.stage parent = bootstrap.run | P0 | **Pass** | `context_harness_obs_test.go` |
| L5-5-5-02 | harness Operation registry 对账 | P1 | **Pass** | registry_test + obs coverage test |

**P0 汇总：** 11/11 Pass  
**P1 汇总：** 4/4 Pass  
**P2 汇总：** 1/1 Pass  

**灰度约束：** 默认 `context_engine.harness.enabled=false`；生产行为与 V4 一致，直至显式开启。

---

## S6/S7 待办（合入 main 后）

- [ ] 合入 main
- [ ] 更新 `openspec/demand-archive-index.md`
- [ ] 执行 `/openspec-archive devrix-harness-bootstrap`
- [ ] 灰度文档：启用 harness 的配置与回滚步骤

---

## Verdict

**S5 自动化验收：通过。** 可进入 PR 合入与归档流程。
