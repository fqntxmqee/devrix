#!/usr/bin/env python3
"""One-shot migration: L5 test IDs → DSAFT T-layer IDs in Go source comments."""

from __future__ import annotations

import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]

# Symbolic L5 IDs → DSAFT T IDs (authority: openspec/t-registry.md)
SYMBOLIC: dict[str, str] = {
    # CROSS / D0 layer lint
    "L5-0-0-01": "CROSS-A01-T01",
    "L5-0-0-02": "CROSS-A02-T03",
    "L5-0-0-03": "CROSS-A01-T02",
    "L5-0-0-04": "CROSS-A01-T04",
    "L5-0-1-06": "D0-S1-A01-T06",
    "L5-0-1-07": "D0-S1-A01-T07",
    # D2 CTX symbolic
    "L5-CTX-01": "D2-S3-A01-T01",
    "L5-CTX-02": "D2-S3-A01-T01",
    "L5-CTX-03": "D2-S2-A01-T01",
    "L5-CTX-04": "D2-S2-A01-T02",
    "L5-CTX-05": "D2-S3-A01-T02",
    "L5-CTX-06": "D2-S1-A01-T01",
    "L5-CTX-08": "D2-S2-A01-T05",
    "L5-CTX-09": "D2-S1-A01-T03",
    "L5-CTX-10": "D2-S3-A01-T05",
    "L5-CTX-11": "D2-S1-A01-T11",
    "L5-CTX-12": "D2-S2-A01-T03",
    "L5-CTX-13": "D2-S2-A01-T04",
    "L5-CTX-14": "D2-S1-A02-T02",
    "L5-CTX-15": "D2-S1-A02-T06",
    "L5-CTX-16": "D2-S4-A01-T01",
    "L5-CTX-17": "D2-S0-A01-T01",
    "L5-CTX-18": "D2-S0-A01-T02",
    "L5-CTX-19": "D2-S3-A01-T03",
    "L5-CTX-20": "D2-S1-A03-T07",
    "L5-CTX-21": "D2-S1-A03-T08",
    "L5-CTX-22": "D2-S3-A01-T03",
    "L5-CTX-23": "D2-S3-A01-T04",
    "L5-CTX-24": "D2-S0-A01-T03",
    "L5-CTX-26": "D2-S1-A02-T09",
    "L5-CTX-27": "D2-S1-A02-T10",
    "L5-CTX-28": "D2-S1-A01-T12",
    "L5-CTX-29": "D2-S1-A01-T11",
    "L5-CTX-30": "D2-S2-A01-T07",
    "L5-CTX-31": "D2-S2-A01-T06",
    "L5-CTX-32": "D2-S3-A01-T06",
    "L5-CTX-33": "D2-S2-A01-T07",
    "L5-CTX-34": "D2-S10-A01-T34",
    "L5-CTX-35": "D2-S10-A01-T35",
    "L5-CTX-36": "D2-S10-A01-T36",
    "L5-CTX-37": "D2-S10-A01-T37",
    "L5-CTX-38": "D2-S10-A01-T38",
    "L5-CTX-39": "D2-S10-A01-T39",
    "L5-CTX-40": "D2-S10-A01-T40",
    "L5-CTX-41": "D2-S10-A01-T41",
    "L5-CTX-42": "D2-S10-A01-T42",
  # Tools / sandbox
    "L5-TOOL-01": "D2-S8-A01-T01",
    "L5-TOOL-03": "D2-S9-A03-T05",
    "L5-TOOL-04": "D2-S8-A01-T01",
    # D1 COMM symbolic (gateway session / permission)
    "L5-COMM-01": "D1-S1-A01-T01",
    "L5-COMM-02": "D1-S1-A01-T01",
    "L5-COMM-03": "D1-S1-A01-T02",
    "L5-COMM-04": "D1-S3-A01-T01",
    "L5-COMM-05": "D1-S3-A01-T02",
    "L5-COMM-06": "D1-S3-A01-T03",
    "L5-COMM-07": "D1-S1-A01-T04",
    "L5-COMM-08": "D1-S1-A01-T03",
    # D3 LLM symbolic
    "L5-LLM-03": "D3-S3-A01-T01",
    "L5-LLM-04": "D3-S3-A01-T02",
    "L5-LLM-05": "D3-S3-A01-T03",
    "L5-LLM-06": "D3-S3-A01-T04",
    "L5-LLM-07": "D3-S5-A01-T01",
    "L5-LLM-08": "D3-S5-A01-T02",
    "L5-LLM-09": "D3-S6-A01-T01",
    "L5-LLM-10": "D3-S4-A01-T02",
    "L5-LLM-11": "D3-S4-A01-T03",
    "L5-LLM-12": "D3-S4-A01-T01",
    "L5-LLM-13": "D3-S2-A01-T01",
    "L5-LLM-14": "D3-S2-A01-T03",
    "L5-LLM-16": "D3-S2-A01-T02",
    "L5-LLM-17": "D3-S1-A01-T01",
    "L5-LLM-18": "D3-S1-A01-T03",
    "L5-LLM-19": "D3-S5-A01-T03",
    "L5-LLM-20": "D3-S2-A01-T04",
    "L5-LLM-21": "D3-S2-A01-T05",
    "L5-LLM-01": "D3-S1-A01-T01",
    "L5-LLM-02": "D3-S1-A01-T02",
    "L5-LLM-23": "D3-S3-A01-T05",
    "L5-2-11-TD01": "D2-S11-A01-TD01",
    "L5-2-11-TD03": "D2-S11-A01-TD03",
    "L5-TOOL-02": "D2-S1-A01-T04",
    # D5 OBS symbolic
    "L5-OBS-15": "D5-S4-A01-T03",
    "L5-OBS-18": "D5-S1-A01-T01",
    "L5-OBS-19": "D5-S2-A01-T06",
    "L5-OBS-20": "D5-S2-A01-T07",
    "L5-OBS-FIX-01": "D5-S2-A01-T03",
    "L5-OBS-FIX-02": "D5-S2-A01-T04",
    "L5-OBS-FIX-03": "D5-S1-A01-T01",
    "L5-OBS-FIX-04": "D5-S3-A01-T03",
    "L5-OBS-FIX-05": "D5-S2-A01-T05",
    "L5-OBS-FIX-06": "D5-S3-A01-T02",
    "L5-OBS-FIX-08": "D5-S1-A01-T02",
    "L5-OBS-EXPORT-01": "D5-S4-A01-T01",
    "L5-OBS-EXPORT-02": "D5-S4-A01-T02",
    "L5-OBS-METRICS-01": "D5-S2-A01-T03",
    "L5-OBS-METRICS-02": "D5-S2-A01-T04",
    "L5-OBS-METRICS-03": "D5-S2-A01-T05",
    "L5-OBS-METRICS-04": "D5-S2-A01-T05",
    "L5-OBS-TRACE-03": "D5-S1-A01-T01",
    "L5-OBS-TRACE-04": "D5-S1-A01-T01",
    "L5-OBS-TRACE-05": "D5-S1-A01-T01",
    "L5-OBS-TRACE-06": "D5-S1-A01-T01",
    "L5-OBS-GENAI-ATTR": "D5-S2-A01-T05",
    "L5-OBS-DECISION-01": "D5-S4-A01-T01",
    "L5-OBS-DECISION-02": "D5-S4-A01-T02",
    "L5-OBS-DECISION-03": "D5-S4-A01-T03",
    # ORCH
    "L5-ORCH-01": "ORCH-S2-T01",
    "L5-ORCH-10": "ORCH-S2-T10",
    "L5-ORCH-11": "ORCH-S2-T11",
    "L5-ORCH-12": "ORCH-S2-T12",
    "L5-ORCH-13": "ORCH-S2-T13",
    "L5-ORCH-14": "ORCH-S2-T14",
    "L5-ORCH-15": "ORCH-S2-T15",
    "L5-ORCH-16": "ORCH-S2-T16",
    "L5-ORCH-17": "ORCH-S2-T17",
    "L5-ORCH-18": "ORCH-S2-T18",
    "L5-ORCH-19": "ORCH-S2-T19",
    "L5-ORCH-20": "ORCH-S2-T20",
    "L5-ORCH-21": "ORCH-S2-T21",
    # D6 eval symbolic
    "L5-6-3-09": "D6-S3-A01-T09",
    "L5-6-3-10": "D6-S3-A01-T10",
    "L5-6-3-14": "D6-S3-A01-T14",
    "L5-6-3-15": "D6-S3-A01-T15",
}

