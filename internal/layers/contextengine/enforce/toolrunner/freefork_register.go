package toolrunner

import "fmt"

// W7 — D4-S11-A02 + D4-S13-A02 (alias G5) free_fork tool 注册 helper。
//
// 单一入口 RegisterFreeForkTool(reg) 让 bootstrap 调用。

// RegisterFreeForkTool 把 freeforkRunner 注册到 reg。重复注册返回错误。
func RegisterFreeForkTool(reg *ToolRegistry) error {
	if reg == nil {
		return fmt.Errorf("freefork: registry is nil")
	}
	return reg.Register(&freeforkRunner{})
}
