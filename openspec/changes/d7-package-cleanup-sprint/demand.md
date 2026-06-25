# Demand: d7-package-cleanup-sprint (DM-20260625-018)

**Demand ID:** DM-20260625-018
**Status:** S1_Demand → S2_Proposal → S3_Design → S4_Implemented → S5_Accepted → S7_Archived
**Priority:** P1
**Sprint:** d7-v6 收尾（v6.0.0 follow-up 序列之外）
**PR Count:** 3 (PR-1 runregistry + PR-2 toolpolicy + PR-3 d7spans+sessionqueue)
**Created:** 2026-06-25
**Change ID:** d7-package-cleanup-sprint
**Related:** devrix-d7-six-s-simplification (DM-20260626-001) · devrix-d7-mups-package-migration (DM-20260626-002) · devrix-d7-hardening-cross-cutting (DM-20260626-003) · devrix-d7-6s-package-merge (DM-20260626-004) · devrix-d7-6s-verify-promotion (DM-20260626-005) · devrix-d7-6s-bootstrap-slim (DM-20260626-007)

---

## §1 背景

v6.0.0 域升级（DM-20260626-001/002/003/004/005/007）已经把 D7 编排层从 14 S 精简为 6 S + 1 横切（hardening），并把大型子包（turn/、execute/、learn/、exit_reason/、observe/）做了物理合并。

**遗留问题**：仍有 4 个小子包是 v6.0.0 之前的"逻辑分离"产物，每个只有 2-3 个文件、零独立测试价值：

| 子包 | 父包 | 文件数 | 内部 D7 importer | 跨域 importer |
|------|------|-------|------------------|---------------|
| `runregistry/` | `workmodel/` | 3 (含 1 test) | 6 | 1 bootstrap |
| `toolpolicy/` | `decisionplanning/` | 6 (3 prod + 3 test) | 0 | 4 (bootstrap + D2 + CLI + 1 integration test) |
| `sessionqueue/` | `executionflow/` | 3 (1 prod + 2 test) | 2 (executionflow/hub/) | 4 bootstrap + 1 testutil |
| `d7spans/` | `hardening/` | 2 (1 prod + 1 test) | 5 | 0 |

## §2 问题

1. **目录浏览噪音**：读 D7 目录需要下钻一层才知道逻辑，14 个子目录 = 6 S + 1 横切 + 4 遗留 + 3 工具（escape/mups/wavescheduler）
2. **import 路径碎片化**：`workmodel/run_spawn.go` 已经 import `workmodel` 父包却还要 import `runregistry` 子包（同级不同包，跨包访问）
3. **新成员认知负担**：每个子包都要解释一次"为什么独立"
4. **review 噪音**：4 个子包导致 4 个独立的 import graph 节点需要 review

## §3 范围

### In-Scope（必做）

- 4 个子包物理合并到父包（git mv + import 路径替换 + package 改名）
- CODEOWNERS + audit-property-rights.sh 中 runregistry 兜底分支清理
- 相关 spec/docs 同步
- 1 个 OpenSpec change（`d7-package-cleanup-sprint`），3 个 PR 渐进交付

### Out-of-Scope（不做）

- 不改任何函数签名（pure physical migration）
- 不动任何业务逻辑
- 不动 test 框架 / 断言
- 不动跨域契约（toolpolicy 与 D2 的接口签名不变）

## §4 验收标准

| AC | 描述 | 验证方法 |
|----|------|---------|
| AC1 | PR-1 合入后 `workmodel` 包含原 runregistry 3 文件，runregistry 目录物理消失 | `ls internal/layers/orchestration/` 不含 runregistry |
| AC2 | PR-1 合入后 `go test -race ./internal/layers/orchestration/workmodel/...` 全 PASS | 命令行 0 失败 |
| AC3 | PR-1 合入后 `bash scripts/audit-property-rights.sh` 0 violations | 命令行 0 violations |
| AC4 | PR-2 合入后 `toolpolicy` 目录物理消失，decisionplanning 包扩至 ~6 个 prod 文件 | `ls internal/layers/orchestration/` 不含 toolpolicy |
| AC5 | PR-2 合入后 `tests/integration/tool_surface_test.go` PASS | 命令行 0 失败 |
| AC6 | PR-2 合入后 D2 引用 D7 的注释路径已更新（`enforce/contracts.go:14`） | `rg "toolpolicy" internal/layers/contextengine/` 0 命中 |
| AC7 | PR-3a 合入后 `d7spans` 目录物理消失，hardening 包含 emitter | `ls internal/layers/orchestration/` 不含 d7spans |
| AC8 | PR-3b 合入后 `sessionqueue` 目录物理消失，executionflow 父级有 3 个 .go + 1 doc.go | `ls internal/layers/orchestration/executionflow/` 包含 session_queue.go |
| AC9 | 3 PR 全合并后 `go test -race ./internal/layers/orchestration/...` 22/22 PASS | 命令行 0 失败 |
| AC10 | 3 PR 全合并后 `bash scripts/verify-archive.sh` 12/12 PASS | 命令行 0 失败 |
| AC11 | 3 PR 全合并后 `go vet ./...` 0 警告 | 命令行 0 警告 |
| AC12 | 3 PR 全合并后 devrix 二进制构建 + 飞书端发消息主路径正常 | 用户验收 |

## §5 风险

详见 `tasks.md` §风险矩阵 + `design.md` §7 关键决策点。

## §6 后续 follow-up

合并后 D7 编排层目录结构：

```
orchestration/
├── decisionplanning/    (扩至 ~10 文件)
├── delegatetools/       (未动)
├── escape/              (未动)
├── executionflow/       (扩至父级 3+1 文件, 子包不变)
│   ├── bridge/
│   ├── hub/
│   ├── imsink/
│   ├── verify/
│   └── workplan/
├── hardening/           (扩至 ~7 文件)
├── mups/                (未动)
├── orchtypes/           (未动)
├── sessionorchestrator/ (未动)
├── wavescheduler/       (未动)
└── workmodel/           (扩至 ~45 文件)
```

11 个子目录 = 6 S + 1 横切 + 1 escape + 1 orchtypes + 1 delegatetools + 1 wavescheduler。达到理想态。
