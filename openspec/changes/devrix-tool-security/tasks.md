# Tasks: devrix-tool-security

**Change ID:** devrix-tool-security
**Status:** S5 Accepted
**Based on:** design.md, `specs/tool-security/spec.md`

---

## Milestone 1: Sandbox（P0）

### Definition of Done
- [x] 命令白名单生效
- [x] 危险模式检测生效
- [x] 工作目录锁定
- [x] 审计日志记录

### Tasks

- [x] **T1**: 实现 `sandbox.go` CommandPolicy
  - L5: L5-TOOL-01

- [x] **T2**: 修改 `tool_runner.go` runBash 集成沙箱
  - L5: L5-TOOL-01

- [x] **T3**: 新增 `shared/config/tool_config.go` 配置结构

---

## Milestone 2: Plugin Registry（P0）

### Definition of Done
- [x] ToolRunner 接口定义
- [x] ToolRegistry 注册/分发可用
- [x] 内置工具适配为 ToolRunner
- [x] PEV 引擎通过 registry 执行工具

### Tasks

- [x] **T4**: 实现 `tool_plugin.go` PluginRunner 接口 + ToolRegistry
  - L5: L5-TOOL-03

- [x] **T5**: 重构 `tool_runner.go` + bootstrap 接线
  - L5: L5-TOOL-03

---

## Milestone 3: Concurrency Control + Permission

### Definition of Done
- [x] 并发工具执行限制
- [x] CRITICAL 风险永不自动批准

### Tasks

- [x] **T6**: 实现 `tool_limiter.go` + 权限细化测试
  - L5: L5-TOOL-02, L5-TOOL-04

---

## Milestone 4: Test（P0）

### Definition of Done
- [x] 4 个 L5 测试点全部 IMPLEMENTED
- [x] 现有工具测试回归通过

### Tasks

- [x] **T7**: 沙箱 + 插件 + 并发 + 权限测试
  - L5: L5-TOOL-01, -02, -03, -04

- [x] **T8**: S5 验收报告与 L5 注册表更新
  - `acceptance-report.md`, `openspec/l5-registry.md`

---

## 任务统计

| Milestone | 任务数 | 状态 |
|-----------|--------|------|
| M1 Sandbox | 3 | Done |
| M2 Plugin Registry | 2 | Done |
| M3 Concurrency | 1 | Done |
| M4 Test + S5 | 2 | Done |
