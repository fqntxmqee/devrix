package toolrunner

// LSP wire helpers — bootstrap entry points for LSP tool.
//
// 使用方在配置启用 LSP 时调用 RegisterLSPTool(reg, cfg) 即可把 lspRunner
// 注册到 tool registry。cfg 为 nil 时默认 disabled(等同 devrix.yaml
// 未配置 lsp 节)。

// RegisterLSPTool 把 lsp tool 注册到 reg,cfg 可空。
func RegisterLSPTool(reg *ToolRegistry, cfg *LSPConfig) error {
	if reg == nil {
		return nil
	}
	return reg.Register(newLSPRunner(cfg))
}
