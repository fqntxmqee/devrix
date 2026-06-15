# acceptance-report — Background Task 工具 + Phase 3 Wave 对接

| 属性 | 值 |
|------|-----|
| Change ID | devrix-background-task-tools |
| DM ID | DM-20260611-009 |
| 验收日期 | 2026-06-15 |
| 验收人 | Claude Code (AI) |
| Decision | **ACCEPTED** |
| 归档 | openspec/archive/2026-06-15-devrix-background-task-tools/ |

## AC 裁决

| # | AC | 裁决 | 证据 |
|---|----|------|------|
| AC-01 | BackgroundRegistry.RegisterWithCancel + Cancel + List | ✅ PASS | `nested/background.go`: RegisterWithCancel/Cancel/List 全部实现 |
| AC-02 | RunBackground 接入 cancel ctx + CompleteCancelled | ✅ PASS | `nested/background.go`: RunBackground 带 ctx.CancelFunc 注册 |
| AC-03 | task_stop 工具注册 + task_output block/poll + timeout | ✅ PASS | `background_task_tools.go`: task_stop/task_output/task_list_background |
| AC-04 | task_stop idempotent (重复 cancel) | ✅ PASS | `background_cancel_test.go`: TestBackgroundRegistry_CancelIdempotent |
| AC-05 | task_output block=false 返回 running 状态 | ✅ PASS | D2-S9-A01-T17: running 状态 + partial result |
| AC-06 | task_output block=true 阻塞至 terminal 或 timeout | ✅ PASS | D2-S9-A01-T18: block wait + 600s 超时 |
| AC-07 | cancel 后 SessionQueue 不发 completed notification (tombstone) | ✅ PASS | D2-S9-A01-T19: tombstone 协议 |
| AC-08 | WorkerCancelRegistry 接口定义 (Phase 3) | ✅ PASS | `wave/types.go`: WorkerCancelRegistry{Cancel, IsTerminal} |
| AC-09 | IsTerminal 方法 + D2-S9-A01-T20 测试 | ✅ PASS | `nested/background.go`: IsTerminal; 测试覆盖 unknown/running/cancelled/completed/failed |
| AC-10 | DM-007 文档引用 Cancel 协议 | ✅ PASS | `archive/...wave-scheduler/design.md` §6.2 已更新 |

## T 层覆盖

| T ID | 描述 | Status | Test 文件 |
|------|------|--------|-----------|
| D2-S9-A01-T16 | stop running task → cancelled (idempotent) | IMPLEMENTED | `background_cancel_test.go` + `background_task_tools_test.go` |
| D2-S9-A01-T17 | output block=false on running | IMPLEMENTED | `background_cancel_test.go` + `background_task_tools_test.go` |
| D2-S9-A01-T18 | output block=true waits complete (max 600s) | IMPLEMENTED | `background_cancel_test.go` + `background_task_tools_test.go` |
| D2-S9-A01-T19 | cancel suppresses completed notification (tombstone) | IMPLEMENTED | `background_cancel_test.go` |
| D2-S9-A01-T20 | IsTerminal for completed/cancelled/failed (Phase 3) | IMPLEMENTED | `background_cancel_test.go` |

## Phase 3 Wave 对接

| Deliverable | 路径 | 状态 |
|------------|------|------|
| WorkerCancelRegistry 接口 | `internal/layers/orchestration/wave/types.go` | ✅ 2 method (Cancel + IsTerminal) |
| BackgroundRegistry 实现 | `internal/layers/contextengine/nested/background.go` | ✅ IsTerminal + Cancel |
| DM-007 文档更新 | `openspec/archive/...wave-scheduler/design.md` | ✅ §6.2 引用 |
| T20 测试 | `internal/layers/contextengine/nested/background_cancel_test.go` | ✅ 5 用例 |

## 变更摘要

### 实现文件

| 文件 | 行数 | 描述 |
|------|------|------|
| `nested/background.go` | ~300 | RegisterWithCancel, Cancel, List, IsTerminal |
| `background_task_tools.go` | ~250 | task_stop, task_output, task_list_background |
| `background_task_tools_test.go` | ~170 | 工具集成测试 |
| `nested/background_cancel_test.go` | ~220 | Cancel + IsTerminal 测试 |
| `wave/types.go` | +12 | WorkerCancelRegistry 接口 |

### 已知限制

1. **Go 编译检查注释化**: `var _ wave.WorkerCancelRegistry = (*BackgroundRegistry)(nil)` 因 import cycle 保持注释状态，Phase 4 可通过 interface 提取到 contracts 包解决
2. **BackgroundRegistry 磁盘持久化** deferred 至 v1.1
3. **D6 BackgroundTaskProbe** deferred 至 v1.1

## Decision

**ACCEPTED** — 10 AC 全部 PASS。5 T 点全部 IMPLEMENTED。Phase 3 Wave 对接完成（WorkerCancelRegistry 接口 + IsTerminal）。v1.0 交付质量合格，可归档。
