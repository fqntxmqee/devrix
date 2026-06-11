# Acceptance Report: devrix-queryloop-context

**Demand ID:** DM-20260610-012  
**Date:** 2026-06-10  
**Scope:** v1.0 (CC s01–s07) + v1.1 (Fork/Background) + v2.0 (Hub-Spoke / ExecutionFlow / D4 Delegate)

## P0 L5 Results — v1.0 QueryLoop

| ID | Description | Status | Evidence |
|----|-------------|--------|----------|
| L5-CTX-34 | Multi-turn tool loop until no tools | PASS | `query/loop_test.go`, `query_loop_pev_test.go` |
| L5-CTX-35 | UserContext prepend not in snapshot | PASS | `usercontext/provider.go`, Assembler `OmitAgentsFromSystem` |
| L5-CTX-36 | plan_mode attachment throttle | PASS | `attachments/registry.go` |
| L5-CTX-37 | plan mode write filter | PASS | `permission/mode_test.go`, `queryloop_tools_test.go` |
| L5-CTX-38 | task disk persist + list | PASS | `tasks/disk_store_test.go` |
| L5-CTX-39 | harness.enabled=false regression | PASS | `query_loop_integration_test.go`; default `query_loop.enabled=false` |
| L5-CTX-40 | SubQuery Explore read-only + agent depth | PASS | `query/subquery_test.go`, `multiagent/builtin/agents.go` |
| L5-CTX-41 | Fork placeholder tool_results 一致 | PASS | `query/fork_test.go` |
| L5-CTX-42 | sidechain transcript resume | PASS | `transcript/sidechain_test.go`, `query/subquery_test.go` |

## P0 L5 Results — v2.0 Hub-Spoke + Delegate

| ID | Description | Status | Evidence |
|----|-------------|--------|----------|
| L5-4-10-01 | Leader delegate_explore + MaxWorkers / fallback | PASS | `multiagent/delegate/service_test.go` |
| L5-4-10-02 | Worker Run 设置 SC.AgentID，sidechain 隔离 | PASS | `worker_tools_test.go`, `agent/worker_engine_test.go` |
| L5-4-10-03 | Worker 不能 delegate_* 或 Fork | PASS | `worker_tools_test.go`, `agent/worker_engine_test.go` |
| L5-4-10-04 | delegate-progress 仅 Leader Drain | PASS | `queue/delegate_progress_test.go`, `orchestration/flow/hub_test.go` |
| L5-4-10-05 | worker_progress 到达 Gateway/IM | PASS | `orchestration/imsink/gateway_test.go` |
| L5-4-10-06 | SubQuery 与 D4 Worker 共用 FlowEvent schema | PASS | `orchestration/flow/hub_test.go`, `delegate_fallback_flow_test.go` |
| L5-4-10-07 | FlowStarted 自动 task owner + in_progress | PASS | `delegate_tools_test.go`, `orchestration/flow/hub_test.go` |
| L5-4-10-08 | D4 未启用 delegate 降级 SubQuery，IM 仍可见进度 | PASS | `delegate_fallback_flow_test.go`, `delegate/service_test.go` |
| L5-4-10-09 | 用户单会话：无第二对话入口 | PASS | `bootstrap/cli_events_test.go` |
| L5-ORCH-01 | WorkPlan.Snapshot 含 Task + ExecutionFlow | PASS | `orchestration/workplan/service_test.go`, `orchestration/flow/hub_test.go` |
| L5-4-12-01 | Worktree enter 后 write 不污染主 WorkDir | PASS | `worktree/manager_test.go`, `delegate/service_worktree_test.go` |

## Configuration

Enable QueryLoop v1 in `devrix.yaml`:

```yaml
context_engine:
  query_loop:
    enabled: true
  user_context:
    mode: prepend
  tasks:
    mode: v2
    store_dir: ~/.devrix/tasks
```

Enable v2 Hub-Spoke + Delegate:

```yaml
context_engine:
  query_loop:
    enabled: true
  execution_flow:
    enabled: true
    link_tasks: true
    im_progress: true
  worktree:
    enabled: true
    base_dir: ~/.devrix/worktrees
multi_agent:
  enabled: true
  delegate:
    enabled: true
    allow_async: true
```

## Manual E2E (recommended)

1. **Path A:** `query_loop.enabled=false` — existing harness/PEV flow unchanged  
2. **Path B:** `enabled=true` — multi-turn bash/read until final text  
3. **Path C:** `enter_plan_mode` → write non-plan file denied → write plan file allowed  
4. **Path D:** `task_create` → restart process → `task_list` shows same task  
5. **Path E:** AGENTS.md appears in prepend meta-user, not in system `<agents_context>`  
6. **Path F (v2):** `delegate_explore` → Feishu/CLI worker_progress cards → Leader 单会话汇总  
7. **Path G (v2):** `worktree_slug` delegate → 主 WorkDir 无脏文件

## v1.1 (T15–T18)

| Item | Status |
|------|--------|
| Fork `BuildForkedMessages` + `RunExploreFork` | PASS |
| `StreamingToolExecutor` parallel safe tools | PASS |
| `SessionQueue` + `RunBackground` notifications | PASS |
| Spec delta v1.1 requirements | DONE |

## Notes

- v3.0: D7 Work Orchestration 升格（见 `design-orchestration-v3.md`，T24–T27 未交付）  
- Legacy `context_assembler.go` removed  
- Spec delta: `specs/context-engine/spec.md` (V6)  
- Pre-existing unrelated failure: `feishu_progress_test.go` TestAppendAgentStreamText_UsesDedicatedCard

## Sign-off

- [ ] Product owner  
- [ ] Tech lead  

## Conclusion

**Verdict: ACCEPTED (P0)** — v1.0/v1.1/v2.0 P0 L5 全部通过单元/集成测试；v3.0 能力留待后续变更。
