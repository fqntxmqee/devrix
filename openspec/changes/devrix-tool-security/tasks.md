# Tasks: devrix-tool-security

**Change ID:** devrix-tool-security
**Status:** S2 Design
**Based on:** design.md, `specs/tool-security/spec.md`

---

## Milestone 1: Sandbox（P0）

### Definition of Done
- [ ] 命令白名单生效
- [ ] 危险模式检测生效
- [ ] 工作目录锁定
- [ ] 审计日志记录

### Tasks

- [ ] **T1**: 实现 `sandbox.go` CommandPolicy
  - 命令白名单校验
  - 危险正则模式匹配
  - 可配置 allowlist_extra / deny_patterns_extra
  - L5: L5-TOOL-01
  - Estimate: 3h
  - Dependencies: None

- [ ] **T2**: 修改 `tool_runner.go` runBash 集成沙箱
  - 命令执行前调用 CommandPolicy.Validate
  - 设置受限环境变量（HOME/PATH/PWD）
  - 增加审计日志回调
  - L5: L5-TOOL-01
  - Estimate: 2h
  - Dependencies: T1

- [ ] **T3**: 新增 `shared/config/tool_config.go` 配置结构
  - ToolSandboxConfig 定义（enabled, allowlist_extra, deny_patterns_extra）
  - 默认值 + YAML 解析
  - Estimate: 1h
  - Dependencies: None

---

## Milestone 2: Plugin Registry（P0）

### Definition of Done
- [ ] ToolRunner 接口定义
- [ ] ToolRegistry 注册/分发可用
- [ ] 内置工具适配为 ToolRunner
- [ ] PEV 引擎替换硬编码 switch

### Tasks

- [ ] **T4**: 实现 `tool_plugin.go` ToolRunner 接口 + ToolRegistry
  - ToolRunner 接口（Name, Schema, Execute）
  - ToolRegistry（Register, Execute, List）
  - L5: L5-TOOL-03
  - Estimate: 2.5h
  - Dependencies: None

- [ ] **T5**: 重构 `tool_runner.go` + `pev_engine.go`
  - BuiltinToolRunner 拆分为 bashRunner / readFileRunner / writeFileRunner
  - PEV 引擎替换 switch 为 toolRegistry.Execute
  - L5: L5-TOOL-03
  - Estimate: 2.5h
  - Dependencies: T4

---

## Milestone 3: Concurrency Control + Permission

### Definition of Done
- [ ] 并发工具执行限制
- [ ] CRITICAL 风险永不自动批准

### Tasks

- [ ] **T6**: 实现 `tool_limiter.go` + 权限细化
  - ToolLimiter 信号量控制
  - RiskLevelCritical 权限处理
  - L5: L5-TOOL-02, L5-TOOL-04
  - Estimate: 2h
  - Dependencies: T2, T4

---

## Milestone 4: Test（P0）

### Definition of Done
- [ ] 4 个 L5 测试点全部 IMPLEMENTED
- [ ] 现有工具测试回归通过

### Tasks

- [ ] **T7**: 编写沙箱 + 插件 + 并发测试
  - `sandbox_test.go`: 白名单/黑名单/危险模式/命令替换验证
  - `tool_plugin_test.go`: 注册/重复/未知工具/List 验证
  - `tool_limiter_test.go`: 并发限制/超时取消/隔离验证
  - L5: L5-TOOL-01, -02, -03, -04
  - Estimate: 3h
  - Dependencies: T1-T6

---

## 任务统计

| Milestone | 任务数 | 预估 |
|-----------|--------|------|
| M1 Sandbox | 3 | 6h |
| M2 Plugin Registry | 2 | 5h |
| M3 Concurrency | 1 | 2h |
| M4 Test | 1 | 3h |
| **合计** | **7** | **~16h** |

---

## 依赖关系图

```
T1 ── T2 ── T6 ──┐
                  ├── T7
T4 ── T5 ────────┘

T3 (独立)
```

## 执行顺序建议

1. **并行**: T1, T3, T4
2. **串行**: T2 → (等待 T1)
3. **串行**: T5 → (等待 T4)
4. **串行**: T6 → (等待 T2, T5)
5. **最终**: T7 → (等待 T1-T6)
