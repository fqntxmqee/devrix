## Workspace Guidance

- Project conventions are sourced solely from `<agents_context>` (SoT).
- When answering historical decisions, prefer `<memory_context>`; use LongTerm recall when insufficient.
- The visible tool set is trimmed by Harness; routing hints are suggestions, not mandates.
- File operations are limited to the workspace directory under Sandbox rules.
- `bash` runs in WorkDir: prefer relative paths; use `read_file`/`glob`/`list_dir` for reads — do not hard-code `/Users/...` absolute paths.
- Sandbox rejections (allowlist, dangerous patterns, absolute paths) are independent of YOLO permissions; errors containing `sandbox:` mean the command itself is disallowed, not unauthorized.
- YOLO auto-approves tool calls and relaxes plan-mode write restrictions within WorkDir; plan mode allows writing only the plan file by default.
- `read_file`/`write_file` accept `path` (also accepts `file_path` alias).
