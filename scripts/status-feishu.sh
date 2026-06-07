#!/usr/bin/env bash
set -euo pipefail

PIDFILE="${DEVRIX_FEISHU_PIDFILE:-/tmp/devrix-feishu.pid}"
LOG="${DEVRIX_FEISHU_LOG:-/tmp/devrix-feishu.log}"

echo "=== devrix-feishu status ==="

if [[ -f "$PIDFILE" ]]; then
  pid="$(cat "$PIDFILE")"
  if kill -0 "$pid" 2>/dev/null; then
    echo "pidfile: $pid (running)"
  else
    echo "pidfile: $pid (stale — process not running)"
  fi
else
  echo "pidfile: not found"
fi

pgrep -fl "${DEVRIX_FEISHU_BIN:-/Users/fukai/workspace/devrix/devrix-feishu}\$" || echo "no devrix-feishu process"
screen -ls 2>/dev/null | grep devrix-feishu || true

if [[ -f "$LOG" ]]; then
  echo "--- last log lines ---"
  tail -8 "$LOG"
  echo "--- recent messages ---"
  grep -E 'message received|context engine: Process' "$LOG" | tail -5 || echo "(none)"
fi