# D1-S9 EventBus: legacy L5-2-3-NN (was D2-S3 memory numbering collision)
EVENTBUS = {
    "L5-2-3-01": "D1-S9-A01-T01",
    "L5-2-3-02": "D1-S9-A02-T02",
    "L5-2-3-03": "D1-S9-A02-T03",
    "L5-2-3-04": "D1-S9-A02-T04",
    "L5-2-3-05": "D1-S9-A01-T05",
    "L5-2-3-06": "D1-S9-A01-T06",
    "L5-2-3-07": "D1-S9-A01-T07",
}
SYMBOLIC.update(EVENTBUS)

ACTIVITY_RULES: dict[tuple[str, str], dict[str, str]] = {
    ("2", "1"): {"02": "A02", "05": "A02", "06": "A02", "09": "A02", "10": "A02", "07": "A03", "08": "A03"},
    ("2", "9"): {"05": "A03", "14": "A03", "10": "A02", "12": "A02", "13": "A02"},
    ("2", "11"): {},  # all A01
    ("4", "2"): {"02": "A02", "03": "A02"},
    ("4", "3"): {"05": "A02"},
    ("4", "6"): {f"{i:02d}": "A02" for i in range(2, 8)},
    ("4", "10"): {f"{i:02d}": "A02" for i in range(4, 8)},
    ("1", "2"): {f"{i:02d}": "A02" for i in range(3, 9)},
    ("1", "9"): {"02": "A02", "03": "A02", "04": "A02"},
    ("6", "3"): {"02": "A02"},
}


