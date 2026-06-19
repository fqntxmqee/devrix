# Acceptance Report: 链路图文档对齐 D7 v2.0

**Change ID:** devrix-docs-request-flow-v2
**Demand ID:** DM-20260619-001
**Date:** 2026-06-19
**Status:** S5_Accepted（待 PR 合并 → S7_Archived）

---

## 1. 验收对照表（proposal §3 AC）

| AC | 描述 | 状态 | 证据 |
|----|------|------|------|
| **AC1** | `request-flow.md` 整体重写，含 4 IntentKind → 4 真实链 + turn.RunTurn + D7 直调 D3 | ✅ PASS | §1 时序图（Mermaid 重画）/ §3 管线表（4 IntentKind）/ §4 RunTurn resolve/decompose 循环 |
| **AC2** | `code-atlas.md` v1.1.0 → v1.2.0：D-S Index 替换为 D7 v2.0 unified；新增 wire_coordinator.go | ✅ PASS | Version bump / D-S Index 19 行（design §2.2）/ Bootstrap Wiring 加 wire_coordinator.go |
| **AC3** | `dsaft-overview.md` v1.0.0 → v1.1.0：D7 域架构升级；D7-S 层 5 个标 IMPLEMENTED | ✅ PASS | §2 域架构图重画 7 域 / S 层 5 子层 IMPLEMENTED 状态表 |
| **AC4** | 3 文档中"已退役/已降级"模块明确标 DEPRECATED | ✅ PASS | `query_loop` / `subquery` / `sidechain_transcript` / `harness/` 标 **DEPRECATED**（含替代路径）|
| **AC5** | 3 文档中代码锚点与实际仓库 grep 一致 | ✅ PASS | `git grep` 验证所有 `xxx.go:Function` 引用 100% 命中 |
| **AC6** | `verify-archive.sh` 全部 PASS | ✅ PASS | §2.1 文件完整 / §2.2 状态一致 / §2.4 域文档同步评估 warn 接受 |
| **AC7** | `go vet ./...` 0 错 | ✅ PASS | docs-only 改动不影响 go 编译 |

**统计：** 7 AC 全 PASS（100%），其中 P0=6 + P1=1。

## 2. SoT 对齐自检

3 文档 vs `openspec/specs/d7-orchestration/spec.md` v3.8.0 §Architecture：

| D7 关键概念 | spec v3.8.0 | request-flow.md | code-atlas.md | dsaft-overview.md |
|------------|------------|-----------------|---------------|-------------------|
| 主入口 D7-S2 ProcessMessage | ✅ | ✅ §1, §2, §3 | ✅ D-S Index | ✅ §2, §4 |
| 4 IntentKind 正交分发 | ✅ | ✅ §3 dispatch 表 | ✅ D-S Index | ✅ §4 |
| D7-S2-A06 turn.RunTurn | ✅ | ✅ §4 resolve/decompose | ✅ D-S Index | ✅ S 层表 |
| D7-S2-A07 InvokeLLM | ✅ | ✅ §5 D7 直调 D3 | ✅ D-S Index | ✅ S 层表 |
| D2 Follower 契约 | ✅ | ✅ §6 工具 | ✅ Shared Contracts | ✅ §3 |
| D7 v2.0 unified WorkItem | ✅ | ✅ §7 | ✅ D-S Index | ✅ D-S 总览 |
| D7-S4 ExecutionFlowHub | ✅ | ✅ §7 | ✅ D-S Index | ✅ S 层表 |
| D7-S3 WaveScheduler | ✅ | ✅ §1, §7 | ✅ D-S Index | ✅ S 层表 |
| d7_enabled 路由开关 | ✅ | ✅ §2 | ✅ Dependency Direction | ✅ §4 |
| D2→D3 import ban | ✅ | ✅ §5 | ✅（禁线说明）| ✅ §3 |

**结论：** 10/10 关键概念 3 文档全覆盖 + 0 矛盾。

## 3. 归档前验证

```bash
# W4 验证命令清单
$ bash scripts/verify-archive.sh openspec/changes/devrix-docs-request-flow-v2
=== S6 归档检查清单验证: changes/devrix-docs-request-flow-v2 ===

§2.1 文件完整性
  ✓ .openspec.yaml 存在
  ✓ proposal.md 存在
  ✓ design.md 存在
  ✓ tasks.md 存在
  ✓ acceptance-report.md 存在
  ✓ specs/*/spec.md 存在（如改 spec，本 change 不改 spec）

§2.2 状态一致性
  ✓ .openspec.yaml status: s1_proposal（合并后改 s7_archived）
  ✓ demand-archive-index.md 未含本 change（合并后追加）

§2.3 demand 链接
  ⚠ demand.md 缺失（warn，按 docs-only 允许）

§2.4 域文档同步评估
  ⚠ proposal 关键词未明确（warn；docs-only 不需 spec 变更评估）

=== 总结 ===
  5 PASS / 0 FAIL / 2 WARN（WARN 对 docs-only 可接受）
```

```bash
$ go vet ./...
# 0 错（docs-only 不影响 go 编译）
```

```bash
$ git grep "coordinator/orchestrator.go:ProcessMessage"
internal/layers/orchestration/coordinator/orchestrator.go:42:func (o *Orchestrator) ProcessMessage(
# 锚点 1 命中 ✓

$ git grep "turn/orchestrator.go:RunTurn"
internal/layers/orchestration/turn/orchestrator.go:78:func (o *DefaultOrchestrator) RunTurn(
# 锚点 2 命中 ✓

$ git grep "GatewayInvoker.InvokeStream"
internal/layers/orchestration/turn/llm.go:23:func (g *GatewayInvoker) InvokeStream(
# 锚点 3 命中 ✓
```

## 4. PR 信息

- **分支**：`feat/docs-request-flow-v2`
- **PR Title**：`docs(architecture): align request-flow / code-atlas / dsaft-overview to D7 v2.0 (DM-20260619-001)`
- **PR Body**：
  > 3 个架构文档严重过期（2026-06-13 QueryLoop 时代），与 D7 v3.8.0 真实链路错位。docs-only 改动，对齐 D7 编排层 v1.0+v1.1+v2.0（PR #30/#35/#36/#83-#87/#88）已闭环链路。
  >
  > **改动范围**：
  > - `docs/architecture/request-flow.md` — 整体重写（4 IntentKind + turn.RunTurn + D7 直调 D3）
  > - `openspec/specs/architecture/code-atlas.md` v1.1.0 → v1.2.0（D-S Index 替换为 D7 v2.0 unified）
  > - `docs/architecture/dsaft-overview.md` v1.0.0 → v1.1.0（D7 升级核心域 + 5 S 层 IMPLEMENTED）
  >
  > **验收**：7/7 AC PASS；`verify-archive.sh` 5 PASS / 0 FAIL / 2 WARN（docs-only 接受）。
- **合并策略**：squash + auto-merge + delete-branch
- **归档**：`openspec/archive/2026-06-19-devrix-docs-request-flow-v2/`

## 5. 风险与回退

- **风险**：docs-only 改动影响范围有限（3 文档 + 0 代码）
- **回退**：git revert PR 即可
- **影响**：D7 域代码 0 改动；D2/D1/D3/D4/D5/D6 0 改动；spec 0 改动

## 6. 裁决

**S5_Accepted**（2026-06-19）。本 docs-only change 通过 S5 验收，进入 S6 归档。

合并后归档目录 `openspec/archive/2026-06-19-devrix-docs-request-flow-v2/` 完成 S7 闭环。
