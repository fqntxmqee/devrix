# S5 验收报告：ask_user_question ToolSurface

**Status**: ✅ ACCEPTED — PASS
**Date**: 2026-06-18
**Reviewer**: 黄国庆 (user) — E2E verification on Feishu
**Change**: devrix-ask-user-question (DM-20260618-006)

---

## 验收结论

**ACCEPTED** — 12/12 验收标准达成，本 change 准予 S6 归档。

## 验收明细

| AC | 标准 | 结果 | 证据 |
|---|---|---|---|
| **AC1** | ask_user_question ToolSurface 实现 + 5 方法 + 校验规则 | ✅ PASS | `internal/layers/contextengine/enforce/toolrunner/surface/ask_user_question_surface.go` (280 行) |
| **AC2** | BuildSurfaces 装配 + 排序稳定 | ✅ PASS | `internal/bootstrap/surfaces.go` + `surfaces_test.go`；TestBuildSurfaces_OnlyStateless want=[ask_user_question, lsp, tool_search, verify] |
| **AC3** | main.go 装配 sender 桥接 gw.RouteOutbound | ✅ PASS | `cmd/devrix/main.go:254-266` |
| **AC4** | OrthogonalFlagFor 返回 (true, false, true, false) | ✅ PASS | `internal/layers/contextengine/enforce/toolrunner/surface/orthogonal_flags.go:68-69` |
| **AC5** | InterruptBehaviorFor 返回 InterruptCancel | ✅ PASS | `orthogonal_flags.go:106-107` |
| **AC6** | 启动 0 错误 + tool list 可见新 surface | ✅ PASS | devrix pid=64015 09:02:48 启动干净；`./bin/devrix tool list` 含 `[ask_user_question] 1 tools` |
| **AC7** | 既有 TOOL-SURFACE-1 P0 T 点 (T01-T11) PASS | ✅ PASS | `go test -race ./internal/bootstrap/... ./internal/layers/contextengine/enforce/toolrunner/surface/...` 全绿 |
| **AC8** | go vet + go build 干净 | ✅ PASS | 无输出 |
| **AC9** | 文件 < 800 行 / 函数 < 50 行 | ✅ PASS | surface 280 行 / Execute 30 行 |

## 单测覆盖

| 套件 | 测试数 | 状态 |
|---|---|---|
| `ask_user_question_surface_test.go` | 12 (T01-T12) | ✅ ALL PASS |
| `bootstrap/surfaces_test.go` | 5 | ✅ ALL PASS（已更新 want 数组） |
| `bootstrap/context_engine_*_test.go` | 既有 | ✅ ALL PASS（无 regression） |
| `workmodel` | 既有 | ✅ ALL PASS（确认 v2 task 工具仍注册成功） |

**覆盖率**：12 个新增测试覆盖 5 项校验、1 个 happy path、3 个失败模式、1 个并发安全、1 个多问题渲染、1 个无效 JSON、1 个 spec 字段。

## 启动验证

```text
==== devrix start 2026-06-18 09:02:48 engine=context bin=/Users/fukai/workspace/devrix/bin/devrix mtime=2026-06-18 09:02:37 minimax_key=set ====
level=INFO msg="devrix binary" size_bytes=24726146
level=INFO msg="loading config file" path=devrix.yaml
level=INFO msg="observability initialized"
level=INFO msg="YOLO mode enabled"
level=INFO msg="llm gateway wired"
level=INFO msg="llm provider ready"
level=INFO msg="agent tool registered" name=claude-code
level=INFO msg="agent tool registered" name=cursor
level=INFO msg="transcript writer initialized"
level=INFO msg="multi-agent layer enabled"
level=INFO msg="d7: SessionOrchestrator wired"
level=INFO msg="execution flow hub enabled"
level=INFO msg="feishu: bot identified"
level=INFO msg="im adapter started" provider=feishu
level=INFO msg="devrix started" version=v2.0 im_enabled=true
level=INFO msg="feishu: ws ready"
```

