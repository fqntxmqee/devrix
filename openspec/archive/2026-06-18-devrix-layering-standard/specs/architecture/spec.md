# Spec: Devrix 分层 ID 规范标准化

**Change ID:** devrix-layering-standard
**Demand ID:** DM-20260608-005
**Status:** S7_Archived (2026-06-18; S0_Deferred; out-of-band delivery)

## 1. 当前采用方案（L1-L2 格式）

Devrix 实际使用的 T 点 ID 格式：

```
{T}-{L1}-{L2}-{NN}
```

示例：
- `D2-S5-A03-F01-T05` 风格未采用
- 实际：`T-D2-LLMGateway-001` 风格（或 `{T}-{L1}-{L2}-{NN}` 等价表达）

具体映射见 `openspec/t-registry.md` 与 `openspec/specs/d{N}-*/t-registry.md`。

## 2. 域文档体系（替代方案）

| 文档 | 内容 |
|------|------|
| `openspec/specs/architecture/code-layout.md` | 目录结构与 ID 命名约定 |
| `openspec/specs/architecture/layering.md` | D1-D7 分层定义 |
| `openspec/specs/architecture/code-atlas.md` | 代码模块索引 |
| `openspec/t-registry.md` | 顶层 T 点注册表 |
| `openspec/specs/d{N}-*/t-registry.md` | 各域 T 点注册表 |
| `openspec/specs/d{N}-*/spec.md` | 各域规范 |

## 3. 决策

D-S-A-F-T 完整 ID 分层方案不再实施。基础设施通过 L1-L2 + 域文档体系满足。

## 4. 归档

**Status:** S7_Archived (2026-06-18)
**Verdict:** S0_Deferred → Archived；out-of-band delivery 完整；D-S-A-F-T 方案终止。