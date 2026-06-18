# Design: Devrix 分层 ID 规范标准化

**Change ID:** devrix-layering-standard
**Demand ID:** DM-20260608-005

> **归档说明 (2026-06-18):** D-S-A-F-T 完整方案未实施；分层 ID 规范基础设施通过其他渠道落地（详见 `coverage-review.md`）。本文档记录方案设计作为历史参考。

## 1. 设计目标

建立统一 ID 分层体系：D-S-A-F-T 格式，实现：
- **唯一标识**: 所有业务能力（需求/功能/测试）唯一 ID
- **可追溯**: 需求 → 功能 → 测试 的完整链路
- **可导航**: 通过 ID 直接定位代码/文档/测试

## 2. D-S-A-F-T 格式定义

| 层级 | 含义 | 示例 |
|------|------|------|
| D | Domain (域) | `D2-S5` 表示 D2 (Context Engine) 的 S5 子系统 |
| S | Subsystem (子系统) | `D2-S5` |
| A | Ability (能力) | `D2-S5-A03` 表示 S5 下第 3 个能力 |
| F | Feature (特性) | `D2-S5-A03-F01` 表示 A03 的第 1 个特性 |
| T | Test Point (测试点) | `D2-S5-A03-F01-T05` 表示 F01 的第 5 个测试点 |

## 3. 与 L1-L2 格式的冲突

| 维度 | D-S-A-F-T | L1-L2 (现网) |
|------|-----------|--------------|
| 标识颗粒度 | Domain/Subsystem/Ability/Feature/Test (5 层) | Test/L1/L2/Number (4 层) |
| 与目录结构映射 | 弱（D 域与 L1 子目录非 1:1） | 强（直接对应目录层级） |
| 需求追溯能力 | 强（Domain→S→A→F→T 全链路） | 弱（仅 T 点维度） |
| 实施成本 | 高（需重命名所有 T 点 + 重构目录） | 低（维持现状） |

## 4. 现网采用方案（L1-L2）

- `openspec/t-registry.md` 与 `openspec/specs/d{N}-*/t-registry.md` 采用 `{T}-{L1}-{L2}-{NN}` 格式
- L1 = 域（D1-D7），L2 = 子系统/子域
- 与目录结构天然对应：`internal/layers/<l1>/<l2>/...`

## 5. 替代基础设施（已落地）

| 文件 | 内容 | 状态 |
|------|------|------|
| `openspec/specs/architecture/code-layout.md` | 目录 ID 规范 | ✅ |
| `openspec/specs/architecture/layering.md` | D1-D7 分层定义 | ✅ |
| `openspec/specs/architecture/code-atlas.md` | 代码索引 | ✅ |
| `openspec/t-registry.md` | 顶层 T 点注册表 | ✅ |
| `openspec/specs/d{N}-*/t-registry.md` | 各域 T 点注册表 | ✅ |

## 6. 决策与归档

**Decision (2026-06-18):** D-S-A-F-T 完整方案不再实施；基础设施通过 L1-L2 + 域文档体系满足。归档为 S7_Archived（S0_Deferred → Archived；out-of-band delivery）。

## 7. 风险与缓解

| 风险 | 缓解 |
|------|------|
| 未来需求追溯痛点出现 | 已在 demand-archive-index.md 中标记"如需重新评估，可重开此 change" |
| L1-L2 命名与未来变化冲突 | t-registry.md 中保留 L1-L2 命名空间，便于扩展 |