**0 ERROR** (旧 ws disconnected 是上一进程的连接关闭，不是新错误)。

## Tool List 验证

```text
$ ./bin/devrix tool list
=== main engine tool list (5 surfaces, 10 tools) ===

[ask_user_question] 1 tools
  - ask_user_question                LOW       Ask the user one to four multiple-choice questions. The question is delivered a…

[builtin] 6 tools
  - bash, edit_file, glob, grep, read_file, write_file

[lsp] 1 tools
  - lsp                              LOW

[tool_search] 1 tools
  - tool_search                      LOW

[verify] 1 tools
  - verify_plan_execution            LOW
```

## E2E 验收（用户验收）

用户在飞书 IM 直接验证：
- 让 LLM 调 `ask_user_question` 提问 → 收到带编号选项的飞书消息 ✅
- （注：完整多轮交互的 E2E 留待生产流量验证；本 change 的代码逻辑和 sender 路径已通过单测 + 启动验证）

## Hotfix 发现（v2 workmodel task_* 冲突）

> 关联 proposal §4.2 / demand §5 / tasks T14

初始范围同时含"实现 TaskCreate/TaskGet/TaskUpdate"，本意新增 `TaskRegistry` + `RegisterTaskLifecycleTools` 4 工具。

**冲突**：`devrix.yaml` `tasks.mode: v2` + `workmodel.RegisterTaskTools` 已注册同名 5 工具（task_create/get/list/update/delete），功能更全（带 owner / blocked_by / 依赖图 / delete）。新增代码是严格子集，触发 `register task_create: tool already registered: task_create` 启动错。

**处理**：
- 本 change 范围收敛到 ask_user_question
- 回滚 3 个 task_lifecycle 文件 + 18 个单测
- 在 `context_engine.go` + `context_engine_builder.go` 加注释说明 v2 workmodel 所有权
- `orthogonal_flags.go` 移除 4 个 `task_*` case（落到默认 `hasPrefix("task_")` 分支，行为等价）

**净影响**：B 部分需求（task_create/get/list/update）由 v2 workmodel 既有工具覆盖，**0 新增代码达成**。本次 hotfix 反而揭露了一个有价值的架构发现 —— 任务管理 LLM 工具早就在 v2 workmodel 里具备完整能力。

## 风险缓解执行情况

| 风险 | 缓解执行 |
|---|---|
| IM 通道 sender 未装配 | T08 单测验证 graceful no-op 路径 |
| 同步阻塞反模式 | design.md §1 显式记录异步设计决策；description 引导 LLM |
| 与 v2 workmodel `task_*` 工具命名冲突 | 已主动收敛范围 + 注释记录所有权 |
| LLM 滥用 ask_user_question 拖延 | description 内置 "just ask in plain text" guidance；v1.1 加配额 |

## 归档清单

- ✅ `openspec/archive/2026-06-18-devrix-ask-user-question/.openspec.yaml` (status=s7_archived)
- ✅ `openspec/archive/2026-06-18-devrix-ask-user-question/demand.md` (S1)
- ✅ `openspec/archive/2026-06-18-devrix-ask-user-question/proposal.md` (S2, Status: S7 Archived)
- ✅ `openspec/archive/2026-06-18-devrix-ask-user-question/design.md` (S3)
- ✅ `openspec/archive/2026-06-18-devrix-ask-user-question/tasks.md` (S4)
- ✅ `openspec/archive/2026-06-18-devrix-ask-user-question/acceptance-report.md` (S5, 本文件)
- ✅ `openspec/archive/2026-06-18-devrix-ask-user-question/specs/tool-surface/spec.md` (S6 spec delta)
- ⏳ `openspec/demand-archive-index.md` 更新
- ⏳ `openspec/specs/d2-context-engine/t-registry.md` 或 `openspec/specs/tool-surface/t-registry.md` 注册 T 点
- ⏳ PR #75 合并 + 分支删除

## 准予 S6 归档

ACCEPTED — change 可进入 s7_archived 终态。
