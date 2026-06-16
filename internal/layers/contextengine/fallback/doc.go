// Package fallback — D2 Legacy Harness Fallback (deprecated).
//
// #deprecated: This package will be removed in v2.0.
// The legacy harness path is only active when query_loop.enabled=false is explicitly set.
// The production path is the QueryLoop (query/ package).
//
// When active, the harness provides:
//   - A01 Bootstrap: 工作区扫描、工具发现、Harness 初始化
//   - A02 Preflight: 预检评估、工具可见性过滤
//   - A03 Route: 提示路由（消息→工具匹配）
//   - A04 ToolPool: 工具池管理
//
// Migration: set query_loop.enabled=true (default after DM-20260611-004)
// and remove all harness.enabled references from config.
package fallback
