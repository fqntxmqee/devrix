# D7 4 子包清理 Spec

**Module:** D7 Orchestration / 全部 6 S + 1 横切
**Change:** `d7-package-cleanup-sprint` (DM-20260625-018)
**Status:** S3_Design → S4_Implemented → S5_Accepted → S7_Archived
**Spec Version:** v1.0
**依赖:** devrix-d7-six-s-simplification (DM-20260626-001) + devrix-d7-6s-package-merge (DM-20260626-004) + devrix-d7-6s-verify-promotion (DM-20260626-005) + devrix-d7-6s-bootstrap-slim (DM-20260626-007) 全部 S7_Archived

---

## ADDED

### Requirement: runregistry 包合并到 workmodel/

`internal/layers/orchestration/workmodel/` 必须包含原 `runregistry/` 3 个文件，类型 `RunRegistry` + 函数 `NewRunRegistry` + helper `awaitRun` 0 变化。

<!-- T: D7-S6-A50-T01 -->

#### Scenario: workmodel/registry.go contains RunRegistry

- GIVEN 原 `internal/layers/orchestration/runregistry/registry.go` 包含 `RunRegistry` struct + `NewRunRegistry` 函数
- AND `package runregistry` 声明在原文件中
- WHEN 执行 `git mv internal/layers/orchestration/runregistry/registry.go internal/layers/orchestration/workmodel/registry.go`
- AND `package runregistry` 改为 `package workmodel`
- THEN `internal/layers/orchestration/workmodel/registry.go` 存在
- AND 包含 `RunRegistry` struct 0 变化
- AND 包含 `NewRunRegistry` 函数 0 变化
- AND `internal/layers/orchestration/runregistry/` 目录不存在

### Requirement: toolpolicy 包合并到 decisionplanning/

`internal/layers/orchestration/decisionplanning/` 必须包含原 `toolpolicy/` 6 个文件（3 prod + 3 test），类型 `Filter` + `PlanMode` + adapter 0 变化。

<!-- T: D7-S6-A50-T02 -->

#### Scenario: decisionplanning/filter.go contains Filter

- GIVEN 原 `internal/layers/orchestration/toolpolicy/filter.go` 包含 `Filter` struct
- AND `package toolpolicy` 声明在原文件中
- WHEN 执行 `git mv internal/layers/orchestration/toolpolicy/filter.go internal/layers/orchestration/decisionplanning/filter.go`
- AND `package toolpolicy` 改为 `package decisionplanning`
- THEN `internal/layers/orchestration/decisionplanning/filter.go` 存在
- AND 包含 `Filter` struct 0 变化
- AND `internal/layers/orchestration/toolpolicy/` 目录不存在

### Requirement: d7spans 包合并到 hardening/

`internal/layers/orchestration/hardening/` 必须包含原 `d7spans/` 2 个文件，emitter 0 变化。

<!-- T: D7-S6-A50-T03 -->

#### Scenario: hardening/emitter.go contains SpanEmitter

- GIVEN 原 `internal/layers/orchestration/d7spans/emitter.go` 包含 `SpanEmitter` struct + 桥接函数
- AND `package d7spans` 声明在原文件中
- WHEN 执行 `git mv internal/layers/orchestration/d7spans/emitter.go internal/layers/orchestration/hardening/emitter.go`
- AND `package d7spans` 改为 `package hardening`
- THEN `internal/layers/orchestration/hardening/emitter.go` 存在
- AND 包含 `SpanEmitter` struct 0 变化
- AND `internal/layers/orchestration/d7spans/` 目录不存在

### Requirement: sessionqueue 包扁平到 executionflow/ 父级

`internal/layers/orchestration/executionflow/` 父级必须包含原 `sessionqueue/` 3 个文件 + 1 个 `doc.go` 注释。

<!-- T: D7-S6-A50-T04 -->

#### Scenario: executionflow/session_queue.go at parent level

- GIVEN 原 `internal/layers/orchestration/sessionqueue/session_queue.go` 包含 `SessionQueue` struct
- AND `package sessionqueue` 声明在原文件中
- WHEN 执行 `git mv internal/layers/orchestration/sessionqueue/session_queue.go internal/layers/orchestration/executionflow/session_queue.go`
- AND `package sessionqueue` 改为 `package executionflow`
- THEN `internal/layers/orchestration/executionflow/session_queue.go` 存在
- AND 包含 `SessionQueue` struct 0 变化
- AND `internal/layers/orchestration/sessionqueue/` 目录不存在
- AND `internal/layers/orchestration/executionflow/doc.go` 存在并说明父级为何有 .go 文件

---

## MODIFIED

### Requirement: D7 目录结构达到 11 个子目录理想态

修改 `openspec/specs/d7-orchestration/d7-domain.md` §目录结构 图：

**Before (本 change 之前)**:
```
orchestration/  (15 子目录)
├── runregistry/         (待清理)
├── toolpolicy/          (待清理)
├── sessionqueue/        (待清理)
├── d7spans/             (待清理)
├── decisionplanning/    workmodel/ sessionorchestrator/ wavescheduler/
├── executionflow/       (仅子包, 父级 0 Go 文件)
├── hardening/           mups/ escape/ orchtypes/ delegatetools/
```

**After (本 change 之后)**:
```
orchestration/  (11 子目录, 6 S + 1 横切 + 4 工具)
├── decisionplanning/    (S5, 扩至 ~10 文件)
├── executionflow/       (S4, 父级 3+1 文件, 子包不变)
├── hardening/           (横切, 扩至 ~7 文件)
├── mups/                (S6)
├── sessionorchestrator/ (S2)
├── wavescheduler/       (S3)
├── workmodel/           (S1, 扩至 ~45 文件)
├── escape/              (工具, DM-20260625-003)
├── orchtypes/           (类型共享)
├── delegatetools/       (工具)
```

### Requirement: CI 资源不再引用 4 子包

修改 `.github/CODEOWNERS`：删除 `runregistry/` 行。
修改 `scripts/audit-property-rights.sh:26`：删除 `*orchestration/runregistry/*` 兜底分支。

---

## REMOVED

### Requirement: orchestration/runregistry/ 目录不存在

- THEN `ls internal/layers/orchestration/runregistry/` 报错（目录不存在）
- AND `rg "orchestration/runregistry" internal/ cmd/ tests/` 0 命中

### Requirement: orchestration/toolpolicy/ 目录不存在

- THEN `ls internal/layers/orchestration/toolpolicy/` 报错
- AND `rg "orchestration/toolpolicy" internal/ cmd/ tests/` 0 命中

### Requirement: orchestration/sessionqueue/ 目录不存在

- THEN `ls internal/layers/orchestration/sessionqueue/` 报错
- AND `rg "orchestration/sessionqueue" internal/ cmd/ tests/` 0 命中

### Requirement: orchestration/d7spans/ 目录不存在

- THEN `ls internal/layers/orchestration/d7spans/` 报错
- AND `rg "orchestration/d7spans" internal/ cmd/ tests/` 0 命中
