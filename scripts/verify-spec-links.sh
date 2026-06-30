#!/bin/bash
# verify-spec-links.sh — CHANGELOG.md archive 链接有效性校验
#
# 校验 openspec/specs/d{1..7}-*/CHANGELOG.md 中所有 [archive](../../archive/...) 链接：
#   - 归档目录存在
#   - change-id 与目录名一致
#   - .openspec.yaml status=s7_archived
#
# Usage:
#   ./scripts/verify-spec-links.sh           # 默认 WARN 模式
#   ./scripts/verify-spec-links.sh --strict  # 严格模式（链接失效 Exit 1）
#   ./scripts/verify-spec-links.sh --domain d4  # 仅校验指定域
#   ./scripts/verify-spec-links.sh --help    # 帮助
#
# Exit: 0 = 全部有效（或仅 WARN）, 1 = --strict 模式下存在失效链接

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

STRICT=false
DOMAIN_FILTER=""

while [ $# -gt 0 ]; do
  case "$1" in
    --strict) STRICT=true; shift ;;
    --domain) DOMAIN_FILTER="$2"; shift 2 ;;
    --help)
      sed -n '2,15p' "$0" | sed 's/^# //; s/^#//'
      exit 0
      ;;
    *) echo "ERROR: unknown arg $1" >&2; exit 2 ;;
  esac
done

cd "$(dirname "$0")/.."
REPO_ROOT="$(pwd)"

pass_count=0
warn_count=0
fail_count=0

pass() { echo -e "  ${GREEN}✓${NC} $1"; pass_count=$((pass_count + 1)); }
warn() { echo -e "  ${YELLOW}⚠${NC} $1"; warn_count=$((warn_count + 1)); }
fail() { echo -e "  ${RED}✗${NC} $1"; fail_count=$((fail_count + 1)); }

echo "=== Spec Links Validation ==="
echo "Mode: $([ "$STRICT" = true ] && echo STRICT || echo WARN)"
[ -n "$DOMAIN_FILTER" ] && echo "Filter: $DOMAIN_FILTER"
echo

# 1. 扫描 CHANGELOG.md
if [ -n "$DOMAIN_FILTER" ]; then
  CHANGELOGS=$(find "openspec/specs/${DOMAIN_FILTER}-"*/CHANGELOG.md 2>/dev/null | sort)
else
  CHANGELOGS=$(find openspec/specs/d{1,2,3,4,5,6,7}-*/CHANGELOG.md 2>/dev/null | sort)
fi

if [ -z "$CHANGELOGS" ]; then
  echo "ERROR: no CHANGELOG.md found"
  exit 2
fi

file_count=$(echo "$CHANGELOGS" | wc -l | tr -d ' ')
total_links=0
echo "扫描 $file_count 个 CHANGELOG.md 文件..."
echo

# 2. 对每个 CHANGELOG.md 提取并验证链接
for changelog in $CHANGELOGS; do
  domain=$(basename "$(dirname "$changelog")")
  echo "## $domain"

  # 提取 [archive](../../archive/YYYY-MM-DD-devrix-{name}/) 链接
  links=$(grep -oE '\[archive\]\(\.\./\.\./archive/[^)]+/\)' "$changelog" 2>/dev/null | \
          sed -E 's|\[archive\]\(\.\./\.\./archive/([^)]+)/\)|\1|' | sort -u)

  if [ -z "$links" ]; then
    warn "no archive links found"
    echo
    continue
  fi

  link_count=$(echo "$links" | wc -l | tr -d ' ')
  total_links=$((total_links + link_count))

  for link in $links; do
    archive_dir="openspec/archive/$link"

    # 验证 1：目录存在
    if [ ! -d "$archive_dir" ]; then
      fail "$link: archive directory not found: $archive_dir"
      continue
    fi

    # 验证 2：.openspec.yaml 存在（legacy archives 2026-06-18 之前缺失，仅 WARN）
    openspec_yaml="$archive_dir/.openspec.yaml"
    if [ ! -f "$openspec_yaml" ]; then
      warn "$link: .openspec.yaml not found (legacy archive, predates convention)"
      continue
    fi

    # 验证 3：status=s7_archived
    status=$(grep -E "^status:" "$openspec_yaml" | sed 's/status: *//')
    if [ "$status" != "s7_archived" ]; then
      warn "$link: status=$status (expected s7_archived)"
      continue
    fi

    # 验证 4：change-id 与目录名一致
    change_id=$(grep -E "^change_id:" "$openspec_yaml" | sed 's/change_id: *//')
    # 目录名格式 YYYY-MM-DD-devrix-{name} → change-id devrix-{name}
    expected_id="${link#*-*-*-}"  # 去掉日期前缀
    if [ "$change_id" != "$expected_id" ]; then
      warn "$link: change_id=$change_id != expected $expected_id"
      continue
    fi

    pass "$link"
  done

  echo
done

# 3. 汇总
echo "=== Summary ==="
echo "扫描文件: $file_count"
echo "链接总数: $total_links"
echo "  ✓ Pass: $pass_count"
echo "  ⚠ Warn: $warn_count"
echo "  ✗ Fail: $fail_count"
echo

if [ $fail_count -gt 0 ]; then
  echo -e "${RED}FAILED${NC}: $fail_count invalid archive link(s)"
  exit 1
elif [ "$STRICT" = true ] && [ $warn_count -gt 0 ]; then
  echo -e "${YELLOW}STRICT MODE FAILED${NC}: $warn_count warning(s)"
  exit 1
else
  echo -e "${GREEN}OK${NC}: all archive links valid"
  exit 0
fi