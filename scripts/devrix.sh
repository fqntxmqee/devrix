#!/usr/bin/env bash
# Devrix standard launcher — single entry for Feishu bot, CLI, build, and process control.
#
# Why two binaries?
#   cmd/devrix-feishu  — Feishu-only daemon (recommended for 飞书机器人)
#   cmd/devrix         — CLI REPL (+ optional Feishu if ~/.devrix/config enables IM)
# Only ONE should hold the Feishu WebSocket; this script stops conflicts before start.
#
# Usage:
#   ./scripts/devrix.sh feishu start|stop|restart|status   # default mode
#   ./scripts/devrix.sh cli                                # interactive CLI
#   ./scripts/devrix.sh build                              # rebuild both binaries
#   ./scripts/devrix.sh stop-all                           # stop every devrix server
#
# Env:
#   DEVRIX_FEISHU_FORCE=1   restart even if pidfile says running
#   DEVRIX_FEISHU_FG=1      start feishu with nohup + log tail hint
#   DEVRIX_FEISHU_SCREEN=1  start feishu inside GNU screen
#   DEVRIX_ENGINE=context   context engine mode (default)
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck disable=SC1091
source "$ROOT/scripts/lib/devrix-process.sh"

usage() {
  sed -n '4,14p' "$0" | sed 's/^# \{0,1\}//'
  exit "${1:-0}"
}

cmd_feishu_status() {
  devrix_process_init
  echo "=== devrix feishu status ==="
  if [[ -f "$PIDFILE" ]]; then
    local pid
    pid="$(cat "$PIDFILE")"
    if kill -0 "$pid" 2>/dev/null; then
      echo "pidfile: $pid (running) — $(ps -p "$pid" -o command= 2>/dev/null | head -c 120)"
    else
      echo "pidfile: $pid (stale)"
    fi
  else
    echo "pidfile: not found"
  fi
  echo "--- matching server processes ---"
  list_devrix_server_pids | while read -r pid; do
    [[ -n "$pid" ]] || continue
    echo "  $pid $(ps -p "$pid" -o command= 2>/dev/null | head -c 120)"
  done
  screen -ls 2>/dev/null | grep devrix-feishu || true
  if [[ -f "$LOG" ]]; then
    echo "--- last log lines ($LOG) ---"
    tail -8 "$LOG"
    echo "--- recent inbound ---"
    grep -E 'RouteInbound called|message received|context engine: Process' "$LOG" | tail -5 || echo "(none)"
  fi
}

cmd_feishu_stop() {
  devrix_process_init
  if [[ -f "$STALE_BIN" ]]; then
    echo "removing stale demo binary: $STALE_BIN"
    rm -f "$STALE_BIN"
  fi
  echo "stopping all devrix server processes..."
  stop_all_devrix_servers
  echo "done"
}

