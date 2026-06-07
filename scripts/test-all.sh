#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

./scripts/test-unit.sh
./scripts/test-integration.sh
./scripts/test-e2e.sh
./scripts/test-acceptance.sh
