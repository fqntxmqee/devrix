#!/usr/bin/env python3
"""check_t_aliases.py — D3 t-registry.md §Legacy Archive 100% 覆盖校验

校验目标：openspec/specs/d3-llm-gateway/t-registry.md
校验内容：
  1. 解析 §Legacy Archive 表格的 (old_id, new_id) 映射
  2. 解析当前注册表的所有 T ID
  3. 确保每个旧 ID 在新表中都有对应的新 ID
  4. 扫描 *_test.go 文件中的 // T: D3-S*-A*-T* 注释，确认旧 T ID 仍有 alias 追溯
  5. 扫描 spec.md / design.md / layer-delta.md 中的 `D3-S*-A*-T*` 引用

退出码：
  0 — alias 100% 覆盖
  1 — 有旧 T ID 缺 alias / 引用未对齐
  2 — t-registry.md 解析失败
"""
import re
import sys
from pathlib import Path
from typing import Dict, List, Set, Tuple

REPO_ROOT = Path(__file__).resolve().parents[1]  # scripts/.. → repo root
D3_SPEC_DIR = REPO_ROOT / "openspec" / "specs" / "d3-llm-gateway"
D3_TEST_DIR = REPO_ROOT / "internal" / "layers" / "llmgateway"
D3_BRIDGE_DIR = REPO_ROOT / "internal" / "bridges" / "llm"

OLD_T_RE = re.compile(r"D3-S[1-7]-A\d{2}-T\d{2}")
NEW_T_RE = re.compile(r"D3-(?:S[1-6]|X)-A\d{2}(?:-F\d{2})?-T\d{2}")


def parse_legacy_archive(t_registry: Path) -> Dict[str, str]:
    """从 t-registry.md §Legacy Archive 提取 (old_id, new_id) 映射。"""
    mapping: Dict[str, str] = {}
    if not t_registry.exists():
        print(f"ERROR: t-registry.md not found: {t_registry}", file=sys.stderr)
        sys.exit(2)

    in_legacy = False
    text = t_registry.read_text(encoding="utf-8")
    for line in text.splitlines():
        if re.match(r"^##\s+.*Legacy Archive", line):
            in_legacy = True
            continue
        if in_legacy and line.startswith("## "):
            break  # 离开 Legacy Archive 段
        if not in_legacy:
            continue
        # Legacy Archive 表格行格式：| D3-S1-A01-T01 | D3-S2-A01-T01 | <note> |
        # 新 ID 支持 D3-S[1-6] (5+1 S) 或 D3-X- (CROSS 跨域锚点)
        m = re.match(
            r"\|\s*(D3-S[1-7]-A\d{2}-T\d{2})\s*\|\s*(D3-(?:S[1-6]|X)-A\d{2}(?:-F\d{2})?-T\d{2})\s*\|",
            line,
        )
        if m:
            mapping[m.group(1)] = m.group(2)
    return mapping


def parse_current_t_ids(t_registry: Path) -> Set[str]:
    """从 t-registry.md 当前段（§D3-S1 等）提取所有 T ID。"""
    ids: Set[str] = set()
    text = t_registry.read_text(encoding="utf-8")
    for m in NEW_T_RE.finditer(text):
        ids.add(m.group(0))
    return ids


def scan_test_references(test_dirs: List[Path]) -> Set[str]:
    """扫描 *_test.go 中的 // T: D3-S*-A*-T* 注释。"""
    refs: Set[str] = set()
    for d in test_dirs:
        if not d.exists():
            continue
        for f in d.rglob("*_test.go"):
            text = f.read_text(encoding="utf-8", errors="ignore")
            for m in re.finditer(r"//\s*T:\s*(D3-S[1-7]-A\d{2}-T\d{2})", text):
                refs.add(m.group(1))
    return refs


def scan_spec_references(spec_dir: Path) -> Set[str]:
    """扫描 spec.md / design.md / layer-delta.md 中的 T ID 引用。"""
    refs: Set[str] = set()
    for fname in ["spec.md", "design.md", "layer-delta.md"]:
        f = spec_dir / fname
        if not f.exists():
            continue
        text = f.read_text(encoding="utf-8", errors="ignore")
        for m in re.finditer(r"D3-S[1-7]-A\d{2}-T\d{2}", text):
            refs.add(m.group(0))
    return refs


def main() -> int:
    t_registry = D3_SPEC_DIR / "t-registry.md"
    print(f"==> 解析 t-registry.md: {t_registry}")
    legacy_map = parse_legacy_archive(t_registry)
    print(f"    Legacy Archive 映射数: {len(legacy_map)}")

    current_ids = parse_current_t_ids(t_registry)
    print(f"    当前 T ID 数（含 alias）: {len(current_ids)}")

    test_refs = scan_test_references([D3_TEST_DIR, D3_BRIDGE_DIR])
    print(f"    测试文件 // T: 注释引用数: {len(test_refs)}")

    spec_refs = scan_spec_references(D3_SPEC_DIR)
    print(f"    spec/design/layer-delta T 引用数: {len(spec_refs)}")

    # 1. 校验：所有旧 T ID 都在 Legacy Archive 中
    all_old_refs = test_refs | spec_refs
    missing_alias: List[str] = []
    for old_id in sorted(all_old_refs):
        # 如果是已对齐的（用了新 ID 格式 D3-S[1-6] 或 D3-X），不需要 alias
        if re.match(r"D3-(?:S[1-6]|X)-A\d{2}(-F\d{2})?-T\d{2}", old_id):
            continue
        # 旧 ID 格式 (D3-S[1-7]) 必须有 alias
        if old_id not in legacy_map:
            missing_alias.append(old_id)

    # 2. 校验：Legacy Archive 中每个 (old → new) 映射，new ID 必须在 current_ids
    invalid_alias: List[Tuple[str, str]] = []
    for old, new in legacy_map.items():
        if new not in current_ids:
            invalid_alias.append((old, new))

    # 3. 输出结果
    print()
    print("==> 校验结果")
    if missing_alias:
        print(f"  ❌ 缺 alias 的旧 T ID 数: {len(missing_alias)}")
        for tid in missing_alias[:10]:
            print(f"     - {tid}")
        if len(missing_alias) > 10:
            print(f"     ... ({len(missing_alias) - 10} more)")
    else:
        print(f"  ✅ 所有旧 T ID 都有 alias（覆盖数: {len(all_old_refs & set(legacy_map.keys()))}）")

    if invalid_alias:
        print(f"  ❌ alias 指向不存在的 T ID 数: {len(invalid_alias)}")
        for old, new in invalid_alias[:10]:
            print(f"     - {old} → {new} (new ID not in current registry)")
    else:
        print(f"  ✅ 所有 alias 都指向 current T ID")

    # 4. 退出码
    if missing_alias or invalid_alias:
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
