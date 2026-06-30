# Design: devrix-verify-spec-links

**Change ID:** devrix-verify-spec-links
**Demand ID:** DM-20260630-010
**Status:** S3_Design
**Template:** `docs/methodology/detail-design-framework.md`（六段式）
**Created:** 2026-06-30

---

## ① 架构目标

- 新增 `scripts/verify-spec-links.sh` ~80 行 Bash
- 扫描 7 个 d{N} 域 CHANGELOG.md 中的 archive 链接
- 验证归档目录存在 + change-id 一致 + status=s7_archived
- Exit 0 / 1

## ② 架构原则

1. **复用 verify-archive.sh 模式**（Bash + pass/fail/warn + 颜色）
2. **零依赖**（grep + sed + 标准 Unix 工具）
3. **WARN 而非 FAIL**（链接失效仅 WARN，不阻断 CI）
4. **单一职责**（仅校验 archive 链接，不校验跨域 SoT 引用）
5. **可独立调用**（不依赖 verify-archive.sh）

## ③ 业务流程

```
./scripts/verify-spec-links.sh
  ↓ 扫描 openspec/specs/d{1..7}-*/CHANGELOG.md
  ↓ 提取 [archive](../../archive/YYYY-MM-DD-devrix-{name}/) 链接
  ↓ 验证每个链接
  ↓ 输出汇总
  ↓ Exit 0 / 1
```

## ④ 领域模型

输入：7 个 CHANGELOG.md 文件 + openspec/archive/ 目录结构
输出：扫描统计 + 失效链接列表（若有）

## ⑤ 核心链路图

```
verify-spec-links.sh
  ├─→ find openspec/specs/d{1..7}-*/CHANGELOG.md
  │   ↓
  │   grep -oE '\[archive\]\(\.\./\.\./archive/([^/]+)/\)' 
  │   ↓
  │   validate:
  │     ├─→ 目录 archive/<name>/ 存在
  │     ├─→ 目录名匹配 YYYY-MM-DD-devrix-{name} 格式
  │     ├─→ .openspec.yaml status=s7_archived
  │     └─→ change-id 与目录名一致
  └─→ 汇总 + Exit
```

## ⑥ 接口 / API 设计

```
Usage: ./scripts/verify-spec-links.sh [options]
  --strict       链接失效时 Exit 1（默认：WARN only）
  --domain D{N}  仅校验指定域
  --help         显示帮助

Exit:
  0 = 全部有效（或仅 WARN）
  1 = --strict 模式下存在失效链接
```

---

## 附录 A：File Manifest

| 文件 | 类型 | 行数 | 说明 |
|------|------|------|------|
| `scripts/verify-spec-links.sh` | NEW | ~80 | CI 工具 |
| 6 change docs | NEW | — | S1-S5 |