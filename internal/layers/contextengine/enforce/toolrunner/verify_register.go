package toolrunner

import "fmt"

// W6 — D6-S11-A02 (alias G4) verify_plan_execution tool 注册 helper。
//
// 单一入口 RegisterVerifyTool(reg) 让 bootstrap 调用，免去在多处硬编码 tool name。

// RegisterVerifyTool 把 verifyRunner 注册到 reg。重复注册返回错误。
func RegisterVerifyTool(reg *ToolRegistry) error {
	if reg == nil {
		return fmt.Errorf("verify: registry is nil")
	}
	return reg.Register(&verifyRunner{})
}
