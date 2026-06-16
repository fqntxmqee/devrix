// Package query — D2-S2 ExecuteQuery: LLM↔Tool 执行循环。
//
// S2 编排 3 个 A 层:
//   - A01 RunLoop: LLM 调用 → 工具执行 → 重复的主循环
//   - A02 ExecuteToolRound: 单轮工具调用解析、执行、结果处理
//   - A03 StreamResponse: 流式文本输出到事件通道
//
// 核心合约定义在 types.go（ToolExecutor, PermissionChecker）。
// query.Loop 是 D2-S2 的入口点，由 engine.go 中的 Process 方法调用。
package query
