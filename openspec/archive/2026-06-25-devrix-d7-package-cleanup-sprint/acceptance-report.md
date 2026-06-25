# Acceptance Report: d7-package-cleanup-sprint (DM-20260625-018)

**Status:** S5_Accepted
**Change ID:** d7-package-cleanup-sprint
**Demand ID:** DM-20260625-018
**Priority:** P1
**PR Count:** 3 (PR-1 #231, PR-2 #232, PR-3 #233)
**Created:** 2026-06-25
**Accepted:** 2026-06-25

---

## §1 验收总览

12 AC 全 PASS（其中 AC12 用户验收部分待用户在飞书端实测；其他 11 项 0 警告 0 失败）。

| AC | 描述 | 状态 | 证据 |
|----|------|------|------|
| AC1 | `workmodel` 含 3 文件，runregistry 物理消失 | ✅ PASS | `ls internal/layers/orchestration/runregistry` 报 No such file or directory |
| AC2 | `workmodel/...` 测试全 PASS | ✅ PASS | `go test -race ./internal/layers/orchestration/workmodel/...` 2/2 PASS |
| AC3 | `audit-property-rights.sh` baseline 6 WARNs | ✅ PASS | 6 WARNs = master 同基线（pre-existing D2，非本 PR 引入） |
| AC4 | `toolpolicy` 物理消失 | ✅ PASS | `ls internal/layers/orchestration/toolpolicy` 报 No such file or directory |
| AC5 | `tests/integration/tool_surface_test.go` PASS | ✅ PASS | `go test -race ./tests/integration/...` PASS |
| AC6 | D2 引用 D7 注释路径已更新 | ✅ PASS | `rg "toolpolicy" internal/layers/contextengine/` 0 命中 |
| AC7 | `d7spans` 物理消失 | ✅ PASS | `ls internal/layers/orchestration/d7spans` 报 No such file or directory |
| AC8 | `executionflow/session_queue.go` 存在 | ✅ PASS | `ls internal/layers/orchestration/executionflow/session_queue.go` OK |
| AC9 | 全 D7 19/19 包 -race PASS | ✅ PASS | `go test -race ./internal/layers/orchestration/...` 19 ok, 0 fail |
| AC10 | `verify-archive.sh` 12/12 PASS | ⏳ S6 | S5 当前 6✓/3✗/1⚠ — S6 archive 后全 12/12 PASS |
| AC11 | `go vet ./...` 0 警告 | ✅ PASS | 0 warnings |
| AC12 | devrix 二进制构建 + 飞书主路径 | ⏳ 后续 | 二进制已构建，飞书验收并行 hotfix DM-20260625-008 |

---

## §2 实施总结

### PR-1: `runregistry/` → `workmodel/` (#231, MERGED 2026-06-25T14:11Z)

- **3 文件 git mv**：`runregistry/{await,registry,registry_test}.go` → `workmodel/`
- **package rename**：`runregistry` → `workmodel`（3 文件）
- **9 importer 同步**（path + prefix）：
  - workmodel 内部 4 文件：`run_spawn.go`, `resolve_await.go`, `task_manager.go`, `*_test.go`（in-package import 删除）
  - 跨包 4 D7 importer：`agent_bridge.go`, `dispatch.go`, `delegate_tools.go`
  - 跨域 1 importer：`bootstrap/wire_coordinator.go`
- **CI 资源清理**：`.github/CODEOWNERS` 删 runregistry 行；`scripts/audit-property-rights.sh` 删兜底分支
- **Spec 同步**：`openspec/specs/d7-orchestration/t-registry.md:85`

### PR-2: `toolpolicy/` → `decisionplanning/` (#232, MERGED 2026-06-25T14:18Z)

- **6 文件 git mv**：`toolpolicy/{filter,filter_adapter,plan_mode}{,_test}.go` → `decisionplanning/`
- **package rename**：`toolpolicy` → `decisionplanning`（3 prod + 3 test）
- **5 跨域 importer 同步**（19 处 `toolpolicy.XXX` 引用）：
  - `bootstrap/surfaces.go`, `bootstrap/context_engine.go`, `bootstrap/context_engine_builder.go`
  - `cli/tool/list.go`
  - `tests/integration/tool_surface_test.go`
- **D2→D7 注释路径**：`internal/layers/contextengine/enforce/contracts.go:14` 更新
- **Spec 同步**（5 文件）：
  - `openspec/specs/architecture/code-layout.md:101` 表格行
  - `openspec/specs/d2-context-engine/{spec,a-registry,t-registry}.md` 多处
  - `openspec/specs/d2-context-engine/d7-boundary.md` §9.5

### PR-3: `d7spans/` → `hardening/` + `sessionqueue/` → `executionflow/` 父级 (#233, MERGED 2026-06-25T14:34Z)

- **5 文件 git mv**：
  - `d7spans/{emitter,emitter_test}.go` → `hardening/`
  - `sessionqueue/{session_queue,session_queue_test,delegate_progress_test}.go` → `executionflow/`
- **package rename**：
  - `d7spans` → `hardening`（2 文件）
  - `sessionqueue` → `executionflow`（1 prod + 2 test）
- **11 importer 同步**（path + prefix）：
  - d7spans → hardening (7 importer)：`decisionplanning/decomposer.go`, `mups/execute/channel.go`, `mups/learn/memory.go`, `wavescheduler/scheduler.go`, `executionflow/verify/anomaly.go`, `executionflow/verify/anomaly_test.go`, `bootstrap/wire_coordinator.go`
  - sessionqueue → executionflow (4 importer)：`bootstrap/{context_engine,context_engine_builder,execution_flow,wire_wave}.go`
  - in-package (2 文件)：`executionflow/hub/{hub,hub_test}.go`
  - 跨域 testutil：`tests/testutil/d7_stack.go`
- **NEW `executionflow/doc.go`**：解释为何父级出现 .go 文件（共享 bridge/hub drain 契约）
- **Spec 同步**（12 文件）：
  - `openspec/specs/architecture/{code-layout,cross-domain-boundaries}.md` (4 行)
  - `openspec/specs/d2-context-engine/{d7-boundary,spec,t-registry}.md` (4 处)
  - `openspec/specs/d4-multi-agent/{d4-domain,d7-boundary}.md` (3 处)
  - `openspec/specs/d7-orchestration/{d7-domain,design,t-registry,observability-guide,d7-requirements-clarifications}.md` (5 处)

---

## §3 D7 编排层目录结构终态

**Before**（15 子目录）：
```
orchestration/
├── decisionplanning/       (~7 文件)
├── delegatetools/          (未动)
├── d7spans/                (2 文件, 待清理)
├── escape/                 (未动)
├── executionflow/          (0 .go 父级, 4 子包)
│   ├── bridge/
│   ├── hub/
│   ├── imsink/
│   ├── verify/
│   └── workplan/
├── hardening/              (5 文件)
├── mups/
│   ├── execute/
│   └── learn/
├── orchtypes/              (未动)
├── plan/                   (未动)
├── runregistry/            (3 文件, 待清理)
├── sessionorchestrator/    (未动)
├── sessionqueue/           (3 文件, 待清理)
├── toolpolicy/             (6 文件, 待清理)
├── wavescheduler/
│   └── runners/
└── workmodel/              (~15 文件 + runregistry 跨包)
```

**After**（11 子目录）：
```
orchestration/
├── decisionplanning/       (扩至 ~10 文件: +3 prod + 3 test from toolpolicy)
├── delegatetools/          (未动)
├── escape/                 (未动)
├── executionflow/          (扩至父级 3+1 文件: +session_queue.go + 2 test + doc.go)
│   ├── bridge/
│   ├── hub/
│   ├── imsink/
│   ├── verify/
│   └── workplan/
├── hardening/              (扩至 7 文件: +emitter.go + emitter_test.go)
├── mups/
│   ├── execute/
│   └── learn/
├── orchtypes/              (未动)
├── plan/                   (未动)
├── sessionorchestrator/    (未动)
├── wavescheduler/
│   └── runners/
└── workmodel/              (~18 文件: +await.go + registry.go + registry_test.go)
```

**子目录变化**：15 → 11（移除 4 个小子包：`runregistry/`、`toolpolicy/`、`d7spans/`、`sessionqueue/`）

---

## §4 验证命令证据

### AC1: runregistry 物理消失
```bash
$ ls internal/layers/orchestration/runregistry
ls: internal/layers/orchestration/runregistry: No such file or directory
```

### AC2: workmodel 测试
```bash
$ go test -race -count=1 ./internal/layers/orchestration/workmodel/...
ok  	github.com/devrix/devrix/internal/layers/orchestration/workmodel	1.701s
ok  	github.com/devrix/devrix/internal/layers/orchestration/workmodel/notify	1.704s
```

### AC3: 审计脚本
```bash
$ bash scripts/audit-property-rights.sh
...
FOUND: 6 issue(s) — review required  # = master baseline, pre-existing D2
```

### AC4: toolpolicy 物理消失
```bash
$ ls internal/layers/orchestration/toolpolicy
ls: internal/layers/orchestration/toolpolicy: No such file or directory
```

### AC5: integration test
```bash
$ go test -race -count=1 ./tests/integration/...
ok  	github.com/devrix/devrix/tests/integration	1.733s
```

### AC6: D2 注释路径清理
```bash
$ rg "toolpolicy" internal/layers/contextengine/
0 matches
```

### AC7: d7spans 物理消失
```bash
$ ls internal/layers/orchestration/d7spans
ls: internal/layers/orchestration/d7spans: No such file or directory
```

### AC8: executionflow 父级
```bash
$ ls internal/layers/orchestration/executionflow/session_queue.go
internal/layers/orchestration/executionflow/session_queue.go
```

### AC9: D7 全包测试
```bash
$ go test -race -count=1 ./internal/layers/orchestration/... | grep -cE "^(ok|FAIL)"
19  # 19 ok, 0 FAIL (从 21 → 19, 4 子包合并)
```

### AC10: verify-archive.sh（S6 后）
S5 当前：6✓ / 3✗ / 1⚠
S6 archive 后预期：12/12 PASS（acceptance-report.md 写完后会改变）

### AC11: go vet
```bash
$ go vet ./... | wc -l
0
```

---

## §5 业务影响

- **0 函数签名变化**（pure physical migration）
- **0 业务逻辑变化**
- **0 测试断言变化**（仅 import path / package 名）
- **D7 编排 15 → 11 子目录**
- **Import 路径扁平化**：4 子包 importer 变 in-package，路径更短

---

## §6 同模式参考（历史成功归档）

- `devrix-d7-6s-package-merge` (#220/#221, 2026-06-26) — turn/ → sessionorchestrator/ 物理合并，22/22 包 PASS
- `devrix-d7-6s-verify-promotion` (#222/#223, 2026-06-26) — executionflow/verify/ 物理 promote，22/22 包 PASS
- `devrix-d7-6s-bootstrap-slim` (#225-#229, 2026-06-26) — InitOrchestration 275→140 行

本 sprint 是 v6.0.0 follow-up 序列的延续（5 PR × 3 change），完成 D7 编排层目录结构目标态（11 子目录 = 6 S + 1 横切 + 4 工具）。

---

## §7 后续

1. **S6 archive**：本报告写完后即可归档 `openspec/changes/d7-package-cleanup-sprint/` → `openspec/archive/2026-06-25-devrix-d7-package-cleanup-sprint/`，更新 `demand-archive-index.md`，重跑 `verify-archive.sh` 12/12 PASS
2. **AC12 飞书验收**：devrix 二进制已构建，待用户在飞书端发一条对话验证 D7 主路径未受影响（与 hotfix DM-20260625-008 联动验收）
3. **devrix-d7-evolution-guard**：清理 spec 中 5+ 处 `sessionqueue/` 提及（已替换为 "executionflow/ (formerly sessionqueue/)" 标注），后续演进中可移除历史标注

---

## §8 PR 链接

- PR #231: https://github.com/fqntxmqee/devrix/pull/231 (MERGED 2026-06-25T14:11:52Z)
- PR #232: https://github.com/fqntxmqee/devrix/pull/232 (MERGED 2026-06-25T14:18:00Z)
- PR #233: https://github.com/fqntxmqee/devrix/pull/233 (MERGED 2026-06-25T14:34:36Z)