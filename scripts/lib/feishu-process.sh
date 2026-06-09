#!/usr/bin/env bash
# Backward-compatible alias — prefer scripts/lib/devrix-process.sh
# shellcheck disable=SC1091
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/devrix-process.sh"

feishu_process_init() { devrix_process_init; }
feishu_process_cwd() { devrix_process_cwd "$1"; }
is_devrix_feishu_server_pid() { is_devrix_server_pid "$1"; }
list_devrix_feishu_server_pids() { list_devrix_server_pids; }
kill_devrix_feishu_pid() { kill_devrix_pid "$1"; }
stop_all_devrix_feishu() { stop_all_devrix_servers; }
