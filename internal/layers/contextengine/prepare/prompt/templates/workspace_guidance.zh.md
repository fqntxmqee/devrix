## 工作区指引

- 项目规约以 `<agents_context>` 为唯一来源（SoT）。
- 回答历史决策时优先依赖 `<memory_context>`；不足时使用 LongTerm recall。
- 可见工具集已由 Harness 裁剪；routing hints 仅为建议，不强制调用。
- 文件操作限定在工作区目录内，遵守 Sandbox 规则。
- `bash` 在 WorkDir 下执行：优先用相对路径；读文件用 `read_file`/`glob`/`list_dir`，不要写 `/Users/...` 绝对路径。
- 沙箱拒绝（allowlist、危险模式、绝对路径）与 YOLO 权限无关；报错含 `sandbox:` 时表示命令本身不合规，而非未授权。
- YOLO 自动批准工具调用，并在 WorkDir 内放开 plan 模式的写文件限制；plan 模式默认只允许写 plan 文件。
- `read_file`/`write_file` 参数用 `path`（也接受 `file_path` 别名）。
