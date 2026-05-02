#!/usr/bin/env bash
# scripts/dev.sh
# Runs ghost-silicon in development mode on Linux/macOS.
# Usage: ./scripts/dev.sh [--config configs/ghost-silicon.yaml]

set -euo pipefail

CONFIG="${1:-configs/ghost-silicon.yaml}"

export GS_LOG_LEVEL=debug
export GS_LOG_FORMAT=text

echo "Starting ghost-silicon (dev mode)..."
go run ./cmd/ghost-silicon -config "$CONFIG"