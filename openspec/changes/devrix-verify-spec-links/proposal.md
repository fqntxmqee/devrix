# Proposal: devrix-verify-spec-links（CI 工具 — 校验 CHANGELOG.md 链接有效性）

**Change ID:** devrix-verify-spec-links
**Demand ID:** DM-20260630-010
**Status:** S2_Proposal
**Date:** 2026-06-30

---

## 1. 方案对比

### 方案 A：Bash 脚本（推荐）

新增 `scripts/verify-spec-links.sh`，复用现有 `verify-archive.sh` 的 Bash + pass/fail/warn 模式。

**优势**：
- 复用现有模式（颜色 + 计数 + exit code）
- 与 `verify-archive.sh` / `lint-*.sh` 工具链一致
- 简单（~50 行 Bash）
- 无依赖（grep + sed + 标准 Unix 工具）

**劣势**：
- 无类型安全（shell 脚本）

### 方案 B：Go CLI 工具

新增 `cmd/verify-spec-links/main.go`，作为 Go 二进制工具。

**优势**：
- 类型安全 + 可测试
- 可被 Go 项目其他 lint 工具引用

**劣势**：
- 编译依赖 + 维护成本
- 与其他 verify-* shell 脚本不一致
- 单一职责工具不值得编译

### 方案 C：Python 脚本

**劣势**：
- 增加 Python 依赖（项目目前无 Python 依赖）
- 不一致

## 2. 决策

**方案 A**：Bash 脚本。复用现有 `verify-archive.sh` 模式。

## 3. 工作量

1 PR (含 1 个 Bash 脚本 + 6 change docs)，~80 行 Bash + ~150 行 markdown。

## 4. 风险

| 风险 | 缓解 |
|------|------|
| 链接格式变化导致正则失效 | 单行 grep 正则 + 模式测试 |
| 归档目录意外删除导致误报 | 仅 WARN（不阻断），待人工修复 |
| 跨域链接（spec.md → d4-domain.md）也需校验 | 范围限定为 archive 链接，跨域 SoT 引用另行处理 |

## 5. 验收

10 AC（详见 demand.md）

## 6. 复用参考

- `scripts/verify-archive.sh`（Bash 模式 + 颜色）
- `scripts/lint-d1-imports.sh`（lint 工具）