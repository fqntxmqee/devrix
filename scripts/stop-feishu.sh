#!/usr/bin/env bash
# Deprecated wrapper — use ./scripts/devrix.sh feishu stop
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
exec "$ROOT/scripts/devrix.sh" feishu stop