def activity_for(d: str, s: str, nn: str) -> str:
    rules = ACTIVITY_RULES.get((d, s), {})
    return rules.get(nn, "A01")


def convert_numeric(l5_id: str) -> str | None:
    m = re.fullmatch(r"L5-(\d+)-(\d+)-(\d+)", l5_id)
    if not m:
        return None
    d, s, nn = m.group(1), m.group(2), m.group(3)
  # Special domain fixes
    if d == "4" and s == "12":
        return "D2-S12-A01-T01"
    if d == "2" and s == "11":
        suffix = nn
        if nn.startswith("TD") or nn == "D6PR":
            return f"D2-S11-A01-{nn}"
        return f"D2-S11-A01-T{nn}"
    a = activity_for(d, s, f"{int(nn):02d}" if nn.isdigit() else nn)
    t_part = f"T{nn}" if nn.isdigit() else f"T{nn}"
    if nn.isdigit():
        t_part = f"T{int(nn):02d}"
    return f"D{d}-S{s}-{a}-{t_part}"


def convert_l5(l5_id: str) -> str:
    if l5_id in SYMBOLIC:
        return SYMBOLIC[l5_id]
    numeric = convert_numeric(l5_id)
    if numeric:
        return numeric
    # ORCH with extra suffix
    m = re.fullmatch(r"L5-ORCH-(\d+)", l5_id)
    if m:
        return f"ORCH-S2-T{m.group(1)}"
    return l5_id  # leave unchanged — reported later


L5_PATTERN = re.compile(r"L5-(?:[A-Z]+-)?[A-Z0-9-]+")


def migrate_line(line: str) -> str:
    if "L5-" not in line:
        return line
    # Covers: → T:
    line = re.sub(r"//\s*Covers:\s*", "// T: ", line)
    def repl(match: re.Match[str]) -> str:
        old = match.group(0)
        new = convert_l5(old)
        return new

    return L5_PATTERN.sub(repl, line)


def should_process(path: Path) -> bool:
    parts = path.parts
    if ".claude" in parts or "vendor" in parts:
        return False
    return path.suffix == ".go"


def main() -> int:
    unknown: set[str] = set()
    changed_files = 0

    for path in ROOT.rglob("*.go"):
        if not should_process(path):
            continue
        text = path.read_text(encoding="utf-8")
        if "L5-" not in text:
            continue
        lines = text.splitlines(keepends=True)
        new_lines = []
        file_changed = False
        for line in lines:
            new_line = migrate_line(line)
            if new_line != line:
                file_changed = True
            for m in L5_PATTERN.finditer(line):
                old = m.group(0)
                new = convert_l5(old)
                if new == old:
                    unknown.add(old)
            new_lines.append(new_line)
        if file_changed:
            path.write_text("".join(new_lines), encoding="utf-8")
            changed_files += 1
            print(f"updated: {path.relative_to(ROOT)}")

    if unknown:
        print("\nUnmapped L5 IDs (still present):")
        for u in sorted(unknown):
            print(f"  {u}")
    print(f"\n{changed_files} files updated")
    return 1 if unknown else 0


if __name__ == "__main__":
    sys.exit(main())
