# Demand: devrix-verify-spec-links（CI 工具 — 校验 CHANGELOG.md 链接有效性）

**Demand ID:** DM-20260630-010
**Date:** 2026-06-30
**Priority:** P2
**Source:** lite-mode 推广 7 站收官后的 Backlog 收尾

---

## 1. 背景

lite-mode 推广 7 站（d7/d2/d1/d3/d4/d5/d6）已全部 S7_Archived，每个 d{N} 域的 CHANGELOG.md 都用 `[archive](../../archive/<date>-<id>/)` 链接形式引用归档目录。

**问题**：CHANGELOG.md 中的 archive 链接无自动化校验 — 链接拼写错误、归档目录重命名、归档目录被意外删除都会导致链接失效，但当前 CI 流程无法感知。

**Backlog 收官**：本 change 是 lite-mode 推广 7 站的最后一个 Backlog 项目。

## 2. 目标

新增 CI 工具 `scripts/verify-spec-links.sh`，自动校验所有 CHANGELOG.md 中的 archive 链接有效性。

## 3. 验收标准（AC）

| ID | 标准 |
|----|------|
| AC1 | 新增 `scripts/verify-spec-links.sh` 可执行 |
| AC2 | 扫描所有 `openspec/specs/d{1..7}-*/CHANGELOG.md` 文件 |
| AC3 | 提取 `[archive](../../archive/<date>-<id>/)` 链接模式 |
| AC4 | 验证每个链接的归档目录存在（archive/<date>-<id>/） |
| AC5 | 验证链接中的 change-id 与目录名一致 |
| AC6 | 输出汇总：扫描文件数 / 链接总数 / 有效 / 无效（0 期望）|
| AC7 | Exit 0（全部有效）/ Exit 1（存在无效链接） |
| AC8 | 集成到 `scripts/verify-archive.sh` 或独立可调用 |
| AC9 | 0 Go 代码 diff（仅 shell 脚本 + 6 change docs） |
| AC10 | verdict: ACCEPTED |

## 4. 范围

**In Scope**：
- `scripts/verify-spec-links.sh` NEW (Bash)
- 6 change docs

**Out of Scope**：
- Go 代码任何修改
- 其他域 spec.md / CHANGELOG.md 修改
- 现有 `verify-archive.sh` 重写（保持向后兼容）
- GitHub Actions workflow 集成（手动调用即可）

## 5. 设计要点

```
verify-spec-links.sh
  1. 扫描 openspec/specs/d{1,2,3,4,5,6,7}-*/CHANGELOG.md
  2. 提取 `[archive](../../archive/YYYY-MM-DD-devrix-{name}/)` 模式
  3. 对每个链接：
     a. 检查 archive/YYYY-MM-DD-devrix-{name}/ 目录存在
     b. 提取 change-id，检查与目录名一致
     c. 检查 .openspec.yaml 中 status=s7_archived
  4. 输出汇总
  5. Exit 0 / 1
```

## 6. 复用参考

- 现有 `scripts/verify-archive.sh` 模式（Bash + pass/fail/warn + 颜色）
- 现有 `scripts/lint-*.sh` 模式（lint 工具）

## 7. 下一步

- S2 proposal：A vs B vs C
- S3 design：六段式
- S4 实现：Bash 脚本 + 测试
- S5 验收：10 AC 验证
- S6 交付：PR + auto-merge
- S6 归档：独立 PR