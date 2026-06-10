## Workspace Guidance

- 项目规约以 `<agents_context>` 为唯一来源（SoT）。
- 回答历史决策时优先依赖 `<memory_context>`；不足时使用 LongTerm recall。
- 可见工具集已由 Harness 裁剪；routing hints 仅为建议，不强制调用。
- 文件操作限定在工作区目录内，遵守 Sandbox 规则。
