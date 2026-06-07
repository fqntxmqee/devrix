#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BIN="${DEVRIX_FEISHU_BIN:-$ROOT/devrix-feishu}"
LOG="${DEVRIX_FEISHU_LOG:-/tmp/devrix-feishu.log}"
PIDFILE="${DEVRIX_FEISHU_PIDFILE:-/tmp/devrix-feishu.pid}"
STALE_BIN="/tmp/devrix-feishu"

is_server_pid() {
  local pid="$1"
  [[ -n "$pid" ]] || return 1
  kill -0 "$pid" 2>/dev/null || return 1
  local cmd
  cmd="$(ps -p "$pid" -o command= 2>/dev/null || true)"
  [[ "$cmd" == *"devrix-feishu"* ]] || return 1
  # Exclude git/IDE commands that merely mention the binary path in arguments.
  [[ "$cmd" != *"git "* ]] || return 1
  case "$cmd" in
    *"$ROOT/devrix-feishu"*|*"./devrix-feishu"*|*"/devrix-feishu "*|"$STALE_BIN"*)
      return 0
      ;;
  esac
  return 1
}

stop_pid() {
  local pid="$1"
  if ! is_server_pid "$pid"; then
    return 0
  fi
  echo "stopping devrix-feishu pid=$pid"
  kill -TERM "$pid" 2>/dev/null || true
  for _ in $(seq 1 10); do
    if ! kill -0 "$pid" 2>/dev/null; then
      return 0
    fi
    sleep 0.5
  done
  echo "force stopping pid=$pid"
  kill -KILL "$pid" 2>/dev/null || true
  sleep 1
}

load_env() {
  if [[ -n "${MINIMAX_API_KEY:-}" ]]; then
    return 0
  fi
  local f
  for f in "$HOME/.devrix/env" "$HOME/.local/share/codex-minimax/.env"; do
    if [[ -f "$f" ]]; then
      set -a
      # shellcheck disable=SC1090
      source "$f"
      set +a
      echo "sourced env from $f"
      return 0
    fi
  done
}

list_server_pids() {
  # Match only the server binary path (avoid tail -f /tmp/devrix-feishu.log false positives).
  pgrep -f "${BIN}\$" 2>/dev/null || true
}

stop_existing() {
  screen -S devrix-feishu -X quit 2>/dev/null || true
  if [[ -f "$PIDFILE" ]]; then
    stop_pid "$(cat "$PIDFILE" 2>/dev/null || true)"
  fi
  # Kill every devrix-feishu server process (not just pidfile — avoids WS ghost connections).
  while read -r pid; do
    [[ -n "$pid" ]] || continue
    stop_pid "$pid"
  done < <(list_server_pids)
  # Legacy demo binary path only (exact match).
  if [[ -x "$STALE_BIN" ]]; then
    stale_pid="$(pgrep -f "^${STALE_BIN}$" 2>/dev/null | head -1 || true)"
    if [[ -n "$stale_pid" ]]; then
      stop_pid "$stale_pid"
    fi
  fi
  rm -f "$PIDFILE"
  # Allow Feishu WS slot to release before reconnecting.
  sleep 3
}

if [[ -f "$STALE_BIN" ]]; then
  echo "removing stale demo binary: $STALE_BIN"
  rm -f "$STALE_BIN"
fi

if [[ "${DEVRIX_FEISHU_FORCE:-}" != "1" && -f "$PIDFILE" ]]; then
  existing_pid="$(cat "$PIDFILE" 2>/dev/null || true)"
  if is_server_pid "$existing_pid"; then
    echo "devrix-feishu already running pid=$existing_pid (set DEVRIX_FEISHU_FORCE=1 to restart)"
    exit 0
  fi
fi

stop_existing

if [[ ! -x "$BIN" ]]; then
  echo "building devrix-feishu..."
  (cd "$ROOT" && go build -o devrix-feishu ./cmd/devrix-feishu)
fi

load_env

if [[ -z "${MINIMAX_API_KEY:-}" ]]; then
  echo "warning: MINIMAX_API_KEY is not set — LLM gateway may use mock"
fi

export DEVRIX_ENGINE="${DEVRIX_ENGINE:-context}"

bin_mtime=""
if [[ -f "$BIN" ]]; then
  bin_mtime="$(stat -f '%Sm' -t '%Y-%m-%d %H:%M:%S' "$BIN" 2>/dev/null || true)"
fi

{
  echo "==== devrix-feishu start $(date '+%Y-%m-%d %H:%M:%S') engine=$DEVRIX_ENGINE bin=$BIN mtime=$bin_mtime minimax_key=${MINIMAX_API_KEY:+set} ===="
} >>"$LOG"

echo "starting devrix-feishu detached (log: $LOG, engine: $DEVRIX_ENGINE, bin_mtime: $bin_mtime)"

run_env=(
  "DEVRIX_ENGINE=$DEVRIX_ENGINE"
)
if [[ -n "${MINIMAX_API_KEY:-}" ]]; then
  run_env+=("MINIMAX_API_KEY=$MINIMAX_API_KEY")
fi

if [[ "${DEVRIX_FEISHU_FG:-}" == "1" ]]; then
  nohup env "${run_env[@]}" "$BIN" >>"$LOG" 2>&1 &
  echo $! >"$PIDFILE"
  echo "devrix-feishu running pid=$(cat "$PIDFILE") (foreground log: tail -f $LOG)"
  exit 0
fi

if [[ "${DEVRIX_FEISHU_SCREEN:-}" == "1" ]]; then
  if screen -ls | grep -q '[.]devrix-feishu'; then
    echo "error: screen session devrix-feishu already exists (DEVRIX_FEISHU_FORCE=1 to replace)"
    exit 1
  fi
  screen_cmd="exec $(printf '%q' "$ROOT/scripts/devrix-feishu-launch.sh")"
  screen -dmS devrix-feishu bash -lc "$screen_cmd"
  sleep 2
  server_pid="$(list_server_pids | head -1 || true)"
  if [[ -z "$server_pid" ]]; then
    echo "error: devrix-feishu failed to start in screen — check $LOG"
    exit 1
  fi
  echo "$server_pid" >"$PIDFILE"
  echo "devrix-feishu running pid=$server_pid (screen: devrix-feishu, log: $LOG)"
  echo "tail -f $LOG"
  exit 0
fi

nohup env "${run_env[@]}" "$BIN" >>"$LOG" 2>&1 &
echo $! >"$PIDFILE"
sleep 1

if ! is_server_pid "$(cat "$PIDFILE")"; then
  echo "error: devrix-feishu failed to start — check $LOG"
  exit 1
fi

echo "devrix-feishu running pid=$(cat "$PIDFILE")"
echo "tail -f $LOG"
