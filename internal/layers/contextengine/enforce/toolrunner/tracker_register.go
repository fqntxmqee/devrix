package toolrunner

import "fmt"

// W8 — D5-S23-A02 (alias G6) query_diagnostics tool 注册 helper。
//
// 单一入口 RegisterTrackerTool(reg) 让 bootstrap 调用。

// RegisterTrackerTool 把 trackerRunner 注册到 reg。重复注册返回错误。
func RegisterTrackerTool(reg *ToolRegistry) error {
	if reg == nil {
		return fmt.Errorf("tracker: registry is nil")
	}
	return reg.Register(&trackerRunner{})
}
