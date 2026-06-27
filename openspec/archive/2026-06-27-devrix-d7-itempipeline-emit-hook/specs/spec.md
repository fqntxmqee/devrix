# Spec: devrix-d7-itempipeline-emit-hook

## Feature: ItemPipelineRunner emit hook

作为 D7 编排层 per-WorkItem ReAct loop 路径的事件总线，ItemPipelineRunner 必须把 LLM↔Tool 迭代产生的事件（text/thinking/tool_call/tool_result）透传到 session-level emit，确保 D1 gateway 收到完整事件流。

## Scenario: Emit hook happy path (D7-S5-A54-T01)

**Given:** ItemPipelineRunner.Emit 已设置 session-level emit 函数
**When:** DefaultWorkItemExecutor 执行 LLM↔Tool 迭代
**Then:** 每个 text chunk / thinking chunk / tool_call / tool_result 都通过 runner.Emit 触发 session-level emit 函数，事件带 SessionID

## Scenario: Nil emit bridge (D7-S5-A54-T02)

**Given:** DefaultWorkItemExecutor.Emit 为 nil（legacy 调用方）
**When:** DefaultWorkItemExecutor 执行 LLM↔Tool 迭代
**Then:** emit 调用点 nil-check 通过，no-op 不爆，与修复前行为一致

## Scenario: Coverage test registry consistency

**Given:** 3 个新 inner observability span 已注册（D7_Worktree_Op / D7_SubWorktree_Run / D7_SubTurn_Iteration）
**When:** 运行 `TestAllOperations_should_match_telemetry_constants`
**Then:** registry size 81（之前） → 84（修复后），expected 列表同步 +3
**And:** LayerAndComponent 返回正确的 layer (orchestration) + component (worktree/executor)

## Scenario: User verification

**Given:** devrix binary 已 build + restart (pid=85935 @ 09:45:39)
**When:** 用户飞书发 "review d2领域代码"
**Then:** 飞书卡片可见 tools 列表 + LLM text/thinking/tool_result 事件
**Evidence:** 用户 09:32 反馈 "tools有了"