#!/usr/bin/env bash
# Print line and block coverage by architecture domain (D1-D5).
# Usage: ./scripts/coverage-domains.sh [--json] [output_dir]
set -euo pipefail

cd "$(dirname "$0")/.."

OUT_DIR="${1:-coverage-reports}"
JSON=false
if [[ "${1:-}" == "--json" ]]; then
  JSON=true
  OUT_DIR="${2:-coverage-reports}"
fi

mkdir -p "$OUT_DIR"

python3 - "$OUT_DIR" "$JSON" <<'PY'
import json, re, subprocess, sys
from pathlib import Path
from collections import defaultdict

root = Path(".")
out_dir = Path(sys.argv[1])
as_json = sys.argv[2].lower() == "true"

domains = {
    "D1 Communication": "internal/layers/communication",
    "D2 Context Engine": "internal/layers/contextengine",
    "D3 LLM Gateway": "internal/layers/llmgateway",
    "D4 Multi-Agent": "internal/layers/multiagent",
    "D5 Observability": "internal/layers/observability",
    "Shared": "internal/shared",
    "Bridges": "internal/bridges",
    "Bootstrap": "internal/bootstrap",
}

integration_tags = "integration,d1,d2,d3,d4,d5,cross"


def parse_coverprofile(path: Path):
    blocks = []
    if not path.exists() or path.stat().st_size == 0:
        return blocks
    for line in path.read_text().splitlines():
        if line.startswith("mode:") or not line.strip():
            continue
        m = re.match(r"(.+):(\d+\.\d+),(\d+\.\d+) (\d+) (\d+)", line)
        if not m:
            continue
        blocks.append({
            "file": m.group(1),
            "num_stmt": int(m.group(4)),
            "count": int(m.group(5)),
            "covered": int(m.group(5)) > 0,
        })
    return blocks


def merge(*profiles):
    combined = {}
    for blocks in profiles:
        for b in blocks:
            key = (b["file"], b["num_stmt"])
            if key not in combined or b["count"] > combined[key]["count"]:
                combined[key] = b
    return list(combined.values())


def stats(blocks, prefix=None):
    if prefix:
        blocks = [b for b in blocks if prefix in b["file"]]
    ts = sum(b["num_stmt"] for b in blocks)
    cs = sum(b["num_stmt"] for b in blocks if b["covered"])
    tb = len(blocks)
    cb = sum(1 for b in blocks if b["covered"])
    if ts == 0:
        return {"line_pct": 0.0, "block_pct": 0.0, "covered_stmt": 0, "total_stmt": 0, "covered_blocks": 0, "total_blocks": 0}
    return {
        "line_pct": round(100.0 * cs / ts, 1),
        "block_pct": round(100.0 * cb / tb, 1),
        "covered_stmt": cs,
        "total_stmt": ts,
        "covered_blocks": cb,
        "total_blocks": tb,
    }


def run_go_test(args, prof: Path):
    cmd = ["go", "test"] + args + [
        f"-coverprofile={prof}",
        "-covermode=atomic",
        "-count=1",
    ]
    r = subprocess.run(cmd, cwd=root, capture_output=True, text=True)
    return r.returncode, parse_coverprofile(prof)


rows = []
all_merged = []

for name, path in domains.items():
    if not (root / path).exists():
        continue
    uprof = out_dir / f"unit_{path.replace('/', '_')}.out"
    iprof = out_dir / f"int_{path.replace('/', '_')}.out"
    _, ub = run_go_test([f"./{path}/...", "-timeout=180s"], uprof)
    _, ib = run_go_test([
        "-tags", integration_tags,
        "./tests/integration/...",
        f"-coverpkg=./{path}/...",
        "-timeout=300s",
    ], iprof)
    merged = merge(ub, ib)
    all_merged.extend(merged)
    unit = stats(ub, path)
    both = stats(merged, path)
    rows.append({
        "domain": name,
        "path": path,
        "unit_line_pct": unit["line_pct"],
        "unit_block_pct": unit["block_pct"],
        "merged_line_pct": both["line_pct"],
        "merged_block_pct": both["block_pct"],
        "total_stmt": both["total_stmt"],
    })

layer_blocks = [b for b in merge(*[parse_coverprofile(out_dir / f"unit_{p.replace('/', '_')}.out") for p in domains.values() if (root / p).exists()]) if "/layers/" in b["file"]]
# recompute totals from rows for layers only
layer_rows = [r for r in rows if r["domain"].startswith("D")]
if layer_rows:
    total_stmt = sum(r["total_stmt"] for r in layer_rows)
else:
    total_stmt = 0

summary_path = out_dir / "coverage-by-domain.md"
lines = [
    "# Coverage by Domain",
    "",
    "| Domain | Unit Line | Unit Block | +Integration Line | +Integration Block | Stmts |",
    "|--------|-----------|------------|-------------------|---------------------|-------|",
]
for r in rows:
    lines.append(
        f"| {r['domain']} | {r['unit_line_pct']:.1f}% | {r['unit_block_pct']:.1f}% | "
        f"{r['merged_line_pct']:.1f}% | {r['merged_block_pct']:.1f}% | {r['total_stmt']} |"
    )
lines.extend(["", "_Block % approximates branch coverage (Go coverprofile blocks)._"])
summary_path.write_text("\n".join(lines) + "\n")

if as_json:
    print(json.dumps({"domains": rows, "report": str(summary_path)}, indent=2))
else:
    print(summary_path.read_text())
PY