cmd_feishu_start() {
  devrix_process_init

  if [[ -f "$STALE_BIN" ]]; then
    echo "removing stale demo binary: $STALE_BIN"
    rm -f "$STALE_BIN"
  fi

  if [[ "${DEVRIX_FEISHU_FORCE:-}" != "1" && -f "$PIDFILE" ]]; then
    local existing_pid
    existing_pid="$(cat "$PIDFILE" 2>/dev/null || true)"
    if is_devrix_server_pid "$existing_pid"; then
      echo "devrix-feishu already running pid=$existing_pid (DEVRIX_FEISHU_FORCE=1 to restart)"
      exit 0
    fi
  fi

  stop_all_devrix_servers

  if [[ ! -x "$BIN_FEISHU" ]]; then
    devrix_build
  fi

  devrix_load_env
  if [[ -z "${MINIMAX_API_KEY:-}" ]]; then
    echo "warning: MINIMAX_API_KEY is not set — LLM gateway may use mock"
  fi

  export DEVRIX_ENGINE="${DEVRIX_ENGINE:-context}"

  local bin_mtime=""
  if [[ -f "$BIN_FEISHU" ]]; then
    bin_mtime="$(stat -f '%Sm' -t '%Y-%m-%d %H:%M:%S' "$BIN_FEISHU" 2>/dev/null || stat -c '%y' "$BIN_FEISHU" 2>/dev/null | cut -d. -f1 || true)"
  fi

  {
    echo "==== devrix-feishu start $(date '+%Y-%m-%d %H:%M:%S') engine=$DEVRIX_ENGINE bin=$BIN_FEISHU mtime=$bin_mtime minimax_key=${MINIMAX_API_KEY:+set} ===="
  } >>"$LOG"

  echo "starting devrix-feishu (log: $LOG, engine: $DEVRIX_ENGINE, mtime: $bin_mtime)"

  local run_env=("DEVRIX_ENGINE=$DEVRIX_ENGINE")
  if [[ -n "${MINIMAX_API_KEY:-}" ]]; then
    run_env+=("MINIMAX_API_KEY=$MINIMAX_API_KEY")
  fi

  if [[ "${DEVRIX_FEISHU_SCREEN:-}" == "1" ]]; then
    if screen -ls | grep -q '[.]devrix-feishu'; then
      echo "error: screen session devrix-feishu already exists"
      exit 1
    fi
    local screen_cmd="exec $(printf '%q' "$ROOT/scripts/devrix-feishu-launch.sh")"
    screen -dmS devrix-feishu bash -lc "$screen_cmd"
    sleep 2
    local server_pid
    server_pid="$(list_devrix_server_pids | head -1 || true)"
    if [[ -z "$server_pid" ]]; then
      echo "error: devrix-feishu failed to start in screen — check $LOG"
      exit 1
    fi
    echo "$server_pid" >"$PIDFILE"
    echo "devrix-feishu running pid=$server_pid (screen: devrix-feishu)"
    echo "tail -f $LOG"
    return 0
  fi

  nohup env "${run_env[@]}" "$BIN_FEISHU" >>"$LOG" 2>&1 &
  echo $! >"$PIDFILE"
  sleep 1

  if ! is_devrix_server_pid "$(cat "$PIDFILE")"; then
    echo "error: devrix-feishu failed to start — check $LOG"
    exit 1
  fi

  echo "devrix-feishu running pid=$(cat "$PIDFILE")"
  echo "tail -f $LOG"
}

cmd_feishu() {
  local action="${1:-start}"
  case "$action" in
    start) cmd_feishu_start ;;
    stop) cmd_feishu_stop ;;
    restart)
      cmd_feishu_stop
      DEVRIX_FEISHU_FORCE=1 cmd_feishu_start
      ;;
    status) cmd_feishu_status ;;
    -h|--help|help) usage 0 ;;
    *)
      echo "unknown feishu action: $action (use start|stop|restart|status)"
      exit 1
      ;;
  esac
}

cmd_cli() {
  devrix_process_init
  devrix_load_env
  if [[ ! -x "$BIN_CLI" ]]; then
    devrix_build
  fi
  echo "starting CLI ($BIN_CLI) — if Feishu IM is enabled in ~/.devrix/config.yaml,"
  echo "prefer ./scripts/devrix.sh feishu start to avoid duplicate WebSocket connections."
  exec "$BIN_CLI"
}

cmd_build() {
  devrix_build
}

cmd_stop_all() {
  cmd_feishu_stop
}

main() {
  local top="${1:-feishu}"
  case "$top" in
    feishu) cmd_feishu "${2:-start}" ;;
    cli) cmd_cli ;;
    build) cmd_build ;;
    stop-all|stop) cmd_stop_all ;;
    -h|--help|help) usage 0 ;;
    start)
      # ./scripts/devrix.sh start → feishu start (backward compat with run-feishu.sh)
      cmd_feishu start
      ;;
    restart)
      cmd_feishu restart
      ;;
    status)
      cmd_feishu status
      ;;
    *)
      echo "unknown command: $top"
      usage 1
      ;;
  esac
}

main "$@"
