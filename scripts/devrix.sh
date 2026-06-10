#!/usr/bin/env bash
# Devrix standard launcher — single binary, IM enabled via ~/.devrix/config.yaml.
#
# Usage:
#   ./scripts/devrix.sh start|stop|restart|status   # server (IM when configured)
#   ./scripts/devrix.sh cli                         # interactive CLI (DEVRIX_CLI=1)
#   ./scripts/devrix.sh build                       # rebuild bin/devrix
#
# Env:
#   DEVRIX_FORCE=1        restart even if pidfile says running
#   DEVRIX_SCREEN=1       start inside GNU screen session "devrix"
#   DEVRIX_ENGINE=context context engine mode (default)
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck disable=SC1091
source "$ROOT/scripts/lib/devrix-process.sh"

usage() {
  sed -n '4,12p' "$0" | sed 's/^# \{0,1\}//'
  exit "${1:-0}"
}

cmd_status() {
  devrix_process_init
  echo "=== devrix status ==="
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
  screen -ls 2>/dev/null | grep '[.]devrix' || true
  if [[ -f "$LOG" ]]; then
    echo "--- last log lines ($LOG) ---"
    tail -8 "$LOG"
    echo "--- recent inbound ---"
    grep -E 'RouteInbound called|message received|context engine: Process' "$LOG" | tail -5 || echo "(none)"
  fi
}

cmd_stop() {
  devrix_process_init
  local stale
  for stale in "${STALE_BINS[@]}"; do
    if [[ -f "$stale" ]]; then
      echo "removing stale binary: $stale"
      rm -f "$stale"
    fi
  done
  echo "stopping all devrix server processes..."
  stop_all_devrix_servers
  echo "done"
}

cmd_start() {
  devrix_process_init

  local stale
  for stale in "${STALE_BINS[@]}"; do
    if [[ -f "$stale" ]]; then
      echo "removing stale binary: $stale"
      rm -f "$stale"
    fi
  done

  if [[ "${DEVRIX_FORCE:-}" != "1" && -f "$PIDFILE" ]]; then
    local existing_pid
    existing_pid="$(cat "$PIDFILE" 2>/dev/null || true)"
    if is_devrix_server_pid "$existing_pid"; then
      echo "devrix already running pid=$existing_pid (DEVRIX_FORCE=1 to restart)"
      exit 0
    fi
  fi

  stop_all_devrix_servers

  if [[ ! -x "$BIN" ]]; then
    devrix_build
  fi

  devrix_load_env
  if [[ -z "${MINIMAX_API_KEY:-}" ]]; then
    echo "warning: MINIMAX_API_KEY is not set — LLM gateway may use mock"
  fi

  export DEVRIX_ENGINE="${DEVRIX_ENGINE:-context}"

  local bin_mtime=""
  if [[ -f "$BIN" ]]; then
    bin_mtime="$(stat -f '%Sm' -t '%Y-%m-%d %H:%M:%S' "$BIN" 2>/dev/null || stat -c '%y' "$BIN" 2>/dev/null | cut -d. -f1 || true)"
  fi

  {
    echo "==== devrix start $(date '+%Y-%m-%d %H:%M:%S') engine=$DEVRIX_ENGINE bin=$BIN mtime=$bin_mtime minimax_key=${MINIMAX_API_KEY:+set} ===="
  } >>"$LOG"

  echo "starting devrix (log: $LOG, engine: $DEVRIX_ENGINE, mtime: $bin_mtime)"
  echo "IM provider from ~/.devrix/config.yaml — enable im.enabled + im.platform.provider"

  local run_env=("DEVRIX_ENGINE=$DEVRIX_ENGINE")
  if [[ -n "${MINIMAX_API_KEY:-}" ]]; then
    run_env+=("MINIMAX_API_KEY=$MINIMAX_API_KEY")
  fi

  if [[ "${DEVRIX_SCREEN:-}" == "1" ]]; then
    if screen -ls | grep -q '[.]devrix'; then
      echo "error: screen session devrix already exists"
      exit 1
    fi
    local screen_cmd="exec $(printf '%q' "$BIN")"
    screen -dmS devrix bash -lc "$screen_cmd >>$LOG 2>&1"
    sleep 2
    local server_pid
    server_pid="$(list_devrix_server_pids | head -1 || true)"
    if [[ -z "$server_pid" ]]; then
      echo "error: devrix failed to start in screen — check $LOG"
      exit 1
    fi
    echo "$server_pid" >"$PIDFILE"
    echo "devrix running pid=$server_pid (screen: devrix)"
    echo "tail -f $LOG"
    return 0
  fi

  nohup env "${run_env[@]}" "$BIN" >>"$LOG" 2>&1 &
  echo $! >"$PIDFILE"
  sleep 1

  if ! is_devrix_server_pid "$(cat "$PIDFILE")"; then
    echo "error: devrix failed to start — check $LOG"
    exit 1
  fi

  echo "devrix running pid=$(cat "$PIDFILE")"
  echo "tail -f $LOG"
}

cmd_cli() {
  devrix_process_init
  devrix_load_env
  if [[ ! -x "$BIN" ]]; then
    devrix_build
  fi
  echo "starting CLI — if IM is enabled in ~/.devrix/config.yaml, only run one devrix process."
  exec env DEVRIX_CLI=1 "$BIN"
}

cmd_build() {
  devrix_build
}

main() {
  local top="${1:-start}"
  case "$top" in
    start) cmd_start ;;
    stop) cmd_stop ;;
    restart)
      cmd_stop
      DEVRIX_FORCE=1 cmd_start
      ;;
    status) cmd_status ;;
    cli) cmd_cli ;;
    build) cmd_build ;;
    stop-all) cmd_stop ;;
    -h|--help|help) usage 0 ;;
    *)
      echo "unknown command: $top"
      usage 1
      ;;
  esac
}

main "$@"
