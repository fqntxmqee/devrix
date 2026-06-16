// Package nested — D2-S5 NestedExecution: 嵌套子代理执行。
//
// S5 编排 3 个 A 层:
//   - A01 SpawnSubquery: 创建嵌套 D2 实例
//   - A02 RunBackgroundTask: 后台任务生命周期
//   - A03 MergeSubResult: 子查询结果合并回父会话
//
// SidechainRecorder 合约定义在 subquery.go。
package nested
