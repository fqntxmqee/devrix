// Package enforce — D2-S3 EnforcePolicy: 安全策略执行。
//
// S3 编排 4 个 A 层:
//   - A01 CheckPermission: 权限门禁（Ask/Allow/Deny）
//   - A02 FilterTools: 工具可见性过滤（Plan Mode + Agent Role）
//   - A03 SandboxExecution: 工具执行隔离与沙箱
//   - A04 RegisterTools: 工具注册与发现（Builtin + Custom）
//
// DSAFT: D2-S3 (formerly D2-S5/D2-S18 under the legacy numbering)
package enforce

// PermissionChecker 由 enforce/permission 子包实现。
// ToolRunner/ToolRegistry 合约定义在 enforce/tools/contracts.go。
// AgentRoleToolFilter 合约定义在 orchestration/decisionplanning（D7 注入）。
