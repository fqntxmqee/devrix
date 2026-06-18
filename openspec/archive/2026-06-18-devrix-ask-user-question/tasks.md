# S4 任务：ask_user_question ToolSurface

## 任务清单

| ID | 任务 | 状态 | 提交 | 关联 AC |
|---|---|---|---|---|
| T01 | 实现 `AskUserQuestionSurface` struct + Tools/RiskLevel/Execute/InterruptBehavior/CheckPermission 5 方法 | ✅ DONE | 71ae692 | AC1, AC4, AC5 |
| T02 | 实现 `validateQuestions`（1-4 / 2-4 / header≤12 / label 唯一 / label 非空） | ✅ DONE | 71ae692 | AC1 |
| T03 | 实现 `renderQuestionsForIM`（多问题 + header chip + multi_select + "其他" hint） | ✅ DONE | 71ae692 | AC1 |
| T04 | 实现 `SetAskUserQuestionSender` / `currentAskSender`（process-global RWMutex） | ✅ DONE | 71ae692 | AC3 |
| T05 | 在 `internal/bootstrap/surfaces.go` 把 `NewAskUserQuestionSurface()` 加入 `BuildSurfaces` 输出 | ✅ DONE | 71ae692 | AC2 |
| T06 | 更新 `surfaces_test.go` 的 `BuildSurfaces_OnlyStateless` / `BuildSurfaces_FullDeps_AlphabeticalOrder` 期望 | ✅ DONE | 71ae692 | AC2 |
| T07 | 在 `orthogonal_flags.go` 的 `OrthogonalFlagFor` 加 `ask_user_question` case (true,false,true,false) | ✅ DONE | 71ae692 | AC4 |
| T08 | 在 `orthogonal_flags.go` 的 `InterruptBehaviorFor` 加 `ask_user_question` → InterruptCancel | ✅ DONE | 71ae692 | AC5 |
| T09 | 在 `cmd/devrix/main.go` 装配 sender 桥接到 `gw.RouteOutbound` | ✅ DONE | 71ae692 | AC3 |
| T10 | 写 12 个单测 (T01-T12)，`go test -race` 通过 | ✅ DONE | 71ae692 | AC1, AC6, AC7, AC8 |
| T11 | `go vet ./...` + `go build ./...` 通过 | ✅ DONE | 71ae692 | AC8 |
| T12 | 重编 devrix 二进制 (`./scripts/devrix.sh build`) + 重启 (`./scripts/devrix.sh restart`)，启动无错 | ✅ DONE | (build artifact) | AC6, AC7 |
| T13 | 验证 `./bin/devrix tool list` 含 `[ask_user_question] 1 tools` | ✅ DONE | (manual) | AC6 |
| T14 | **回滚冗余 task_lifecycle 代码**（与 v2 workmodel `task_create` 冲突），保留 context_engine.go 注释 | ✅ DONE | 71ae692 | AC6 (回归) |
| T15 | S6 归档（hotfix-style 轻量） | ✅ DONE | (this archive) | — |
| T16 | PR + auto-merge | 🔄 IN PROGRESS | PR #75 | — |

## 任务详情

### T01-T04：surface 实现（核心）

文件 `internal/layers/contextengine/enforce/toolrunner/surface/ask_user_question_surface.go`：
- `AskUserQuestionSurface` struct（无字段，stateless）
- `AskUserQuestionSender` type + `globalAskUserQuestionSender` + `SetAskUserQuestionSender`
- `Question` / `QuestionOption` / `askUserQuestionInput` / `askUserQuestionOutput` 数据类型
- `validateQuestions` 5 项校验
- `renderQuestionsForIM` 格式化输出
- `Execute` 流程：unmarshal → validate → 取 sessionID → render → 调 sender → 组装 output

### T05-T06：bootstrap 装配

`internal/bootstrap/surfaces.go` 在 VerifySurface 之后追加 `NewAskUserQuestionSurface()`。两次 `sort.Slice` 保持稳定排序。

`internal/bootstrap/surfaces_test.go`:
- `TestBuildSurfaces_OnlyStateless`: want 从 3 个 surface 变 4 个
- `TestBuildSurfaces_FullDeps_AlphabeticalOrder`: want 数组前部追加 `"ask_user_question"`

### T07-T08：orthogonal flags 表更新

`internal/layers/contextengine/enforce/toolrunner/surface/orthogonal_flags.go`:
- 顶部真值表注释加 `ask_user_question` 行
- `OrthogonalFlagFor` switch 加 `case "ask_user_question": return true, false, true, false`
- `InterruptBehaviorFor` switch 加 `case "free_fork", "ask_user_question": return InterruptCancel`

### T09：main.go sender 装配

`cmd/devrix/main.go` 在 `gw := capture.NewCommunicationGateway(...)` 之后插入 `asksurface.SetAskUserQuestionSender(...)` 闭包，metadata 标记 `source=ask_user_question, blocking=false`。

### T10：12 个单测

文件 `internal/layers/contextengine/enforce/toolrunner/surface/ask_user_question_surface_test.go`。覆盖 spec / 5 项 validation / happy path / no-sender / sender error / multi-question / invalid JSON / concurrent race-free。

### T14：回滚冗余 task_lifecycle

初版实现里同时新建了 `TaskRegistry` + `RegisterTaskLifecycleTools` 4 工具。启动时报 `register task_create: tool already registered: task_create`，因为 `devrix.yaml` `tasks.mode: v2` 时 `workmodel.RegisterTaskTools` 已经注册同名工具且功能更全。

回滚：
- 删除 `task_lifecycle_tools.go` / `task_lifecycle_tools_test.go` / `task_registry.go`
- 移除 `context_engine.go` + `context_engine_builder.go` 里的 `RegisterTaskLifecycleTools` 调用
- 加注释说明 v2 workmodel 所有权
- 移除 `orthogonal_flags.go` 里的 4 个 `task_*` case（落到默认 `hasPrefix("task_")` 分支，行为等价）

> 用户 B 需求（task_create/get/list/update）实际由 v2 workmodel 既有工具覆盖，**0 新增代码达成**。

### T15-T16：归档与 PR

- 本目录 `openspec/archive/2026-06-18-devrix-ask-user-question/`
- PR #75 走 GitHub Flow：`feat/devrix-ask-user-question` 分支 + squash + auto-merge
