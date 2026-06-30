# Acceptance Report: devrix-verify-spec-links

**Change ID:** devrix-verify-spec-links
**Demand ID:** DM-20260630-010
**Status:** S5_Acceptance
**Verdict:** **ACCEPTED** (10/10 AC)

---

## 1. AC 满足度

| ID | 标准 | 状态 | 验证 |
|----|------|------|------|
| AC1 | `scripts/verify-spec-links.sh` 可执行 | ✅ PASS | `chmod +x` + `bash -n` 通过 |
| AC2 | 扫描 7 个 CHANGELOG.md | ✅ PASS | d1/d2/d3/d4/d5/d6/d7 全部命中 |
| AC3 | 提取 `[archive](../../archive/...)` 链接 | ✅ PASS | `grep -oE` 正则 |
| AC4 | 验证归档目录存在 | ✅ PASS | `[ ! -d ]` 检查 |
| AC5 | 验证 change-id 与目录名一致 | ✅ PASS | `${link#*-*-*}` 提取 |
| AC6 | 输出汇总：扫描文件数 / 链接总数 / 有效 / 无效 | ✅ PASS | Summary 段输出 |
| AC7 | Exit 0 / 1 | ✅ PASS | OK → 0, FAIL → 1, STRICT warn → 1 |
| AC8 | 可独立调用 | ✅ PASS | 不依赖 verify-archive.sh |
| AC9 | 0 Go 代码 diff | ✅ PASS | `git diff --stat internal/` = 0 |
| AC10 | verdict: ACCEPTED | ✅ PASS | 本报告 |

---

## 2. 实际运行结果

```
=== Spec Links Validation ===
扫描文件: 7
链接总数: 103
  ✓ Pass: 39
  ⚠ Warn: 64 (legacy archives predates .openspec.yaml convention)
  ✗ Fail: 0

OK: all archive links valid
```

**已知 WARN**（不影响 FAIL）：
- 10 个 legacy archive (2026-06-07 ~ 06-18) 缺 `.openspec.yaml`（predates convention）
- 部分 archive 的 status 不是 s7_archived（如 PR-C S3_Design, observe-merge-cancel s1_cancelled）

---

## 3. 改动统计

| 文件 | 类型 | 行数 | 说明 |
|------|------|------|------|
| `scripts/verify-spec-links.sh` | NEW | ~115 | Bash 校验脚本 |

**总计**：1 个新 Bash 脚本（0 Go diff，0 其他文件 diff）。

---

## 4. 决策记录

**方案 A 复用 verify-archive.sh 模式**：
- Bash + pass/fail/warn + 颜色 + 计数
- 与现有 verify-archive.sh / lint-*.sh 工具链一致
- 零依赖（grep + sed + 标准 Unix）
- ~115 行 Bash（含说明注释）

---

## 5. 工具能力

```bash
./scripts/verify-spec-links.sh              # 默认 WARN 模式（仅记录不阻断）
./scripts/verify-spec-links.sh --strict     # 严格模式（warn 也 Exit 1）
./scripts/verify-spec-links.sh --domain d4  # 仅校验指定域
./scripts/verify-spec-links.sh --help       # 帮助
```

**集成方式**：
- 手动调用（开发流程中）
- 可后续集成到 GitHub Actions（手动 trigger 或 PR comment）
- 不修改现有 verify-archive.sh（向后兼容）

---

## 6. Verdict

**ACCEPTED** — 10/10 AC

lite-mode 推广 7 站收官工具。`scripts/verify-spec-links.sh` 自动校验 7 个 d{N} 域 CHANGELOG.md 中 103 个 archive 链接，发现 39 个 PASS + 64 个 WARN（legacy archives predates convention）+ 0 个 FAIL。所有 lite-mode 7 站 PR 的链接（DM-20260630-003/004/005/006/007/008/009）全部 PASS。