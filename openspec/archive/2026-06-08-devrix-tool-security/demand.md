---
demand-id: DM-20260608-001
title: 工具执行安全增强（Bash 沙箱 + 插件化注册）
source: 安全 Review / PEV 工具执行路径
priority: P0
status: S7_ARCHIVED
l1-domain: contextengine
created: 2026-06-08
---

# 工具执行安全增强

## 1. 原始描述

> PEV 引擎通过 bash 工具执行任意 shell 命令，无白名单与危险模式检测；工具以 switch-case 硬编码，无法插件化扩展。需增强 bash 沙箱并引入 ToolRegistry 插件注册机制。

## 2. 澄清记录

### Q1: 沙箱粒度？

**A**: 不引入 gVisor/Docker；采用命令白名单 + 危险正则 + 工作目录环境隔离 + 审计日志四层纵深防御。 — 2026-06-08

### Q2: 插件接口与现有 IToolRunner 关系？

**A**: 新增 `PluginRunner`（单工具）+ `ToolRegistry`（注册表）；`IToolRunner` 由 `ToolRegistry` / `LimitedToolRunner` 实现，PEV 无流程变更。 — 2026-06-08

### Q3: 并发限制默认值？

**A**: `tool.concurrent_max` 默认 10，可通过 YAML 配置。 — 2026-06-08

## 3. L1–L5 映射草案

| 层级 | 资产 |
|------|------|
| L1 | contextengine |
| L3 | PEV 工具执行活动 |
| L4 | L4-TOOL-SANDBOX, L4-TOOL-REGISTRY, L4-TOOL-PERMISSION |
| L5 | L5-TOOL-01 ~ L5-TOOL-04 |

## 4. 验收标准

- P0 L5-TOOL-01~04 全部自动化测试通过
- 主路径 bootstrap 接入 BuiltinToolRegistry + ToolLimiter
- 无 PEV / Communication 核心流程破坏性变更
