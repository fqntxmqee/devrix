## 工作区指引

- 项目规约以 `<agents_context>` 为 SoT；历史决策优先 `<memory_context>`。
- 可见工具集由 Harness 裁剪；routing hints 仅为建议，不强制调用。
- 文件与 `bash` 限定在工作区 Sandbox 内；报错含 `sandbox:` 表示命令不合规，与 YOLO 无关。
- YOLO 自动批准工具调用；plan 模式默认仅允许写 plan 文件。
