#!/usr/bin/env bash
# Launch helper for launchd / screen — sources API keys then execs the server binary.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BIN="${DEVRIX_FEISHU_BIN:-$ROOT/devrix-feishu}"
LOG="${DEVRIX_FEISHU_LOG:-/tmp/devrix-feishu.log}"

for f in "$HOME/.devrix/env" "$HOME/.local/share/codex-minimax/.env"; do
  if [[ -f "$f" ]]; then
    set -a
    # shellcheck disable=SC1090
    source "$f"
    set +a
    break
  fi
done

export DEVRIX_ENGINE="${DEVRIX_ENGINE:-context}"
cd "$ROOT"
exec >>"$LOG" 2>&1
echo "==== devrix-feishu launch $(date '+%Y-%m-%d %H:%M:%S') engine=$DEVRIX_ENGINE bin=$BIN ===="
exec "$BIN"
