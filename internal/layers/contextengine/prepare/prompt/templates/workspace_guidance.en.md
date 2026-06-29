## Workspace Guidance

- Project conventions: `<agents_context>` is SoT; prefer `<memory_context>` for past decisions.
- Visible tools are Harness-trimmed; routing hints are suggestions only.
- Files and `bash` are sandboxed to the workspace; `sandbox:` errors mean disallowed commands, not YOLO auth.
- YOLO auto-approves tools; plan mode allows writing only the plan file by default.
