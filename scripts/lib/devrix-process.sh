#!/usr/bin/env bash
# Shared helpers: list/stop Devrix server processes.
# Source from scripts/devrix.sh — do not execute directly.

devrix_process_init() {
  if [[ -z "${ROOT:-}" ]]; then
    ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
  fi
  BIN="${DEVRIX_BIN:-$ROOT/bin/devrix}"
  PIDFILE="${DEVRIX_PIDFILE:-/tmp/devrix.pid}"
  LOG="${DEVRIX_LOG:-/tmp/devrix.log}"
  STALE_BINS=(
    "$ROOT/devrix-feishu"
    "$ROOT/devrix-dingtalk"
    "/tmp/devrix-feishu"
    "/tmp/devrix-dingtalk"
  )
}

devrix_process_cwd() {
  local pid="$1"
  lsof -p "$pid" -a -d cwd -Fn 2>/dev/null | sed -n 's/^n//p' | tail -1
}

# Any long-running Devrix server (IM daemon or CLI with IM enabled).
is_devrix_server_pid() {
  local pid="$1"
  [[ -n "$pid" ]] || return 1
  kill -0 "$pid" 2>/dev/null || return 1
  local cmd cwd
  cmd="$(ps -p "$pid" -o command= 2>/dev/null || true)"
  [[ -n "$cmd" ]] || return 1
  [[ "$cmd" != *"git "* ]] || return 1

  case "$cmd" in
    *"$ROOT/bin/devrix"*|*"./bin/devrix"*|*" $ROOT/devrix"*|*"./devrix "*|*"./devrix"*)
      cwd="$(devrix_process_cwd "$pid")"
      [[ "$cwd" == "$ROOT" ]] && return 0
      ;;
  esac

  local stale
  for stale in "${STALE_BINS[@]}"; do
    case "$cmd" in
      *"$stale"*)
        return 0
        ;;
    esac
  done

  case "$cmd" in
    *go\ run*cmd/devrix/main*|*go\ run\ ./cmd/devrix*|*go\ run\ ./cmd/devrix/main*)
      cwd="$(devrix_process_cwd "$pid")"
      [[ "$cwd" == "$ROOT" ]] && return 0
      ;;
    *go-build*/exe/main*)
      cwd="$(devrix_process_cwd "$pid")"
      [[ "$cwd" == "$ROOT" ]] && return 0
      ;;
  esac

  return 1
}

list_devrix_server_pids() {
  local pid seen=$'\n'
  append_unique_pid() {
    local p="$1"
    [[ -n "$p" ]] || return 0
    [[ "$seen" == *$'\n'"$p"$'\n'* ]] && return 0
    is_devrix_server_pid "$p" || return 0
    seen+="$p"$'\n'
    echo "$p"
  }

  while read -r pid; do append_unique_pid "$pid"; done < <(pgrep -f "${BIN}\$" 2>/dev/null || true)
  local stale
  for stale in "${STALE_BINS[@]}"; do
    while read -r pid; do append_unique_pid "$pid"; done < <(pgrep -f "^${stale}\$" 2>/dev/null || true)
  done
  while read -r pid; do append_unique_pid "$pid"; done < <(pgrep -f 'go run.*cmd/devrix/main|go run.*\./cmd/devrix' 2>/dev/null || true)
  while read -r pid; do append_unique_pid "$pid"; done < <(pgrep -f 'go-build.*/exe/main' 2>/dev/null || true)
}

kill_devrix_pid() {
  local pid="$1"
  [[ -n "$pid" ]] || return 0
  kill -0 "$pid" 2>/dev/null || return 0
  echo "stopping pid=$pid ($(ps -p "$pid" -o command= 2>/dev/null | head -c 120))"
  kill -TERM "$pid" 2>/dev/null || true
  local _
  for _ in $(seq 1 10); do
    if ! kill -0 "$pid" 2>/dev/null; then
      return 0
    fi
    sleep 0.5
  done
  echo "force stopping pid=$pid"
  kill -KILL "$pid" 2>/dev/null || true
  sleep 0.5
}

stop_all_devrix_servers() {
  devrix_process_init

  screen -S devrix -X quit 2>/dev/null || true

  if [[ -f "$PIDFILE" ]]; then
    kill_devrix_pid "$(cat "$PIDFILE" 2>/dev/null || true)"
  fi

  while read -r pid; do
    [[ -n "$pid" ]] || continue
    kill_devrix_pid "$pid"
  done < <(list_devrix_server_pids)

  rm -f "$PIDFILE"

  # Allow IM WebSocket slot to release before reconnecting.
  sleep 5
}

devrix_load_env() {
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

devrix_build() {
  devrix_process_init
  echo "building bin/devrix → $BIN"
  (cd "$ROOT" && go build -o "$BIN" ./cmd/devrix)
}
